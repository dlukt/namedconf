# namedconf (base layer)

Lossless parser/writer for BIND9 `named.conf` in Go.

## Install

```bash
go get github.com/dlukt/namedconf@latest
```

## Usage

```go
package main

import (
    "fmt"
    "os"
    nc "github.com/dlukt/namedconf"
)

func main() {
    f, err := nc.ParseFile("/etc/named.conf")
    if err != nil { panic(err) }

    // Example: print all top-level keywords
    f.Walk(func(n nc.Node) bool {
        if s, ok := n.(*nc.Stmt); ok {
            fmt.Println(s.Keyword)
        }
        return true
    })

    // Save back unchanged (lossless)
    if err := f.Save("/tmp/named.conf.out"); err != nil { panic(err) }
    _ = os.Rename("/tmp/named.conf.out", "/tmp/named.conf")
}
```

## Design

- Concrete-syntax aware: preserves comments/whitespace.
- Generic `Stmt` with optional nested `Body` for `{ ... }`.
- Unmodified statements write exact original bytes; modified ones are regenerated with minimal formatting.

## Status

Base layer: parser + writer + round-trip tests. Next layers: typed accessors (zones/options), formatter, editor APIs.

---
