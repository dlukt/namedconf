package namedconf

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTripEmbedded(t *testing.T) {
	// Minimal sample demonstrating comments, blocks, and semicolons
	src := []byte(`# comment\noptions {\n  recursion no;\n};\ninclude \"/etc/named.rfc1912.zones\";\n`)
	f, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := f.Bytes()
	if !bytes.Equal(src, out) {
		t.Fatalf("round-trip mismatch\nIN:\n%q\nOUT:\n%q", string(src), string(out))
	}
}

// If the testdata files exist, verify round-trip byte equality.
func TestRoundTripSamplesIfPresent(t *testing.T) {
	// Allow running tests where user drops files into ./testdata
	paths := []string{
		filepath.Join("testdata", "named.conf"),
		filepath.Join("testdata", "named3.conf"),
		// Also allow environment pointing to arbitrary sample files
		os.Getenv("NAMEDCONF_SAMPLE1"),
		os.Getenv("NAMEDCONF_SAMPLE2"),
	}
	for _, p := range paths {
		if p == "" {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		f, err := Parse(b)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		out := f.Bytes()
		if !bytes.Equal(b, out) {
			t.Fatalf("round-trip mismatch for %s", p)
		}
	}
}
