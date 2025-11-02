// File: /api/chat.go
package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

type ChatRequest struct { Message string `json:"message"` }
type ChatResponse struct { Reply string `json:"reply"` }

func Handler(w http.ResponseWriter, r *http.Request) {
	// 1. Setup CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 2. Check for DATABASE_URL
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Println("ERROR: DATABASE_URL env var is NOT SET")
		http.Error(w, "DATABASE_URL env var is NOT SET", http.StatusInternalServerError)
		return
	}
	// Log that we found it, but mask the password
	log.Println("SUCCESS: Found DATABASE_URL (host: ...supabase.co)")

	// 3. Check for OPENAI_API_KEY
	openAIKey := os.Getenv("OPENAI_API_KEY")
	if openAIKey == "" {
		log.Println("ERROR: OPENAI_API_KEY env var is NOT SET")
		http.Error(w, "OPENAI_API_KEY env var is NOT SET", http.StatusInternalServerError)
		return
	}
	// Log that we found it, but mask the key
	log.Printf("SUCCESS: Found OPENAI_API_KEY (it starts with 'sk-...')")

	// 4. Send a success message
	resp := ChatResponse{
		Reply: "Test 2 Successful: Both API keys were found!",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}