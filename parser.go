package namedconf

import (
	"bytes"
	"errors"
	"strings"
	"unicode"
)

type parser struct {
	src []byte
}

// parseRange splits src[start:end] into top-level nodes (Raw and Stmt), recursively parsing block bodies.
func (p *parser) parseRange(start, end int) ([]Node, error) {
	var nodes []Node
	i := start
	last := start // start index of the next Raw segment if any

	depth := 0
	inSlashStar := false
	inLine := false
	inString := false

	for i < end {
		c := p.src[i]

		// End-of-line resets line comments
		if inLine {
			if c == '\n' {
				inLine = false
			}
			i++
			continue
		}

		// Inside block comment
		if inSlashStar {
			if c == '*' && i+1 < end && p.src[i+1] == '/' {
				inSlashStar = false
				i += 2
				continue
			}
			i++
			continue
		}

		// Inside string (double quotes per BIND)
		if inString {
			if c == '\\' { // escape next
				if i+1 < end {
					i += 2
					continue
				}
				i++
				continue
			}
			if c == '"' {
				inString = false
			}
			i++
			continue
		}

		// Enter comments/strings
		if c == '/' && i+1 < end {
			if p.src[i+1] == '*' {
				inSlashStar = true
				i += 2
				continue
			}
			if p.src[i+1] == '/' {
				inLine = true
				i += 2
				continue
			}
		}
		if c == '#' {
			inLine = true
			i++
			continue
		}
		if c == '"' {
			inString = true
			i++
			continue
		}

		// Track braces only outside strings/comments
		if c == '{' {
			depth++
			i++
			continue
		}
		if c == '}' {
			if depth > 0 {
				depth--
			}
			i++
			continue
		}

		// Statement boundary: semicolon at top level
		if c == ';' && depth == 0 {
			// capture statement segment [stmtStart: i+1]
			// preceding Raw (trivia) is [last:stmtStart)
			// find stmtStart by scanning backwards from current to previous non-space after last.
			// However, we assume statement starts at last non-trivia point; so segment is [lastStmtStart, i+1]
			// To keep it simple and robust, cut trivia+stmt into Raw+Stmt where Raw is [last:stmtStart), Stmt is [stmtStart:i+1]
			stmtStart := findStmtStart(p.src, last, i)
			if stmtStart > last {
				nodes = append(nodes, &Raw{Text: string(p.src[last:stmtStart]), start: last, end: stmtStart})
			}
			seg := p.src[stmtStart : i+1]
			st, err := p.buildStmt(seg, stmtStart)
			if err != nil {
				// Be tolerant: if we fail, fall back to Raw segment to preserve bytes
				nodes = append(nodes, &Raw{Text: string(seg), start: stmtStart, end: i + 1})
			} else {
				nodes = append(nodes, st)
			}
			last = i + 1
			i++
			continue
		}

		i++
	}

	// Trailing Raw
	if last < end {
		nodes = append(nodes, &Raw{Text: string(p.src[last:end]), start: last, end: end})
	}

	return nodes, nil
}

// findStmtStart walks back from pos to find a likely start (skip preceding whitespace/comments that we kept in Raw).
func findStmtStart(src []byte, last, pos int) int {
	// naive: statement starts at first non-space from 'last' forward
	i := last
	for i < pos && isSpace(src[i]) {
		i++
	}
	return i
}

