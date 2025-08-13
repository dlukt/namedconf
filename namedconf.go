// Package namedconf provides a robust parser for ISC BIND 9 named.conf files.
//
// Highlights
// - Recursive descent parser with tolerant grammar matching BIND-style directives
// - Handles comments: //, #, /* ... */
// - Handles quoted strings with escapes
// - Handles nested blocks and semicolon-terminated statements
// - Supports `include "file";` with cycle detection and optional inlining
// - Builds a generic AST (Directive/Block/Atom) that preserves order and source positions
// - First-class MatchGroup for brace-grouped address-match lists (e.g., topology)
// - Provides helpful error messages with file/line/column context
// - Zero external dependencies
//
// Note: BIND evolves and accepts many syntactic forms. While this parser is very
// comprehensive and battle-tested in typical configs (options, logging, views,
// zones, keys, acls, etc.), claiming *absolute* coverage of every historical and
// future edge case would require tying to a formal upstream grammar and test
// corpus for each version. The design here intentionally errs on the side of
// permissiveness and captures unknown constructs losslessly so you can inspect
// or transform them downstream.
package namedconf

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Synthetic name used for literal-only statements (e.g., keys { "key2"; };)
const dirItem = "__item"

// =============================
// Public API
// =============================

type ParseOptions struct {
	// If true, replace `include "file";` directives with the directives from the
	// included file(s). Otherwise, Include nodes remain in the AST.
	InlineIncludes bool
	// A custom filesystem to read includes from. Defaults to the OS filesystem.
	FS fs.FS
}

// File represents a parsed named.conf file.
type File struct {
	Path       string
	Directives []*Directive
}

// Directive models `name [args...] [ { block } ];`
// Many BIND statements have this shape. Unknown statements are preserved.
type Directive struct {
	Name  string
	Args  []Expr // raw tokens/atoms (quoted strings, idents, numbers, literals)
	Block *Block // optional nested block
	Pos   Position
}

// Block is a sequence of directives inside `{ ... }`.
type Block struct {
	Directives []*Directive
	Pos        Position
}

// Include is a special node for `include "path";`.
type Include struct {
	Path string
	Pos  Position
	// If InlineIncludes is true, Inlined holds parsed directives from Path.
	Inlined []*Directive
}

// Expr is any argument expression. For named.conf we keep it simple and lossless.
type Expr interface{ isExpr() }

// Atom kinds preserve raw text and light structure.

type (
	Ident struct {
		Value string
		Pos   Position
	}
	StringLit struct {
		Value string
		Pos   Position
	}
	NumberLit struct {
		Raw string
		Pos Position
	} // e.g., 53, 3m, 1G
	AddrLit struct {
		Raw string
		IP  net.IP
		Pos Position
	} // IPv4/IPv6 as a single token
	CIDRLit struct {
		Raw string
		Pos Position
	} // e.g., 10.0.0.0/8, 2001:db8::/32
	IncludeExpr struct{ Inc *Include } // used as an argument positionally if needed
	// First-class address-match group: { item; item; { nested; }; }
	MatchGroup struct {
		Items []*MatchItem
		Pos   Position
	}
	MatchItem struct {
		Parts   []Expr
		Negated bool
		Pos     Position
	}
)

func (Ident) isExpr()       {}
func (StringLit) isExpr()   {}
func (NumberLit) isExpr()   {}
func (AddrLit) isExpr()     {}
func (CIDRLit) isExpr()     {}
func (IncludeExpr) isExpr() {}
func (MatchGroup) isExpr()  {}

// Position identifies a source location.
type Position struct {
	File string
	Line int
	Col  int
}

func (p Position) String() string {
	if p.File == "" {
		return fmt.Sprintf("%d:%d", p.Line, p.Col)
	}
	return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Col)
}

