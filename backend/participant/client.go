package participant

import (
	"backend/logger"
	"sync"
)

// Use this client struct as later on we dont just need pc we would need userId, roomId etc a lot of things that is why use this struct.

// TODO : Currently we generate the userId using UUID on each request which is not okay so implement auth and then we need to get the userId on the server side from the cookie/jwt.
// Audio or VideoBool being false means the client's mic or camera is on.
type Client struct {
	UserId         string            `json:"userId"`
	RoomId         string            `json:"roomId"`
	Publisher      *Publisher        `json:"publisher"`
	Subscriber     *Subscriber       `json:"subscriber"`
	MidToPublisher map[string]string `json:"midToPublisher"`
	AudioBool      bool              `json:"audioBool"`
	VideoBool      bool              `json:"videoBool"`
	ClientClosed   bool

	Mu sync.RWMutex
}

func NewClient(roomId string, userId string, callbacks ClientCallbacks) (*Client, error) {
	client := &Client{
		RoomId:         roomId,
		UserId:         userId,
		MidToPublisher: make(map[string]string),
		AudioBool:      false,
		VideoBool:      false,
	}

	// Create a publisher.
	pub, err := NewPublisher(callbacks.GetIceServers(), callbacks.PubCallbacks, client)
	if err != nil {
		return nil, err
	}

	// Create a subscriber.
	sub, err := NewSubscriber(callbacks.GetIceServers(), callbacks.SubCallbacks, client)
	if err != nil {
		return nil, err
	}

	client.Publisher = pub
	client.Subscriber = sub

	return client, nil
}

// This clean up only happens when the client leaves not on reconnections as this destroys the PC and connections everything client had with the server.
func (c *Client) CleanUpClient() {
	logger.Infof("CleanUp for client itself with userId : %v has been triggered", c.UserId)
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

	// We don't close the channel as it may cause panic issues with any parallel go routine working on it we just flip the ClientClosed bool to prevent any more functions.
}
