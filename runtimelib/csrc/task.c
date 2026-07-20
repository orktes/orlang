/*
 * Orlang runtime: green threads (tasks) and CSP channels.
 *
 * A cooperative ucontext-based scheduler backing the language's `go`
 * statement and channel builtins. Tasks yield explicitly, on channel
 * operations, and inside the runtime's IO wrappers (which poll() when
 * every task is blocked, so a server handling one slow connection keeps
 * accepting others).
 *
 * GC integration: task stacks are GC_malloc'd and reachable from the
 * global task list, so the collector's ordinary object scan traverses
 * every sleeping task's stack. Two hooks (declared weak in runtime.c)
 * tell the collector how to scan the *active* stacks when a collection
 * happens while a task is running.
 *
 * Deliberately not a third-party coroutine library: external schedulers
 * allocate stacks the conservative collector cannot see, which would let
 * it free objects still referenced from a sleeping coroutine.
 */

#define _XOPEN_SOURCE 700

#include <errno.h>
#include <fcntl.h>
#include <poll.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <ucontext.h>
#include <unistd.h>

#ifndef ORLANG_WEAK
#define ORLANG_WEAK __attribute__((weak))
#endif

void *GC_malloc(size_t size);

#define ORL_TASK_STACK_SIZE (256 * 1024)

typedef enum {
  ORL_TASK_READY,
  ORL_TASK_RUNNING,
  ORL_TASK_IO_BLOCKED,
  ORL_TASK_CHAN_BLOCKED,
  ORL_TASK_DONE,
} orl_task_state_t;

typedef struct orl_task {
  ucontext_t ctx;
  char *stack;
  size_t stack_size;
  orl_task_state_t state;

  /* entry point: closure function + environment */
  void (*fn)(void *env);
  void *env;

  /* IO blocking info */
  int wait_fd;
  short wait_events;

  /* channel blocking: value in flight (send) or received (recv) */
  int64_t chan_value;
  int chan_delivered;

  /* select participation: set while parked in chan_select */
  int in_select;
  int select_committed;
  int32_t select_fired; /* case index committed by a counterpart */

  struct orl_task *all_next;   /* all-tasks list (GC root chain) */
  struct orl_task *queue_next; /* ready/wait queue link */
} orl_task_t;

/* Globals live in .data/.bss, which the GC scans as roots — everything
 * reachable from here (tasks, stacks, channel buffers) stays alive. */
static orl_task_t *orl_all_tasks = NULL;
static orl_task_t *orl_main_task = NULL;
static orl_task_t *orl_current = NULL;
static orl_task_t *orl_ready_head = NULL;
static orl_task_t *orl_ready_tail = NULL;
static uintptr_t orl_main_saved_sp = 0;

static void orl_task_fatal(const char *msg) {
  fprintf(stderr, "orlang runtime: %s\n", msg);
  exit(1);
}

static void orl_ready_push(orl_task_t *t) {
  t->state = ORL_TASK_READY;
  t->queue_next = NULL;
  if (orl_ready_tail != NULL) {
    orl_ready_tail->queue_next = t;
  } else {
    orl_ready_head = t;
  }
  orl_ready_tail = t;
}

static orl_task_t *orl_ready_pop(void) {
  orl_task_t *t = orl_ready_head;
  if (t != NULL) {
    orl_ready_head = t->queue_next;
    if (orl_ready_head == NULL) {
      orl_ready_tail = NULL;
    }
    t->queue_next = NULL;
  }
  return t;
}

static void orl_task_init_main(void) {
  if (orl_main_task != NULL) {
    return;
  }
  orl_task_t *t = (orl_task_t *)GC_malloc(sizeof(orl_task_t));
  t->state = ORL_TASK_RUNNING;
  t->all_next = NULL;
  orl_main_task = t;
  orl_current = t;
  orl_all_tasks = t;
}

/* Wakes every IO-blocked task whose fd is ready. When nothing is ready
 * and no task is runnable, blocks in poll() until one is. Returns the
 * number of tasks woken. */
static int orl_poll_io(int block) {
  struct pollfd fds[64];
  orl_task_t *waiters[64];
  int n = 0;
  for (orl_task_t *t = orl_all_tasks; t != NULL && n < 64; t = t->all_next) {
    if (t->state == ORL_TASK_IO_BLOCKED) {
      fds[n].fd = t->wait_fd;
      fds[n].events = t->wait_events;
      fds[n].revents = 0;
      waiters[n] = t;
      n++;
    }
  }
  if (n == 0) {
    return 0;
  }
  int ready = poll(fds, (nfds_t)n, block ? -1 : 0);
  if (ready <= 0) {
    return 0;
  }
  int woken = 0;
  for (int i = 0; i < n; i++) {
    if (fds[i].revents != 0) {
      orl_ready_push(waiters[i]);
      woken++;
    }
  }
  return woken;
}

