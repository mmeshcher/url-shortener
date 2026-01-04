package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	custommiddleware "github.com/mmeshcher/url-shortener/internal/middleware"
)

func (h *Handler) SetupRouter() *chi.Mux {
	r := chi.NewRouter()

	r.Use(custommiddleware.GzipMiddleware)
	r.Use(custommiddleware.Logger(h.logger))

	r.Route("/", func(r chi.Router) {
		r.Post("/", h.ShortenHandler)
		r.Get("/ping", h.PingHandler)
		r.Route("/{shortID}", func(r chi.Router) {
			r.Get("/", h.RedirectHandler)
		})
		r.Route("/api", func(r chi.Router) {
			r.Route("/shorten", func(r chi.Router) {
				r.Post("/", h.ShortenJSONHandler)
			})
		})
	})

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	})

	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	})

	return r
}
