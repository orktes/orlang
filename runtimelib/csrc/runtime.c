/*
 * Orlang runtime library.
 *
 * This file is embedded into the orlang compiler binary and automatically
 * compiled and linked into every produced executable, so binaries work
 * without any external runtime dependencies (no bdw-gc, no pkg-config).
 *
 * It provides:
 *   - A conservative mark-and-sweep garbage collector exposing the same
 *     entry points the code generator emits (GC_init / GC_malloc).
 *   - The built-in map runtime (string-keyed hash map with i64 values).
 *
 * Environment knobs (mainly for testing):
 *   ORLANG_GC_STRESS=1  collect before every allocation
 *   ORLANG_GC_OFF=1     never collect (allocations are still tracked)
 */

#include <setjmp.h>
#include <stddef.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#ifndef ORLANG_WEAK
#if defined(_WIN32)
#define ORLANG_WEAK
#else
/* Weak so user code that ships its own implementation still links. */
#define ORLANG_WEAK __attribute__((weak))
#endif
#endif

/* ------------------------------------------------------------------ */
/* Garbage collector                                                  */
/* ------------------------------------------------------------------ */

typedef struct {
  uintptr_t start;
  size_t size;
  unsigned char mark;
} orl_alloc_t;

static orl_alloc_t *orl_allocs = NULL;
static size_t orl_alloc_count = 0;
static size_t orl_alloc_cap = 0;
static size_t orl_heap_bytes = 0;
static size_t orl_gc_threshold = (size_t)1 << 20; /* 1 MiB */
static uintptr_t orl_stack_bottom = 0;
static int orl_gc_stress = 0;
static int orl_gc_off = 0;

/* Mark worklist (indices into orl_allocs). */
static size_t *orl_worklist = NULL;
static size_t orl_worklist_len = 0;
static size_t orl_worklist_cap = 0;

/* Main-executable data/bss segment bounds (globals are GC roots).
 * Provided by the linker/libc on ELF platforms; weak so that platforms
 * without them still link (the segment scan is skipped). */
#if defined(__ELF__)
extern char __data_start[] __attribute__((weak));
extern char _end[] __attribute__((weak));
#endif

extern char **environ;

/* Green-thread hooks (defined in task.c; weak so this file also links
 * standalone, e.g. in the GC unit tests). When a collection runs while a
 * task is active, the stack scan must stop at the task stack's edge and
 * main's dormant frames must be scanned separately. Sleeping task stacks
 * need no special handling: they are GC allocations reachable from the
 * scheduler's globals, so the ordinary object scan covers them. */
extern uintptr_t orl_task_stack_bottom(void) __attribute__((weak));
extern void orl_task_mark_main_stack(void (*)(const void *, const void *),
                                     uintptr_t) __attribute__((weak));

static void orl_fatal(const char *msg) {
  fprintf(stderr, "orlang runtime: %s\n", msg);
  abort();
}

void GC_init(void) {
  if (orl_stack_bottom != 0) {
    return;
  }
  /* The initial process stack holds argv/environ above main's frame, so the
   * environ array address is a safe upper bound that covers main's locals.
   * Fall back to our own frame if environ looks unusable (it is captured
   * before user code runs, so it still points into the initial stack). */
  uintptr_t frame = (uintptr_t)__builtin_frame_address(0);
  uintptr_t env = (uintptr_t)environ;
  if (env > frame && env - frame < ((uintptr_t)64 << 20)) {
    orl_stack_bottom = env;
  } else {
    orl_stack_bottom = frame;
  }

  const char *v = getenv("ORLANG_GC_STRESS");
  if (v != NULL && v[0] == '1') {
    orl_gc_stress = 1;
  }
  v = getenv("ORLANG_GC_OFF");
  if (v != NULL && v[0] == '1') {
    orl_gc_off = 1;
  }
}

