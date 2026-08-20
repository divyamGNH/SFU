package main

import (
	"backend/api"
	"backend/logger"
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
	apiHandler *api.ApiHandler,
) {

	// websocket route
	router.HandleFunc("/ws/{roomId}/{clientId}", wsHandler.WebSocketHandler)

	// room routes
	router.HandleFunc("/createroom", roomHandler.CreateRoom).Methods("POST")

	router.HandleFunc("/room/{roomId}/{clientId}/sfu/join", apiHandler.JoinRoom).Methods("POST")

	router.HandleFunc("/room/{roomId}/{clientId}/sfu/leave", apiHandler.LeaveRoom).Methods("POST")

	router.HandleFunc("/room/{roomId}/{clientId}/sfu/offer", apiHandler.Offer).Methods("POST")

	router.HandleFunc("/room/{roomId}/{clientId}/sfu/answer", apiHandler.Answer).Methods("POST")

	router.HandleFunc("/room/{roomId}/{clientId}/sfu/ice", apiHandler.Ice).Methods("POST")

	router.HandleFunc("/viewroom/{roomId}", roomHandler.ViewRoom).Methods("GET")
}

func main() {
	// Initialize custom logger
	logger.InitLogger(logger.Config{
		GlobalLevel: logger.INFO,
		PackageFilters: map[string]logger.LogLevel{
			"sfu": logger.DEBUG, // example: set sfu to debug
		},
		FileFilters: map[string]logger.LogLevel{
			// "websocket.handler.go": logger.WARN,
		},
	})
	logger.Info("Main server has started")

	// Create handlers.
	roomHandler := room.NewRoomHandler()
	wsHandler := &websocket.WsHandler{
		RoomHandler: roomHandler,
	}
	apiHandler := &api.ApiHandler{
		RoomManager: roomHandler,
	}

	router := mux.NewRouter()
	setupRoutes(router, wsHandler, roomHandler, apiHandler)

	// Enable CORS.
	handler := enableCORS(router)

	// Start the server.
	err := http.ListenAndServe(":8080", handler)
	if err != nil {
		logger.Fatal("Could not start the HTTP server")
	}
}
