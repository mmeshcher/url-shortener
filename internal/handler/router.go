package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	custommiddleware "github.com/mmeshcher/url-shortener/internal/middleware"
	"go.uber.org/zap"
)

func (h *Handler) SetupRouter(logger *zap.Logger) *chi.Mux {
	r := chi.NewRouter()

	r.Use(custommiddleware.GzipMiddleware)

	if logger != nil {
		r.Use(custommiddleware.Logger(logger))
	} else {
		r.Use(chimiddleware.Logger)
	}

	r.Route("/", func(r chi.Router) {
		r.Post("/", h.ShortenHandler)
		r.Route("/{shortID}", func(r chi.Router) {
			r.Get("/", h.RedirectHandler)
		})
		r.Route("/api", func(r chi.Router) {
			r.Route("/shorten", func(r chi.Router) {
				r.Post("/", h.APIShortenHandler)
			})
		})
	})

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	})

	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	})

	return r
}
