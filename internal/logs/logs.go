package logs

import (
	"log/slog"

	"github.com/dusted-go/logging/prettylog"
)

func NewPrettyLogger() *slog.Logger {
	return slog.New(prettylog.NewHandler(nil))
}
