package main

import (
	"log"
	"net/http"

	"backend/sfu"
	"backend/websocket"
)

func setupRoutes(wsHandler *websocket.WsHandler) {
	http.HandleFunc("/ws", wsHandler.WebSocketHandler)
}

func main() {
	log.Println("Main server has started")

	sfuInstance := sfu.NewSFU()

	wsHandler := &websocket.WsHandler{
		SFU: sfuInstance,
	}

	setupRoutes(wsHandler)

	err := http.ListenAndServe(":8080", nil)
	//This is a fatal error that is why we do a log.Fatal
	if err != nil {
		log.Fatal("Could not start the HTTP server")
	}
}
