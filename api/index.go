package handler

import (
	"net/http"

	"github.com/drewfoos/league-finder/internal/handlers"
	"github.com/drewfoos/league-finder/internal/middleware"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	mux := http.NewServeMux()
	mux.HandleFunc("/search", handlers.SearchHandler)

	handler := middleware.CorsMiddleware(mux)
	handler.ServeHTTP(w, r)
}

// package handler

// import (
// 	"net/http"

// 	"github.com/drewfoos/league-finder/internal/handlers"
// 	"github.com/drewfoos/league-finder/internal/middleware"
// )

// func Handler(w http.ResponseWriter, r *http.Request) {
// 	mux := http.NewServeMux()
// 	mux.HandleFunc("/search", handlers.SearchHandler)

// 	handler := middleware.CorsMiddleware(mux)
// 	handler.ServeHTTP(w, r)
// }
