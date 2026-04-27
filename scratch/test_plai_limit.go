package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type PlaiRequest struct {
	Input string `json:"input"`
}

func main() {
	endpoint := "https://plai-api-core.cencosud.ai/api/assistant"
	agentId := "69e44065f9b8bce2d1a4dda2"
	apiKey := "TX9LQsu18igdWZYXXVPD3qHqDzva60Oc5OSgcN3YUiZPB6fO7Y1Dhe7ZhXzxGEo2"

	if apiKey == "" {
		fmt.Println("ERROR: Debes configurar la variable de entorno PLAI_API_KEY")
		os.Exit(1)
	}

	fmt.Printf("--- INICIANDO TEST DE LÍMITE WAF (PLAI) ---\n")
	fmt.Printf("Endpoint: %s\n", endpoint)
	fmt.Printf("AgentID: %s\n\n", agentId)

	// Tamaños a probar (en KB)
	sizes := []int{10, 12, 14, 15, 16, 17, 18, 20, 24, 32}

	client := &http.Client{Timeout: 30 * time.Second}

	for _, kb := range sizes {
		sizeBytes := kb * 1024
		
		// Crear un input de tamaño específico
		input := "Test de tamaño " + strings.Repeat("a", sizeBytes)
		
		plaiReq := PlaiRequest{Input: input}
		reqBody, _ := json.Marshal(plaiReq)
		actualSize := len(reqBody)

		fmt.Printf("[%d KB] Enviando %d bytes... ", kb, actualSize)

		req, _ := http.NewRequest("POST", endpoint, bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("x-agent-id", agentId)

		start := time.Now()
		resp, err := client.Do(req)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			continue
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		
		if resp.StatusCode == 201 || resp.StatusCode == 200 {
			fmt.Printf("OK (%d) - %v\n", resp.StatusCode, duration)
		} else {
			fmt.Printf("FAIL (%d) - %v\n", resp.StatusCode, duration)
			if resp.StatusCode == 403 {
				fmt.Printf("   -> BLOQUEADO POR WAF\n")
			} else {
				fmt.Printf("   -> Respuesta: %s\n", string(body))
			}
		}
	}
	fmt.Println("\n--- TEST FINALIZADO ---")
}
