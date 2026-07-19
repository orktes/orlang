package runtimelib

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestGreenThreadsAndChannels compiles the full runtime with a C harness
// exercising the scheduler and channels: spawn ordering, rendezvous and
// buffered channels, close semantics, and garbage collection while tasks
// with live stacks are parked.
func TestGreenThreadsAndChannels(t *testing.T) {
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang not available")
	}

	harness := `
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

void GC_init(void);
void *GC_malloc(size_t);
void task_spawn(void *fn, void *env);
void task_yield(void);
void *chan_new(long long capacity);
void chan_send(void *ch, long long value);
long long chan_recv(void *ch);
void chan_close(void *ch);
int chan_closed(void *ch);

static void *results;

static void producer(void *env) {
  void *ch = env;
  for (int i = 1; i <= 5; i++) {
    /* Allocate garbage plus a live payload; the payload pointer travels
     * through the channel while GC runs on other stacks. */
    for (int j = 0; j < 100; j++) {
      GC_malloc(1024);
    }
    char *payload = (char *)GC_malloc(64);
    snprintf(payload, 64, "value-%d", i);
    chan_send(ch, (long long)payload);
    task_yield();
  }
  chan_close(ch);
}

static void consumer(void *env) {
  void *ch = env;
  char *out = (char *)results;
  while (!chan_closed(ch)) {
    long long v = chan_recv(ch);
    if (v == 0) {
      break;
    }
    strcat(out, (char *)v);
    strcat(out, " ");
  }
}

int main(void) {
  GC_init();
  results = GC_malloc(256);

  /* Unbuffered rendezvous between two tasks */
  void *ch = chan_new(0);
  task_spawn(producer, ch);
  task_spawn(consumer, ch);

  /* Main churns allocations to force collections while tasks hold
   * live data on their stacks. */
  for (int i = 0; i < 50; i++) {
    GC_malloc(4096);
    task_yield();
  }

  printf("rendezvous: %s\n", (char *)results);

  /* Buffered channel: fill, drain, close */
  void *b = chan_new(3);
  chan_send(b, 10);
  chan_send(b, 20);
  chan_send(b, 30);
  chan_close(b);
  long long sum = 0;
  while (!chan_closed(b)) {
    sum += chan_recv(b);
  }
  printf("buffered sum: %lld\n", sum);
  return 0;
}
`

	dir := t.TempDir()
	var inputs []string
	for _, name := range SourceNames {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, Sources[name], 0644); err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, p)
	}
	harnessC := filepath.Join(dir, "harness.c")
	if err := os.WriteFile(harnessC, []byte(harness), 0644); err != nil {
		t.Fatal(err)
	}
	inputs = append(inputs, harnessC)

	bin := filepath.Join(dir, "harness")
	args := append([]string{"-O2", "-w", "-o", bin}, inputs...)
	out, err := exec.Command("clang", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("compiling harness: %v\n%s", err, out)
	}

	out, err = exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("harness failed: %v\n%s", err, out)
	}
	want := "rendezvous: value-1 value-2 value-3 value-4 value-5 \nbuffered sum: 60\n"
	if string(out) != want {
		t.Fatalf("unexpected output:\n%q\nwant:\n%q", out, want)
	}
}
