package httpmiddleware

// Middleware is a net/http middleware.
// type Middleware = func(http.Handler) http.Handler

// // InjectLogger injects logger into request context.
// func InjectLogger(lg *zap.Logger) Middleware {
// 	return func(next http.Handler) http.Handler {
// 		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 			reqCtx := r.Context()
// 			req := r.WithContext(zctx.Base(reqCtx, lg))
// 			next.ServeHTTP(w, req)
// 		})
// 	}
// }
