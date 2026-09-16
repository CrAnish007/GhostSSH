package log_test

import (
	"fmt"
	"gossh/internal/log"
	"gossh/internal/utils/test"
	"strings"
	"testing"
)

// TestLog_LevelVerbosity ensures the LogLevel-to-verbosity mapping remains stable.
func TestLog_LevelVerbosity(t *testing.T) {
	for _, i := range []struct {
		log.LogLevel
		int
	}{
		{log.TRACE, 5},
		{log.DEBUG, 4},
		{log.INFO, 3},
		{log.WARN, 2},
		{log.ERROR, 1},
	} {
		if i.int != i.GetVerbosity() {
			t.Logf("expected %v to equal %v but was %v", i.LogLevel, i.int, i.GetVerbosity())
			t.Fail()
		}
	}
}

type lineTest struct {
	level  log.LogLevel
	format string
	args   []any
}

// expectLoggedLines verifies that only the expected log lines are written for a given verbosity level.
func expectLoggedLines(t *testing.T, level log.LogLevel, given []lineTest, expected []string) {
	lw := &test.NoOpLineWriter{}
	testLogger := log.NewLogger(lw, level.GetVerbosity())

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
			level:  log.TRACE,
			format: "Test %v",
			args:   []any{log.TRACE},
		},
		{
			level:  log.DEBUG,
			format: "Test %v",
			args:   []any{log.DEBUG},
		},
		{
			level:  log.INFO,
			format: "Test %v",
			args:   []any{log.INFO},
		},
		{
			level:  log.WARN,
			format: "Test %v",
			args:   []any{log.WARN},
		},
		{
			level:  log.ERROR,
			format: "Test %v",
			args:   []any{log.ERROR},
		},
	}

	for _, test := range []struct {
		name  string
		level log.LogLevel
		lines []lineTest
	}{
		{
			name:  "TraceAndBelow",
			level: log.TRACE,
			lines: testLines,
		},
		{
			name:  "DebugAndBelow",
			level: log.DEBUG,
			lines: testLines,
		},
		{
			name:  "InfoAndBelow",
			level: log.INFO,
			lines: testLines,
		},
		{
			name:  "WarnAndBelow",
			level: log.WARN,
			lines: testLines,
		},
		{
			name:  "ErrorAndBelow",
			level: log.ERROR,
			lines: testLines,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			expect := []string{}
			for _, line := range test.lines {
				if line.level.GetVerbosity() <= test.level.GetVerbosity() {
					expect = append(expect, fmt.Sprintf(line.format, line.args...))
				}
			}
			expectLoggedLines(t, test.level, test.lines, expect)
		})
	}
}

// getLogFuncByLevel returns the logger method associated with the specified log level.
func getLogFuncByLevel(testLogger log.Logger, level log.LogLevel) func(format string, args ...any) {
	switch level {
	case log.TRACE:
		return testLogger.Trace
	case log.DEBUG:
		return testLogger.Debug
	case log.INFO:
		return testLogger.Info
	case log.WARN:
		return testLogger.Warn
	case log.ERROR:
		return testLogger.Error
	}
	return nil
}
