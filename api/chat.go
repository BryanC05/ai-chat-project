// File: /api/chat.go
package handler

import (
	"encoding/json"
	"log"
	"net/http"
)

type ChatRequest struct {
	Message string `json:"message"`
}

type ChatResponse struct {
	Reply string `json:"reply"`
}

// This is the new, simple handler
func Handler(w http.ResponseWriter, r *http.Request) {
	// 1. Setup CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 2. Parse the user's message
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("ERROR: Could not decode request: %v", err) // Log this
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 3. Log that we received it
	log.Printf("SUCCESS: Received message: %s", req.Message)

	// 4. Send a hardcoded reply
	resp := ChatResponse{
		Reply: "This is a hardcoded test reply from Go. The server is working!",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}