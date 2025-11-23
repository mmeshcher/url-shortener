package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func (h *Handler) SetupRouter() *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Logger)

	r.Route("/", func(r chi.Router) {
		r.Post("/", h.ShortenHandler)
		r.Route("/{shortID}", func(r chi.Router) {
			r.Get("/", h.RedirectHandler)
		})
	})

	r.NotFound(h.NotFoundHandler)
	r.MethodNotAllowed(h.MethodNotAllowedHandler)

	return r
}

func (h *Handler) NotFoundHandler(rw http.ResponseWriter, r *http.Request) {
	http.Error(rw, "Bad Request", http.StatusBadRequest)
}

func (h *Handler) MethodNotAllowedHandler(rw http.ResponseWriter, r *http.Request) {
	http.Error(rw, "Bad Request", http.StatusBadRequest)
}
