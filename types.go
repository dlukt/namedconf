package namedconf

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// Node is a concrete-syntax node.
type Node interface {
	isNode()
	// writeTo writes node bytes to buf.
	writeTo(buf *bytes.Buffer)
	// Start and End are byte offsets into the original source for unchanged nodes.
	Start() int
	End() int
}

// File is a parsed named.conf file.
type File struct {
	Nodes []Node
	src   []byte
	path  string
}

// Bytes returns the serialized bytes (lossless if unchanged).
func (f *File) Bytes() []byte {
	var buf bytes.Buffer
	for _, n := range f.Nodes {
		n.writeTo(&buf)
	}
	return buf.Bytes()
}

// Save writes the file to path (or original path if empty).
func (f *File) Save(path string) error {
	if path == "" {
		path = f.path
		if path == "" {
			return fmt.Errorf("no path provided to Save")
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, f.Bytes(), 0o644); err != nil {
		return err
	}
	// Atomic-ish replace on same fs.
	if err := os.Rename(tmp, path); err != nil {
		// Fallback to copy for cross-device moves.
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// Walk walks all nodes depth-first. Return false to stop.
func (f *File) Walk(fn func(Node) bool) {
	for _, n := range f.Nodes {
		if !walkNode(n, fn) {
			return
		}
	}
}

func walkNode(n Node, fn func(Node) bool) bool {
	if !fn(n) {
		return false
	}
	if s, ok := n.(*Stmt); ok {
		for _, cn := range s.Body {
			if !walkNode(cn, fn) {
				return false
			}
		}
	}
	return true
}

// Find returns all statements matching the predicate.
func (f *File) Find(pred func(*Stmt) bool) []*Stmt {
	var out []*Stmt
	f.Walk(func(n Node) bool {
		if s, ok := n.(*Stmt); ok {
			if pred(s) {
				out = append(out, s)
			}
		}
		return true
	})
	return out
}

// ParseFile parses a named.conf file from disk.
func ParseFile(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := Parse(b)
	if err != nil {
		return nil, err
	}
	f.path, _ = filepath.Abs(path)
	return f, nil
}

// Parse parses a named.conf from bytes.
func Parse(src []byte) (*File, error) {
	p := &parser{src: src}
	nodes, err := p.parseRange(0, len(src))
	if err != nil {
		return nil, err
	}
	return &File{Nodes: nodes, src: src}, nil
}

// Raw preserves uninterpreted text (whitespace + comments between statements).
type Raw struct {
	Text       string
	start, end int
}

func (*Raw) isNode()                     {}
func (r *Raw) writeTo(buf *bytes.Buffer) { buf.WriteString(r.Text) }
func (r *Raw) Start() int                { return r.start }
func (r *Raw) End() int                  { return r.end }

// Stmt represents a single named.conf statement ending with ';' (possibly after a block).
// It preserves the exact original text (RawText) for perfect round-tripping when unmodified.
type Stmt struct {
	// Original bytes for lossless re-emit when Modified==false
	RawText    string
	start, end int

	// Structured view (best-effort, tolerant)
	Keyword        string // first identifier-like token (lowercased)
	HeadRaw        string // from stmt start up to the top-level '{' (if any), else everything up to ';'
	HasBlock       bool
	LBraceRaw      string // usually "{" plus adjacent whitespace
	Body           []Node // recursively parsed nodes inside the top-level block
	RBraceRaw      string // usually "}" plus adjacent whitespace
	TrailingAfterR string // text between '}' and final ';' (often empty or spaces)

	// If any field is edited, set Modified=true to regenerate; otherwise RawText is emitted.
	Modified bool
}

func (*Stmt) isNode()      {}
func (s *Stmt) Start() int { return s.start }
func (s *Stmt) End() int   { return s.end }

// Write regenerates if Modified; otherwise emits original RawText.
func (s *Stmt) writeTo(buf *bytes.Buffer) {
	if !s.Modified && s.RawText != "" {
		buf.WriteString(s.RawText)
		return
	}
	// Regenerate with minimal, stable formatting.
	if !s.HasBlock {
		if s.HeadRaw == "" {
			buf.WriteString(s.RawText)
			return
		}
		buf.WriteString(trimRightSpace(s.HeadRaw))
		buf.WriteByte(';')
		return
	}
	// Block stmt
	// Print head, open brace, body (indented), close brace, semicolon.
	head := trimRightSpace(s.HeadRaw)
	if head == "" {
		head = s.Keyword
	}
	buf.WriteString(head)
	buf.WriteString(" {")
	// Indent body by two spaces if not empty.
	if len(s.Body) > 0 {
		buf.WriteByte('\n')
		for _, n := range s.Body {
			// indent each body node
			var inner bytes.Buffer
			n.writeTo(&inner)
			// Ensure each line is indented
			lines := bytes.Split(inner.Bytes(), []byte("\n"))
			for i, ln := range lines {
				if i < len(lines)-1 {
					buf.WriteString("  ")
					buf.Write(ln)
					buf.WriteByte('\n')
				} else if len(ln) > 0 { // last line w/o newline
					buf.WriteString("  ")
					buf.Write(ln)
				}
			}
		}
		// Ensure trailing newline before closing brace
		if last := buf.Bytes(); len(last) == 0 || last[len(last)-1] != '\n' {
			buf.WriteByte('\n')
		}
	}
	buf.WriteString("}")
	buf.WriteString(";")
}
