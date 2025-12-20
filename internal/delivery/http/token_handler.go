package http

import (
	"encoding/json"
	"net/http"
	"time"

	"screener-backend/internal/repository"
)

type TokenHandler struct {
	tokenRepo *repository.TokenRepository
}

func NewTokenHandler(tokenRepo *repository.TokenRepository) *TokenHandler {
	return &TokenHandler{
		tokenRepo: tokenRepo,
	}
}

type RegisterTokenRequest struct {
	Token    string
	Platform string
}

type TokenResponse struct {
	Success bool
	Message string
	Count   int
}

func (h *TokenHandler) HandleRegisterToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RegisterTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Token == "" {
		http.Error(w, "Token is required", http.StatusBadRequest)
		return
	}

	if req.Platform == "" {
		req.Platform = "android"
	}

	h.tokenRepo.RegisterToken(req.Token, req.Platform, time.Now().Unix())

	response := TokenResponse{
		Success: true,
		Message: "Token registered successfully",
		Count:   h.tokenRepo.GetTokenCount(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *TokenHandler) HandleUnregisterToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RegisterTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Token == "" {
		http.Error(w, "Token is required", http.StatusBadRequest)
		return
	}

	h.tokenRepo.UnregisterToken(req.Token)

	response := TokenResponse{
		Success: true,
		Message: "Token unregistered successfully",
		Count:   h.tokenRepo.GetTokenCount(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *TokenHandler) HandleGetTokenCount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := TokenResponse{
		Success: true,
		Message: "Token count retrieved",
		Count:   h.tokenRepo.GetTokenCount(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
