package websocket

import (
	"backend/config"
	"backend/participant"
	"backend/room"
	"backend/types"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WsHandler struct {
	RoomHandler *room.RoomHandler
}

func (wh *WsHandler) CompleteCleanup(client *participant.Client) {
	log.Printf("[WS] Tab closed! Cleaning up user: %s", client.UserId)

	// 1. Utilize your existing Client/Publisher/Subscriber cleanup!
	client.CleanUpClient()
	// 2. Utilize our new helper to remove them from the room
	if client.RoomId != "" && client.UserId != "" {
		wh.RoomHandler.RemoveClient(client.RoomId, client.UserId)
	}
}

func (wh *WsHandler) WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("[WS] Received websocket upgrade request")

	//HTTP -> WS
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("[WS] Error upgrading websocket:", err)
		return
	}

	client := &participant.Client{
		Conn:           conn,
		MidToPublisher: make(map[string]string),
		AudioBool:      false,
		VideoBool:      false,
		Send:           make(chan any, 256),
	}

	defer wh.CompleteCleanup(client)

	iceServers := config.FetchICEServers()
	// Initialize Publisher (and pass the RoomHandler's callback)
	publisher, err := participant.NewPublisher(
		iceServers,
		participant.PublisherCallbacks{
			OnTrackPublished: wh.RoomHandler.OnTrackPublished,
		},
		client,
	)
	if err != nil {
		log.Println("[WS] Error creating publisher:", err)
		return
	}
	client.Publisher = publisher
	// Initialize Subscriber
	subscriber, err := participant.NewSubscriber(
		iceServers,
		participant.SubscriberCallbacks{},
		client,
	)
	if err != nil {
		log.Println("[WS] Error creating subscriber:", err)
		return
	}
	client.Subscriber = subscriber

	go client.WritePump()

	//The backend must listen for the WS events continously so we run a infinite for loop.
	for {
		//We get msgType, msg and the err but we are not handlng the msgType right now hence we put a _ for now
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("[WS] Error reading websocket message:", err)
			break
		}

		//decode to a base type to understand what kind of msg it is.
		var base types.BaseMessage

		err = json.Unmarshal(msg, &base)
		if err != nil {
			log.Println("[WS] Error decoding base message:", err)
			continue
		}

		switch base.Type {

		case "offer":
			//handle offer event
			var signal types.SignalMessage

			err := json.Unmarshal(msg, &signal)
			if err != nil {
				log.Println("[WS] Error decoding offer message:", err)
				continue
			}

			client.Publisher.HandleOffer(signal)

			// log.Println("[WS] Finished SFU.HandleOffer")

		case "ice-candidate":
			//Handle ice candidate event
			var iceMessage types.ICECandidateMessage

			err := json.Unmarshal(msg, &iceMessage)
			if err != nil {
				log.Println("[WS] Error decoding ICE candidate message:", err)
				continue
			}

			client.Publisher.HandleICECandidate(iceMessage, client)

		case "populate-room":
			// User creates or joins the room he/she is eventually entering the room so only one event to just add the roomId to the client struct

			var createRoomMessage types.PopulateRoomMessage

			// Decode the ws message
			err := json.Unmarshal(msg, &createRoomMessage)
			if err != nil {
				log.Println("[WS] Error decoding create-room message:", err)
				return
			}

			// Set the roomid and userid for the client.
			client.RoomId = createRoomMessage.RoomId
			client.UserId = createRoomMessage.UserId

			// Get the room.
			room, ok := wh.RoomHandler.GetRoom(createRoomMessage.RoomId)
			if !ok {
				log.Println("[WS] Room not found")
				return
			}

			// Populate neccesary room maps and arrays.
			room.Mu.Lock()
			room.UserIdToClient[createRoomMessage.UserId] = client
			room.Mu.Unlock()

			// log.Println("[WS] roomId attached to the client successfully")

			wh.RoomHandler.SendRemoteMediaToLocalPeer(client)

		case "peer-left":
			// If the frontend explicitly sends a peer-left websocket event,
			// we can just break the loop as this triggers the defer block at the top,
			// which automatically does CompleteCleanup.
			log.Printf("[WS] Client %s explicitly requested to leave", client.UserId)
			return

		case "subscriber-answer":
			log.Printf("Received subscriber answer")
			answerMsg := &types.SubscriberAnswerMessage{}

			err := json.Unmarshal(msg, answerMsg)
			if err != nil {
				log.Println("[WS] Error decoding subscriber-answer message:", err)
				return
			}

			client.Subscriber.HandleAnswer(answerMsg)

		case "subscriber-ice-candidate":
			var subscriberIceMessage types.ICECandidateMessage

			err := json.Unmarshal(msg, &subscriberIceMessage)
			if err != nil {
				log.Println("[WS] Error decoding subscriber-ice-candidate message:", err)
				return
			}

			client.Subscriber.HandleIce(subscriberIceMessage)

		case "audio-toggle":
			var audioToggleMessage types.AudioToggleMessage

			err := json.Unmarshal(msg, &audioToggleMessage)
			if err != nil {
				log.Println("[WS] Error decoding audio-toggle message:", err)
				return
			}

			wh.RoomHandler.HandleToggleAudio(audioToggleMessage.Muted, client)

		case "video-toggle":
			var videoToggleMessage types.VideoToggleMessage

			err := json.Unmarshal(msg, &videoToggleMessage)
			if err != nil {
				log.Println("[WS] Error decoding video-toggle message:", err)
				return
			}

			wh.RoomHandler.HandleToggleVideo(videoToggleMessage.Muted, client)

		default:
			log.Println("[WS] Unknown message type received:", base.Type)
		}

		// log.Println("[WS] Finished processing current websocket message")
	}

	log.Println("[WS] Read loop ended")
	// wh.SFU.CleanupSFU(client)
}
