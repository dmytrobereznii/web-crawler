package middleware

import "net/http"

func Chain(routeHandler http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	if len(middlewares) == 0 {
		return routeHandler
	}

	currentMiddlewareIndex := len(middlewares) - 1

	return Chain(middlewares[currentMiddlewareIndex](routeHandler), middlewares[:currentMiddlewareIndex]...)
}
