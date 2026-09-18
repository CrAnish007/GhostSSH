//go:build logger

package log

import (
	"fmt"
	"gossh/internal/utils/test"
	"strings"
	"testing"
)

// TestLog_LevelVerbosity ensures the LogLevel-to-verbosity mapping remains stable.
func TestLog_LevelVerbosity(t *testing.T) {
	for _, i := range []struct {
		LogLevel
		int
	}{
		{TRACE, 5},
		{DEBUG, 4},
		{INFO, 3},
		{WARN, 2},
		{ERROR, 1},
	} {
		if i.int != i.getVerbosity() {
			t.Logf("expected %v to equal %v but was %v", i.LogLevel, i.int, i.getVerbosity())
			t.Fail()
		}
	}
}

type lineTest struct {
	level  LogLevel
	format string
	args   []any
}

// expectLoggedLines verifies that only the expected log lines are written for a given verbosity level.
func expectLoggedLines(t *testing.T, level LogLevel, given []lineTest, expected []string) {
	lw := &test.MemLogger{}
	testLogger := NewLogger(level, lw)

	// Write each line to the logger. Messages above the configured verbosity should be ignored.
	for _, test := range given {
		getLogFuncByLevel(testLogger, test.level)(test.format, test.args...)
	}

	// Confirm the logger wrote the expected number of lines.
	// This check alone does not prove the content is correct, only the count.
	if len(lw.Lines()) != len(expected) {
		t.Logf("expected %v lines to be wrote to log, but was %v", len(lw.Lines()), len(expected))
		t.Fail()
		return
	}

	// Verify each expected line was written in order.
	// The logger writes sequentially, so a simple side-by-side comparison is enough.
	// Keep log messages unique when testing to avoid false matches.
	for i, expect := range expected {
		if !strings.Contains(lw.Lines()[i], expect) {
			t.Logf("expected '%v' to contain '%v'", lw.Lines()[i], expect)
			t.Fail()
			return
		}
	}
}

// TestLog_Verbosity ensures logs are emitted according to the configured verbosity level.
func TestLog_Verbosity(t *testing.T) {
	testLines := []lineTest{
		{
			level:  TRACE,
			format: "Test %v",
			args:   []any{TRACE},
		},
		{
			level:  DEBUG,
			format: "Test %v",
			args:   []any{DEBUG},
		},
		{
			level:  INFO,
			format: "Test %v",
			args:   []any{INFO},
		},
		{
			level:  WARN,
			format: "Test %v",
			args:   []any{WARN},
		},
		{
			level:  ERROR,
			format: "Test %v",
			args:   []any{ERROR},
		},
	}

	for _, test := range []struct {
		name  string
		level LogLevel
		lines []lineTest
	}{
		{
			name:  "TraceAndBelow",
			level: TRACE,
			lines: testLines,
		},
		{
			name:  "DebugAndBelow",
			level: DEBUG,
			lines: testLines,
		},
		{
			name:  "InfoAndBelow",
			level: INFO,
			lines: testLines,
		},
		{
			name:  "WarnAndBelow",
			level: WARN,
			lines: testLines,
		},
		{
			name:  "ErrorAndBelow",
			level: ERROR,
			lines: testLines,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			expect := []string{}
			for _, line := range test.lines {
				if line.level.getVerbosity() <= test.level.getVerbosity() {
					expect = append(expect, fmt.Sprintf(line.format, line.args...))
				}
			}
			expectLoggedLines(t, test.level, test.lines, expect)
		})
	}
}

// getLogFuncByLevel returns the logger method associated with the specified log level.
func getLogFuncByLevel(testLogger Logger, level LogLevel) func(format string, args ...any) {
	switch level {
	case TRACE:
		return testLogger.Trace
	case DEBUG:
		return testLogger.Debug
	case INFO:
		return testLogger.Info
	case WARN:
		return testLogger.Warn
	case ERROR:
		return testLogger.Error
	}
	return nil
}
