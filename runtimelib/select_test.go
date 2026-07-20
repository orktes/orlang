package runtimelib

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestChannelSelect exercises the runtime's multi-channel select: ready
// cases, blocking selects committed by counterpart sends/recvs, default,
// close wake-ups, and parked select send cases consumed by plain recv.
func TestChannelSelect(t *testing.T) {
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang not available")
	}

	harness := `
#include <stdio.h>
#include <stdlib.h>

void GC_init(void);
void task_spawn(void *fn, void *env);
void task_yield(void);
void *chan_new(long long capacity);
void chan_send(void *ch, long long value);
long long chan_recv(void *ch);
void chan_close(void *ch);
int chan_select(long long n, void **chans, int *dirs, long long *vals, int has_default);

static void *a, *b, *out;

static void feeder(void *env) {
  (void)env;
  chan_send(b, 200); /* commits the blocked select's case 1 */
}

static void drainer(void *env) {
  (void)env;
  /* consume a parked select SEND case with a plain recv */
  long long v = chan_recv(out);
  printf("drained: %lld\n", v);
}

int main(void) {
  GC_init();
  a = chan_new(1);
  b = chan_new(0);
  out = chan_new(0);

  /* 1. default fires when nothing is ready */
  {
    void *chans[2] = {a, b};
    int dirs[2] = {0, 0};
    long long vals[2] = {0, 0};
    int idx = chan_select(2, chans, dirs, vals, 1);
    printf("default: %d\n", idx);
  }

  /* 2. buffered ready case wins immediately */
  chan_send(a, 100);
  {
    void *chans[2] = {a, b};
    int dirs[2] = {0, 0};
    long long vals[2] = {0, 0};
    int idx = chan_select(2, chans, dirs, vals, 0);
    printf("ready: idx=%d val=%lld\n", idx, vals[idx]);
  }

  /* 3. blocking select committed by a counterpart send on b */
  task_spawn(feeder, NULL);
  {
    void *chans[2] = {a, b};
    int dirs[2] = {0, 0};
    long long vals[2] = {0, 0};
    int idx = chan_select(2, chans, dirs, vals, 0);
    printf("blocked: idx=%d val=%lld\n", idx, vals[idx]);
  }

  /* 4. select with a send case parked, consumed by a plain recv */
  task_spawn(drainer, NULL);
  {
    void *chans[1] = {out};
    int dirs[1] = {1};
    long long vals[1] = {777};
    int idx = chan_select(1, chans, dirs, vals, 0);
    printf("sent: idx=%d\n", idx);
  }
  task_yield();

  /* 5. closed channel makes recv cases ready with zero */
  chan_close(a);
  {
    void *chans[1] = {a};
    int dirs[1] = {0};
    long long vals[1] = {-1};
    int idx = chan_select(1, chans, dirs, vals, 0);
    printf("closed: idx=%d val=%lld\n", idx, vals[idx]);
  }
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
	want := "default: -1\nready: idx=0 val=100\nblocked: idx=1 val=200\ndrained: 777\nsent: idx=0\nclosed: idx=0 val=0\n"
	if string(out) != want {
		t.Fatalf("unexpected output:\n%q\nwant:\n%q", out, want)
	}
}
