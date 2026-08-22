package signalling

import (
	"backend/service"
	"context"
	"fmt"
	"log"
	control "proto-contracts"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var MAX_RETRY_LIMIT = 10

type IrisClient struct {
	conn       *grpc.ClientConn
	grpcClient control.SFUControlClient
	stream     control.SFUControl_ConnectClient
	service    *service.Service

	send     chan *control.Message
	done     chan any
	streamMu sync.RWMutex
}

func NewIrisClient(serverAddress string, service *service.Service) (*IrisClient, error) {
	// Create a gRPC connection to Iris here.
	// TODOINPROD : Here we used insecure creds for local development but we need to use TLS when we go in prod.
	conn, err := grpc.NewClient(serverAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	// Get the client form the conn.
	client := control.NewSFUControlClient(conn)

	return &IrisClient{
		conn:       conn,
		grpcClient: client,
		service:    service,
		send:       make(chan *control.Message, 100),
	}, nil
}

func (ic *IrisClient) Start() error {
	// Call the Connect() function to obtain the stream from the iris gRPC connection

	stream, err := ic.grpcClient.Connect(context.Background())
	if err != nil {
		return err
	}

	ic.streamMu.Lock()
	ic.stream = stream
	ic.streamMu.Unlock()

	// Start the receiver loop as a seperate go routine.
	go ic.receiverLoop(stream)

	// Start the writer loop
	go ic.writerLoop()

	return nil
}

// Define the receiver loop.
func (ic *IrisClient) receiverLoop(stream control.SFUControl_ConnectClient) {
	for {
		msg, err := stream.Recv()

		if err != nil {
			log.Printf("Stream disconnected : %v", err)

			// Call the reconnection handler for reconnecting and maintaining the bidirectional communication.

			ic.Reconnect()
			return
		}

		log.Println("Received a message from the stream")

		// Add the switch case to decode the message from the types etc like ws.
		switch payload := msg.Payload.(type) {
		case *control.Message_JoinRoom:
			ic.service.OnJoinRoom(payload.JoinRoom)

		case *control.Message_LeaveRoom:
			ic.service.OnLeaveRoom(payload.LeaveRoom)

		case *control.Message_PublisherOffer:
			ic.service.OnPublisherOffer(payload.PublisherOffer)

		case *control.Message_SubscriberAnswer:
			ic.service.OnSubscriberAnswer(payload.SubscriberAnswer)

		case *control.Message_ClientPublisherIce:
			ic.service.OnPublisherICECandidate(payload.ClientPublisherIce)

		case *control.Message_ClientSubscriberIce:
			ic.service.OnSubscriberICECandidate(payload.ClientSubscriberIce)

		default:
			log.Printf("Received unknown message type from Iris: %T", payload)
		}
	}
}

// Pop the messages from the send channel and actually Send them using the stream function Send.
func (ic *IrisClient) writerLoop() {
	for {
		select {
		case msg := <-ic.send:
			ic.streamMu.RLock()
			stream := ic.stream
			ic.streamMu.RUnlock()

			if stream != nil {
				if err := ic.stream.Send(msg); err != nil {
					log.Printf("Failed to send message %v", err)
					// Receiver loop will detect the break and trigger the reconnect automatically.
				}
			}

		case <-ic.done:
			log.Println("Shutting down writer loop .....")
			return
		}
	}
}

// Basically send a message to the send channel and wait 2 seconds if that fails return a error gracefully.
func (ic *IrisClient) SendMessageToIris(msg *control.Message) error {
	select {
	case ic.send <- msg:
		return nil

	case <-time.After(2 * time.Second):
		return fmt.Errorf("Send buffer is full, dropping the message")
	}
}

func (ic *IrisClient) Reconnect() {
	retries := 0
	for {
		// We should have a limit retry count for each connection.
		retries++
		if retries > MAX_RETRY_LIMIT {
			log.Fatalf("Max retry limit reached :(%d). Giving up or Iris connection.", MAX_RETRY_LIMIT)
		}

		log.Println("Attempting to reconnect stream .....")

		// Wait a few seconds before trying again so put all this shit behind a sleep.
		time.Sleep(2 * time.Second)

		stream, err := ic.grpcClient.Connect(context.Background())
		if err == nil {
			log.Println("Succesfully reconnected stream !!")

			ic.streamMu.Lock()
			ic.stream = stream
			ic.streamMu.Unlock()

			// Restart the receiver loop for the new stream.
			go ic.receiverLoop(stream)
			return
		}

		log.Printf("Reconnection failed : %v", err)
	}
}

func (ic *IrisClient) Stop() {
	close(ic.done)

	// We can close the connection here as gRPC has a connection pool over HTTP/2 so any goroutine that might use the conn after it closed will not crash but gracefully return. Unlike WS where this would crash.

	// Close() simply kills all the streams between the iris and this particular sfu-node.
	// sfu-node here will kill all the streams with iris here.
	// Right now we have only one stream between a sfu-node and the Iris so it is okay but if we increase the number of streams we will need to close a particular stream with contexts
	// Context cancelling is very common in gRPC based communication systems.
	ic.conn.Close()
}
