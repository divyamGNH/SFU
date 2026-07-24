package main

import (
	"log"
	"net/http"

	"backend/room"
	"backend/websocket"

	"github.com/gorilla/mux"
)

func enableCORS(next http.Handler) http.Handler {
	allowedOrigins := map[string]bool{
		"http://localhost:3000": true,
		"http://127.0.0.1:3000": true,
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			// Fallback if no origin is provided or matched
			w.Header().Set("Access-Control-Allow-Origin", "http://127.0.0.1:3000")
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func setupRoutes(
	router *mux.Router,
	wsHandler *websocket.WsHandler,
	roomHandler *room.RoomHandler,
) {

	// websocket route
	router.HandleFunc("/ws/{roomId}/{clientId}", wsHandler.WebSocketHandler)

	// room routes
	router.HandleFunc("/createroom", roomHandler.CreateRoom).Methods("POST")

	router.HandleFunc("/joinroom/{roomId}", roomHandler.JoinRoom).Methods("POST")

	router.HandleFunc("/leaveroom/{roomId}/{clientId}", roomHandler.LeaveRoom).Methods("POST")

	router.HandleFunc("/viewroom/{roomId}", roomHandler.ViewRoom).Methods("GET")
}

func main() {
	log.Println("Main server has started")

	roomHandler := room.NewRoomHandler()

	wsHandler := &websocket.WsHandler{
		RoomHandler: roomHandler,
	}

	router := mux.NewRouter()
	setupRoutes(router, wsHandler, roomHandler)

	// Enable CORS.
	handler := enableCORS(router)

	// Start the server.
	err := http.ListenAndServe(":8080", handler)
	if err != nil {
		log.Fatal("Could not start the HTTP server")
	}
}
