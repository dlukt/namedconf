# Bind 9 named.conf parser

## Usage Example

Parse all zones in a named.conf file

```go
package main

import (
    "fmt"
    "net"
    "strings"

    // adjust import if your parser lives in this same module
    "your/module/namedconf"
)

type Zone struct {
    Name    string // e.g. "example.com"
    Class   string // often "IN" (optional in configs)
    Type    string // master|slave|primary|stub|redirect|hint|forward
    File    string // path from `file "..." ;`
    Masters []string
}

func main() {
    f, err := namedconf.ParseFile("/etc/named.conf", &namedconf.ParseOptions{
        InlineIncludes: true, // so include "..." files are expanded
    })
    if err != nil {
        panic(err)
    }

    var zones []Zone
    f.Walk(func(d *namedconf.Directive) bool {
        if !strings.EqualFold(d.Name, "zone") || len(d.Args) == 0 {
            return true
        }
        z := Zone{}

        // zone "<name>" [IN] { ... };
        switch v := d.Args[0].(type) {
        case namedconf.StringLit:
            z.Name = v.Value
        case namedconf.Ident:
            z.Name = v.Value
        }

        // Optional class (commonly IN) as the 2nd arg
        if len(d.Args) > 1 {
            if id, ok := d.Args[1].(namedconf.Ident); ok {
                z.Class = id.Value
            }
        }

        // Pull fields from the zone block
        if d.Block != nil {
            for _, cd := range d.Block.Directives {
                switch strings.ToLower(cd.Name) {
                case "type":
                    if len(cd.Args) > 0 {
                        if id, ok := cd.Args[0].(namedconf.Ident); ok {
                            z.Type = id.Value
                        }
                    }
                case "file":
                    if len(cd.Args) > 0 {
                        if s, ok := cd.Args[0].(namedconf.StringLit); ok {
                            z.File = s.Value
                        }
                    }
                case "masters":
                    z.Masters = append(z.Masters, extractListItems(cd)...)
                }
            }
        }

        zones = append(zones, z)
        return true
    })

    // Do whatever you want with them
    for _, z := range zones {
        fmt.Printf("zone %q class=%s type=%s file=%q masters=%v\n",
            z.Name, z.Class, z.Type, z.File, z.Masters)
    }
}

// extracts simple items from a list-style directive like `masters { 10.0.0.1; 10.0.0.2; };`
func extractListItems(d *namedconf.Directive) []string {
    var out []string
    if d.Block == nil {
        // masters can also be written as: masters { address; } — so we focus on block form
        return out
    }
    for _, item := range d.Block.Directives {
        // Most items in `masters` are bare IPs; in our AST they appear as directives
        // whose Name is the IP/CIDR. Handle some common forms.
        name := item.Name
        if ip := net.ParseIP(name); ip != nil {
            out = append(out, name)
            continue
        }
        // CIDR?
        if strings.Contains(name, "/") {
            out = append(out, name)
            continue
        }
        // `key foo;` entries sometimes live in masters; keep them if present
        if strings.EqualFold(name, "key") && len(item.Args) > 0 {
            switch v := item.Args[0].(type) {
            case namedconf.StringLit:
                out = append(out, "key "+v.Value)
            case namedconf.Ident:
                out = append(out, "key "+v.Value)
            }
            continue
        }
        // If your configs use nested groups in masters (unusual), you could handle:
        // if item.Name == "group" { ... } — omitted here for brevity.
    }
    return out
}
```