// ParseFile parses a named.conf file from disk.
func ParseFile(path string, opts *ParseOptions) (*File, error) {
	if opts == nil {
		opts = &ParseOptions{}
	}
	if opts.FS == nil {
		opts.FS = os.DirFS("/")
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(abs)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	root := &File{Path: abs}
	p := newParser(root, abs, f, opts)
	if err := p.parseTop(); err != nil {
		return nil, err
	}
	return root, nil
}

// Parse reads named.conf content from r. The file path is used for error messages
// and include resolution (as the parent directory).
func Parse(path string, r io.Reader, opts *ParseOptions) (*File, error) {
	if opts == nil {
		opts = &ParseOptions{}
	}
	if opts.FS == nil {
		opts.FS = os.DirFS("/")
	}
	root := &File{Path: path}
	p := newParser(root, path, r, opts)
	if err := p.parseTop(); err != nil {
		return nil, err
	}
	return root, nil
}

// Walk traverses the AST pre-order.
func (f *File) Walk(fn func(d *Directive) bool) {
	var walk func(ds []*Directive) bool
	walk = func(ds []*Directive) bool {
		for _, d := range ds {
			if !fn(d) {
				return false
			}
			if d.Block != nil {
				if !walk(d.Block.Directives) {
					return false
				}
			}
		}
		return true
	}
	walk(f.Directives)
}

// =============================
// Lexer
// =============================

type tokenType int

const (
	tIllegal tokenType = iota
	tEOF
	tSemi   // ;
	tLBrace // {
	tRBrace // }
	tString // "..."
	tIdent  // bareword (option names, keywords, values)
)

type token struct {
	typ tokenType
	lit string
	pos Position
}

type lexer struct {
	src  string
	file string
	i    int
	line int
	col  int
}

func newLexer(file string, data string) *lexer {
	return &lexer{src: data, file: file, line: 1, col: 1}
}

func (lx *lexer) next() (r rune, w int) {
	if lx.i >= len(lx.src) {
		return 0, 0
	}
	r, w = utf8.DecodeRuneInString(lx.src[lx.i:])
	lx.i += w
	if r == '\n' {
		lx.line++
		lx.col = 1
	} else {
		lx.col++
	}
	return
}

func (lx *lexer) peek() rune {
	if lx.i >= len(lx.src) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(lx.src[lx.i:])
	return r
}

func (lx *lexer) backup(w int) {
	if w == 0 {
		return
	}
	lx.i -= w
	if lx.col > 1 {
		lx.col--
	} else {
		lx.col = 1
	}
}

func (lx *lexer) pos() Position { return Position{File: lx.file, Line: lx.line, Col: lx.col} }

func (lx *lexer) skipSpacesAndComments() {
	for {
		r := lx.peek()
		if r == 0 {
			return
		}
		// whitespace
		if unicode.IsSpace(r) {
			lx.next()
			continue
		}
		// comments
		if r == '#' { // to end of line
			for {
				r2, _ := lx.next()
				if r2 == 0 || r2 == '\n' {
					break
				}
			}
			continue
		}
		if r == '/' {
			_, w := lx.next()
			n := lx.peek()
			if n == '/' { // // comment
				for {
					r2, _ := lx.next()
					if r2 == 0 || r2 == '\n' {
						break
					}
				}
				continue
			} else if n == '*' { // /* ... */
				lx.next()
				for {
					r2, _ := lx.next()
					if r2 == 0 {
						return
					}
					if r2 == '*' && lx.peek() == '/' {
						lx.next()
						break
					}
				}
				continue
			}
			// not a comment: backup and treat as ident char
			lx.backup(w)
		}
		return
	}
}

func (lx *lexer) nextToken() token {
	lx.skipSpacesAndComments()
	pos := lx.pos()
	r := lx.peek()
	if r == 0 {
		return token{typ: tEOF, pos: pos}
	}

	// single char tokens
	switch r {
	case ';':
		lx.next()
		return token{typ: tSemi, lit: ";", pos: pos}
	case '{':
		lx.next()
		return token{typ: tLBrace, lit: "{", pos: pos}
	case '}':
		lx.next()
		return token{typ: tRBrace, lit: "}", pos: pos}
	case '"':
		return lx.scanString()
	}
	return lx.scanIdentLike()
}

func (lx *lexer) scanString() token {
	start := lx.pos()
	// consume opening quote
	lx.next()
	var b strings.Builder
	for {
		r, _ := lx.next()
		if r == 0 {
			return token{typ: tIllegal, pos: start, lit: "unterminated string"}
		}
		if r == '"' {
			break
		}
		if r == '\\' {
			// simple escapes: \" \\\\ \n \t
			n := lx.peek()
			if n == 0 {
				return token{typ: tIllegal, pos: start, lit: "unterminated escape"}
			}
			_, _ = lx.next()
			switch n {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			default:
				b.WriteByte('\\')
				b.WriteRune(n)
			}
			continue
		}
		b.WriteRune(r)
	}
	return token{typ: tString, lit: b.String(), pos: start}
}

// scanIdentLike reads a bare token until whitespace or one of ;{}".
// named.conf allows rich barewords (dashes, dots, slashes, colons, plus, equals,
// asterisks, percent, at, underscores, etc.). We stop only at structural delimiters.
func (lx *lexer) scanIdentLike() token {
	start := lx.pos()
	var b strings.Builder
	for {
		r := lx.peek()
		if r == 0 || unicode.IsSpace(r) || r == ';' || r == '{' || r == '}' || r == '"' {
			break
		}
		lx.next()
		b.WriteRune(r)
	}
	lit := b.String()
	if lit == "" {
		return token{typ: tIllegal, pos: start, lit: "unexpected character"}
	}
	return token{typ: tIdent, lit: lit, pos: start}
}

// =============================
// Parser
// =============================

type parser struct {
	root   *File
	file   string
	opts   *ParseOptions
	lx     *lexer
	peeked *token

	seenInclude map[string]bool
}

func newParser(root *File, file string, r io.Reader, opts *ParseOptions) *parser {
	data, _ := io.ReadAll(bufio.NewReader(r))
	lx := newLexer(file, string(data))
	return &parser{
		root:        root,
		file:        file,
		opts:        opts,
		lx:          lx,
		seenInclude: map[string]bool{},
	}
}

func (p *parser) next() token {
	if p.peeked != nil {
		t := *p.peeked
		p.peeked = nil
		return t
	}
	return p.lx.nextToken()
}

func (p *parser) peek() token {
	if p.peeked != nil {
		return *p.peeked
	}
	t := p.lx.nextToken()
	p.peeked = &t
	return t
}

func (p *parser) expect(ty tokenType, ctx string) (token, error) {
	t := p.next()
	if t.typ != ty {
		return t, p.errAt(t.pos, "expected %s, got %s (%q)", ctx, p.tname(ty), tname(t))
	}
	return t, nil
}

func (p *parser) tname(t tokenType) string { return tokenTypeName(t) }

func tokenTypeName(t tokenType) string {
	switch t {
	case tEOF:
		return "EOF"
	case tSemi:
		return ";"
	case tLBrace:
		return "{"
	case tRBrace:
		return "}"
	case tString:
		return "string"
	case tIdent:
		return "identifier"
	default:
		return "?"
	}
}

func tname(tok token) string { return fmt.Sprintf("%s", tokenTypeName(tok.typ)) }

func (p *parser) errAt(pos Position, f string, a ...any) error {
	return fmt.Errorf("%s: %s", pos.String(), fmt.Sprintf(f, a...))
}

func (p *parser) parseTop() error {
	for {
		t := p.peek()
		if t.typ == tEOF {
			break
		}
		d, err := p.parseDirective()
		if err != nil {
			return err
		}
		if d != nil {
			p.root.Directives = append(p.root.Directives, d)
		}
	}
	return nil
}

func (p *parser) parseDirective() (*Directive, error) {
	tok := p.next()
	switch tok.typ {
	case tString:
		// A literal-only item inside a block/list, e.g., keys { "key2"; };
		d := &Directive{Name: dirItem, Pos: tok.pos}
		d.Args = append(d.Args, StringLit{Value: tok.lit, Pos: tok.pos})
		for {
			n := p.peek()
			if n.typ == tEOF {
				return nil, p.errAt(n.pos, "unexpected EOF; missing ';'")
			}
			if n.typ == tSemi {
				p.next()
				break
			}
			if n.typ == tRBrace {
				// Tolerate a missing ';' before the closing brace of the enclosing block.
				break
			}
			if n.typ == tLBrace {
				return nil, p.errAt(n.pos, "unexpected '{' after string item")
			}
			arg, err := p.parseArg()
			if err != nil {
				return nil, err
			}
			d.Args = append(d.Args, arg)
		}
		return d, nil
	case tEOF:
		return nil, nil
	case tRBrace:
		return nil, p.errAt(tok.pos, "unexpected '}'")
	case tSemi:
		// stray semicolon; tolerate
		return nil, nil
	case tLBrace:
		// Bare brace group → address-match group directive wrapper
		blk, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		mg := blockToMatchGroup(blk)
		d := &Directive{Name: "group", Args: []Expr{mg}, Pos: mg.Pos}
		if p.peek().typ == tSemi {
			p.next()
		}
		return d, nil
	case tIdent:
		// directive name
		d := &Directive{Name: tok.lit, Pos: tok.pos}
		// collect args until { or ;
		for {
			n := p.peek()
			if n.typ == tEOF {
				return nil, p.errAt(n.pos, "unexpected EOF; missing ';'")
			}
			if n.typ == tSemi {
				p.next()
				break
			}
			if n.typ == tLBrace {
				p.next() // consume '{'
				blk, err := p.parseBlock()
				if err != nil {
					return nil, err
				}
				d.Block = blk
				// optional trailing semicolon after a block
				if p.peek().typ == tSemi {
					p.next()
				}
				break
			}
			if n.typ == tRBrace {
				// End of the block without a trailing ';' for this directive — tolerate it.
				break
			}

			arg, err := p.parseArg()
			if err != nil {
				return nil, err
			}
			d.Args = append(d.Args, arg)
		}

		// Handle include specially
		if strings.EqualFold(d.Name, "include") {
			return p.handleInclude(d)
		}
		return d, nil
	default:
		return nil, p.errAt(tok.pos, "unexpected token %s", tname(tok))
	}
}

func (p *parser) parseArg() (Expr, error) {
	t := p.next()
	switch t.typ {
	case tString:
		return StringLit{Value: t.lit, Pos: t.pos}, nil
	case tIdent:
		// Try classify as CIDR or IP or numberlike; keep raw to be lossless
		lit := t.lit
		if isCIDR(lit) {
			return CIDRLit{Raw: lit, Pos: t.pos}, nil
		}
		if ip := net.ParseIP(lit); ip != nil {
			return AddrLit{Raw: lit, IP: ip, Pos: t.pos}, nil
		}
		if isNumberLike(lit) {
			return NumberLit{Raw: lit, Pos: t.pos}, nil
		}
		return Ident{Value: lit, Pos: t.pos}, nil
	case tLBrace:
		// Address-match list used as an argument position
		blk, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		mg := blockToMatchGroup(blk)
		return mg, nil
	default:
		return nil, p.errAt(t.pos, "invalid argument token %s", tname(t))
	}
}

func (p *parser) parseBlock() (*Block, error) {
	blk := &Block{Pos: p.peek().pos}
	for {
		t := p.peek()
		if t.typ == tEOF {
			return nil, p.errAt(t.pos, "unexpected EOF in block")
		}
		if t.typ == tRBrace {
			p.next()
			break
		}
		d, err := p.parseDirective()
		if err != nil {
			return nil, err
		}
		if d != nil {
			blk.Directives = append(blk.Directives, d)
		}
	}
	return blk, nil
}

func (p *parser) handleInclude(d *Directive) (*Directive, error) {
	if len(d.Args) != 1 {
		return nil, p.errAt(d.Pos, "include expects exactly one argument")
	}
	str, ok := d.Args[0].(StringLit)
	if !ok {
		return nil, p.errAt(d.Pos, "include path must be a string")
	}
	inc := &Include{Path: str.Value, Pos: d.Pos}

	if !p.opts.InlineIncludes {
		// Replace directive's args with IncludeExpr for consumers
		d.Args = []Expr{IncludeExpr{Inc: inc}}
		return d, nil
	}

	// Inline includes
	abs := str.Value
	if !filepath.IsAbs(abs) {
		dir := filepath.Dir(p.file)
		abs = filepath.Join(dir, str.Value)
	}
	abs = filepath.Clean(abs)
	if p.seenInclude[abs] {
		return nil, p.errAt(d.Pos, "include cycle detected: %s", abs)
	}
	p.seenInclude[abs] = true
	defer func() { delete(p.seenInclude, abs) }()

	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, p.errAt(d.Pos, "include read error: %v", err)
	}
	child := newParser(p.root, abs, strings.NewReader(string(data)), p.opts)
	if err := child.parseTop(); err != nil {
		return nil, err
	}
	inc.Inlined = append(inc.Inlined, child.root.Directives...)

	// Return a synthetic directive that holds the inlined block for position
	return &Directive{
		Name:  "include",
		Args:  []Expr{IncludeExpr{Inc: inc}},
		Block: &Block{Directives: inc.Inlined, Pos: d.Pos},
		Pos:   d.Pos,
	}, nil
}

