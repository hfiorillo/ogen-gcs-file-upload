package logs

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGCPLoggerFormatsSeverityAndMessage(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := newGCPLogger(buf)

	logger.Warn("authentication failed", "component", "security")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	require.Equal(t, "WARNING", entry["severity"])
	require.Equal(t, "authentication failed", entry["message"])
	require.Equal(t, "security", entry["component"])

	_, hasLevel := entry["level"]
	require.False(t, hasLevel)
	_, hasMsg := entry["msg"]
	require.False(t, hasMsg)
	_, hasTimestamp := entry["timestamp"]
	require.True(t, hasTimestamp)
}

func TestIsLocal(t *testing.T) {
	require.False(t, isLocal(""))
	require.False(t, isLocal("development"))
	require.False(t, isLocal("production"))
	require.True(t, isLocal("local"))
	require.True(t, isLocal("LOCAL"))
}

func TestLocalLoggerIsReadable(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := newLocalLogger(buf)

	logger.Info("dev log", "component", "security")

	out := buf.String()
	require.Contains(t, out, "dev log")
	require.Contains(t, out, "component")
}
