package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings" // <-- We now need this package

	// We no longer need "github.com/jackc/pgx/v5" or "time"
)

// --- Define the chat message structs ---
type ChatMessage struct {
	Sender string `json:"sender"`
	Text   string `json:"text"`
}
type ChatRequest struct {
	Messages []ChatMessage `json:"messages"`
}
type ChatResponse struct {
	Reply string `json:"reply"`
}

// --- Structs for Gemini API (same as before) ---
type GeminiRequest struct {
	Contents []GeminiContent `json:"contents"`
}
type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
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
	// 1. Setup CORS (same as before)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 2. Get Gemini API Key (we still need this)
	geminiKey := os.Getenv("GEMINI_API_KEY")
	if geminiKey == "" {
		log.Println("ERROR: GEMINI_API_KEY env var is NOT SET")
		http.Error(w, "GEMINI_API_KEY env var is NOT SET", http.StatusInternalServerError)
		return
	}
	
	// --- DATABASE CODE IS REMOVED ---

	// 3. Parse the new request (an array of messages)
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("ERROR: Could not decode request body: %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 4. Go API -> Gemini (Build a prompt *with* history)
	var promptBuilder strings.Builder
	promptBuilder.WriteString("You are a helpful, general-purpose chatbot. Continue the conversation.\n")

	for _, msg := range req.Messages {
		if msg.Sender == "user" {
			promptBuilder.WriteString(fmt.Sprintf("User: %s\n", msg.Text))
		} else if msg.Sender == "bot" {
			promptBuilder.WriteString(fmt.Sprintf("Bot: %s\n", msg.Text))
		}
	}
	// This prompts the AI to generate the *next* bot message
	promptBuilder.WriteString("Bot: ") 

	aiReply, err := callGemini(promptBuilder.String(), geminiKey)
	if err != nil {
		log.Printf("ERROR: Failed to call Gemini: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 5. Go API -> User UI
	resp := ChatResponse{Reply: aiReply}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// --- callGemini function is exactly the same as before ---
func callGemini(prompt string, apiKey string) (string, error) {
	apiURL := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=" + apiKey

	reqBody := GeminiRequest{
		Contents: []GeminiContent{
			{
				Role:  "user",
				Parts: []GeminiPart{{Text: prompt}},
			},
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

	if len(geminiResp.Candidates) > 0 &&
		len(geminiResp.Candidates[0].Content.Parts) > 0 {
		return geminiResp.Candidates[0].Content.Parts[0].Text, nil
	}
	return "", fmt.Errorf("no reply from Gemini")
}