static int orl_alloc_cmp(const void *a, const void *b) {
  uintptr_t sa = ((const orl_alloc_t *)a)->start;
  uintptr_t sb = ((const orl_alloc_t *)b)->start;
  if (sa < sb) {
    return -1;
  }
  if (sa > sb) {
    return 1;
  }
  return 0;
}

/* Binary search for the allocation containing p (interior pointers count).
 * Requires orl_allocs sorted by start address. Returns -1 if none. */
static ptrdiff_t orl_find_alloc(uintptr_t p) {
  size_t lo = 0;
  size_t hi = orl_alloc_count;
  while (lo < hi) {
    size_t mid = lo + (hi - lo) / 2;
    const orl_alloc_t *a = &orl_allocs[mid];
    if (p < a->start) {
      hi = mid;
    } else if (p >= a->start + a->size) {
      lo = mid + 1;
    } else {
      return (ptrdiff_t)mid;
    }
  }
  return -1;
}

static void orl_worklist_push(size_t idx) {
  if (orl_worklist_len == orl_worklist_cap) {
    size_t cap = orl_worklist_cap == 0 ? 256 : orl_worklist_cap * 2;
    size_t *w = (size_t *)realloc(orl_worklist, cap * sizeof(size_t));
    if (w == NULL) {
      orl_fatal("out of memory growing GC worklist");
    }
    orl_worklist = w;
    orl_worklist_cap = cap;
  }
  orl_worklist[orl_worklist_len++] = idx;
}

/* Conservatively scan [from, to) for words that point into the heap. */
static void orl_mark_range(const void *from, const void *to) {
  uintptr_t lo = (uintptr_t)from;
  uintptr_t hi = (uintptr_t)to;
  if (lo >= hi) {
    return;
  }
  /* Align to word boundary. */
  lo = (lo + sizeof(uintptr_t) - 1) & ~(uintptr_t)(sizeof(uintptr_t) - 1);
  for (uintptr_t p = lo; p + sizeof(uintptr_t) <= hi; p += sizeof(uintptr_t)) {
    uintptr_t word = *(const uintptr_t *)p;
    ptrdiff_t idx = orl_find_alloc(word);
    if (idx >= 0 && !orl_allocs[idx].mark) {
      orl_allocs[idx].mark = 1;
      orl_worklist_push((size_t)idx);
    }
  }
}

/* Non-static wrapper with the signature the task hook expects. */
static void orl_mark_range_for_tasks(const void *from, const void *to) {
  orl_mark_range(from, to);
}

static void orl_gc_collect(void) {
  if (orl_alloc_count == 0) {
    return;
  }

  /* Spill callee-saved registers onto the stack so the stack scan sees them. */
  jmp_buf regs;
  setjmp(regs);

  qsort(orl_allocs, orl_alloc_count, sizeof(orl_alloc_t), orl_alloc_cmp);
  for (size_t i = 0; i < orl_alloc_count; i++) {
    orl_allocs[i].mark = 0;
  }
  orl_worklist_len = 0;

  /* Roots: machine stack (regs lives on it, so spilled registers are
   * covered), plus the executable's data/bss segments (globals). When
   * running on a green-thread stack, scan up to that stack's edge and
   * cover main's dormant frames via the task hook. */
  uintptr_t stack_bottom = orl_stack_bottom;
  if (orl_task_stack_bottom != NULL) {
    uintptr_t task_bottom = orl_task_stack_bottom();
    if (task_bottom != 0) {
      stack_bottom = task_bottom;
      if (orl_task_mark_main_stack != NULL) {
        orl_task_mark_main_stack(orl_mark_range_for_tasks, orl_stack_bottom);
      }
    }
  }
  orl_mark_range((void *)&regs, (void *)stack_bottom);
#if defined(__ELF__)
  if (&__data_start[0] != NULL && &_end[0] != NULL && __data_start < _end) {
    orl_mark_range(__data_start, _end);
  }
#endif

  /* Trace: scan every reachable object for more heap pointers. */
  while (orl_worklist_len > 0) {
    size_t idx = orl_worklist[--orl_worklist_len];
    const orl_alloc_t *a = &orl_allocs[idx];
    orl_mark_range((const void *)a->start, (const void *)(a->start + a->size));
  }

  /* Sweep: free unmarked allocations, compacting the table in place. */
  size_t kept = 0;
  size_t live_bytes = 0;
  for (size_t i = 0; i < orl_alloc_count; i++) {
    if (orl_allocs[i].mark) {
      live_bytes += orl_allocs[i].size;
      orl_allocs[kept++] = orl_allocs[i];
    } else {
      free((void *)orl_allocs[i].start);
    }
  }
  orl_alloc_count = kept;
  orl_heap_bytes = live_bytes;

  size_t min_threshold = (size_t)1 << 20;
  orl_gc_threshold = live_bytes * 2;
  if (orl_gc_threshold < min_threshold) {
    orl_gc_threshold = min_threshold;
  }
}

