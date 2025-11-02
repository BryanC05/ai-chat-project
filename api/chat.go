// File: /api/chat.go
package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5"
)

type ChatRequest struct{ Message string `json:"message"` }
type ChatResponse struct{ Reply string `json:"reply"` }

func Handler(w http.ResponseWriter, r *http.Request) {
	// 1. Setup CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 2. Get DATABASE_URL (we know this works)
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Println("ERROR: DATABASE_URL env var is NOT SET")
		http.Error(w, "DATABASE_URL env var is NOT SET", http.StatusInternalServerError)
		return
	}
	log.Println("SUCCESS: Found DATABASE_URL")

	// 3. --- THIS IS THE TEST ---
	// Try to actually connect to the database
	log.Println("Attempting to connect to database...")
	db, err := pgx.Connect(context.Background(), dbURL)
	if err != nil {
		// If it fails, log the error and crash
		log.Printf("ERROR: Unable to connect to database: %v\n", err)
		http.Error(w, "Unable to connect to database", http.StatusInternalServerError)
		return
	}
	defer db.Close(context.Background())
	log.Println("SUCCESS: Connected to database!")

	// 4. Try to ping the database
	err = db.Ping(context.Background())
	if err != nil {
		log.Printf("ERROR: Failed to ping database: %v\n", err)
		http.Error(w, "Failed to ping database", http.StatusInternalServerError)
		return
	}
	log.Println("SUCCESS: Pinged database!")

	// 5. Send a success message
	resp := ChatResponse{
		Reply: "Test 3 Successful: Connected and pinged the database!",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}