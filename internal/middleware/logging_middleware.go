package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type loggerKey struct{}

func LoggingMiddleware(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID, ok := RequestID(r.Context())
			if !ok {
				logger.Error().Msg("request ID not found in request context")
			}

			enrichedLogger := logger.With()
			if requestID != uuid.Nil {
				enrichedLogger = enrichedLogger.Str("requestID", requestID.String())
			}

			enrichedLogger = enrichedLogger.Str("method", r.Method).Str("path", r.RequestURI)

			ctx := context.WithValue(r.Context(), loggerKey{}, enrichedLogger.Logger())
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func Logger(ctx context.Context) (zerolog.Logger, bool) {
	val, ok := ctx.Value(loggerKey{}).(zerolog.Logger)
	return val, ok
}
