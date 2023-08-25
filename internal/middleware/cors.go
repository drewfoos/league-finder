package middleware

import (
	"net/http"

	"github.com/rs/cors"
)

func CorsMiddleware(handler http.Handler) http.Handler {
	return cors.Default().Handler(handler)
}