void *GC_malloc(size_t size) {
  if (size == 0) {
    size = 1;
  }
  if (orl_stack_bottom != 0 && !orl_gc_off &&
      (orl_gc_stress || orl_heap_bytes + size > orl_gc_threshold)) {
    orl_gc_collect();
  }

  /* Zeroed, matching bdw-gc's GC_malloc contract. */
  void *p = calloc(1, size);
  if (p == NULL) {
    orl_gc_collect();
    p = calloc(1, size);
    if (p == NULL) {
      orl_fatal("out of memory");
    }
  }

  if (orl_alloc_count == orl_alloc_cap) {
    size_t cap = orl_alloc_cap == 0 ? 1024 : orl_alloc_cap * 2;
    orl_alloc_t *t =
        (orl_alloc_t *)realloc(orl_allocs, cap * sizeof(orl_alloc_t));
    if (t == NULL) {
      orl_fatal("out of memory growing GC allocation table");
    }
    orl_allocs = t;
    orl_alloc_cap = cap;
  }
  orl_allocs[orl_alloc_count].start = (uintptr_t)p;
  orl_allocs[orl_alloc_count].size = size;
  orl_allocs[orl_alloc_count].mark = 0;
  orl_alloc_count++;
  orl_heap_bytes += size;
  return p;
}

void *GC_realloc(void *ptr, size_t size) {
  if (ptr == NULL) {
    return GC_malloc(size);
  }
  void *p = GC_malloc(size);
  ptrdiff_t idx = -1;
  /* The table may be unsorted between collections; linear probe is fine
   * because GC_realloc is rare. */
  for (size_t i = 0; i < orl_alloc_count; i++) {
    if (orl_allocs[i].start == (uintptr_t)ptr) {
      idx = (ptrdiff_t)i;
      break;
    }
  }
  if (idx >= 0) {
    size_t old = orl_allocs[idx].size;
    memcpy(p, ptr, old < size ? old : size);
  }
  return p;
}

/* ------------------------------------------------------------------ */
/* Built-in map: string keys, 64-bit values                           */
/* ------------------------------------------------------------------ */

typedef struct orl_map_entry {
  char *key;
  int64_t value;
  struct orl_map_entry *next;
} orl_map_entry_t;

typedef struct {
  orl_map_entry_t **buckets;
  int64_t capacity;
  int64_t size;
} orl_map_t;

/* All map memory comes from GC_malloc so stored pointer values stay
 * visible to the collector and the map itself is collected when dead. */

static uint32_t orl_map_hash(const char *str) {
  uint32_t hash = 5381;
  int c;
  while ((c = (unsigned char)*str++) != 0) {
    hash = ((hash << 5) + hash) + (uint32_t)c;
  }
  return hash;
}

static char *orl_strdup_gc(const char *s) {
  size_t n = strlen(s) + 1;
  char *p = (char *)GC_malloc(n);
  memcpy(p, s, n);
  return p;
}

