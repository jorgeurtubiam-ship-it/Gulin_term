// Copyright 2026, GuLiN Terminal
// SPDX-License-Identifier: Apache-2.0

package aiusechat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gulindev/gulin/pkg/aiusechat/chatstore"
	"github.com/gulindev/gulin/pkg/aiusechat/uctypes"
	"github.com/gulindev/gulin/pkg/secretstore"
	"github.com/gulindev/gulin/pkg/web/sse"
)

type plaiBackend struct{}

type PlaiRequest struct {
	Input string `json:"input"`
}

type PlaiMessage struct {
	MessageId string `json:"messageid"`
	Role      string `json:"role"`
	Content   string `json:"content"`
}

func (m *PlaiMessage) GetMessageId() string { return m.MessageId }
func (m *PlaiMessage) GetRole() string      { return m.Role }
func (m *PlaiMessage) GetUsage() *uctypes.AIUsage { return nil }
func (m *PlaiMessage) GetContent() string   { return m.Content }

func (b *plaiBackend) RunChatStep(
	ctx context.Context,
	sseHandler *sse.SSEHandlerCh,
	chatOpts uctypes.GulinChatOpts,
	cont *uctypes.GulinContinueResponse,
) (*uctypes.GulinStopReason, []uctypes.GenAIMessage, *uctypes.RateLimitInfo, error) {
	if sseHandler == nil {
		return nil, nil, nil, fmt.Errorf("sse handler is nil")
	}

	// Obtener el chat
	chat := chatstore.DefaultChatStore.Get(chatOpts.ChatId)
	if chat == nil {
		return nil, nil, nil, fmt.Errorf("chat not found: %s", chatOpts.ChatId)
	}

	// SOPORTE DINÁMICO DE HERRAMIENTAS: 
	// Si chatOpts.Tools está vacío, intentamos usar el generador de la pestaña para obtener herramientas de terminal reales.
	if len(chatOpts.Tools) == 0 && chatOpts.TabStateGenerator != nil {
		tabState, tabTools, _, err := chatOpts.TabStateGenerator()
		if err == nil {
			chatOpts.Tools = tabTools
			if chatOpts.TabState == "" {
				chatOpts.TabState = tabState
			}
		}
	}

	// Construir el input enriquecido (Estructura Limpia)
	var fullInput strings.Builder
	
	// 1. Instrucciones Críticas (System Prompt)
	if len(chatOpts.SystemPrompt) > 0 {
		fullInput.WriteString("### INSTRUCCIONES:\n")
		fullInput.WriteString(strings.Join(chatOpts.SystemPrompt, "\n"))
		fullInput.WriteString("\n\n")
	}

	// 2. Estado de la Terminal (Compacto)
	if chatOpts.TabState != "" {
		fullInput.WriteString("### ESTADO:\n")
		ts := chatOpts.TabState
		if len(ts) > 600 {
			ts = "...[recortado]..." + ts[len(ts)-600:]
		}
		fullInput.WriteString(ts)
		fullInput.WriteString("\n\n")
	}

	// 3. Historial (Ultra-Dieta: Max 3 mensajes, 200 chars)
	msgCount := len(chat.NativeMessages)
	startIdx := 0
	if msgCount > 3 {
		startIdx = msgCount - 3
	}

	if startIdx < msgCount {
		fullInput.WriteString("### RECIENTE:\n")
		for i := startIdx; i < msgCount; i++ {
			msg := chat.NativeMessages[i]
			content := msg.GetContent()
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			role := "U"
			if msg.GetRole() == "assistant" {
				role = "A"
			} else if msg.GetRole() == "tool" {
				role = "R"
			}
			fullInput.WriteString(fmt.Sprintf("%s: %s\n", role, content))
		}
		fullInput.WriteString("\n")
	}

	// 4. Herramientas y Acción (Restaurando Schemas técnicos)
	var toolsToUse []uctypes.ToolDefinition
	if len(chatOpts.Tools) > 0 {
		toolsToUse = chatOpts.Tools
	} else if chatOpts.TabStateGenerator != nil {
		_, tabTools, _, _ := chatOpts.TabStateGenerator()
		toolsToUse = tabTools
	}

	if len(toolsToUse) > 0 {
		fullInput.WriteString("### HERRAMIENTAS DISPONIBLES (USA ESTE FORMATO):\n")
		for _, tool := range toolsToUse {
			schemaBytes, _ := json.Marshal(tool.InputSchema)
			fullInput.WriteString(fmt.Sprintf("- %s: %s. Parámetros JSON: %s\n", tool.Name, tool.Description, string(schemaBytes)))
		}
		fullInput.WriteString("\nREGLA DE ORO: Responde SOLO con un bloque ```bash o ```json con el comando. NO hables.\n\n")
	}

	finalInput := fullInput.String()
	log.Printf("[PLAI-DEBUG] Prompt Final Construido (%d bytes)\n", len(finalInput))
	// log.Printf("[PLAI-DEBUG] Input completo enviado a API:\n%s\n", finalInput)

	// Preparar la petición
	plaiReq := PlaiRequest{
		Input: finalInput,
	}
	reqBody, err := json.Marshal(plaiReq)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to marshal plai request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", chatOpts.Config.Endpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create http request: %w", err)
	}

	// Configurar cabeceras
	req.Header.Set("Content-Type", "application/json")
	
	// Obtener el token (puede estar en la config directamente o en el secret store)
	token := chatOpts.Config.APIToken
	if token == "" && chatOpts.Config.APITokenSecretName != "" {
		s, exists, _ := secretstore.GetSecret(chatOpts.Config.APITokenSecretName)
		if exists {
			token = s
		}
	}
	if token != "" {
		req.Header.Set("x-api-key", token)
	}

	// Configurar x-agent-id
	if chatOpts.Config.AgentID != "" {
		req.Header.Set("x-agent-id", chatOpts.Config.AgentID)
	}

	// Ejecutar la petición
	client := &http.Client{Timeout: 60 * time.Second}
	
	// Notificar inicio en la UI
	msgId := uuid.New().String()
	if cont == nil {
		sseHandler.SetupSSE()
		sseHandler.AiMsgStart(msgId)
	}
	sseHandler.AiMsgStartStep()

	resp, err := client.Do(req)
	if err != nil {
		sseHandler.AiMsgError(fmt.Sprintf("Error de conexión: %v", err))
		return nil, nil, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		sseHandler.AiMsgError(fmt.Sprintf("Error al leer respuesta: %v", err))
		return nil, nil, nil, err
	}
	log.Printf("[PLAI-DEBUG] Respuesta raw de API (Status %d):\n%s\n", resp.StatusCode, string(respBody))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errText := fmt.Sprintf("API error (%d): %s", resp.StatusCode, string(respBody))
		sseHandler.AiMsgError(errText)
		return nil, nil, nil, fmt.Errorf(errText)
	}

	// Intentar parsear la respuesta
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		sseHandler.AiMsgError("Error al decodificar JSON de respuesta")
		return nil, nil, nil, err
	}

	// Buscar el contenido de texto en campos comunes
	content := ""
	for _, field := range []string{"output", "response", "text", "message", "result"} {
		if val, ok := result[field].(string); ok {
			content = val
			break
		}
	}

	// Si no se encuentra un campo de texto conocido, devolver todo el JSON formateado
	if content == "" {
		pretty, _ := json.MarshalIndent(result, "", "  ")
		content = string(pretty)
	}

	// Obtener el ID de mensaje real de la API si existe
	apiMsgId, _ := result["messageId"].(string)
	if apiMsgId == "" {
		apiMsgId = msgId // Usar el generado localmente como fallback
	}

	// --- DETECCIÓN DE HERRAMIENTAS (TOOL USE) ---
	// 1. Detección estándar vía JSON
	jsonRegex := regexp.MustCompile("(?s)```json\\s*(\\{.*?\\})\\s*```")
	match := jsonRegex.FindStringSubmatch(content)
	if match != nil {
		jsonStr := match[1]
		var toolReq struct {
			Name       string                 `json:"name"`
			Parameters map[string]interface{} `json:"parameters"`
		}
		if err := json.Unmarshal([]byte(jsonStr), &toolReq); err == nil && toolReq.Name != "" {
			log.Printf("[PLAI-DEBUG] Detectada llamada a herramienta JSON: %s\n", toolReq.Name)
			assistantMsg := &PlaiMessage{
				MessageId: apiMsgId,
				Role:      "assistant",
				Content:   content,
			}
			return createToolCall(toolReq.Name, toolReq.Parameters, sseHandler, assistantMsg)
		}
	}

	// 2. Detección inteligente de bloques BASH (para ejecución directa en terminal)
	bashRegex := regexp.MustCompile("(?s)```bash\\s*(.*?)\\s*```")
	bashMatch := bashRegex.FindStringSubmatch(content)
	if bashMatch != nil {
		command := strings.TrimSpace(bashMatch[1])
		if command != "" {
			log.Printf("[PLAI-DEBUG] Detectado bloque BASH. Buscando widget_id en contexto...\n")
			widgetId := findPrimaryWidgetId(chatOpts.TabState)
			params := map[string]interface{}{"command": command}
			if widgetId != "" {
				log.Printf("[PLAI-DEBUG] Usando widget_id encontrado: %s\n", widgetId)
				params["widget_id"] = widgetId
			}
			assistantMsg := &PlaiMessage{
				MessageId: apiMsgId,
				Role:      "assistant",
				Content:   content,
			}
			return createToolCall("term_run_command", params, sseHandler, assistantMsg)
		}
	}

	// Transmitir el resultado a la UI
	textId := uuid.New().String()
	sseHandler.AiMsgTextStart(textId)
	sseHandler.AiMsgTextDelta(textId, content)
	sseHandler.AiMsgTextEnd(textId)
	sseHandler.AiMsgFinishStep()

	// Crear el mensaje nativo para guardar en el historial
	assistantMsg := &PlaiMessage{
		MessageId: msgId,
		Role:      "assistant",
		Content:   content,
	}

	stopReason := &uctypes.GulinStopReason{
		Kind: uctypes.StopKindDone,
	}

	return stopReason, []uctypes.GenAIMessage{assistantMsg}, nil, nil
}

