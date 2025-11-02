package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
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

// --- Structs for Gemini API ---
type GeminiRequest struct {
	Contents         []GeminiContent  `json:"contents"`
	GenerationConfig GenerationConfig `json:"generationConfig"`
}
type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}
type GeminiPart struct {
	Text string `json:"text"`
}
type GenerationConfig struct {
	Temperature     float32  `json:"temperature"`
	TopK            int      `json:"topK"`
	TopP            float32  `json:"topP"`
	MaxOutputTokens int      `json:"maxOutputTokens"`
	StopSequences   []string `json:"stopSequences"`
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

	// 2. Get Gemini API Key
	geminiKey := os.Getenv("GEMINI_API_KEY")
	if geminiKey == "" {
		log.Println("ERROR: GEMINI_API_KEY env var is NOT SET")
		http.Error(w, "GEMINI_API_KEY env var is NOT SET", http.StatusInternalServerError)
		return
	}

	// --- ALL DATABASE CODE IS REMOVED ---

	// 3. Parse the request (the array of messages)
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("ERROR: Could not read request body: %v\n", err)
		http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("Received request body: %s", string(bodyBytes))

	var req ChatRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		log.Printf("ERROR: Could not decode request body: %v, body: %s\n", err, string(bodyBytes))
		http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Validate messages
	log.Printf("Parsed request with %d messages", len(req.Messages))
	if len(req.Messages) == 0 {
		log.Printf("ERROR: No messages in request, raw body was: %s", string(bodyBytes))
		http.Error(w, "no messages provided", http.StatusBadRequest)
		return
	}

	// 4. Go API -> Gemini
	aiReply, err := callGemini(req.Messages, geminiKey)
	if err != nil {
		log.Printf("ERROR: Failed to call Gemini: %v\n", err)
		// Return error in JSON format so frontend can read it
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ChatResponse{Reply: fmt.Sprintf("Error: %v", err)})
		return
	}

	// 5. Go API -> User UI
	resp := ChatResponse{Reply: aiReply}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Helper function to call Google Gemini
func callGemini(messages []ChatMessage, apiKey string) (string, error) {
	apiURL := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=" + apiKey

	// Convert chat history to Gemini format
	var geminiContents []GeminiContent

	// Loop through the actual chat history from the UI and convert it to Gemini's format
	for _, msg := range messages {
		// Skip empty messages
		if msg.Text == "" {
			continue
		}

		var role string
		if msg.Sender == "user" {
			role = "user"
		} else {
			role = "model" // Map our "bot" sender to the "model" role
		}

		geminiContents = append(geminiContents, GeminiContent{
			Role:  role,
			Parts: []GeminiPart{{Text: msg.Text}},
		})
	}

	// Ensure we have at least one message (should be a user message)
	if len(geminiContents) == 0 {
		return "", fmt.Errorf("no valid messages to send")
	}

	// Ensure the last message is from user (required by Gemini)
	lastMsg := geminiContents[len(geminiContents)-1]
	if lastMsg.Role != "user" {
		return "", fmt.Errorf("last message must be from user")
	}

	reqBody := GeminiRequest{
		Contents: geminiContents, // Pass the full conversation history
		GenerationConfig: GenerationConfig{
			Temperature:     1.0,
			TopK:            40,
			TopP:            0.95,
			MaxOutputTokens: 8192,
			StopSequences:   []string{},
		},
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(reqBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request to Gemini API: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Gemini API error response: %s", string(bodyBytes))
		return "", fmt.Errorf("gemini API error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(bodyBytes, &geminiResp); err != nil {
		log.Printf("Failed to decode Gemini response: %v, body: %s", err, string(bodyBytes))
		return "", fmt.Errorf("failed to decode response: %v", err)
	}

	if len(geminiResp.Candidates) > 0 &&
		len(geminiResp.Candidates[0].Content.Parts) > 0 {
		return geminiResp.Candidates[0].Content.Parts[0].Text, nil
	}

	log.Printf("Empty or blocked response from Gemini: %+v", geminiResp)
	return "I'm sorry, I couldn't process that response.", nil
}
