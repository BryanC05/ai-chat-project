package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

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

	// 2. Get Gemini API Key
	geminiKey := os.Getenv("GEMINI_API_KEY")
	if geminiKey == "" {
		log.Println("ERROR: GEMINI_API_KEY env var is NOT SET")
		http.Error(w, "GEMINI_API_KEY env var is NOT SET", http.StatusInternalServerError)
		return
	}

	// 3. Parse the user's message
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 4. Go API -> Gemini (New Task!)
	prompt := fmt.Sprintf(
		"You are an expert translator. Translate the following text into Indonesian: '%s'.",
		req.Message,
	)
	aiReply, err := callGemini(prompt, geminiKey)
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
func callGemini(prompt string, apiKey string) (string, error) {
	// We use the gemini-pro model here
	apiURL := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=" + apiKey

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