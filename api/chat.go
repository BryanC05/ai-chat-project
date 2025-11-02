package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
)

// --- Structs for our API ---
type ChatRequest struct{ Message string `json:"message"` }
type ChatResponse struct{ Reply string `json:"reply"` }
type Order struct{ Status string; ETA time.Time }

// --- Structs for Gemini API ---
type GeminiRequest struct {
	Contents []GeminiContent `json:"contents"`
}
type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
}
type GeminiPart struct {
	Text string `json:"text"`
}
type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []GeminiPart `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}


func Handler(w http.ResponseWriter, r *http.Request) {
	// 1. Setup CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 2. Connect to Database
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Println("ERROR: DATABASE_URL env var is NOT SET")
		http.Error(w, "DATABASE_URL env var is NOT SET", http.StatusInternalServerError)
		return
	}
	db, err := pgx.Connect(context.Background(), dbURL)
	if err != nil {
		log.Printf("ERROR: Unable to connect to database: %v\n", err)
		http.Error(w, "Unable to connect to database", http.StatusInternalServerError)
		return
	}
	defer db.Close(context.Background())

	// 3. Get Gemini API Key
	geminiKey := os.Getenv("GEMINI_API_KEY")
	if geminiKey == "" {
		log.Println("ERROR: GEMINI_API_KEY env var is NOT SET")
		http.Error(w, "GEMINI_API_KEY env var is NOT SET", http.StatusInternalServerError)
		return
	}

	// 4. Parse the user's message
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("ERROR: Could not decode request body: %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 5. Go API -> Database
	orderID := "12345" // Hardcoded for demo
	var order Order
	err = db.QueryRow(context.Background(),
		"SELECT status, eta FROM orders WHERE id=$1", orderID).Scan(&order.Status, &order.ETA)
	if err != nil {
		log.Printf("ERROR: Could not find order in database: %v\n", err)
		http.Error(w, "Could not find order", http.StatusNotFound)
		return
	}

	// 6. Go API -> Gemini
	prompt := fmt.Sprintf(
		"You are a helpful customer service agent. The user asked: '%s'. "+
			"The real data for their order is: 'status: %s, eta: %s'. "+
			"Give them a friendly, 1-sentence answer.",
		req.Message,
		order.Status,
		order.ETA.Format("January 2"),
	)
	aiReply, err := callGemini(prompt, geminiKey)
	if err != nil {
		log.Printf("ERROR: Failed to call Gemini: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 7. Go API -> User UI
	resp := ChatResponse{Reply: aiReply}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Helper function to call Google Gemini
func callGemini(prompt string, apiKey string) (string, error) {
	// We use the gemini-pro model here
	apiURL := "https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent?key=" + apiKey

	reqBody := GeminiRequest{
		Contents: []GeminiContent{
			{Parts: []GeminiPart{{Text: prompt}}},
		},
	}
	reqBytes, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", apiURL, bytes.NewBuffer(reqBytes))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return "", fmt.Errorf("Gemini API error (%d): %v", resp.StatusCode, errResp)
	}

	var geminiResp GeminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return "", err
	}

	// Extract the text from the complex response
	if len(geminiResp.Candidates) > 0 &&
		len(geminiResp.Candidates[0].Content.Parts) > 0 {
		return geminiResp.Candidates[0].Content.Parts[0].Text, nil
	}
	return "", fmt.Errorf("no reply from Gemini")
}