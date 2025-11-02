package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log" // <--- ADDED THIS
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
)

// --- Structs (same as before) ---
type ChatRequest struct{ Message string `json:"message"` }
type ChatResponse struct{ Reply string `json:"reply"` }
type Order struct{ Status string; ETA time.Time }
type OpenAIRequest struct{ Model string `json:"model"; Messages []OpenAIMessage `json:"messages"` }
type OpenAIMessage struct{ Role string `json:"role"; Content string `json:"content"` }
type OpenAIResponse struct{ Choices []struct{ Message OpenAIMessage `json:"message"` } `json:"choices"` }

// --- This is the main function Vercel will run ---
func Handler(w http.ResponseWriter, r *http.Request) {
	// 1. Setup CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// --- Connect to Database ---
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// --- ADDED LOGGING ---
		log.Println("ERROR: DATABASE_URL env var is not set")
		http.Error(w, "DATABASE_URL env var is not set", http.StatusInternalServerError)
		return
	}
	db, err := pgx.Connect(context.Background(), dbURL)
	if err != nil {
		// --- ADDED LOGGING ---
		log.Printf("ERROR: Unable to connect to database: %v\n", err)
		http.Error(w, "Unable to connect to database", http.StatusInternalServerError)
		return
	}
	defer db.Close(context.Background())

	// --- Get OpenAI Key ---
	openAIKey := os.Getenv("OPENAI_API_KEY")
	if openAIKey == "" {
		// --- ADDED LOGGING ---
		log.Println("ERROR: OPENAI_API_KEY env var is not set")
		http.Error(w, "OPENAI_API_KEY env var is not set", http.StatusInternalServerError)
		return
	}

	// 2. Parse the user's message
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("ERROR: Could not decode request body: %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 3. Go API -> Database
	orderID := "12345" // Hardcoded for demo
	var order Order
	err = db.QueryRow(context.Background(),
		"SELECT status, eta FROM orders WHERE id=$1", orderID).Scan(&order.Status, &order.ETA)
	if err != nil {
		// --- ADDED LOGGING ---
		log.Printf("ERROR: Could not find order in database: %v\n", err)
		http.Error(w, "Could not find order", http.StatusNotFound)
		return
	}

	// 4. Go API -> OpenAI
	prompt := fmt.Sprintf(
		"You are a helpful customer service agent. The user asked: '%s'. "+
			"The real data for their order is: 'status: %s, eta: %s'. "+
			"Give them a friendly, 1-sentence answer.",
		req.Message,
		order.Status,
		order.ETA.Format("January 2"),
	)
	aiReply, err := callOpenAI(prompt, openAIKey)
	if err != nil {
		// --- ADDED LOGGING ---
		log.Printf("ERROR: Failed to call OpenAI: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 5. Go API -> User UI
	resp := ChatResponse{Reply: aiReply}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Helper function to call OpenAI (needs apiKey passed in)
func callOpenAI(prompt string, apiKey string) (string, error) {
	apiURL := "https://api.openai.com/v1/chat/completions"
	reqBody := OpenAIRequest{
		Model: "gpt-3.5-turbo",
		Messages: []OpenAIMessage{
			{Role: "user", Content: prompt},
		},
	}
	reqBytes, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", apiURL, bytes.NewBuffer(reqBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// --- ADDED ERROR CHECKING FOR AI ---
	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return "", fmt.Errorf("OpenAI API error (%d): %v", resp.StatusCode, errResp)
	}

	var openAIResp OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		return "", err
	}

	if len(openAIResp.Choices) > 0 {
		return openAIResp.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("no reply from AI")
}