/* Switches to the next runnable task. The current task must already be
 * queued/parked/done. */
static void orl_schedule(void) {
  orl_task_t *prev = orl_current;

  orl_task_t *next = orl_ready_pop();
  while (next == NULL) {
    if (orl_poll_io(1) == 0) {
      orl_task_fatal("deadlock: all tasks are blocked");
    }
    next = orl_ready_pop();
  }

  next->state = ORL_TASK_RUNNING;
  orl_current = next;
  if (prev == orl_main_task) {
    /* Record roughly where main's live stack ends so the GC can scan
     * main's dormant frames while other tasks run. */
    char marker;
    orl_main_saved_sp = (uintptr_t)&marker;
  }
  if (prev != next) {
    swapcontext(&prev->ctx, &next->ctx);
  }
}

/* makecontext only portably passes int arguments, so the task pointer is
 * split into two 32-bit halves. */
static void orl_task_trampoline(uint32_t hi, uint32_t lo) {
  orl_task_t *self = (orl_task_t *)(((uintptr_t)hi << 32) | (uintptr_t)lo);
  self->fn(self->env);
  self->state = ORL_TASK_DONE;
  /* Unlink from the all-tasks list so the GC can reclaim the stack. */
  orl_task_t **link = &orl_all_tasks;
  while (*link != NULL) {
    if (*link == self) {
      *link = self->all_next;
      break;
    }
    link = &(*link)->all_next;
  }
  orl_schedule();
  orl_task_fatal("resumed a finished task");
}

/* ------------------------------------------------------------------ */
/* Public API (used by generated code and the runtime itself)         */
/* ------------------------------------------------------------------ */

ORLANG_WEAK void task_spawn(void *fn, void *env) {
  orl_task_init_main();

  orl_task_t *t = (orl_task_t *)GC_malloc(sizeof(orl_task_t));
  t->stack_size = ORL_TASK_STACK_SIZE;
  t->stack = (char *)GC_malloc(t->stack_size);
  t->fn = (void (*)(void *))fn;
  t->env = env;

  if (getcontext(&t->ctx) != 0) {
    orl_task_fatal("getcontext failed");
  }
  t->ctx.uc_stack.ss_sp = t->stack;
  t->ctx.uc_stack.ss_size = t->stack_size;
  t->ctx.uc_link = NULL;

  t->all_next = orl_all_tasks;
  orl_all_tasks = t;

  makecontext(&t->ctx, (void (*)(void))orl_task_trampoline, 2,
              (uint32_t)((uintptr_t)t >> 32), (uint32_t)(uintptr_t)t);

  /* Spawned tasks run when the spawner next yields (cooperative). */
  orl_ready_push(t);
}

ORLANG_WEAK void task_yield(void) {
  orl_task_init_main();
  /* Give IO-blocked tasks a chance to wake even under a busy loop. */
  orl_poll_io(0);
  orl_ready_push(orl_current);
  orl_schedule();
}

/* ------------------------------------------------------------------ */
/* IO integration                                                     */
/* ------------------------------------------------------------------ */

static void orl_io_block(int fd, short events) {
  orl_current->state = ORL_TASK_IO_BLOCKED;
  orl_current->wait_fd = fd;
  orl_current->wait_events = events;
  orl_schedule();
}

static void orl_set_nonblocking(int fd) {
  int flags = fcntl(fd, F_GETFL, 0);
  if (flags >= 0) {
    fcntl(fd, F_SETFL, flags | O_NONBLOCK);
  }
}

/* orl_io_* wrap blocking syscalls: when the call would block, the task
 * parks and the scheduler runs someone else (or polls). Used by the
 * HTTP/net runtime so IO-bound tasks interleave. */

int orl_io_accept(int fd) {
  orl_task_init_main();
  orl_set_nonblocking(fd);
  for (;;) {
    int c = accept(fd, NULL, NULL);
    if (c >= 0) {
      return c;
    }
    if (errno != EAGAIN && errno != EWOULDBLOCK) {
      return -1;
    }
    orl_io_block(fd, POLLIN);
  }
}

