package namedconf_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dlukt/namedconf"
)

func TestNamedConf(t *testing.T) {
	f, err := namedconf.ParseFile("named3.conf", &namedconf.ParseOptions{InlineIncludes: false})
	if err != nil {
		panic(err)
	}
	fmt.Println("Parsed", len(f.Directives), "directives")
	// Walk example: find all zones
	f.Walk(func(d *namedconf.Directive) bool {
		if strings.EqualFold(d.Name, "zone") && len(d.Args) > 0 {
			if s, ok := d.Args[0].(namedconf.StringLit); ok {
				fmt.Println("zone:", s.Value)
			}
		}
		return true
	})
	// Re-render
	fmt.Println(f.String())
}

func TestTopologyNestedGroups(t *testing.T) {
	cfg := `
options {
    topology {
        10 / 8;
        !1.2.3 / 24;
        {
            1.2 / 16;
            3 / 8;
        };
    };
}`
	f, err := namedconf.Parse("topology.conf", strings.NewReader(cfg), &namedconf.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	out := f.String()
	// Spot-check key parts (don’t rely on exact whitespace)
	mustContain(t, out, "topology")
	mustContain(t, out, "!1.2.3 / 24;")
	mustContain(t, out, "1.2 / 16;")
	mustContain(t, out, "3 / 8;")
}

func TestControlsUnixPerms(t *testing.T) {
	cfg := `
controls {
    unix "/var/run/ndc" perm 0600 owner 0 group 0;
}`
	f, err := namedconf.Parse("controls_unix.conf", strings.NewReader(cfg), &namedconf.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	out := f.String()
	mustContain(t, out, `unix "/var/run/ndc" perm 0600 owner 0 group 0;`)
}

func TestAllowQueryMixedItems(t *testing.T) {
	cfg := `
options {
    allow-query {
        any;
        key foo;
        192.0.2.0/24;
        { 198.51.100.0/24; };
    };
}`
	f, err := namedconf.Parse("allow_query.conf", strings.NewReader(cfg), &namedconf.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	out := f.String()
	mustContain(t, out, "allow-query")
	mustContain(t, out, "any;")
	mustContain(t, out, "key foo;")
	mustContain(t, out, "192.0.2.0/24;")
	mustContain(t, out, "198.51.100.0/24;")
}

func TestControlsKeys_StringItem_NoInnerSemicolon(t *testing.T) {
	// This is the case that used to crash: closing brace after a string item.
	cfg := `
controls {
    keys { "sample_key" };
}`
	_, err := namedconf.Parse("controls_keys.conf", strings.NewReader(cfg), &namedconf.ParseOptions{})
	if err != nil {
		t.Fatalf("should parse without error: %v", err)
	}
}

func TestInclude_NonInline(t *testing.T) {
	cfg := `include "child.conf";`
	// Non-inline mode should not try to open the file; Include stays in AST.
	_, err := namedconf.Parse("parent.conf", strings.NewReader(cfg), &namedconf.ParseOptions{InlineIncludes: false})
	if err != nil {
		t.Fatalf("non-inline include should parse without reading files: %v", err)
	}
}

func TestInclude_Inline(t *testing.T) {
	dir := t.TempDir()
	child := `acl good { any; };`
	if err := os.WriteFile(filepath.Join(dir, "child.conf"), []byte(child), 0o600); err != nil {
		t.Fatal(err)
	}
	parent := `include "child.conf";`
	parentPath := filepath.Join(dir, "named.conf")
	if err := os.WriteFile(parentPath, []byte(parent), 0o600); err != nil {
		t.Fatal(err)
	}

	f, err := namedconf.ParseFile(parentPath, &namedconf.ParseOptions{InlineIncludes: true})
	if err != nil {
		t.Fatalf("inline include should parse: %v", err)
	}
	out := f.String()
	mustContain(t, out, "acl good")
	mustContain(t, out, "any;")
}

func TestCommentsAreIgnored(t *testing.T) {
	cfg := `
# top-level comment
options { /* block comment */
    // line comment
    allow-query { any; }; # tail comment
}`
	_, err := namedconf.Parse("comments.conf", strings.NewReader(cfg), &namedconf.ParseOptions{})
	if err != nil {
		t.Fatalf("comments should be ignored: %v", err)
	}
}

// ---------- helpers ----------

func mustContain(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Fatalf("expected output to contain %q\n---- output ----\n%s\n----------------", sub, s)
	}
}
