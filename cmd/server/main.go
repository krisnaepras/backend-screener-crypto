package main

import (
	"log"
	"net/http"
	"os"
	"time"

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
	tradeRepo := repository.NewInMemoryTradeRepository()
	autoScalpRepo := repository.NewInMemoryAutoScalpRepository()

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
	
	// 4. Initialize Auto Scalping Service
	autoScalpService := usecase.NewAutoScalpingService(autoScalpRepo, repo)
	
	// Start auto scalping monitor (every 5 seconds)
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			autoScalpService.MonitorAndExecute()
		}
	}()

	// 5. Start Screener Loop in background
	go uc.Run()

	// 6. Initialize HTTP Handlers
	wsHandler := websocket.NewHandler(repo)
	tokenHandler := httphandler.NewTokenHandler(tokenRepo)
	testHandler := httphandler.NewTestHandler(fcmClient, tokenRepo)
	tradeHandler := httphandler.NewTradeHandler(tradeRepo)
	autoScalpHandler := httphandler.NewAutoScalpHandler(autoScalpService)

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

	// Trade management endpoints
	http.HandleFunc("/api/trades", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			tradeHandler.CreateEntry(w, r)
		} else if r.Method == http.MethodGet {
			if r.URL.Query().Get("status") == "active" {
				tradeHandler.GetActiveEntries(w, r)
			} else {
				tradeHandler.GetHistory(w, r)
			}
		}
	})
	http.HandleFunc("/api/trades/active", tradeHandler.GetActiveEntries)
	http.HandleFunc("/api/trades/history", tradeHandler.GetHistory)
	http.HandleFunc("/api/trades/update", tradeHandler.UpdateEntry)
	http.HandleFunc("/api/trades/delete", tradeHandler.DeleteEntry)

	// Auto Scalping endpoints
	http.HandleFunc("/api/autoscalp/settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			autoScalpHandler.GetSettings(w, r)
		} else if r.Method == http.MethodPost {
			autoScalpHandler.UpdateSettings(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	http.HandleFunc("/api/autoscalp/active", autoScalpHandler.GetActivePositions)
	http.HandleFunc("/api/autoscalp/history", autoScalpHandler.GetHistory)

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