ssize_t orl_io_read(int fd, void *buf, size_t n) {
  orl_task_init_main();
  orl_set_nonblocking(fd);
  for (;;) {
    ssize_t r = read(fd, buf, n);
    if (r >= 0) {
      return r;
    }
    if (errno != EAGAIN && errno != EWOULDBLOCK) {
      return -1;
    }
    orl_io_block(fd, POLLIN);
  }
}

ssize_t orl_io_write(int fd, const void *buf, size_t n) {
  orl_task_init_main();
  orl_set_nonblocking(fd);
  for (;;) {
    ssize_t r = write(fd, buf, n);
    if (r >= 0) {
      return r;
    }
    if (errno != EAGAIN && errno != EWOULDBLOCK) {
      return -1;
    }
    orl_io_block(fd, POLLOUT);
  }
}

/* ------------------------------------------------------------------ */
/* GC hooks (declared weak in runtime.c)                              */
/* ------------------------------------------------------------------ */

/* Returns the scan upper bound for the currently running stack when it
 * is a task stack, or 0 when running on the main stack. */
uintptr_t orl_task_stack_bottom(void) {
  if (orl_current == NULL || orl_current == orl_main_task) {
    return 0;
  }
  return (uintptr_t)(orl_current->stack + orl_current->stack_size);
}

/* Scans main's dormant stack region while a task is running. Sleeping
 * task stacks need no hook: they are GC objects reachable from
 * orl_all_tasks and get content-scanned by the ordinary mark phase. */
void orl_task_mark_main_stack(void (*mark_range)(const void *, const void *),
                              uintptr_t main_stack_bottom) {
  if (orl_current == NULL || orl_current == orl_main_task) {
    return;
  }
  if (orl_main_saved_sp != 0 && orl_main_saved_sp < main_stack_bottom) {
    mark_range((const void *)orl_main_saved_sp, (const void *)main_stack_bottom);
  }
}

/* ------------------------------------------------------------------ */
/* Channels                                                           */
/* ------------------------------------------------------------------ */

typedef struct orl_chan_waiter {
  orl_task_t *task;
  int32_t select_case; /* -1 for plain send/recv waiters */
  int64_t send_value;  /* value carried by a parked select send case */
  struct orl_chan_waiter *next;
} orl_chan_waiter_t;

typedef struct {
  int64_t *buf; /* ring buffer of capacity cap (0 = unbuffered) */
  int64_t cap;
  int64_t len;
  int64_t head;
  int closed;
  orl_chan_waiter_t *send_waiters;
  orl_chan_waiter_t *recv_waiters;
} orl_chan_t;

static void orl_chan_wait_push_case(orl_chan_waiter_t **list, orl_task_t *t,
                                    int32_t select_case, int64_t send_value) {
  orl_chan_waiter_t *w = (orl_chan_waiter_t *)GC_malloc(sizeof(orl_chan_waiter_t));
  w->task = t;
  w->select_case = select_case;
  w->send_value = send_value;
  w->next = NULL;
  while (*list != NULL) {
    list = &(*list)->next;
  }
  *list = w;
}

static void orl_chan_wait_push(orl_chan_waiter_t **list, orl_task_t *t) {
  orl_chan_wait_push_case(list, t, -1, 0);
}

/* Removes every waiter entry belonging to task t from a list (used when a
 * select wakes up and withdraws its remaining registrations). */
static void orl_chan_wait_remove_task(orl_chan_waiter_t **list, orl_task_t *t) {
  while (*list != NULL) {
    if ((*list)->task == t) {
      *list = (*list)->next;
    } else {
      list = &(*list)->next;
    }
  }
}

/* Pops the first waiter still able to complete an operation: plain
 * waiters always can; select waiters only when their select has not
 * already fired through another channel (stale entries are discarded —
 * the owning task also withdraws them when it wakes). */
static orl_chan_waiter_t *orl_chan_pop_eligible(orl_chan_waiter_t **list) {
  for (;;) {
    orl_chan_waiter_t *w = *list;
    if (w == NULL) {
      return NULL;
    }
    *list = w->next;
    w->next = NULL;
    if (w->select_case >= 0 && w->task->select_committed) {
      continue;
    }
    return w;
  }
}

/* Commits a handoff to a waiter: marks select participation and wakes the
 * task. For recv-side waiters the delivered value is in task->chan_value. */
static void orl_chan_commit_waiter(orl_chan_waiter_t *w) {
  w->task->chan_delivered = 1;
  if (w->select_case >= 0) {
    w->task->select_committed = 1;
    w->task->select_fired = w->select_case;
  }
  if (w->task->state == ORL_TASK_CHAN_BLOCKED) {
    orl_ready_push(w->task);
  }
}

