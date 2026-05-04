// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package aiusechat

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gulindev/gulin/pkg/gulinbase"
)

const GulinMemoryDirName = "gulin"
const EmbeddingsFileName = "embeddings.json"
const DefaultOllamaURL = "http://localhost:11434/api/embeddings"
const DefaultEmbeddingModel = "nomic-embed-text"

type GulinEmbeddings map[string][]float32

func GetGulinMemoryDir() string {
	return filepath.Join(gulinbase.GetGulinConfigDir(), GulinMemoryDirName)
}

func EnsureGulinMemoryDir() error {
	dir := GetGulinMemoryDir()
	return os.MkdirAll(dir, 0700)
}

func UpdateGulinMemoryFile(filename string, content string) error {
	if err := EnsureGulinMemoryDir(); err != nil {
		return err
	}
	// Sanitize filename to prevent directory traversal
	filename = filepath.Base(filename)
	if !strings.HasSuffix(filename, ".md") {
		filename += ".md"
	}
	path := filepath.Join(GetGulinMemoryDir(), filename)
	return os.WriteFile(path, []byte(content), 0600)
}

func ReadGulinMemoryFile(filename string) (string, error) {
	filename = filepath.Base(filename)
	if !strings.HasSuffix(filename, ".md") {
		filename += ".md"
	}
	path := filepath.Join(GetGulinMemoryDir(), filename)
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func ListGulinMemoryFiles() ([]string, error) {
	if err := EnsureGulinMemoryDir(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(GetGulinMemoryDir())
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}

func GetGulinSkillContext(skillName string) string {
	if skillName == "" {
		return ""
	}
	// Sanitize skill name to get filename (e.g. "🛡️ Seguridad" -> "seguridad.md")
	clean := strings.ToLower(skillName)
	// Remove emojis and spaces
	reg, _ := regexp.Compile("[^a-z0-9_]+")
	clean = strings.ReplaceAll(clean, " ", "_")
	clean = reg.ReplaceAllString(clean, "")
	clean = strings.Trim(clean, "_")

	content, err := ReadGulinMemoryFile(clean + ".md")
	if err != nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n<active_skill_protocol>\n")
	sb.WriteString(fmt.Sprintf("ESTÁS ACTUANDO COMO UN EXPERTO BAJO EL PROTOCOLO: %s\n", skillName))
	sb.WriteString("Sigue estrictamente las reglas definidas a continuación para esta conversación:\n\n")
	sb.WriteString(content)
	sb.WriteString("\n</active_skill_protocol>\n")
	return sb.String()
}

func GetGulinBrainContext(query string) string {
	files, err := ListGulinMemoryFiles()
	if err != nil || len(files) == 0 {
		return ""
	}

	var relevantFiles []string
	if query != "" {
		// Use semantic search to find top relevant files
		relevantFiles, _ = SearchGulinMemory(query)
	}

	// Falls back to showing a few files if no semantic results or small number of files
	if len(relevantFiles) == 0 {
		if len(files) > 5 {
			return "\n<gulin_brain_memory>\nTienes mucha información guardada. Usa la herramienta 'brain_search' para encontrar lo que necesites.\n</gulin_brain_memory>\n"
		}
		relevantFiles = files
	}

	var sb strings.Builder
	sb.WriteString("\n<gulin_brain_memory>\n")
	sb.WriteString("Esta es tu MEMORIA A LARGO PLAZO (RAG). Contiene hábitos, lecciones aprendidas y contexto importante recuperado automáticamente para esta conversación.\n")
	sb.WriteString("IMPORTANTE: Toda la información que necesitas sobre el usuario ya está aquí abajo. NO uses herramientas de búsqueda (brain_search) si la respuesta ya está en este bloque.\n")

	for _, file := range relevantFiles {
		content, err := ReadGulinMemoryFile(file)
		if err == nil {
			sb.WriteString(fmt.Sprintf("\n### Archivo: %s\n", file))
			sb.WriteString(content)
			sb.WriteString("\n")
		}
	}

	sb.WriteString("</gulin_brain_memory>\n")
	return sb.String()
}

func GetEmbeddings(text string) ([]float32, error) {
	reqBody := map[string]string{
		"model":  DefaultEmbeddingModel,
		"prompt": text,
	}
	jsonData, _ := json.Marshal(reqBody)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "POST", DefaultOllamaURL, strings.NewReader(string(jsonData)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Ollama no responde en %s. Asegúrate de que esté corriendo y el modelo '%s' esté instalado.", DefaultOllamaURL, DefaultEmbeddingModel)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama retornó error: %s", resp.Status)
	}

	var result struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Embedding, nil
}

func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return float32(dotProduct / (math.Sqrt(normA) * math.Sqrt(normB)))
}

func LoadEmbeddings() GulinEmbeddings {
	path := filepath.Join(GetGulinMemoryDir(), EmbeddingsFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return make(GulinEmbeddings)
	}
	var embs GulinEmbeddings
	json.Unmarshal(data, &embs)
	return embs
}

func SaveEmbeddings(embs GulinEmbeddings) error {
	path := filepath.Join(GetGulinMemoryDir(), EmbeddingsFileName)
	data, _ := json.MarshalIndent(embs, "", "  ")
	return os.WriteFile(path, data, 0600)
}

func IndexMemoryFiles() error {
	files, err := ListGulinMemoryFiles()
	if err != nil {
		return err
	}
	embs := LoadEmbeddings()
	updated := false

	for _, file := range files {
		if _, ok := embs[file]; !ok {
			content, err := ReadGulinMemoryFile(file)
			if err == nil {
				embedding, err := GetEmbeddings(content)
				if err == nil {
					embs[file] = embedding
					updated = true
				}
			}
		}
	}

	if updated {
		return SaveEmbeddings(embs)
	}
	return nil
}

func SearchGulinMemory(query string) ([]string, error) {
	queryEmb, err := GetEmbeddings(query)
	if err != nil {
		return nil, err
	}

	// Ensure everything is indexed
	IndexMemoryFiles()

	embs := LoadEmbeddings()
	type result struct {
		file  string
		score float32
	}
	var results []result

	for file, emb := range embs {
		score := CosineSimilarity(queryEmb, emb)
		if score > 0.6 { // Umbral de similitud
			results = append(results, result{file, score})
		}
	}

	// Sort by score
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].score > results[i].score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	var topFiles []string
	for i := 0; i < len(results) && i < 3; i++ {
		topFiles = append(topFiles, results[i].file)
	}
	return topFiles, nil
}
