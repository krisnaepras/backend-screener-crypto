package main

import (
        "log"
        "net/http"
        "os"

        "screener-backend/internal/delivery/websocket"
        "screener-backend/internal/repository"
        "screener-backend/internal/usecase"
)

func main() {
        // 1. Initialize Repository
        repo := repository.NewInMemoryScreenerRepository()

        // 2. Initialize Usecase
        uc := usecase.NewScreenerUsecase(repo)

        // 3. Start Screener Loop in background
        go uc.Run()

        // 4. Initialize Delivery
        wsHandler := websocket.NewHandler(repo)

        http.HandleFunc("/ws", wsHandler.Handle)
        http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
                w.WriteHeader(http.StatusOK)
                w.Write([]byte(`{"status":"ok"}`))
        })

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
