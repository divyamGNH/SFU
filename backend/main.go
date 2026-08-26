package main

import (
	"backend/logger"
	"backend/service"

	"backend/room"
	"backend/signalling"
)

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

	// Create a new sfuService.
	sfuService := service.NewService(roomHandler)

	// Create the gRPC Client that can talk to Iris.
	grpcClient, err := signalling.NewIrisClient("localhost:50051", sfuService)
	if err != nil {
		logger.Fatalf("Failed to create gRPC client: %v", err)
	}

	// Set the message sender that can talk to gRPC layer that talks to Iris.
	sfuService.SetMessageSender(grpcClient)

	// Start the gRPC server.
	// It creates it own goroutines so we dont need to initialize this as a seperate goroutine.
	grpcClient.Start()
}