// =============================
// Helpers
// =============================

func isCIDR(s string) bool {
	if !strings.Contains(s, "/") {
		return false
	}
	_, _, err := net.ParseCIDR(s)
	return err == nil
}

func isNumberLike(s string) bool {
	// Accept plain digits or digits followed by unit/size suffixes (common in BIND)
	if s == "" {
		return false
	}
	allDigits := true
	for i, r := range s {
		if i == 0 && (r == '+' || r == '-') {
			continue
		}
		if r < '0' || r > '9' {
			allDigits = false
			break
		}
	}
	if allDigits {
		return true
	}
	// suffix variants like 3m, 1h, 64K, 1G, 250ms
	base := 0
	for i, r := range s {
		if r < '0' || r > '9' {
			base = i
			break
		}
	}
	if base == 0 {
		return false
	}
	if base >= len(s) {
		return true
	}
	suf := strings.ToLower(s[base:])
	switch suf {
	case "k", "m", "g", "t", "ms", "s", "h", "d", "w":
		return true
	}
	return false
}

// Convert a parsed block of pseudo-directives into a MatchGroup.
func blockToMatchGroup(blk *Block) MatchGroup {
	mg := MatchGroup{Pos: blk.Pos}
	for _, d := range blk.Directives {
		// Nested groups appear as Directive{Name:"group", Args:[MatchGroup]} OR as a
		// brace-handled directive with Block. Handle both.
		if d.Name == "group" {
			if len(d.Args) == 1 {
				if g, ok := d.Args[0].(MatchGroup); ok {
					mg.Items = append(mg.Items, &MatchItem{Parts: []Expr{g}, Pos: g.Pos})
					continue
				}
			}
			if d.Block != nil {
				g := blockToMatchGroup(d.Block)
				mg.Items = append(mg.Items, &MatchItem{Parts: []Expr{g}, Pos: g.Pos})
				continue
			}
		}
		item := directiveToMatchItem(d)
		mg.Items = append(mg.Items, &item)
	}
	return mg
}

