package e2e

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const httpServerSource = `
import { server, Request, Response } from "std/http.or"
import { parse, object, number, text, Json } from "std/json.or"

fn main() {
    var app = server()

    app.get("/", fn (req: Request, res: Response) => void {
        res.send("Hello, world!")
    })

    app.get("/info", fn (req: Request, res: Response) => void {
        res.send("agent=" + req.header("User-Agent") + " q=" + req.query())
    })

    app.post("/sum", fn (req: Request, res: Response) => void {
        var body = parse(req.body())
        if !body.valid() {
            res.status(400)
            res.send("bad json")
        } else {
            var nums = body.get("numbers")
            var total: float64 = 0.0
            for var i = 0; i < nums.count(); i++ {
                total = total + nums.at(i).num()
            }
            var out = object()
            out.set("sum", number(total))
            res.json(out.stringify())
        }
    })

    app.listen(%d)
}
`

// TestHTTPServer builds an orlang program that uses the std/http and
// std/json modules, runs it, and exercises it over real HTTP.
func TestHTTPServer(t *testing.T) {
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang not available")
	}

	// Pick a free port
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	dir := t.TempDir()
	srcPath := filepath.Join(dir, "main.or")
	binPath := filepath.Join(dir, "srv")
	if err := os.WriteFile(srcPath, []byte(fmt.Sprintf(httpServerSource, port)), 0644); err != nil {
		t.Fatal(err)
	}

	// Build with the compiler from this repo
	repoRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	build := exec.Command("go", "run", repoRoot, "build", srcPath, "-o", binPath)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("orlang build failed: %v\n%s", err, out)
	}

	// Run the server
	srv := exec.Command(binPath)
	srv.Stdout = os.Stderr
	srv.Stderr = os.Stderr
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		srv.Process.Kill()
		srv.Wait()
	}()

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForServer(t, base)

	t.Run("root", func(t *testing.T) {
		status, body := get(t, base+"/", nil)
		if status != 200 || body != "Hello, world!" {
			t.Fatalf("got %d %q", status, body)
		}
	})

	t.Run("headers and query", func(t *testing.T) {
		status, body := get(t, base+"/info?x=1&y=2", map[string]string{"User-Agent": "orlang-test"})
		if status != 200 || body != "agent=orlang-test q=x=1&y=2" {
			t.Fatalf("got %d %q", status, body)
		}
	})

	t.Run("json sum", func(t *testing.T) {
		resp, err := http.Post(base+"/sum", "application/json",
			strings.NewReader(`{"numbers": [1.5, 2, 3.5]}`))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 || string(body) != `{"sum":7}` {
			t.Fatalf("got %d %q", resp.StatusCode, body)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("content type %q", ct)
		}
	})

	t.Run("bad json", func(t *testing.T) {
		resp, err := http.Post(base+"/sum", "text/plain", strings.NewReader("not json"))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 400 || string(body) != "bad json" {
			t.Fatalf("got %d %q", resp.StatusCode, body)
		}
	})

	t.Run("not found", func(t *testing.T) {
		status, _ := get(t, base+"/nope", nil)
		if status != 404 {
			t.Fatalf("got %d", status)
		}
	})
}

const concurrentServerSource = `
import { server, Request, Response } from "std/http.or"

var gate: chan int32 = channel(0)

fn main() {
    var app = server()

    app.get("/fast", fn (req: Request, res: Response) => void {
        res.send("fast")
    })

    app.get("/wait", fn (req: Request, res: Response) => void {
        var v = recv(gate)
        res.send("released:" + str(v))
    })

    app.get("/release", fn (req: Request, res: Response) => void {
        send(gate, 42)
        res.send("ok")
    })

    app.listen(%d)
}
`

// TestHTTPServerConcurrency proves requests are handled on independent
// green threads: while one handler is parked on a channel, other requests
// are served, and a later request unblocks the parked one via the channel.
func TestHTTPServerConcurrency(t *testing.T) {
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang not available")
	}

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	dir := t.TempDir()
	srcPath := filepath.Join(dir, "main.or")
	binPath := filepath.Join(dir, "srv")
	if err := os.WriteFile(srcPath, []byte(fmt.Sprintf(concurrentServerSource, port)), 0644); err != nil {
		t.Fatal(err)
	}
	repoRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	build := exec.Command("go", "run", repoRoot, "build", srcPath, "-o", binPath)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("orlang build failed: %v\n%s", err, out)
	}

	srv := exec.Command(binPath)
	srv.Stdout = os.Stderr
	srv.Stderr = os.Stderr
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		srv.Process.Kill()
		srv.Wait()
	}()

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForServer(t, base)

	// Park a request on the channel-gated endpoint
	waitResult := make(chan string, 1)
	go func() {
		_, body := get(t, base+"/wait", nil)
		waitResult <- body
	}()

	// Give the /wait request time to reach its handler and park
	time.Sleep(300 * time.Millisecond)

	// The server must still serve other requests while /wait is parked
	for i := 0; i < 3; i++ {
		status, body := get(t, base+"/fast", nil)
		if status != 200 || body != "fast" {
			t.Fatalf("fast request %d while handler parked: got %d %q", i, status, body)
		}
	}

	// Unblock the parked handler through the channel
	if status, body := get(t, base+"/release", nil); status != 200 || body != "ok" {
		t.Fatalf("release: got %d %q", status, body)
	}

	select {
	case body := <-waitResult:
		if body != "released:42" {
			t.Fatalf("wait body %q", body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("parked request never completed")
	}
}

func waitForServer(t *testing.T, base string) {
	t.Helper()
	for i := 0; i < 50; i++ {
		conn, err := net.DialTimeout("tcp", strings.TrimPrefix(base, "http://"), 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("server did not start")
}

func get(t *testing.T, url string, headers map[string]string) (int, string) {
	t.Helper()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}