func createToolCall(name string, params map[string]interface{}, sseHandler *sse.SSEHandlerCh, assistantMsg uctypes.GenAIMessage) (*uctypes.GulinStopReason, []uctypes.GenAIMessage, *uctypes.RateLimitInfo, error) {
	toolCall := uctypes.GulinToolCall{
		ID:    uuid.New().String(),
		Name:  name,
		Input: params,
	}

	// Notificar en UI que se está llamando a una herramienta
	sseHandler.AiMsgData("data-tooluse", toolCall.ID, uctypes.UIMessageDataToolUse{
		ToolCallId: toolCall.ID,
		ToolName:   toolCall.Name,
		Status:     uctypes.ToolUseStatusPending,
	})

	stopReason := &uctypes.GulinStopReason{
		Kind:      uctypes.StopKindToolUse,
		ToolCalls: []uctypes.GulinToolCall{toolCall},
	}

	// Devolvemos el mensaje del asistente para que Gulin lo guarde en el historial
	return stopReason, []uctypes.GenAIMessage{assistantMsg}, nil, nil
}

// findPrimaryWidgetId intenta extraer el primer ID de terminal (8 hex chars) de la descripción de la pestaña
func findPrimaryWidgetId(tabState string) string {
	if tabState == "" {
		return ""
	}
	// Gulin formatea los widgets como: * (8chexid) local CLI terminal...
	re := regexp.MustCompile(`\* \(([a-f0-9]{8})\)`)
	match := re.FindStringSubmatch(tabState)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func (b *plaiBackend) UpdateToolUseData(chatId string, toolCallId string, toolUseData uctypes.UIMessageDataToolUse) error {
	return nil // No soportado por PLAI actualmente
}

func (b *plaiBackend) RemoveToolUseCall(chatId string, toolCallId string) error {
	return nil // No soportado por PLAI actualmente
}

func (b *plaiBackend) ConvertToolResultsToNativeChatMessage(toolResults []uctypes.AIToolResult) ([]uctypes.GenAIMessage, error) {
	return nil, nil // No soportado por PLAI actualmente
}

func (b *plaiBackend) ConvertAIMessageToNativeChatMessage(message uctypes.AIMessage) (uctypes.GenAIMessage, error) {
	return &PlaiMessage{
		MessageId: message.MessageId,
		Role:      message.Role,
		Content:   message.GetContent(),
	}, nil
}

func (b *plaiBackend) GetFunctionCallInputByToolCallId(aiChat uctypes.AIChat, toolCallId string) *uctypes.AIFunctionCallInput {
	return nil
}

func (b *plaiBackend) ConvertAIChatToUIChat(aiChat uctypes.AIChat) (*uctypes.UIChat, error) {
	uiChat := &uctypes.UIChat{
		ChatId:     aiChat.ChatId,
		APIType:    aiChat.APIType,
		Model:      aiChat.Model,
		APIVersion: aiChat.APIVersion,
		Messages:   make([]uctypes.UIMessage, 0),
	}

	for _, genMsg := range aiChat.NativeMessages {
		uiMsg := uctypes.UIMessage{
			ID:   genMsg.GetMessageId(),
			Role: genMsg.GetRole(),
			Parts: []uctypes.UIMessagePart{
				{
					Type:  "text",
					Text:  genMsg.GetContent(),
					State: "done",
				},
			},
		}
		uiChat.Messages = append(uiChat.Messages, uiMsg)
	}

	return uiChat, nil
}
