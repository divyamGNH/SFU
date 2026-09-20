package main

import (
	"backend/config"
	"backend/logger"
	"backend/service"
	"os"

	"backend/room"
	"backend/signalling"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env file, overriding any existing stuck terminal vars
	if err := godotenv.Overload(); err != nil {
		logger.Warn("No .env file found or failed to load")
	}

	// Temporary debug print
	logger.Info("DEBUG KEY: " + os.Getenv("IRIS_API_KEY"))

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

	// Start the TURN routine.
	config.InitTURNRefresh()

	// Create the gRPC Client that can talk to Iris.
	grpcClient, err := signalling.NewIrisClient("localhost:50051", sfuService)
	if err != nil {
		logger.Fatalf("Failed to create gRPC client: %v", err)
	}

	// Set the message sender that can talk to gRPC layer that talks to Iris.
	sfuService.SetMessageSender(grpcClient)

	// Set the room handler to set up callbacks for allowing SFU node to talk to Iris.
	sfuService.SetUpRoomHandlerCallbacks()

	// Start the gRPC server.
	// It creates it own goroutines so we dont need to initialize this as a seperate goroutine.
	grpcClient.Start()

	// Add this right here!
	sfuService.SendIntialPing()

	// Block the main goroutine from exiting so the background gRPC routines keep running.
	select {}
}
