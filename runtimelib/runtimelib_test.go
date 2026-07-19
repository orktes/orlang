package runtimelib

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestGarbageIsCollected compiles the embedded runtime together with a C
// harness that allocates ~1 GiB of garbage while keeping a small live set,
// and asserts both that the live data survives collection and that peak
// memory stays far below the total allocated (i.e. garbage was freed).
func TestGarbageIsCollected(t *testing.T) {
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang not available")
	}

	harness := `
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/resource.h>

void GC_init(void);
void *GC_malloc(size_t size);

int main(void) {
  GC_init();

  /* Live set: 100 x 1KiB blocks with known contents. */
  char *live[100];
  for (int i = 0; i < 100; i++) {
    live[i] = (char *)GC_malloc(1024);
    memset(live[i], 'A' + (i % 26), 1023);
  }

  /* Garbage: 64Ki x 16KiB = 1 GiB total, all dropped immediately. */
  for (int i = 0; i < 64 * 1024; i++) {
    char *g = (char *)GC_malloc(16 * 1024);
    g[0] = (char)i;
  }

  for (int i = 0; i < 100; i++) {
    if (live[i][0] != 'A' + (i % 26) || live[i][512] != 'A' + (i % 26)) {
      printf("CORRUPTED live block %d\n", i);
      return 1;
    }
  }

  struct rusage ru;
  getrusage(RUSAGE_SELF, &ru);
  /* ru_maxrss is KiB on Linux. 1 GiB allocated; require < 256 MiB peak. */
  if (ru.ru_maxrss > 256 * 1024) {
    printf("PEAK RSS TOO HIGH: %ld KiB\n", ru.ru_maxrss);
    return 1;
  }
  printf("OK\n");
  return 0;
}
`

	dir := t.TempDir()
	runtimeC := filepath.Join(dir, "runtime.c")
	harnessC := filepath.Join(dir, "harness.c")
	bin := filepath.Join(dir, "harness")

	if err := os.WriteFile(runtimeC, Source, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(harnessC, []byte(harness), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command("clang", "-O2", "-o", bin, runtimeC, harnessC).CombinedOutput()
	if err != nil {
		t.Fatalf("compiling harness: %v\n%s", err, out)
	}

	out, err = exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("harness failed: %v\n%s", err, out)
	}
	if string(out) != "OK\n" {
		t.Fatalf("unexpected harness output: %s", out)
	}
}
