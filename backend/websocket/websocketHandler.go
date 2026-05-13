package websocket

import (
	"backend/handlers"
	"backend/models"
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
	RoomHandler *handlers.RoomHandler
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

	log.Println("[WS] Client created succesfully")

	//The backend must listen for the WS events continously so we run a infinite for loop.
	for {

		log.Println("[WS] Waiting for websocket message")

		//We get msgType, msg and the err but we are not handlng the msgType right now hence we put a _ for now
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("[WS] Error reading websocket message:", err)
			break
		}

		log.Println("[WS] Raw message received:", string(msg))

		//decode to a base type to understand what kind of msg it is.
		var base models.BaseMessage

		log.Println("[WS] Decoding base message")

		err = json.Unmarshal(msg, &base)
		if err != nil {
			log.Println("[WS] Error decoding base message:", err)
			continue
		}

		log.Println("[WS] Message type:", base.Type)

		switch base.Type {

		case "offer":
			log.Println("[WS] Handling offer message in the backend")

			//handle offer event
			var signal models.SignalMessage

			err := json.Unmarshal(msg, &signal)
			if err != nil {
				log.Println("[WS] Error decoding offer message:", err)
				continue
			}

			log.Println("[WS] Calling SFU.HandleOffer")

			wh.SFU.HandleOffer(signal, conn, &client)

			log.Println("[WS] Finished SFU.HandleOffer")

		case "ice-candidate":
			log.Println("[WS] Handling ICE candidate message in the backend")

			//Handle ice candidate event
			var iceMessage models.ICECandidateMessage

			err := json.Unmarshal(msg, &iceMessage)
			if err != nil {
				log.Println("[WS] Error decoding ICE candidate message:", err)
				continue
			}

			log.Println("[WS] Calling SFU.HandleICECandidate")

			wh.SFU.HandleICECandidate(iceMessage, conn)

			log.Println("[WS] Finished SFU.HandleICECandidate")

		case "create-room":
			log.Println("[WS] create-room event received in the backend")

			var createRoomMessage models.CreateRoomMessage

			err := json.Unmarshal(msg, &createRoomMessage)
			if err != nil {
				log.Println("[WS] Error decoding create-room message:", err)
				return
			}

			wh.RoomHandler.CreateRoom(&createRoomMessage, &client)

		case "join-room":
			log.Println("[WS] join-room event received in the backend")

			var joinRoomMessage models.JoinRoomMessage

			err := json.Unmarshal(msg, &joinRoomMessage)
			if err != nil {
				log.Println("[WS] Error decoding join-room message:", err)
				return
			}

			wh.RoomHandler.JoinRoom(&joinRoomMessage, &client)

		case "leave-room":
			log.Println("[WS] leave-room event received in the backend")

			var leaveRoomMessage models.LeaveRoomMessage

			err := json.Unmarshal(msg, &leaveRoomMessage)
			if err != nil {
				log.Println("[WS] Error decoding join-room message:", err)
				return
			}

			wh.RoomHandler.LeaveRoom(&leaveRoomMessage, &client)

		default:
			log.Println("[WS] Unknown message type received:", base.Type)
		}

		log.Println("[WS] Finished processing current websocket message")
	}
}
