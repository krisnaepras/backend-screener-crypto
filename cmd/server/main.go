package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	httphandler "screener-backend/internal/delivery/http"
	"screener-backend/internal/delivery/websocket"
	"screener-backend/internal/domain"
	"screener-backend/internal/infrastructure/db"
	"screener-backend/internal/infrastructure/fcm"
	"screener-backend/internal/repository"
	"screener-backend/internal/usecase"
)

func resolveDatabaseURL() string {
	if v := strings.TrimSpace(os.Getenv("DATABASE_URL")); v != "" {
		return v
	}

	// Common case: user references the add-on variable explicitly.
	if v := strings.TrimSpace(os.Getenv("HEROKU_POSTGRESQL_YELLOW_URL")); v != "" {
		return v
	}

	// Fallback: scan any Heroku Postgres add-on URL.
	for _, kv := range os.Environ() {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		val := strings.TrimSpace(parts[1])
		if val == "" {
			continue
		}
		if strings.HasPrefix(key, "HEROKU_POSTGRESQL_") && strings.HasSuffix(key, "_URL") {
			return val
		}
	}

	return ""
}

func main() {
	ctx := context.Background()

	// 1. Initialize Repositories
	repo := repository.NewInMemoryScreenerRepository()
	tokenRepo := repository.NewTokenRepository()
	tradeRepo := repository.NewInMemoryTradeRepository()
	
	// Initialize Binance API Repository with encryption key
	encryptionKey := os.Getenv("API_ENCRYPTION_KEY")
	dbURL := resolveDatabaseURL()
	if dbURL != "" {
		// Production safety: do not allow weak/empty encryption key when persisting secrets.
		if len(encryptionKey) < 32 {
			log.Fatal("API_ENCRYPTION_KEY is required and must be at least 32 characters when Postgres is enabled")
		}
	} else {
		// Dev fallback only (in-memory storage).
		if encryptionKey == "" {
			encryptionKey = "dev-only-default-key-change-in-production"
		}
	}

	var autoScalpRepo domain.AutoScalpRepository
	var binanceAPIRepo domain.BinanceAPIStore

	if dbURL != "" {
		pool, err := db.NewPool(ctx, dbURL, db.DefaultPoolConfig())
		if err != nil {
			log.Fatalf("Failed to create DB pool: %v", err)
		}
		defer pool.Close()

		if err := db.Migrate(ctx, pool); err != nil {
			log.Fatalf("DB migrate failed: %v", err)
		}
		log.Println("✓ Postgres connected (pooled) and migrated")

		autoScalpRepo = repository.NewPostgresAutoScalpRepository(pool)
		binanceAPIRepo = repository.NewPostgresBinanceAPIRepository(pool, encryptionKey)
	} else {
		log.Println("⚠ Postgres not configured (DATABASE_URL / HEROKU_POSTGRESQL_*_URL not set); using in-memory storage")
		autoScalpRepo = repository.NewInMemoryAutoScalpRepository()
		binanceAPIRepo = repository.NewBinanceAPIRepository(encryptionKey)
	}

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
	binanceAPIHandler := httphandler.NewBinanceAPIHandler(binanceAPIRepo)

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

	// Binance API endpoints
	http.HandleFunc("/api/binance/credentials", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			binanceAPIHandler.SaveCredentials(w, r)
		case http.MethodGet:
			binanceAPIHandler.GetCredentials(w, r)
		case http.MethodDelete:
			binanceAPIHandler.DeleteCredentials(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	http.HandleFunc("/api/binance/account", binanceAPIHandler.GetAccountInfo)
	http.HandleFunc("/api/binance/trading-config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			binanceAPIHandler.SaveTradingConfig(w, r)
		} else if r.Method == http.MethodGet {
			binanceAPIHandler.GetTradingConfig(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	http.HandleFunc("/api/binance/test-connection", binanceAPIHandler.TestConnection)

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
