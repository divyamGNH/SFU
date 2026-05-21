package websocket

import (
	"backend/models"
	"backend/room"
	"backend/sfu"
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
	SFU         *sfu.SFU
	RoomHandler *room.RoomHandler
}

func (wh *WsHandler) WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("[WS] Received websocket upgrade request")

	//HTTP -> WS
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("[WS] Error upgrading websocket:", err)
		return
	}

	defer conn.Close()

	log.Println("[WS] Websocket connected successfully")

	client := models.Client{
		Conn: conn,
		Send: make(chan any, 256),
	}
	// log.Println("[WS] Client created succesfully")

	go client.WritePump()
	// log.Println("[HandleOffer] WritePump started")

	//The backend must listen for the WS events continously so we run a infinite for loop.
	for {

		log.Println("[WS] Waiting for websocket message")

		//We get msgType, msg and the err but we are not handlng the msgType right now hence we put a _ for now
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("[WS] Error reading websocket message:", err)
			break
		}

		// log.Println("[WS] Raw message received:", string(msg))

		//decode to a base type to understand what kind of msg it is.
		var base models.BaseMessage

		// log.Println("[WS] Decoding base message")

		err = json.Unmarshal(msg, &base)
		if err != nil {
			log.Println("[WS] Error decoding base message:", err)
			continue
		}

		// log.Println("[WS] Message type:", base.Type)

		switch base.Type {

		case "offer":
			//handle offer event
			var signal models.SignalMessage

			err := json.Unmarshal(msg, &signal)
			if err != nil {
				log.Println("[WS] Error decoding offer message:", err)
				continue
			}

			wh.SFU.HandleOffer(signal, conn, &client)

			// log.Println("[WS] Finished SFU.HandleOffer")

		case "ice-candidate":
			//Handle ice candidate event
			var iceMessage models.ICECandidateMessage

			err := json.Unmarshal(msg, &iceMessage)
			if err != nil {
				log.Println("[WS] Error decoding ICE candidate message:", err)
				continue
			}

			wh.SFU.HandleICECandidate(iceMessage, &client)

		case "populate-room":
			// User creates or joins the room he/she is eventually entering the room so only one event to just add the roomId to the client struct

			var createRoomMessage models.PopulateRoomMessage

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
			room.UserIdToClient[createRoomMessage.UserId] = &client
			room.Mu.Unlock()

			// log.Println("[WS] roomId attached to the client successfully")

		case "leave-room":
			var leaveRoomMessage models.LeaveRoomMessage

			// Decode the ws message
			err := json.Unmarshal(msg, &leaveRoomMessage)
			if err != nil {
				log.Println("[WS] Error decoding join-room message:", err)
				return
			}

			//TODO : Handle cleanup here.
			// wh.RoomHandler.LeaveRoom()

		default:
			log.Println("[WS] Unknown message type received:", base.Type)
		}

		// log.Println("[WS] Finished processing current websocket message")
	}
}
