package aiusechat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	OllamaHost           = "http://localhost:11434"
	OllamaEmbeddingModel = "nomic-embed-text"
	EndpointEmbeddings   = "/api/embeddings"
	ChunkSizeLimit       = 512 // Approximate token limit per chunk
)

// EmbeddingResponse represents the response from Ollama's embedding API
type EmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}

// EmbeddingRequest represents the payload to Ollama
type EmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// GetEmbedding queries the local Ollama instance to get a vector representation of the text
func GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	reqBody := EmbeddingRequest{
		Model:  OllamaEmbeddingModel,
		Prompt: text,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embedding request: %w", err)
	}

	url := fmt.Sprintf("%s%s", OllamaHost, EndpointEmbeddings)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call ollama API (is it running?): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var data EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to parse ollama response: %w", err)
	}

	if len(data.Embedding) == 0 {
		return nil, fmt.Errorf("ollama returned empty embedding")
	}

	return data.Embedding, nil
}

// ChunkText splits a long text document into smaller chunks suitable for embedding models
// This uses a simple character-based chunking strategy with a little overlap.
func ChunkText(text string, chunkSize int, overlap int) []string {
	var chunks []string
	textLen := len(text)

	if textLen == 0 {
		return chunks
	}

	if textLen <= chunkSize {
		chunks = append(chunks, text)
		return chunks
	}

	for i := 0; i < textLen; i += (chunkSize - overlap) {
		end := i + chunkSize
		if end > textLen {
			end = textLen
		}
		chunks = append(chunks, text[i:end])
		if end == textLen {
			break
		}
	}

	return chunks
}
