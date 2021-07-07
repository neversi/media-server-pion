package server

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/neversi/media-server-pion/pkg/pion"
)

type WSServer struct {
	clients  map[string]bool
	upgrader websocket.Upgrader
	ticker *time.Ticker
}

func (wss *WSServer) HomePage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Home Page")
}

func (wss *WSServer) HandleMedia(w http.ResponseWriter, r *http.Request) {
	wss.upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	done := make(chan struct{})
	defer close(done)
	ws, err := wss.upgrader.Upgrade(w, r, nil)
	defer func() {
		log.Println("Client has disconnected...")
		ws.Close()
		done <- struct{}{}
	}()

	if err != nil {
		log.Println(err)
	}
	log.Println("Client Successfully Connected...")

	pion.HandleMedia(ws, done)
}

func NewWSServer() *WSServer {
	var upgrader = websocket.Upgrader {
		ReadBufferSize: 1024,
		WriteBufferSize: 1024,
	}
	return &WSServer{
		clients: make(map[string]bool),
		upgrader: upgrader,
		ticker: time.NewTicker(10 * time.Second),
	}
}

func (ws *WSServer) SetupServer() {
	http.HandleFunc("/", ws.HomePage)
	http.HandleFunc("/ws", ws.HandleMedia)
}