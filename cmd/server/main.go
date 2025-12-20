package main

import (
	"log"
	"net/http"
	"os"

	httphandler "screener-backend/internal/delivery/http"
	"screener-backend/internal/delivery/websocket"
	"screener-backend/internal/infrastructure/fcm"
	"screener-backend/internal/repository"
	"screener-backend/internal/usecase"
)

func main() {
	// 1. Initialize Repositories
	repo := repository.NewInMemoryScreenerRepository()
	tokenRepo := repository.NewTokenRepository()

	// 2. Initialize FCM Client
	fcmClient, err := fcm.NewClient()
	if err != nil {
		log.Printf("Warning: FCM initialization failed: %v", err)
		log.Println("Server will continue without push notifications")
	} else if fcmClient.IsEnabled() {
		log.Println("✓ FCM push notifications enabled")
	} else {
		log.Println("⚠ FCM disabled - set FIREBASE_CREDENTIALS_PATH or FIREBASE_CREDENTIALS_JSON")
	}

	// 3. Initialize Usecase
	binanceBaseURL := os.Getenv("BINANCE_BASE_URL")
	uc := usecase.NewScreenerUsecase(repo, tokenRepo, fcmClient, binanceBaseURL)

	// 4. Start Screener Loop in background
	go uc.Run()

	// 5. Initialize HTTP Handlers
	wsHandler := websocket.NewHandler(repo)
	tokenHandler := httphandler.NewTokenHandler(tokenRepo)
	testHandler := httphandler.NewTestHandler(fcmClient, tokenRepo)

	// Routes
	http.HandleFunc("/ws", wsHandler.Handle)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	
	// Token management endpoints
	http.HandleFunc("/api/register-token", tokenHandler.HandleRegisterToken)
	http.HandleFunc("/api/unregister-token", tokenHandler.HandleUnregisterToken)
	http.HandleFunc("/api/token-count", tokenHandler.HandleGetTokenCount)
	
	// Test notification endpoint
	http.HandleFunc("/api/test-notification", testHandler.SendTestNotification)

	// Get port from environment variable (Heroku sets this)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
