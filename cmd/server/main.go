package main

import (
	"log"
	"net/http"

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

	log.Println("Server executing on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
