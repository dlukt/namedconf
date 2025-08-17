package namedconf

import "bytes"

// WriteTo writes the entire file to w.
func (f *File) WriteTo(w *bytes.Buffer) {
	for _, n := range f.Nodes {
		n.writeTo(w)
	}
}
