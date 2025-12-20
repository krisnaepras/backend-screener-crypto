package websocket

import (
	"log"
	"net/http"
	"time"

	"screener-backend/internal/domain"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

type Handler struct {
	repo domain.ScreenerRepository
}

func NewHandler(repo domain.ScreenerRepository) *Handler {
	return &Handler{
		repo: repo,
	}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	log.Println("New Client Connected")

	// Send initial data immediately
	coins := h.repo.GetCoins()
	if err := conn.WriteJSON(coins); err != nil {
		log.Println("Write error:", err)
		return
	}

	ticker := time.NewTicker(5 * time.Second) // Poll every 5 seconds to match update cycle broadly
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Fetch latest
			currentCoins := h.repo.GetCoins()
			// Optimizaion: Diff? Or just send all.
			// Send all for now.
			if err := conn.WriteJSON(currentCoins); err != nil {
				log.Println("Write error:", err)
				return
			}
		}
	}
}
