package core

import (
	"log"
	"strings"
)

// StructuredLogger emits log lines in the daemon's `component=X level=Y
// event=Z key=val ...` format, so packages outside daemon (telemetry,
// detect, config, integrations) can converge on the same shape.
//
// Use NewLogger(component) per package; pass to functions that want to
// emit log lines without taking a daemon Service handle. Defaults to
// always-on; pass a Verbose() function (or the all-true ones below) to
// gate.
type StructuredLogger struct {
	component string
	verbose   func() bool
}

// NewLogger returns a logger for the given component (e.g. "telemetry",
// "detect"). All emitted lines start with `component=<component> level=...`.
func NewLogger(component string) *StructuredLogger {
	return &StructuredLogger{component: component, verbose: alwaysTrue}
}

// WithVerbose returns a copy of the logger that only emits when verbose()
// returns true. Use to gate on a runtime --verbose flag.
func (l *StructuredLogger) WithVerbose(verbose func() bool) *StructuredLogger {
	if verbose == nil {
		verbose = alwaysTrue
	}
	return &StructuredLogger{component: l.component, verbose: verbose}
}

// Infof emits an info-level line.
func (l *StructuredLogger) Infof(event, format string, args ...any) {
	l.emit("info", event, format, args...)
}

// Warnf emits a warn-level line.
func (l *StructuredLogger) Warnf(event, format string, args ...any) {
	l.emit("warn", event, format, args...)
}

func (l *StructuredLogger) emit(level, event, format string, args ...any) {
	if l == nil || (l.verbose != nil && !l.verbose()) {
		return
	}
	prefix := "component=" + l.component + " level=" + level + " event=" + event
	if strings.TrimSpace(format) == "" {
		log.Print(prefix)
		return
	}
	log.Printf(prefix+" "+format, args...)
}

func alwaysTrue() bool { return true }
