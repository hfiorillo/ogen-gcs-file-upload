package logs

import (
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/blendle/zapdriver"
	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
	"go.uber.org/zap/zapcore"
)

// NewLogger returns an slog logger backed by zap.
// Production uses GCP-friendly zapdriver JSON, local uses zap console output.
func NewLogger(environment string) *slog.Logger {
	if isLocal(environment) {
		return newLocalLogger(os.Stdout)
	}
	return newGCPLogger(os.Stdout)
}

func isLocal(environment string) bool {
	return strings.EqualFold(strings.TrimSpace(environment), "local")
}

func newGCPLogger(w io.Writer) *slog.Logger {
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zapdriver.NewProductionEncoderConfig()),
		zapcore.AddSync(w),
		zap.InfoLevel,
	)
	zl := zap.New(core)
	return slog.New(zapslog.NewHandler(zl.Core()))
}

func newLocalLogger(w io.Writer) *slog.Logger {
	enc := zap.NewDevelopmentEncoderConfig()
	enc.TimeKey = ""
	enc.CallerKey = ""
	enc.NameKey = ""
	enc.StacktraceKey = ""
	enc.EncodeLevel = zapcore.CapitalColorLevelEncoder

	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(enc),
		zapcore.AddSync(w),
		zap.DebugLevel,
	)
	zl := zap.New(core)
	return slog.New(zapslog.NewHandler(zl.Core()))
}
