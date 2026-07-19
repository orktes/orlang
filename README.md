[![Coverage Status](https://coveralls.io/repos/github/orktes/orlang/badge.svg?branch=master)](https://coveralls.io/github/orktes/orlang?branch=master)
[![Build Status](https://travis-ci.org/orktes/orlang.svg?branch=master)](https://travis-ci.org/orktes/orlang)
[![GoDoc](https://godoc.org/github.com/orktes/orlang?status.svg)](http://godoc.org/github.com/orktes/orlang)


# orlang
My toy programming language, parser and compiler to play around with different ideas for a perfect (IMO) language.


[Try Orlang](http://orktes.github.io/orlang)

<img width="724" alt="screenshot 2017-09-10 23 49 39" src="https://user-images.githubusercontent.com/606347/30252874-c20b9770-9682-11e7-9608-89e57ffaa9ab.png">


## Building native executables

Orlang compiles to self-contained native binaries via LLVM. The only
external requirement is `clang` on your PATH — the garbage collector and
the built-in map runtime are embedded in the compiler and linked into every
program automatically.

```sh
go install github.com/orktes/orlang    # build the compiler
orlang build main.or                   # produces ./main
orlang run main.or                     # compile and run in one step
orlang build main.or --target llvm     # emit LLVM IR only (main.ll)
orlang build main.or --target js       # compile to JavaScript
```

A minimal program needs no declarations at all:

```orlang
fn main() {
    println("hello", "world", 42, 1.5, true)
}
```

### Language highlights

- Static typing with implicit safe numeric widening (`var x: int64 = 5`,
  `var f: float64 = 1.5`, mixed-width arithmetic)
- `const` declarations with enforced immutability
- Structs with methods, interfaces, enums, tuples, closures and
  first-class functions, operator overloading
- Built-in maps: literals, indexing, `len`, `contains`, `delete`, and
  `for var key, value in m` iteration
- Slices and fixed arrays with `len`/`append`, string concatenation and
  indexing
- `print`/`println`/`str` builtins; C interop via `extern`,
  `include "header.h"`, and `link` directives
- Numeric literals: hex `0xFF`, binary `0b1010`, octal `0o17`, digit
  separators `1_000_000`, float exponents `2.5e-3`
- Conservative mark-and-sweep garbage collection (set `ORLANG_GC_STRESS=1`
  to collect before every allocation when hunting GC bugs)

### Standard library

Programs import standard library modules straight from the compiler
binary — no files to install, and executables stay fully self-contained:

```orlang
import { server, Request, Response } from "std/http.or"
import { parse, object, text, number, Json } from "std/json.or"

fn main() {
    var app = server()

    app.get("/", fn (req: Request, res: Response) => void {
        res.send("Hello, world!")
    })

    app.post("/echo", fn (req: Request, res: Response) => void {
        var body = parse(req.body())
        var out = object()
        out.set("you_sent", body.get("message"))
        res.json(out.stringify())
    })

    app.listen(8080)
}
```

- `std/http.or` — Express-style HTTP server: `get`/`post`/`put`/`delete`/`all`
  routing, request method/path/query/headers/body, response
  status/headers/`send`/`json`. Implemented on POSIX sockets in the
  embedded runtime; no external libraries.
- `std/json.or` — JSON parsing and building: `parse(s).get("key").at(0).str()`
  with safe chaining on missing values, `object()`/`array()`/`text()`/
  `number()`/`boolean()` builders, and `stringify()`.

See `examples/http_server` for a complete JSON todo API.

### Testing

```sh
go test ./...        # compiler unit tests
cd e2e && ./run.sh   # end-to-end tests (build + run every program)
```
