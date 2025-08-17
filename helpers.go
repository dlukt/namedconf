package namedconf

// Convenience helpers for common queries (thin veneer for now).

// TopLevel returns top-level statements with the given keyword (e.g., "options", "zone").
func (f *File) TopLevel(keyword string) []*Stmt {
	keyword = normalize(keyword)
	var out []*Stmt
	for _, n := range f.Nodes {
		if s, ok := n.(*Stmt); ok && s.Keyword == keyword {
			out = append(out, s)
		}
	}
	return out
}

func normalize(s string) string {
	// Keep only ascii lower-case for now
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b = append(b, c)
	}
	return string(b)
}
