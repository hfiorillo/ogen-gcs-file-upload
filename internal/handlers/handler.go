package handlers

// type HTTPHandler func(
// 	context.Context,
// 	trace.Span,
// 	*logrus.Entry,
// 	http.ResponseWriter,
// 	*http.Request) (render.Renderer, Status)

// func WrapHTTPHandler(
// 	handler HTTPHandler,
// 	cfg config.Config,
// 	spanName string) http.HandlerFunc {

// 	h, _ := os.Hostname()

// 	return func(w http.ResponseWriter, r *http.Request) {

// 		ctx, span, rid := getTracer(r.Context(), r, spanName, cfg.Otel.IsEnabled)
// 		defer span.End()

// 		logger := logrus.WithField("host", h).
// 			WithField("app", "http").
// 			WithField("method", spanName).
// 			WithField("request_id", rid).
// 			WithContext(ctx)

// 		// add common log labels
// 		if doesWorkspaceExistInContext(r.Context()) {
// 			logger = logger.WithField("workspace_id", getWorkspaceFromContext(r.Context()).ID)
// 		}

// 		if doesUserExistInContext(r.Context()) {
// 			logger = logger.WithField("user_id", getUserFromContext(r.Context()).ID)
// 		}

// 		resp, status := handler(ctx, span, logger, w, r)
// 		switch status {
// 		case StatusFailed:

// 			span.SetStatus(codes.Error, "")

// 		case StatusSuccess:
// 			span.SetStatus(codes.Ok, "")

// 		default:
// 			_ = render.Render(w, r, newAPIStatus(500, "unknown error"))
// 			return
// 		}

// 		_ = render.Render(w, r, resp)
// 	}
// }
