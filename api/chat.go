package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io" // <-- Added this import
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
	GenerationConfig GenerationConfig `json:"generationConfig"` // Added for safety
}
type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}
type GeminiPart struct {
	Text string `json:"text"`
}
// Added GenerationConfig to prevent unsafe responses
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

	// 3. Parse the new request (an array of messages)
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("ERROR: Could not decode request body: %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 4. Go API -> Gemini (Pass the conversation history directly)
	aiReply, err := callGemini(req.Messages, geminiKey)
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

// Helper function to call Google Gemini
func callGemini(messages []ChatMessage, apiKey string) (string, error) {
	apiURL := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=" + apiKey

	// --- THIS IS THE NEW LOGIC ---
	// 1. Create the system instruction.
	// We'll add this to the start of the conversation.
	systemInstruction := GeminiContent{
		Role: "user", // System prompts are sent as the first "user" message
		Parts: []GeminiPart{{
			Text: "You are a helpful, general-purpose chatbot. Please continue the conversation. If you are asked for a recipe, provide it.",
		}},
	}
	
	// The bot's first "hello" (to set the context)
	modelHello := GeminiContent{
		Role: "model",
		Parts: []GeminiPart{{
			Text: "Hello! I'm ready to chat. How can I help you?",
		}},
	}

	// 2. Convert our chat history into Gemini's format
	var geminiContents []GeminiContent
	geminiContents = append(geminiContents, systemInstruction, modelHello) // Start with the system prompt and first reply

	// 3. Add the rest of the messages from the UI
	// We skip the first 2 messages from the UI, as we've already added our own.
	// This prevents the "hello" loop you saw.
	var history []ChatMessage
	if len(messages) > 2 {
		history = messages[2:] // Get all messages *after* the initial "hi" and "hello"
	}
	
	for _, msg := range history {
		var role string
		if msg.Sender == "user" {
			role = "user"
		} else {
			role = "model" // Map "bot" to "model"
		}

		geminiContents = append(geminiContents, GeminiContent{
			Role:  role,
			Parts: []GeminiPart{{Text: msg.Text}},
		})
	}
	// --- END NEW LOGIC ---


	reqBody := GeminiRequest{
		Contents: geminiContents, // Pass the full, correctly formatted conversation
		// Add default generation config for safety
		GenerationConfig: GenerationConfig{
			Temperature:     0.7,
			TopK:            1,
			TopP:            1.0,
			MaxOutputTokens: 2048,
			StopSequences:   []string{},
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
		bodyBytes, _ := io.ReadAll(resp.Body) // Read the error body
		return "", fmt.Errorf("Gemini API error (%d): %s", resp.StatusCode, string(bodyBytes))
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
	
	// Log the full empty response if no text is found
	log.Printf("Empty or blocked response from Gemini: %+v", geminiResp)
	return "I'm sorry, I couldn't process that response.", nil
}