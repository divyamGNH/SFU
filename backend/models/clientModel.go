package models

import (
	"log"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

// Use this client struct as later on we dont just need pc we would need userId, roomId etc a lot of things that is why use this struct.
type Client struct {
	UserId string                 `json:"userId"`
	RoomId string                 `json:"roomId"`
	Conn   *websocket.Conn        `json:"conn"`
	PC     *webrtc.PeerConnection `json:"pc"`

	Send chan any
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