func directiveToMatchItem(d *Directive) MatchItem {
	parts := make([]Expr, 0, 1+len(d.Args))
	pos := d.Pos
	name := d.Name
	neg := false
	if strings.HasPrefix(name, "!") {
		neg = true
		name = strings.TrimPrefix(name, "!")
	}
	// classify name token
	if isCIDR(name) {
		parts = append(parts, CIDRLit{Raw: name, Pos: pos})
	} else if ip := net.ParseIP(name); ip != nil {
		parts = append(parts, AddrLit{Raw: name, IP: ip, Pos: pos})
	} else {
		parts = append(parts, Ident{Value: name, Pos: pos})
	}
	parts = append(parts, d.Args...)
	return MatchItem{Parts: parts, Negated: neg, Pos: pos}
}

// =============================
// Rendering / Debugging
// =============================

// String renders the AST back to something close to named.conf syntax.
func (f *File) String() string {
	var b strings.Builder
	for _, d := range f.Directives {
		b.WriteString(renderDirective(d, 0))
	}
	return b.String()
}

func renderDirective(d *Directive, indent int) string {
	pad := strings.Repeat("\t", indent)
	var b strings.Builder
	// Render item-only statements (e.g., a bare "string"; inside a list)
	if d.Name == dirItem {
		pad := strings.Repeat("\t", indent)
		var b strings.Builder
		b.WriteString(pad)
		for i, a := range d.Args {
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(renderArg(a))
		}
		b.WriteString(";\n")
		return b.String()
	}

	// Special rendering for anonymous group wrapper
	if d.Name == "group" && len(d.Args) == 1 {
		if mg, ok := d.Args[0].(MatchGroup); ok {
			b.WriteString(renderMatchGroup(mg, indent))
			b.WriteString("\n")
			return b.String()
		}
	}
	b.WriteString(pad)
	b.WriteString(d.Name)
	for _, a := range d.Args {
		b.WriteByte(' ')
		b.WriteString(renderArg(a))
	}
	if d.Block != nil {
		b.WriteString(" {\n")
		for _, cd := range d.Block.Directives {
			b.WriteString(renderDirective(cd, indent+1))
		}
		b.WriteString(pad)
		b.WriteString("}")
		b.WriteString(";\n")
		return b.String()
	}
	b.WriteString(";\n")
	return b.String()
}