ORLANG_WEAK void *map_create(void) {
  orl_map_t *m = (orl_map_t *)GC_malloc(sizeof(orl_map_t));
  m->capacity = 16;
  m->size = 0;
  m->buckets =
      (orl_map_entry_t **)GC_malloc((size_t)m->capacity * sizeof(void *));
  return m;
}

static void orl_map_grow(orl_map_t *m) {
  int64_t new_cap = m->capacity * 2;
  orl_map_entry_t **buckets =
      (orl_map_entry_t **)GC_malloc((size_t)new_cap * sizeof(void *));
  for (int64_t i = 0; i < m->capacity; i++) {
    orl_map_entry_t *e = m->buckets[i];
    while (e != NULL) {
      orl_map_entry_t *next = e->next;
      uint32_t idx = orl_map_hash(e->key) % (uint32_t)new_cap;
      e->next = buckets[idx];
      buckets[idx] = e;
      e = next;
    }
  }
  m->buckets = buckets;
  m->capacity = new_cap;
}

ORLANG_WEAK void map_insert(void *map, const char *key, int64_t value) {
  orl_map_t *m = (orl_map_t *)map;
  uint32_t idx = orl_map_hash(key) % (uint32_t)m->capacity;
  for (orl_map_entry_t *e = m->buckets[idx]; e != NULL; e = e->next) {
    if (strcmp(e->key, key) == 0) {
      e->value = value;
      return;
    }
  }
  orl_map_entry_t *e = (orl_map_entry_t *)GC_malloc(sizeof(orl_map_entry_t));
  e->key = orl_strdup_gc(key);
  e->value = value;
  e->next = m->buckets[idx];
  m->buckets[idx] = e;
  m->size++;
  if (m->size > m->capacity * 3 / 4) {
    orl_map_grow(m);
  }
}

ORLANG_WEAK int64_t map_get(void *map, const char *key) {
  orl_map_t *m = (orl_map_t *)map;
  uint32_t idx = orl_map_hash(key) % (uint32_t)m->capacity;
  for (orl_map_entry_t *e = m->buckets[idx]; e != NULL; e = e->next) {
    if (strcmp(e->key, key) == 0) {
      return e->value;
    }
  }
  return 0;
}

ORLANG_WEAK int64_t map_contains(void *map, const char *key) {
  orl_map_t *m = (orl_map_t *)map;
  uint32_t idx = orl_map_hash(key) % (uint32_t)m->capacity;
  for (orl_map_entry_t *e = m->buckets[idx]; e != NULL; e = e->next) {
    if (strcmp(e->key, key) == 0) {
      return 1;
    }
  }
  return 0;
}

ORLANG_WEAK void map_delete(void *map, const char *key) {
  orl_map_t *m = (orl_map_t *)map;
  uint32_t idx = orl_map_hash(key) % (uint32_t)m->capacity;
  orl_map_entry_t **link = &m->buckets[idx];
  while (*link != NULL) {
    if (strcmp((*link)->key, key) == 0) {
      *link = (*link)->next;
      m->size--;
      return;
    }
    link = &(*link)->next;
  }
}

ORLANG_WEAK int64_t map_len(void *map) {
  return ((orl_map_t *)map)->size;
}

/* Returns a GC-allocated array of the map's keys (in bucket order), used
 * by `for key, value in map` iteration. The array and the keys it points
 * to are GC-managed, so the caller never frees them. */
ORLANG_WEAK void **map_keys(void *map) {
  orl_map_t *m = (orl_map_t *)map;
  size_t count = m->size > 0 ? (size_t)m->size : 1;
  void **keys = (void **)GC_malloc(count * sizeof(void *));
  size_t idx = 0;
  for (int64_t i = 0; i < m->capacity; i++) {
    for (orl_map_entry_t *e = m->buckets[i]; e != NULL; e = e->next) {
      keys[idx++] = e->key;
    }
  }
  return keys;
}

/* map_free is a no-op under GC; kept for source compatibility with the
 * old hand-written map runtimes. */
ORLANG_WEAK void map_free(void *map) {
  (void)map;
}
