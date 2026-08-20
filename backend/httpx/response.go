package httpx

import (
	"backend/logger"
	"backend/types"
	"encoding/json"
	"net/http"
)

func WriteError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	err := json.NewEncoder(w).Encode(types.ErrorResponse{
		Message: message,
	})
	if err != nil {
		logger.Error("Error encoding error response : ", err)
	}
}

func WriteJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		logger.Error("Error encoding the JSON response", err)
	}
}

func ReadJSON(w http.ResponseWriter, r *http.Request, dst interface{}) error {

	// Prevent memory attacks majorly DOS.
	r.Body = http.MaxBytesReader(w, r.Body, 1_048_576)

	decoder := json.NewDecoder(r.Body)

	// GO silently ignores unknown fields this forces it to throw an error.
	decoder.DisallowUnknownFields()

	err := decoder.Decode(dst)
	if err != nil {
		return err
	}

	return nil
}
