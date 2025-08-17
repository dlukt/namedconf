package namedconf

import (
	"strings"
)

// MarkModified marks this statement as edited, forcing regeneration on write.
func (s *Stmt) MarkModified() { s.Modified = true }

// ReplaceHead replaces the raw head portion and marks the statement modified.
// Useful for quick edits like toggling a top-level option.
func (s *Stmt) ReplaceHead(newHead string) {
	s.HeadRaw = newHead
	s.Modified = true
}

// AppendToBody appends a child node (e.g., another Stmt) to the body and marks modified.
func (s *Stmt) AppendToBody(n Node) {
	if !s.HasBlock {
		// upgrade to block if previously simple
		s.HasBlock = true
		s.LBraceRaw = " {"
		s.RBraceRaw = "}"
		s.TrailingAfterR = ""
		s.Body = nil
	}
	s.Body = append(s.Body, n)
	s.Modified = true
}

// NewSimpleStmt builds a simple statement from a head string (no trailing semicolon).
func NewSimpleStmt(head string) *Stmt {
	head = strings.TrimRight(head, "; \t\r\n")
	return &Stmt{Keyword: strings.ToLower(firstIdent(head)), HeadRaw: head, HasBlock: false, Modified: true}
}

// NewBlockStmt builds a block statement with a head and body nodes.
func NewBlockStmt(head string, body []Node) *Stmt {
	head = strings.TrimSpace(head)
	return &Stmt{Keyword: strings.ToLower(firstIdent(head)), HeadRaw: head, HasBlock: true, Body: body, LBraceRaw: " {", RBraceRaw: "}", Modified: true}
}
