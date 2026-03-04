package observability

import (
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type tracingTransport struct {
	Wrapped http.RoundTripper
}

func (t *tracingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	propagator := otel.GetTextMapPropagator()
	propagator.Inject(req.Context(), propagation.HeaderCarrier(req.Header))
	return t.Wrapped.RoundTrip(req)
}

// NewTracingClient creates an HTTP client that propagates trace context.
func NewTracingClient() *http.Client {
	return &http.Client{
		Transport: &tracingTransport{Wrapped: http.DefaultTransport},
	}
}
