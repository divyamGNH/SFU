package config

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/pion/webrtc/v3"
)

type TurnCredentialsResponse struct {
	Username   string   `json:"username"`
	Credential string   `json:"credential"`
	TTL        int      `json:"ttl"`
	URIs       []string `json:"uris"`
}

var cachedICEServers []webrtc.ICEServer
var cacheMutex sync.RWMutex

// InitTURNRefresh must be called from main.go on SFU startup.
// It fetches initial credentials and starts the async background refresh.
func InitTURNRefresh() {
	defaultStun := webrtc.ICEServer{
		URLs: []string{"stun:187.126.112.54:3478"},
	}

	cacheMutex.Lock()
	cachedICEServers = []webrtc.ICEServer{defaultStun}
	cacheMutex.Unlock()

	FetchTURNCredentials()

	go func() {
		for {
			// We sleep first so the next fetch happens in 55 mins
			time.Sleep(55 * time.Minute)
			FetchTURNCredentials()
		}
	}()
}

// FetchICEServers is called when a user joins. Zero delay!
func FetchICEServers() []webrtc.ICEServer {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	return cachedICEServers
}

func FetchTURNCredentials() {
	IRIS_API_URL := os.Getenv("IRIS_API_URL")
	IRIS_API_KEY := os.Getenv("IRIS_API_KEY")

	if IRIS_API_URL == "" || IRIS_API_KEY == "" {
		log.Println("IRIS_API_URL or IRIS_API_KEY is not set! Using STUN only.")
		return
	}

	postUrl := IRIS_API_URL + "/v1/turn/credentials"
	body := []byte(`{"ttl" : 3600}`)

	// Create the request first
	r, err := http.NewRequest("POST", postUrl, bytes.NewBuffer(body))
	if err != nil {
		log.Println("Error creating POST request to TURN server : ", err)
		return
	}

	// Set headers to the request.
	r.Header.Set("Authorization", "Bearer "+IRIS_API_KEY)
	r.Header.Set("Content-Type", "application/json")

	// Create a http client and actually send the request.
	client := &http.Client{}
	res, err := client.Do(r)
	if err != nil {
		log.Println("Error executing POST request : ", err)
		return
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(res.Body)
		log.Println("TURN server returned bad status : ", res.StatusCode, " Response: ", string(bodyBytes))
		return
	}

	// Decode the body.
	var TURNRes TurnCredentialsResponse
	err = json.NewDecoder(res.Body).Decode(&TURNRes)
	if err != nil {
		log.Println("Error decoding TURN credentials : ", err)
		return
	}

	// Update the cache.
	newServers := []webrtc.ICEServer{
		{
			URLs: []string{"stun:187.126.112.54:3478"}, // Primary STUN
		},
		{
			URLs:       TURNRes.URIs, // Fallback TURN
			Username:   TURNRes.Username,
			Credential: TURNRes.Credential,
		},
	}

	cacheMutex.Lock()
	cachedICEServers = newServers
	cacheMutex.Unlock()

	log.Println("Successfully refreshed and cached TURN credentials!")
}
