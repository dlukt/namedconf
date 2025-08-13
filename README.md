# Bind 9 named.conf parser

## Key Features

1. **Complete Data Structures**: Supports all major BIND9 configuration elements:
   - Zones (master, slave, stub, forward, etc.)
   - Global options block
   - ACLs (Access Control Lists)
   - TSIG keys
   - Logging configuration
   - Controls
   - Masters definitions
   - Server statements
   - Views
   - Statistics channels
   - Include statements

2. **Robust Parsing**
   - Handles comments (both `//` and `#` style)
   - Properly tokenizes quoted strings
   - Manages nested blocks with braces
   - Supports escape sequences in strings

3. **Error Handling**: Provides line number information for parsing errors

4. **Extensible**: Unknown statements are captured in an `Unknown` slice for custom handling

## Usage Example

```go
package main

import (
    "fmt"
    "log"
    "os"
)

func main() {
    file, err := os.Open("/etc/bind/named.conf")
    if err != nil {
        log.Fatal(err)
    }
    defer file.Close()
    
    parser := namedconf.NewParser(file)
    config, err := parser.Parse()
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Parsed configuration:\n%s", config.String())
    
    // Access specific elements
    for _, zone := range config.Zones {
        fmt.Printf("Zone: %s, Type: %s, File: %s\n", 
            zone.Name, zone.Type, zone.File)
    }
}
```

## Example named.conf it can parse

```conf
options {
    directory "/var/cache/bind";
    dnssec-validation auto;
    listen-on port 53 { 127.0.0.1; 192.168.1.10; };
    forwarders { 8.8.8.8; 8.8.4.4; };
    recursion yes;
};

zone "example.com" {
    type master;
    file "/etc/bind/zones/db.example.com";
    allow-transfer { trusted; };
};

acl "trusted" {
    127.0.0.0/8;
    192.168.1.0/24;
};

key "rndc-key" {
    algorithm hmac-sha256;
    secret "base64secrethere==";
};
```

The library handles the complexity of BIND9's configuration syntax while providing a clean, structured Go API for working with the parsed data. It's designed to be both comprehensive and extensible for your specific needs.
