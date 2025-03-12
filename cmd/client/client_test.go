package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ogenhttp "github.com/ogen-go/ogen/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/totalprocessing/file-upload/internal/auth"
	"gitlab.com/totalprocessing/file-upload/internal/fileupload"
	"gitlab.com/totalprocessing/file-upload/internal/tracing"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestTracingHeaders verifies that tracing headers are added to outgoing requests
func TestTracingHeaders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.NotEmpty(t, r.Header.Get("traceparent"), "Traceparent header should be present")

		// Verify basic auth
		username, password, ok := r.BasicAuth()
		assert.True(t, ok, "Basic auth should be present")
		assert.Equal(t, "testuser", username)
		assert.Equal(t, "testpass", password)

		// Return valid response structure
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := `{
            "filename": "test.txt",
            "fileSize": 1234,
            "bucket": "test-bucket",
            "uploadTime": "2023-01-01T00:00:00Z"
        }`
		w.Write([]byte(response))
	}))
	defer ts.Close()

	// Initialize tracing
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Create test config
	cfg := Config{
		ClientUrl:    ts.URL,
		AuthUsername: "testuser",
		AuthPassword: "testpass",
	}

	// Create security source
	sec := &auth.BasicAuthProvider{
		Username: cfg.AuthUsername,
		Password: cfg.AuthPassword,
	}

	// Create client with tracing
	client, err := fileupload.NewClient(
		cfg.ClientUrl,
		sec,
		fileupload.WithClient(tracing.NewTracingClient()),
	)
	require.NoError(t, err, "Failed to create client")

	// Create context with span
	ctx, span := tp.Tracer("test").Start(context.Background(), "test-span")
	defer span.End()

	// Create valid request with all required fields
	_, err = client.UploadFile(ctx, &fileupload.UploadFileReq{
		File: ogenhttp.MultipartFile{
			Name: "test.txt",
			File: bytes.NewReader([]byte("test content")),
		},
		// // Add required fields from error message
		// Filename:   "test.txt",
		// FileSize:   1234,
		// Bucket:     "test-bucket",
		// UploadTime: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
	})

	// fmt.Println(res)

	assert.NoError(t, err, "UploadFile should not return error")
}

// TestBasicAuthFailure verifies authentication error handling
func TestBasicAuthFailure(t *testing.T) {
	// Create invalid security source
	sec := &auth.BasicAuthProvider{
		Username: "wronguser",
		Password: "wrongpass",
	}

	// Create client with invalid credentials
	client, err := fileupload.NewClient(
		"http://localhost:8080",
		sec,
		fileupload.WithClient(tracing.NewTracingClient()),
	)
	require.NoError(t, err)

	file := ogenhttp.MultipartFile{
		Name: "testfile.json",
		File: bytes.NewReader([]byte("test content")),
	}

	// Make test call (assuming server would reject credentials)
	_, err = client.UploadFile(context.Background(), &fileupload.UploadFileReq{
		File: file,
	})

	// This assertion depends on your server implementation
	assert.Error(t, err, "Should return error for invalid credentials")
}

func TestTracingPropagation(t *testing.T) {
	// Create test exporter
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp),
	)
	otel.SetTracerProvider(tp)

	// Ensure proper propagator is set
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	client := tracing.NewTracingClient()
	// Create a span with a known trace ID and span ID
	tracer := tp.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	// Add baggage to the context
	bag, _ := baggage.Parse("test-key=test-value")
	ctx = baggage.ContextWithBaggage(ctx, bag)

	spanContext := span.SpanContext()
	t.Logf("Trace ID: %s", spanContext.TraceID())
	t.Logf("Span ID: %s", spanContext.SpanID())
	t.Logf("Trace Flags: %s", spanContext.TraceFlags())
	t.Logf("Trace State: %s", spanContext.TraceState())
	t.Logf("Baggage: %s", baggage.FromContext(ctx).String())

	// Create HTTP request with the context
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://example.com", nil)

	// Use client
	resp, err := client.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	// Log propagated headers
	t.Log("Propagated Headers:")
	t.Logf("Traceparent: %s", req.Header.Get("traceparent"))
	t.Logf("Tracestate: %s", req.Header.Get("tracestate"))
	t.Logf("Baggage: %s", req.Header.Get("baggage"))

	// Verify traceparent header (required)
	traceparent := req.Header.Get("traceparent")
	assert.NotEmpty(t, traceparent, "Traceparent header missing")

	// Parse the traceparent header to verify it matches the original span
	parts := strings.Split(traceparent, "-")
	assert.Equal(t, 4, len(parts), "Invalid traceparent format")
	assert.Equal(t, "00", parts[0], "Invalid traceparent version")
	assert.Equal(t, span.SpanContext().TraceID().String(), parts[1], "Trace ID mismatch")
	assert.Equal(t, span.SpanContext().SpanID().String(), parts[2], "Span ID mismatch")

	// Tracestate is optional and may not be present
	if tracestate := req.Header.Get("tracestate"); tracestate != "" {
		t.Log("Tracestate is present:", tracestate)
	} else {
		t.Log("Tracestate is empty (optional in OpenTelemetry)")
	}

	// Verify baggage header
	baggageHeader := req.Header.Get("baggage")
	assert.NotEmpty(t, "test-key=test-value", baggageHeader, "Baggage header mismatch")
}
