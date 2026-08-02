package cdn

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/motionmesh/server/api/internal/auth"
	"github.com/motionmesh/server/shared/models"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/cdn/domains", h.HandleListDomains)
	r.Post("/cdn/domains", h.HandleAddDomain)
	r.Get("/cdn/domains/{id}", h.HandleGetDomainStatus)
	r.Delete("/cdn/domains/{id}", h.HandleDeleteDomain)
	r.Get("/cdn/stats", h.HandleGetStats)
}

func (h *Handler) HandleListDomains(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(auth.AccountContextKey).(*models.Account)
	
	domains, err := h.service.ListDomains(r.Context(), user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if domains == nil {
		domains = []models.CDNDomain{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(domains)
}

type AddDomainRequest struct {
	Hostname string `json:"hostname"`
}

func (h *Handler) HandleAddDomain(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(auth.AccountContextKey).(*models.Account)

	var req AddDomainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Hostname == "" {
		http.Error(w, "hostname is required", http.StatusBadRequest)
		return
	}

	domain, err := h.service.AddCustomDomain(r.Context(), user.ID, req.Hostname)
	if err != nil {
		if err.Error() == "Cloudflare not configured" {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(domain)
}

func (h *Handler) HandleGetDomainStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	
	domain, err := h.service.RefreshDomainStatus(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(domain)
}

func (h *Handler) HandleDeleteDomain(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	err := h.service.DeleteDomain(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) HandleGetStats(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(auth.AccountContextKey).(*models.Account)

	stats, err := h.service.GetStats(r.Context(), user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
