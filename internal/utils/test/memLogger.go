package test

// MemLogger is a test util that implements io.Writer.
type MemLogger struct {
	lines []string
}

// Lines returns the lines written to the MemLogger.
func (w *MemLogger) Lines() []string {
	return w.lines
}

// Write a single line to the NoOpWriter.
func (w *MemLogger) Write(b []byte) (int, error) {
	w.lines = append(w.lines, string(b))
	return len(b), nil
}

// Clear all lines in the NoOpWriter.
func (w *MemLogger) Clear() {
	w.lines = []string{}
}
