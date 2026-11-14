package runtime

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// StartupContext is cancelled by SIGINT/SIGTERM. Use it for work that must not
// block shutdown, such as waiting for MySQL during boot; Kratos keeps its own
// signal handling for the serving phase.
func StartupContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}
