package tracing

import (
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// Middleware is a net/http middleware.
type Middleware = func(http.Handler) http.Handler

// tracingTransport injects OpenTelemetry headers into requests
type TracingTransport struct {
	Wrapped http.RoundTripper
}

// Inject tracing headers
func (t *TracingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	propagator := otel.GetTextMapPropagator()
	propagator.Inject(req.Context(), propagation.HeaderCarrier(req.Header))
	return t.Wrapped.RoundTrip(req)
}

// NewTracingClient creates an HTTP client with tracing support
func NewTracingClient() *http.Client {
	return &http.Client{
		Transport: &TracingTransport{Wrapped: http.DefaultTransport},
	}
}

// ------------- EXAMPLES!
// e.g. Middleware to attach the OTel context into request headers
// func addOtelContextToHeaders(ctx context.Context, app2URL string) error {
// 	// Create HTTP request with context
// 	req, _ := http.NewRequestWithContext(ctx, "GET", app2URL, nil)

// 	// Inject OpenTelemetry context into headers
// 	propagator := otel.GetTextMapPropagator()
// 	propagator.Inject(ctx, propagation.HeaderCarrier(req.Header))

// 	// Execute request
// 	client := http.Client{}
// 	_, err := client.Do(req)
// 	return err
// }

// func extractOtelContext(w http.ResponseWriter, r *http.Request) {
// 	// Extract context from headers
// 	propagator := otel.GetTextMapPropagator()
// 	ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

// 	// Start span using extracted context
// 	tracer := otel.Tracer("app2-tracer")
// 	ctx, span := tracer.Start(ctx, "app2-handler")
// 	defer span.End()

// 	// Your business logic here
// 	// ...
// }
