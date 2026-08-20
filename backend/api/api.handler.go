package api

import (
	"backend/httpx"
	"backend/room"
	"backend/types"
	"net/http"

	"github.com/gorilla/mux"
)

// join, offer, answer, ice, subscribe, unsubscribe, leave,

type ApiHandler struct {
	RoomManager *room.RoomHandler
}

func createNewApiHandler(rm *room.RoomHandler) *ApiHandler {
	return &ApiHandler{
		RoomManager: rm,
	}
}

func (ah *ApiHandler) JoinRoom(w http.ResponseWriter, r *http.Request) {
	// Get the variables from the link.
	vars := mux.Vars(r)
	roomId := vars["roomId"]
	clientId := vars["clientId"]

	// Perform the checks.
	if roomId == "" || clientId == "" {
		httpx.WriteError(w, "roomId and clientId are required", http.StatusBadRequest)
		return
	}

	// Call the engine.
	err := ah.RoomManager.JoinRoom(roomId, clientId)
	if err != nil {
		httpx.WriteError(w, err.Error(), http.StatusNotFound)
		return
	}

	// Send a succesfull response.
	res := types.JoinResponse{
		RoomID:   roomId,
		ClientID: clientId,
		SFUUrl:   "", // Ignored by iris-backend, left blank intentionally
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

func (ah *ApiHandler) LeaveRoom(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	roomId := vars["roomId"]
	clientId := vars["clientId"]

	if roomId == "" || clientId == "" {
		httpx.WriteError(w, "RoomId and ClientId are required", http.StatusBadRequest)
		return
	}

	err := ah.RoomManager.LeaveRoom(roomId, clientId)
	if err != nil {
		httpx.WriteError(w, err.Error(), http.StatusNotFound)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, struct{}{})
}

func (ah *ApiHandler) Offer(w http.ResponseWriter, r *http.Request) {
	// Get the variables from the link.
	vars := mux.Vars(r)
	roomId := vars["roomId"]
	clientId := vars["clientId"]

	// Perform the checks.
	if roomId == "" || clientId == "" {
		httpx.WriteError(w, "RoomId and ClientId are required", http.StatusBadRequest)
		return
	}

	// Get the Offer from the body.
	var req types.OfferResponse
	if err := httpx.ReadJSON(w, r, req); err != nil {
		httpx.WriteError(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	// Get the answer from the engine.
	answerSDP, err := ah.RoomManager.HandleOffer(roomId, clientId, req.SDP.SDP)
	if err != nil {
		httpx.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create a response
	res := &types.OfferResponse{
		SDP: types.SDPPayload{
			Type: "answer",
			SDP:  answerSDP,
		},
	}

	// Send a response.
	httpx.WriteJSON(w, http.StatusOK, res)
}

func (ah *ApiHandler) Answer(w http.ResponseWriter, r *http.Request) {

}

func (ah *ApiHandler) Ice(w http.ResponseWriter, r *http.Request) {

}