/* Wakes one waiter to re-check a buffered channel (no value handoff). */
static void orl_chan_wake_one(orl_chan_waiter_t **list) {
  orl_chan_waiter_t *w = orl_chan_pop_eligible(list);
  if (w != NULL && w->task->state == ORL_TASK_CHAN_BLOCKED) {
    orl_ready_push(w->task);
  }
}

ORLANG_WEAK void *chan_new(int64_t capacity) {
  orl_task_init_main();
  if (capacity < 0) {
    capacity = 0;
  }
  orl_chan_t *ch = (orl_chan_t *)GC_malloc(sizeof(orl_chan_t));
  ch->cap = capacity;
  if (capacity > 0) {
    ch->buf = (int64_t *)GC_malloc((size_t)capacity * sizeof(int64_t));
  }
  return ch;
}

ORLANG_WEAK void chan_send(void *chp, int64_t value) {
  orl_chan_t *ch = (orl_chan_t *)chp;
  if (ch == NULL) {
    orl_task_fatal("send on null channel");
  }
  for (;;) {
    if (ch->closed) {
      orl_task_fatal("send on closed channel");
    }

    /* Unbuffered: hand the value directly to a waiting receiver, or
     * park until one arrives. */
    if (ch->cap == 0) {
      orl_chan_waiter_t *w = orl_chan_pop_eligible(&ch->recv_waiters);
      if (w != NULL) {
        w->task->chan_value = value;
        orl_chan_commit_waiter(w);
        return;
      }
      /* Park as a sender holding the value. */
      orl_current->chan_value = value;
      orl_current->chan_delivered = 0;
      orl_current->state = ORL_TASK_CHAN_BLOCKED;
      orl_chan_wait_push(&ch->send_waiters, orl_current);
      orl_schedule();
      if (orl_current->chan_delivered) {
        return;
      }
      continue; /* woken for another reason (e.g. close) — recheck */
    }

    /* Buffered: enqueue if there is room. */
    if (ch->len < ch->cap) {
      ch->buf[(ch->head + ch->len) % ch->cap] = value;
      ch->len++;
      orl_chan_wake_one(&ch->recv_waiters);
      return;
    }

    /* Full: park until a receiver frees a slot. */
    orl_current->chan_delivered = 0;
    orl_current->state = ORL_TASK_CHAN_BLOCKED;
    orl_chan_wait_push(&ch->send_waiters, orl_current);
    orl_schedule();
  }
}

ORLANG_WEAK int64_t chan_recv(void *chp) {
  orl_chan_t *ch = (orl_chan_t *)chp;
  if (ch == NULL) {
    orl_task_fatal("receive on null channel");
  }
  for (;;) {
    if (ch->cap == 0) {
      /* Unbuffered: take from a parked sender if one is waiting. */
      orl_chan_waiter_t *w = orl_chan_pop_eligible(&ch->send_waiters);
      if (w != NULL) {
        int64_t value = w->select_case >= 0 ? w->send_value : w->task->chan_value;
        orl_chan_commit_waiter(w);
        return value;
      }
      if (ch->closed) {
        return 0;
      }
      /* Park as a receiver. */
      orl_current->chan_delivered = 0;
      orl_current->state = ORL_TASK_CHAN_BLOCKED;
      orl_chan_wait_push(&ch->recv_waiters, orl_current);
      orl_schedule();
      if (orl_current->chan_delivered) {
        return orl_current->chan_value;
      }
      continue; /* woken by close — loop reports closed */
    }

    if (ch->len > 0) {
      int64_t value = ch->buf[ch->head];
      ch->head = (ch->head + 1) % ch->cap;
      ch->len--;
      orl_chan_wake_one(&ch->send_waiters);
      return value;
    }
    if (ch->closed) {
      return 0;
    }

    orl_current->chan_delivered = 0;
    orl_current->state = ORL_TASK_CHAN_BLOCKED;
    orl_chan_wait_push(&ch->recv_waiters, orl_current);
    orl_schedule();
  }
}

ORLANG_WEAK void chan_close(void *chp) {
  orl_chan_t *ch = (orl_chan_t *)chp;
  if (ch == NULL || ch->closed) {
    return;
  }
  ch->closed = 1;
  /* Wake everyone; they re-check state and observe the close. */
  orl_chan_waiter_t *w;
  while ((w = orl_chan_pop_eligible(&ch->recv_waiters)) != NULL) {
    if (w->task->state == ORL_TASK_CHAN_BLOCKED) {
      orl_ready_push(w->task);
    }
  }
  while ((w = orl_chan_pop_eligible(&ch->send_waiters)) != NULL) {
    if (w->task->state == ORL_TASK_CHAN_BLOCKED) {
      orl_ready_push(w->task);
    }
  }
}

