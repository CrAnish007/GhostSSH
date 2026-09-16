package test

// NoOpLineWriter is a test util that implements io.Writer.
type NoOpLineWriter struct {
	lines []string
}

// Lines returns the lines written to the NoOpLineWriter.
func (w *NoOpLineWriter) Lines() []string {
	return w.lines
}

// Write a single line to the NoOpWriter.
func (w *NoOpLineWriter) Write(b []byte) (int, error) {
	w.lines = append(w.lines, string(b))
	return len(b), nil
}

// Clear all lines in the NoOpWriter.
func (w *NoOpLineWriter) Clear() {
	w.lines = []string{}
}
