package models

import (
	"log"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

// Use this client struct as later on we dont just need pc we would need userId, roomId etc a lot of things that is why use this struct.

// TODO : Currently we get the userId from the frontend which is not okay even with auth implemented we need to generate the userId on the server side
type Client struct {
	UserId  string                 `json:"userId"`
	RoomId  string                 `json:"roomId"`
	Conn    *websocket.Conn        `json:"conn"`
	PC      *webrtc.PeerConnection `json:"pc"`
	SFUPeer *SFUPeer               `json:"peer"`

	Send chan any
}

type SFUPeer struct {
	PC                *webrtc.PeerConnection
	RemoteDescSet     bool
	PendingCandidates []ICECandidateMessage
	Mu                sync.RWMutex
}

// Write pump is a function owned by the Client struct only
func (c *Client) WritePump() {
	for msg := range c.Send {
		err := c.Conn.WriteJSON(msg)
		if err != nil {
			log.Println("[WritePump] Error in emitting the event: ", err)
			return
		}
	}
}

//TODO : Implement PING/PONG Hearbeat solution for zombie clients.

func (c *Client) SafeSend(client *Client, msg any) bool {
	select {
	case client.Send <- msg:
		return true

	default:
		log.Println("The WS event Send channel is full")
		//TODO : prod approach is to simple close the client and disconnect it as it simply can not keep up.

		return false
	}
}
