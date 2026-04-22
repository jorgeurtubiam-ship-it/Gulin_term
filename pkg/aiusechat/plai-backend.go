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
		_, tabTools, _, err := chatOpts.TabStateGenerator() // Ignoramos tabState aquí
		if err == nil {
			chatOpts.Tools = tabTools
		}
	}

	// Ensamble asincrónico para el escudo de contexto
	var sysPart, statePart, historyPart strings.Builder

	// 1. Instrucciones Críticas (System Prompt)
	if len(chatOpts.SystemPrompt) > 0 {
		sysPart.WriteString("### INSTRUCCIONES:\n")
		sysPart.WriteString("REGLA DE CONTEXTO: Estás en modo REACTIVO. No tienes visión directa del terminal actual. Si necesitas analizar errores, ver el listado de archivos que se generó, o revisar qué hay en pantalla, DEBES usar la herramienta 'term_get_scrollback'. No asumas ni inventes resultados.\n")
		sysPart.WriteString(strings.Join(chatOpts.SystemPrompt, "\n"))
		sysPart.WriteString("\n\n")
	}

	// 2. Estado de la Terminal (DESACTIVADO el estado completo por modelo Reactivo, pero enviamos el ID)
	if chatOpts.TabState != "" {
		widgetId := findPrimaryWidgetId(chatOpts.TabState)
		if widgetId != "" {
			statePart.WriteString(fmt.Sprintf("### CONTEXTO ACTIVO:\nEl ID de tu widget de terminal actual es: %s\nUsa este ID para el parámetro 'widget_id' en tus herramientas de terminal.\n\n", widgetId))
		}
	}

	// 3. Historial (Max 10 mensajes, 400 chars)
	msgCount := len(chat.NativeMessages)
	startIdx := 0
	if msgCount > 10 {
		startIdx = msgCount - 10
	}

	if startIdx < msgCount {
		historyPart.WriteString("### RECIENTE:\n")
		for i := startIdx; i < msgCount; i++ {
			msg := chat.NativeMessages[i]
			content := msg.GetContent()
			limit := 400
			if msg.GetRole() == "tool" {
				limit = 4000 // Permitir más visibilidad para resultados de comandos
			}
			if len(content) > limit {
				content = content[:limit] + "...[recortado]"
			}
			role := "U"
			if msg.GetRole() == "assistant" {
				role = "A"
			} else if msg.GetRole() == "tool" {
				role = "U" // FINGIR QUE EL USUARIO ENVIÓ EL RESULTADO para obligar a la IA a leerlo
				content = "[SISTEMA - RESULTADO DE HERRAMIENTA. OBLIGATORIO ANALIZAR Y DAR RESUMEN AL USUARIO]:\n" + content
			}
			historyPart.WriteString(fmt.Sprintf("%s: %s\n", role, content))
		}
		historyPart.WriteString("\n")
	}

	// 4. Herramientas y Acción (Restaurando Schemas técnicos)
	var toolsToUse []uctypes.ToolDefinition
	if len(chatOpts.Tools) > 0 {
		toolsToUse = chatOpts.Tools
	} else if chatOpts.TabStateGenerator != nil {
		_, tabTools, _, _ := chatOpts.TabStateGenerator()
		toolsToUse = tabTools
	}

	var toolsSection strings.Builder
	if len(toolsToUse) > 0 {
		toolsSection.WriteString("### HERRAMIENTAS DISPONIBLES:\nPara ejecutar una herramienta, DEBES usar un bloque ```json con EXACTAMENTE esta estructura:\n```json\n{\n  \"name\": \"nombre_herramienta\",\n  \"parameters\": { ... }\n}\n```\n\nHerramientas:\n")
		for _, tool := range toolsToUse {
			// Sin filtro estricto: ahora pasamos todas las herramientas permitidas por el experto
			// para que tenga sus capacidades completas, ya que la API soporta +20KB.
			
			schemaBytes, _ := json.Marshal(tool.InputSchema)
			toolsSection.WriteString(fmt.Sprintf("- %s: %s. Esquema de 'parameters': %s\n", tool.Name, tool.Description, string(schemaBytes)))
		}
		toolsSection.WriteString("\nREGLA 1: Si la tarea requiere comandos, usa el bloque ```json o un bloque ```bash directo. NUNCA pidas al usuario que lo ejecute.\nREGLA 2: Si en el RECIENTE ves un resultado de herramienta (R: ...), DEBES obligatoriamente leerlo y responder con un resumen en español claro. No te quedes callado.\n\n")
	}

	// --- ENSAMBLAJE FINAL CON ESCUDO DE CONTEXTO ---
	finalInput := ""
	const MaxSafeSize = 18000 // Aumentado a 18k ya que la prueba empírica demostró que la API lo soporta bien
	
	systemSection := sysPart.String()
	stateSection := statePart.String()
	historySection := historyPart.String()
	toolsText := toolsSection.String()

	totalSize := len(systemSection) + len(stateSection) + len(historySection) + len(toolsText)
	log.Printf("[PLAI-DEBUG] Tamaño pre-ensamblaje: %d bytes (System:%d, State:%d, History:%d, Tools:%d)\n", 
		totalSize, len(systemSection), len(stateSection), len(historySection), len(toolsText))

	if totalSize > MaxSafeSize {
		diff := totalSize - MaxSafeSize
		log.Printf("[PLAI-DEBUG] ESCUDO ACTIVADO: Recortando %d bytes...\n", diff)
		// 1. Recortar Historial (si es necesario)
		if diff > 0 && len(historySection) > 300 {
			toCut := diff
			if toCut > len(historySection)-300 {
				toCut = len(historySection) - 300
			}
			historySection = "...[recortado]\n" + historySection[toCut:]
			diff -= toCut
		}
		// 2. Recortar Sistema/Brain (si aún es necesario)
		if diff > 0 && len(systemSection) > 2000 {
			toCut := diff
			if toCut > len(systemSection)-2000 {
				toCut = len(systemSection) - 2000
			}
			systemSection = systemSection[:2000] + "...[instrucciones recortadas]\n\n"
			diff -= toCut
		}
		// 3. Recortar Herramientas (Último recurso)
		if diff > 0 && len(toolsText) > 5000 {
			toCut := diff
			if toCut > len(toolsText)-5000 {
				toCut = len(toolsText) - 5000
			}
			toolsText = toolsText[:len(toolsText)-toCut] + "...[tools recortadas]\n"
			diff -= toCut
		}
	}

	finalInput = systemSection + stateSection + historySection + toolsText
	log.Printf("[PLAI-DEBUG] Prompt Final Construido (%d bytes)\n", len(finalInput))
	log.Printf("[PLAI-DEBUG] URL: %s | AgentID: %s\n", chatOpts.Config.Endpoint, chatOpts.Config.AgentID)

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
		} else {
			// Fallback: Si el modelo olvidó el wrapper y mandó directamente los parámetros
			var params map[string]interface{}
			if err := json.Unmarshal([]byte(jsonStr), &params); err == nil {
				toolName := ""
				if _, hasCmd := params["command"]; hasCmd {
					toolName = "term_run_command"
				} else if _, hasCount := params["count"]; hasCount {
					toolName = "term_get_scrollback"
				} else if _, hasLineStart := params["line_start"]; hasLineStart {
					toolName = "term_get_scrollback"
				}

				if toolName != "" {
					log.Printf("[PLAI-DEBUG] Fallback: Detectada llamada implícita a herramienta JSON: %s\n", toolName)
					
					if _, hasWidget := params["widget_id"]; !hasWidget {
						widgetId := findPrimaryWidgetId(chatOpts.TabState)
						if widgetId != "" {
							params["widget_id"] = widgetId
						}
					}
					
					assistantMsg := &PlaiMessage{
						MessageId: apiMsgId,
						Role:      "assistant",
						Content:   content,
					}
					return createToolCall(toolName, params, sseHandler, assistantMsg)
				}
			}
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
	if len(toolResults) == 0 {
		return nil, nil
	}
	var rtn []uctypes.GenAIMessage
	for _, res := range toolResults {
		content := res.Text
		if res.ErrorText != "" {
			content = fmt.Sprintf("Error: %s", res.ErrorText)
		}
		msg := &PlaiMessage{
			MessageId: uuid.New().String(),
			Role:      "tool",
			Content:   content,
		}
		rtn = append(rtn, msg)
	}
	return rtn, nil
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
		if genMsg.GetRole() == "tool" {
			continue // Ocultar los resultados raw de las herramientas en la interfaz UI
		}
		
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