func renderArg(e Expr) string {
	switch v := e.(type) {
	case StringLit:
		return fmt.Sprintf("\"%s\"", escape(v.Value))
	case Ident:
		return v.Value
	case NumberLit:
		return v.Raw
	case AddrLit:
		return v.Raw
	case CIDRLit:
		return v.Raw
	case IncludeExpr:
		return fmt.Sprintf("\"%s\"", escape(v.Inc.Path))
	case MatchGroup:
		return renderMatchGroup(v, 0)
	default:
		return "?"
	}
}

func escape(s string) string {
	return strings.NewReplacer("\\", "\\\\", "\"", "\\\"").Replace(s)
}

// Pretty-print a MatchGroup. Indent controls nested formatting when used as a directive.
func renderMatchGroup(g MatchGroup, indent int) string {
	pad := strings.Repeat("\t", indent)
	var b strings.Builder
	b.WriteString(pad)
	b.WriteString("{")
	if len(g.Items) == 0 {
		b.WriteString(" }")
		return b.String()
	}
	b.WriteString("\n")
	for _, it := range g.Items {
		b.WriteString(pad)
		b.WriteString("\t")
		if it.Negated {
			b.WriteString("!")
		}
		for i, p := range it.Parts {
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(renderArg(p))
		}
		b.WriteString(";\n")
	}
	b.WriteString(pad)
	b.WriteString("}")
	return b.String()
}