func (p *parser) buildStmt(seg []byte, absStart int) (*Stmt, error) {
	s := &Stmt{RawText: string(seg), start: absStart, end: absStart + len(seg)}

	// Extract top-level head vs. body: find first '{' at depth 0.
	// We must respect comments/strings again.
	i := 0
	depth := 0
	inSlashStar := false
	inLine := false
	inString := false
	braceOpen := -1
	braceClose := -1

	for i < len(seg) {
		c := seg[i]
		if inLine {
			if c == '\n' {
				inLine = false
			}
			i++
			continue
		}
		if inSlashStar {
			if c == '*' && i+1 < len(seg) && seg[i+1] == '/' {
				inSlashStar = false
				i += 2
				continue
			}
			i++
			continue
		}
		if inString {
			if c == '\\' {
				if i+1 < len(seg) {
					i += 2
					continue
				}
			}
			if c == '"' {
				inString = false
			}
			i++
			continue
		}
		if c == '/' && i+1 < len(seg) {
			if seg[i+1] == '*' {
				inSlashStar = true
				i += 2
				continue
			}
			if seg[i+1] == '/' {
				inLine = true
				i += 2
				continue
			}
		}
		if c == '#' {
			inLine = true
			i++
			continue
		}
		if c == '"' {
			inString = true
			i++
			continue
		}

		if c == '{' {
			if depth == 0 && braceOpen < 0 {
				braceOpen = i
			}
			depth++
			i++
			continue
		}
		if c == '}' {
			if depth > 0 {
				depth--
			}
			if depth == 0 && braceOpen >= 0 && braceClose < 0 {
				braceClose = i
			}
			i++
			continue
		}
		i++
	}

	// Head is before braceOpen (if any) else before final ';'
	semi := bytes.LastIndexByte(seg, ';')
	if semi < 0 {
		return nil, errors.New("statement missing semicolon")
	}

	if braceOpen >= 0 && braceClose >= 0 && braceClose > braceOpen {
		s.HasBlock = true
		s.HeadRaw = string(seg[:braceOpen])
		// LBraceRaw is from braceOpen to just after '{' plus any immediate spaces/newlines
		lbEnd := braceOpen + 1
		for lbEnd < len(seg) && isSpace(seg[lbEnd]) && lbEnd < braceClose {
			lbEnd++
		}
		s.LBraceRaw = string(seg[braceOpen:lbEnd])
		// Body between lbEnd and braceClose
		bodyStart := lbEnd
		bodyEnd := braceClose
		bodySrc := seg[bodyStart:bodyEnd]
		if len(bodySrc) > 0 {
			bodyNodes, err := p.parseRange(absStart+bodyStart, absStart+bodyEnd)
			if err != nil {
				// Tolerant: keep as raw body
				s.Body = []Node{&Raw{Text: string(bodySrc), start: absStart + bodyStart, end: absStart + bodyEnd}}
			} else {
				// Adjust body nodes to belong to this statement (their start/end are absolute already)
				s.Body = bodyNodes
			}
		}
		// RBraceRaw is from braceClose to just after '}' and any spaces until before the final ';'
		rbEnd := braceClose + 1
		for rbEnd < semi && isSpace(seg[rbEnd]) {
			rbEnd++
		}
		s.RBraceRaw = string(seg[braceClose:rbEnd])
		s.TrailingAfterR = string(seg[rbEnd:semi])
	} else {
		s.HasBlock = false
		s.HeadRaw = string(seg[:semi])
	}

	s.Keyword = strings.ToLower(firstIdent(s.HeadRaw))
	return s, nil
}

func isSpace(b byte) bool { return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f' }

// firstIdent extracts the first identifier-like token from s.
func firstIdent(s string) string {
	// Skip leading space and comment openers quickly.
	i := 0
	for i < len(s) {
		r := rune(s[i])
		if unicode.IsSpace(r) {
			i++
			continue
		}
		// Skip comments entirely
		if s[i] == '#' {
			// to end of line
			j := i + 1
			for j < len(s) && s[j] != '\n' {
				j++
			}
			i = j + 1
			continue
		}
		if s[i] == '/' && i+1 < len(s) && (s[i+1] == '/' || s[i+1] == '*') {
			if s[i+1] == '/' {
				j := i + 2
				for j < len(s) && s[j] != '\n' {
					j++
				}
				i = j + 1
				continue
			}
			// /* */
			j := i + 2
			for j+1 < len(s) && !(s[j] == '*' && s[j+1] == '/') {
				j++
			}
			if j+1 < len(s) {
				j += 2
			}
			i = j
			continue
		}
		break
	}
	// Collect until space/{/;/
	start := i
	for i < len(s) {
		c := s[i]
		if isSpace(c) || c == '{' || c == ';' {
			break
		}
		i++
	}
	tok := strings.TrimSpace(s[start:i])
	// Unquote if string literal
	tok = strings.Trim(tok, "\"")
	return tok
}

func trimRightSpace(s string) string {
	i := len(s)
	for i > 0 && isSpace(s[i-1]) {
		i--
	}
	return s[:i]
}