ORLANG_WEAK int32_t chan_closed(void *chp) {
  orl_chan_t *ch = (orl_chan_t *)chp;
  if (ch == NULL) {
    return 1;
  }
  /* A channel still holding buffered values is not "drained". */
  return ch->closed && ch->len == 0 && ch->send_waiters == NULL;
}

/* ------------------------------------------------------------------ */
/* Select                                                             */
/* ------------------------------------------------------------------ */

/* Tries every case once, in order (deterministic under the cooperative
 * scheduler, unlike Go's randomized choice). dirs[i]: 0 = recv, 1 = send.
 * vals[i] carries send values in and receive results out.
 * Returns the fired case index or -2 when nothing is ready. */
static int32_t orl_select_try(int64_t n, void **chans, int32_t *dirs, int64_t *vals) {
  for (int64_t i = 0; i < n; i++) {
    orl_chan_t *ch = (orl_chan_t *)chans[i];
    if (ch == NULL) {
      continue;
    }
    if (dirs[i] == 0) { /* recv */
      if (ch->cap == 0) {
        orl_chan_waiter_t *w = orl_chan_pop_eligible(&ch->send_waiters);
        if (w != NULL) {
          vals[i] = w->select_case >= 0 ? w->send_value : w->task->chan_value;
          orl_chan_commit_waiter(w);
          return (int32_t)i;
        }
      } else if (ch->len > 0) {
        vals[i] = ch->buf[ch->head];
        ch->head = (ch->head + 1) % ch->cap;
        ch->len--;
        orl_chan_wake_one(&ch->send_waiters);
        return (int32_t)i;
      }
      if (ch->closed) {
        vals[i] = 0; /* closed channels are always ready with zero */
        return (int32_t)i;
      }
    } else { /* send */
      if (ch->closed) {
        orl_task_fatal("send on closed channel");
      }
      if (ch->cap == 0) {
        orl_chan_waiter_t *w = orl_chan_pop_eligible(&ch->recv_waiters);
        if (w != NULL) {
          w->task->chan_value = vals[i];
          orl_chan_commit_waiter(w);
          return (int32_t)i;
        }
      } else if (ch->len < ch->cap) {
        ch->buf[(ch->head + ch->len) % ch->cap] = vals[i];
        ch->len++;
        orl_chan_wake_one(&ch->recv_waiters);
        return (int32_t)i;
      }
    }
  }
  return -2;
}

/* chan_select blocks until one case can proceed and returns its index
 * (receive results are written to vals). With has_default it returns -1
 * immediately when no case is ready. */
ORLANG_WEAK int32_t chan_select(int64_t n, void **chans, int32_t *dirs,
                                int64_t *vals, int32_t has_default) {
  orl_task_init_main();
  for (;;) {
    int32_t idx = orl_select_try(n, chans, dirs, vals);
    if (idx >= 0) {
      return idx;
    }
    if (has_default) {
      return -1;
    }

    /* Park registered on every channel; whichever counterpart completes
     * first commits exactly one case (select_committed gate). */
    orl_current->select_committed = 0;
    orl_current->select_fired = -1;
    orl_current->chan_delivered = 0;
    for (int64_t i = 0; i < n; i++) {
      orl_chan_t *ch = (orl_chan_t *)chans[i];
      if (ch == NULL) {
        continue;
      }
      if (dirs[i] == 0) {
        orl_chan_wait_push_case(&ch->recv_waiters, orl_current, (int32_t)i, 0);
      } else {
        orl_chan_wait_push_case(&ch->send_waiters, orl_current, (int32_t)i, vals[i]);
      }
    }
    orl_current->state = ORL_TASK_CHAN_BLOCKED;
    orl_schedule();

    /* Withdraw the registrations that did not fire. */
    for (int64_t i = 0; i < n; i++) {
      orl_chan_t *ch = (orl_chan_t *)chans[i];
      if (ch == NULL) {
        continue;
      }
      orl_chan_wait_remove_task(&ch->recv_waiters, orl_current);
      orl_chan_wait_remove_task(&ch->send_waiters, orl_current);
    }

    if (orl_current->select_committed) {
      int32_t fired = orl_current->select_fired;
      orl_current->select_committed = 0;
      if (dirs[fired] == 0) {
        vals[fired] = orl_current->chan_value;
      }
      return fired;
    }
    /* Woken by a close or a buffered-state change: retry. */
  }
}
