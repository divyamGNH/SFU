package participant

import (
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

// Use this client struct as later on we dont just need pc we would need userId, roomId etc a lot of things that is why use this struct.

// TODO : Currently we generate the userId using UUID on each request which is not okay so implement auth and then we need to get the userId on the server side from the cookie/jwt.
// Audio or VideoBool being false means the client's mic or camera is on.
type Client struct {
	UserId         string            `json:"userId"`
	RoomId         string            `json:"roomId"`
	Conn           *websocket.Conn   `json:"conn"`
	Publisher      *Publisher        `json:"publisher"`
	Subscriber     *Subscriber       `json:"subscriber"`
	MidToPublisher map[string]string `json:"midToPublisher"`
	AudioBool      bool              `json:"audioBool"`
	VideoBool      bool              `json:"videoBool"`
	ClientClosed   bool

	Mu   sync.RWMutex
	Send chan any
}

// This clean up only happens when the client leaves not on reconnections as this destroys the PC and connections everything client had with the server.
func (c *Client) CleanUpClient() {
	log.Printf("CleanUp for client itself with userId : %v has been triggered", c.UserId)
	c.Mu.Lock()
	if c.ClientClosed {
		c.Mu.Unlock()
		return
	}

	// mark the client as closed and cleanedUp so that no further actions could be taken by any parallel go routine.
	c.ClientClosed = true
	c.Mu.Unlock()

	if c.Publisher != nil {
		c.Publisher.CleanUpPublisher()
		c.Publisher = nil
	}

	if c.Subscriber != nil {
		c.Subscriber.CleanUpSubscriber()
		c.Subscriber = nil
	}

	c.MidToPublisher = nil

	if c.Conn != nil {
		err := c.Conn.Close()
		if err != nil {
			log.Printf("Error closing the client connection for client with userId: %v with roomId: %v", c.UserId, c.RoomId)
		}
		c.Conn = nil
	}

	// We don't close the channel as it may cause panic issues with any parallel go routine working on it we just flip the ClientClosed bool to prevent any more functions.
}

// Write pump is a function owned by the Client struct only
func (c *Client) WritePump() {

	c.Mu.RLock()
	if c.ClientClosed || c.Conn == nil {
		c.Mu.RUnlock()
		return
	}
	c.Mu.RUnlock()

	for msg := range c.Send {
		err := c.Conn.WriteJSON(msg)
		if err != nil {
			log.Println("[WritePump] Error in emitting the event: ", err)
			return
		}
	}
}

// TODO : Implement PING/PONG Hearbeat solution for zombie clients.
func (c *Client) SafeSend(msg any) bool {

	c.Mu.RLock()

	if c.ClientClosed {
		c.Mu.RUnlock()
		return false
	}

	send := c.Send

	c.Mu.RUnlock()

	select {
	case send <- msg:
		return true

	default:
		log.Printf("[Client %s] send channel full", c.UserId)
		//TODO : prod approach is to simple close the client and disconnect it as it simply can not keep up.
		return false
	}
}
