package app

import (
	"fmt"
	"os"

	"github.com/inconshreveable/log15"
)

var (
	// Log Is the global log variable.
	Log = log15.New()

	// Default handler used in the package.
	defaultHandler log15.Handler
)

func init() {
	stdoutHandler := log15.StreamHandler(os.Stdout, log15.LogfmtFormat())
	SetDefaultLogHandler(log15.LvlFilterHandler(log15.LvlInfo, stdoutHandler))
	Log.SetHandler(defaultHandler)
}

// NewLog creates a new instance of the logger using the current default handler
// for its output.
func NewLog(ctx ...interface{}) log15.Logger {
	l := log15.New(ctx...)
	l.SetHandler(defaultHandler)
	return l
}

// Logf is short hand to create a message string using fmt.Sprintf.
func Logf(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}

// SetDefaultLogHandler sets the handler for the logger. It wraps handlers in a SyncHandler. You
// should not pass in handlers that are already wrapped in a SyncHandler.
func SetDefaultLogHandler(handler log15.Handler) {
	defaultHandler = log15.SyncHandler(handler)
	Log.SetHandler(defaultHandler)
}

// DefaultLogHandler returns the current handler. It can be used to create additional
// logger instances that all use the same handler for output.
func DefaultLogHandler() log15.Handler {
	return defaultHandler
}