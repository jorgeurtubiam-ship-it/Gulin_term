// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package aiusechat

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gulindev/gulin/pkg/aiusechat/aiutil"
	"github.com/gulindev/gulin/pkg/aiusechat/chatstore"
	"github.com/gulindev/gulin/pkg/aiusechat/uctypes"
	"github.com/gulindev/gulin/pkg/secretstore"
	"github.com/gulindev/gulin/pkg/telemetry"
	"github.com/gulindev/gulin/pkg/telemetry/telemetrydata"
	"github.com/gulindev/gulin/pkg/util/ds"
	"github.com/gulindev/gulin/pkg/util/logutil"
	"github.com/gulindev/gulin/pkg/util/utilfn"
	"github.com/gulindev/gulin/pkg/gulinappstore"
	"github.com/gulindev/gulin/pkg/gulinbase"
	"github.com/gulindev/gulin/pkg/gulinobj"
	"github.com/gulindev/gulin/pkg/web/sse"
	"github.com/gulindev/gulin/pkg/wps"
	"github.com/gulindev/gulin/pkg/wshrpc"
	"github.com/gulindev/gulin/pkg/wshrpc/wshclient"
	"github.com/gulindev/gulin/pkg/wstore"
)

const DefaultAPI = uctypes.APIType_OpenAIResponses
const DefaultMaxTokens = 4 * 1024
const BuilderMaxTokens = 24 * 1024

var (
	globalRateLimitInfo = &uctypes.RateLimitInfo{Unknown: true}
	rateLimitLock       sync.Mutex

	activeChats = ds.MakeSyncMap[context.CancelFunc]() // key is chatid
)

func CancelActiveChat(chatId string) {
	if cancel, ok := activeChats.GetEx(chatId); ok {
		log.Printf("canceling active chat %s\n", chatId)
		cancel()
		activeChats.Delete(chatId)
	}
}

func getSystemPrompt(apiType string, model string, isBuilder bool, hasToolsCapability bool, widgetAccess bool, aiMode string) []string {
	if isBuilder {
		return []string{}
	}

	var prompts []string
	useNoToolsPrompt := !hasToolsCapability || !widgetAccess

	modelLower := strings.ToLower(model)
	isLiteModel := strings.Contains(modelLower, "lite") || strings.Contains(modelLower, "flash") || strings.Contains(modelLower, "mini")

	// Verificar si es un modo de Agente Experto específico
	for expertID, expert := range Experts {
		if strings.Contains(aiMode, string(expertID)) {
			prompts = append(prompts, expert.SystemPrompt)
			goto finalize
		}
	}

	// Si es el Orquestador y NO es un modelo Lite, usamos el prompt de Comandante
	if strings.Contains(aiMode, "@orchestrate") && !isLiteModel {
		prompts = append(prompts, SystemPrompt_Orchestrator)
	} else {
		basePrompt := SystemPromptText_OpenAI
		if useNoToolsPrompt {
			basePrompt = SystemPromptText_NoTools
		}
		prompts = append(prompts, basePrompt)

		if !useNoToolsPrompt {
			if strings.HasSuffix(aiMode, "@plan") {
				prompts = append(prompts, SystemPrompt_Plan)
			} else if strings.HasSuffix(aiMode, "@act") {
				prompts = append(prompts, SystemPrompt_Act)
			}
		}
	}

finalize:
	// Los modelos Lite (Gemini Flash, GPT-4o-mini) en el Bridge se confunden con el Strict AddOn.
	// Solo lo usaremos para modelos locales conocidos por ser difíciles.
	needsStrictToolAddOn, _ := regexp.MatchString(`(?i)\b(mistral|o?llama|qwen|mixtral|yi|phi|deepseek)\b`, modelLower)
	if needsStrictToolAddOn && !useNoToolsPrompt {
		prompts = append(prompts, SystemPromptText_StrictToolAddOn)
	}
	return prompts
}

func isLocalEndpoint(endpoint string) bool {
	if endpoint == "" {
		return false
	}
	endpointLower := strings.ToLower(endpoint)
	return strings.Contains(endpointLower, "localhost") || strings.Contains(endpointLower, "127.0.0.1")
}

func getGulinAISettings(premium bool, builderMode bool, rtInfo gulinobj.ObjRTInfo, aiModeName string) (*uctypes.AIOptsType, error) {
	maxTokens := DefaultMaxTokens
	if builderMode {
		maxTokens = BuilderMaxTokens
	}
	if rtInfo.GulinAIMaxOutputTokens > 0 {
		maxTokens = rtInfo.GulinAIMaxOutputTokens
	}
	aiMode, config, err := resolveAIMode(aiModeName, premium)
	if err != nil {
		return nil, err
	}
	if config.GulinAICloud && !telemetry.IsTelemetryEnabled() {
		return nil, fmt.Errorf("Gulin AI cloud modes require telemetry to be enabled")
	}
	apiToken := config.APIToken
	if apiToken == "" && config.APITokenSecretName != "" {
		secret, exists, err := secretstore.GetSecret(config.APITokenSecretName)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve secret %s: %w", config.APITokenSecretName, err)
		}
		secret = strings.TrimSpace(secret)
		if !exists || secret == "" {
			return nil, fmt.Errorf("secret %s not found or empty", config.APITokenSecretName)
		}
		apiToken = secret
	}

	var baseUrl string
	if config.Endpoint != "" {
		baseUrl = config.Endpoint
	} else {
		return nil, fmt.Errorf("no ai:endpoint configured for AI mode %s", aiMode)
	}

	thinkingLevel := config.ThinkingLevel
	if thinkingLevel == "" {
		thinkingLevel = uctypes.ThinkingLevelMedium
	}
	verbosity := config.Verbosity
	if verbosity == "" {
		verbosity = uctypes.VerbosityLevelMedium // default to medium
	}
	opts := &uctypes.AIOptsType{
		Provider:      config.Provider,
		APIType:       config.APIType,
		Model:         config.Model,
		MaxTokens:     maxTokens,
		ThinkingLevel: thinkingLevel,
		Verbosity:     verbosity,
		AIMode:        aiMode,
		Endpoint:      baseUrl,
		Capabilities:  config.Capabilities,
		GulinAIPremium: config.GulinAIPremium,
		BridgeProvider: config.BridgeProvider,
	}
	if apiToken != "" {
		opts.APIToken = apiToken
	}
	return opts, nil
}

func shouldUseChatCompletionsAPI(model string) bool {
	m := strings.ToLower(model)
	// Chat Completions API is required for older models: gpt-3.5-*, gpt-4, gpt-4-turbo, o1-*
	return strings.HasPrefix(m, "gpt-3.5") ||
		strings.HasPrefix(m, "gpt-4-") ||
		m == "gpt-4" ||
		strings.HasPrefix(m, "o1-")
}

func shouldUsePremium() bool {
	info := GetGlobalRateLimit()
	if info == nil || info.Unknown {
		return true
	}
	if info.PReq > 0 {
		return true
	}
	nowEpoch := time.Now().Unix()
	if nowEpoch >= info.ResetEpoch {
		return true
	}
	return false
}

func updateRateLimit(info *uctypes.RateLimitInfo) {
	if info == nil {
		return
	}
	rateLimitLock.Lock()
	defer rateLimitLock.Unlock()
	globalRateLimitInfo = info
	go func() {
		wps.Broker.Publish(wps.GulinEvent{
			Event: wps.Event_GulinAIRateLimit,
			Data:  info,
		})
	}()
}

func GetGlobalRateLimit() *uctypes.RateLimitInfo {
	rateLimitLock.Lock()
	defer rateLimitLock.Unlock()
	return globalRateLimitInfo
}

func runAIChatStep(ctx context.Context, sseHandler *sse.SSEHandlerCh, backend UseChatBackend, chatOpts uctypes.GulinChatOpts, cont *uctypes.GulinContinueResponse) (*uctypes.GulinStopReason, []uctypes.GenAIMessage, error) {
	if chatOpts.Config.APIType == uctypes.APIType_OpenAIResponses && shouldUseChatCompletionsAPI(chatOpts.Config.Model) {
		return nil, nil, fmt.Errorf("Chat completions API not available (must use newer OpenAI models)")
	}
	stopReason, messages, rateLimitInfo, err := backend.RunChatStep(ctx, sseHandler, chatOpts, cont)
	updateRateLimit(rateLimitInfo)
	return stopReason, messages, err
}

func getUsage(msgs []uctypes.GenAIMessage) uctypes.AIUsage {
	var rtn uctypes.AIUsage
	var found bool
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		if usage := msg.GetUsage(); usage != nil {
			if !found {
				rtn = *usage
				found = true
			} else {
				rtn.InputTokens += usage.InputTokens
				rtn.OutputTokens += usage.OutputTokens
				rtn.NativeWebSearchCount += usage.NativeWebSearchCount
			}
		}
	}
	return rtn
}

func GetChatUsage(chat *uctypes.AIChat) uctypes.AIUsage {
	usage := getUsage(chat.NativeMessages)
	usage.APIType = chat.APIType
	usage.Model = chat.Model
	return usage
}

func updateToolUseDataInChat(backend UseChatBackend, chatOpts uctypes.GulinChatOpts, toolCallID string, toolUseData uctypes.UIMessageDataToolUse) {
	if err := backend.UpdateToolUseData(chatOpts.ChatId, toolCallID, toolUseData); err != nil {
		log.Printf("failed to update tool use data in chat: %v\n", err)
	}
}

func processToolCallInternal(ctx context.Context, backend UseChatBackend, toolCall uctypes.GulinToolCall, chatOpts uctypes.GulinChatOpts, toolDef *uctypes.ToolDefinition, sseHandler *sse.SSEHandlerCh) uctypes.AIToolResult {
	if toolCall.ToolUseData == nil {
		return uctypes.AIToolResult{
			ToolName:  toolCall.Name,
			ToolUseID: toolCall.ID,
			ErrorText: "Invalid Tool Call",
		}
	}

	if toolCall.ToolUseData.Status == uctypes.ToolUseStatusError {
		errorMsg := toolCall.ToolUseData.ErrorMessage
		if errorMsg == "" {
			errorMsg = "Unspecified Tool Error"
		}
		return uctypes.AIToolResult{
			ToolName:  toolCall.Name,
			ToolUseID: toolCall.ID,
			ErrorText: errorMsg,
		}
	}

	if toolDef != nil && toolDef.ToolVerifyInput != nil {
		if err := toolDef.ToolVerifyInput(toolCall.Input, toolCall.ToolUseData); err != nil {
			errorMsg := fmt.Sprintf("Input validation failed: %v", err)
			toolCall.ToolUseData.Status = uctypes.ToolUseStatusError
			toolCall.ToolUseData.ErrorMessage = errorMsg
			return uctypes.AIToolResult{
				ToolName:  toolCall.Name,
				ToolUseID: toolCall.ID,
				ErrorText: errorMsg,
			}
		}
		// ToolVerifyInput can modify the toolusedata.  re-send it here.
		_ = sseHandler.AiMsgData("data-tooluse", toolCall.ID, *toolCall.ToolUseData)
		updateToolUseDataInChat(backend, chatOpts, toolCall.ID, *toolCall.ToolUseData)
	}

	if toolCall.ToolUseData.Approval == uctypes.ApprovalNeedsApproval {
		log.Printf("  waiting for approval...\n")
		approval, err := WaitForToolApproval(sseHandler.Context(), toolCall.ID)
		if err != nil || approval == "" {
			approval = uctypes.ApprovalCanceled
		}
		log.Printf("  approval result: %q\n", approval)
		toolCall.ToolUseData.Approval = approval

		if !toolCall.ToolUseData.IsApproved() {
			errorMsg := "Tool use denied or timed out"
			if approval == uctypes.ApprovalUserDenied {
				errorMsg = "Tool use denied by user"
			} else if approval == uctypes.ApprovalTimeout {
				errorMsg = "Tool approval timed out"
			} else if approval == uctypes.ApprovalCanceled {
				errorMsg = "Tool approval canceled"
			}
			toolCall.ToolUseData.Status = uctypes.ToolUseStatusError
			toolCall.ToolUseData.ErrorMessage = errorMsg
			return uctypes.AIToolResult{
				ToolName:  toolCall.Name,
				ToolUseID: toolCall.ID,
				ErrorText: errorMsg,
			}
		}

		// this still happens here because we need to update the FE to say the tool call was approved
		_ = sseHandler.AiMsgData("data-tooluse", toolCall.ID, *toolCall.ToolUseData)
		updateToolUseDataInChat(backend, chatOpts, toolCall.ID, *toolCall.ToolUseData)
	}

	toolCall.ToolUseData.RunTs = time.Now().UnixMilli()
	result := ResolveToolCall(ctx, toolDef, toolCall, chatOpts)

	if result.ErrorText != "" {
		toolCall.ToolUseData.Status = uctypes.ToolUseStatusError
		toolCall.ToolUseData.ErrorMessage = result.ErrorText
	} else {
		toolCall.ToolUseData.Status = uctypes.ToolUseStatusCompleted
	}

	return result
}

func processToolCall(ctx context.Context, backend UseChatBackend, toolCall uctypes.GulinToolCall, chatOpts uctypes.GulinChatOpts, sseHandler *sse.SSEHandlerCh, metrics *uctypes.AIMetrics) uctypes.AIToolResult {
	inputJSON, _ := json.Marshal(toolCall.Input)
	logutil.DevPrintf("TOOLUSE name=%s id=%s input=%s approval=%q\n", toolCall.Name, toolCall.ID, utilfn.TruncateString(string(inputJSON), 40), toolCall.ToolUseData.Approval)

	toolDef := chatOpts.GetToolDefinition(toolCall.Name)
	
	// Interceptar la llamada al experto para el Orquestador
	if toolCall.Name == "call_expert" {
		expertID, _ := toolCall.Input.(map[string]any)["expert_id"].(string)
		task, _ := toolCall.Input.(map[string]any)["task"].(string)
		log.Printf("ORCHESTRATOR delegando tarea a %s: %s\n", expertID, task)
		
		resultText, err := runExpertSubChat(ctx, backend, chatOpts, sseHandler, expertID, task)
		if err != nil {
			toolCall.ToolUseData.Status = uctypes.ToolUseStatusError
			toolCall.ToolUseData.ErrorMessage = fmt.Sprintf("error delegando al experto %s: %v", expertID, err)
			_ = sseHandler.AiMsgData("data-tooluse", toolCall.ID, *toolCall.ToolUseData)
			updateToolUseDataInChat(backend, chatOpts, toolCall.ID, *toolCall.ToolUseData)
			return uctypes.AIToolResult{
				ToolUseID: toolCall.ID,
				ToolName:  toolCall.Name,
				ErrorText: fmt.Sprintf("error delegando al experto %s: %v", expertID, err),
			}
		}
		toolCall.ToolUseData.Status = uctypes.ToolUseStatusCompleted
		_ = sseHandler.AiMsgData("data-tooluse", toolCall.ID, *toolCall.ToolUseData)
		updateToolUseDataInChat(backend, chatOpts, toolCall.ID, *toolCall.ToolUseData)
		return uctypes.AIToolResult{
			ToolUseID: toolCall.ID,
			ToolName:  toolCall.Name,
			Text:      resultText,
		}
	}

	result := processToolCallInternal(ctx, backend, toolCall, chatOpts, toolDef, sseHandler)

	if result.ErrorText != "" {
		log.Printf("  error=%s\n", result.ErrorText)
		metrics.ToolUseErrorCount++
	} else {
		log.Printf("  result=%s\n", utilfn.TruncateString(result.Text, 40))
	}

	if toolDef != nil && toolDef.ToolLogName != "" {
		metrics.ToolDetail[toolDef.ToolLogName]++
	}

	if toolCall.ToolUseData != nil {
		_ = sseHandler.AiMsgData("data-tooluse", toolCall.ID, *toolCall.ToolUseData)
		updateToolUseDataInChat(backend, chatOpts, toolCall.ID, *toolCall.ToolUseData)
	}

	return result
}

func processAllToolCalls(ctx context.Context, backend UseChatBackend, stopReason *uctypes.GulinStopReason, chatOpts uctypes.GulinChatOpts, sseHandler *sse.SSEHandlerCh, metrics *uctypes.AIMetrics) {
	// Create and send all data-tooluse packets at the beginning
	for i := range stopReason.ToolCalls {
		toolCall := &stopReason.ToolCalls[i]
		// Create toolUseData from the tool call input
		var argsJSON string
		if toolCall.Input != nil {
			argsBytes, err := json.Marshal(toolCall.Input)
			if err == nil {
				argsJSON = string(argsBytes)
			}
		}
		toolUseData := aiutil.CreateToolUseData(toolCall.ID, toolCall.Name, argsJSON, chatOpts)
		stopReason.ToolCalls[i].ToolUseData = &toolUseData
		log.Printf("AI data-tooluse %s\n", toolCall.ID)
		_ = sseHandler.AiMsgData("data-tooluse", toolCall.ID, toolUseData)
		updateToolUseDataInChat(backend, chatOpts, toolCall.ID, toolUseData)
		if toolUseData.Approval == uctypes.ApprovalNeedsApproval {
			RegisterToolApproval(toolCall.ID, sseHandler)
		}
	}
	// At this point, all ToolCalls are guaranteed to have non-nil ToolUseData

	var toolResults []uctypes.AIToolResult
	for _, toolCall := range stopReason.ToolCalls {
		if sseHandler.Err() != nil {
			log.Printf("AI tool processing stopped: %v\n", sseHandler.Err())
			break
		}
		result := processToolCall(ctx, backend, toolCall, chatOpts, sseHandler, metrics)
		toolResults = append(toolResults, result)
	}

	// Cleanup: unregister approvals, remove incomplete/canceled tool calls, and filter results
	var filteredResults []uctypes.AIToolResult
	for i, toolCall := range stopReason.ToolCalls {
		UnregisterToolApproval(toolCall.ID)
		hasResult := i < len(toolResults)
		shouldRemove := !hasResult || (toolCall.ToolUseData != nil && toolCall.ToolUseData.Approval == uctypes.ApprovalCanceled)
		if shouldRemove {
			backend.RemoveToolUseCall(chatOpts.ChatId, toolCall.ID)
		} else if hasResult {
			filteredResults = append(filteredResults, toolResults[i])
		}
	}

	if len(filteredResults) > 0 {
		toolResultMsgs, err := backend.ConvertToolResultsToNativeChatMessage(filteredResults)
		if err != nil {
			log.Printf("Failed to convert tool results to native chat messages: %v", err)
		} else {
			for _, msg := range toolResultMsgs {
				if err := chatstore.DefaultChatStore.PostMessage(chatOpts.ChatId, &chatOpts.Config, msg); err != nil {
					log.Printf("Failed to post tool result message: %v", err)
				}
			}
		}
	}
}

func RunAIChat(ctx context.Context, sseHandler *sse.SSEHandlerCh, backend UseChatBackend, chatOpts uctypes.GulinChatOpts) (*uctypes.AIMetrics, error) {
	chatCtx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	if !activeChats.SetUnless(chatOpts.ChatId, cancelFn) {
		return nil, fmt.Errorf("chat %s is already running", chatOpts.ChatId)
	}
	defer activeChats.Delete(chatOpts.ChatId)
	ctx = chatCtx

	stepNum := chatstore.DefaultChatStore.CountUserMessages(chatOpts.ChatId)
	aiProvider := chatOpts.Config.Provider
	if aiProvider == "" {
		aiProvider = uctypes.AIProvider_Custom
	}
	isLocal := isLocalEndpoint(chatOpts.Config.Endpoint)
	metrics := &uctypes.AIMetrics{
		ChatId:  chatOpts.ChatId,
		StepNum: stepNum,
		Usage: uctypes.AIUsage{
			APIType: chatOpts.Config.APIType,
			Model:   chatOpts.Config.Model,
		},
		WidgetAccess:  chatOpts.WidgetAccess,
		ToolDetail:    make(map[string]int),
		ThinkingLevel: chatOpts.Config.ThinkingLevel,
		AIMode:        chatOpts.Config.AIMode,
		AIProvider:    aiProvider,
		IsLocal:       isLocal,
	}
	firstStep := true
	var cont *uctypes.GulinContinueResponse
	if strings.Contains(chatOpts.Config.AIMode, "@orchestrate") {
		// Orchestrator optimization: only provide the delegation tool to the Orchestrator
		// to maintain precision and keep the prompt focused.
		var filteredTools []uctypes.ToolDefinition
		for _, tool := range chatOpts.Tools {
			if tool.Name == "call_expert" {
				filteredTools = append(filteredTools, tool)
			}
		}
		if len(filteredTools) > 0 {
			chatOpts.Tools = filteredTools
			chatOpts.TabTools = nil
		}
	}
	for {
		if chatOpts.TabStateGenerator != nil {
			tabState, tabTools, tabId, tabErr := chatOpts.TabStateGenerator()
			if tabErr == nil {
				chatOpts.TabState = tabState
				chatOpts.TabTools = tabTools
				chatOpts.TabId = tabId
			}
		}
		if chatOpts.BuilderAppGenerator != nil {
			appGoFile, appStaticFiles, platformInfo, appErr := chatOpts.BuilderAppGenerator()
			if appErr == nil {
				chatOpts.AppGoFile = appGoFile
				chatOpts.AppStaticFiles = appStaticFiles
				chatOpts.PlatformInfo = platformInfo
			}
		}
		stopReason, rtnMessages, err := runAIChatStep(ctx, sseHandler, backend, chatOpts, cont)
		metrics.RequestCount++
		if chatOpts.Config.IsGulinProxy() {
			metrics.ProxyReqCount++
			if chatOpts.Config.IsPremiumModel() {
				metrics.PremiumReqCount++
			}
		}
		if stopReason != nil {
			logutil.DevPrintf("stopreason: %s (%s) (%s) (%s)\n", stopReason.Kind, stopReason.ErrorText, stopReason.ErrorType, stopReason.RawReason)
		}
		if len(rtnMessages) > 0 {
			usage := getUsage(rtnMessages)
			log.Printf("usage: input=%d output=%d websearch=%d\n", usage.InputTokens, usage.OutputTokens, usage.NativeWebSearchCount)
			metrics.Usage.InputTokens += usage.InputTokens
			metrics.Usage.OutputTokens += usage.OutputTokens
			metrics.Usage.NativeWebSearchCount += usage.NativeWebSearchCount
			if usage.Model != "" && metrics.Usage.Model != usage.Model {
				metrics.Usage.Model = "mixed"
			}
		}
		if firstStep && err != nil {
			metrics.HadError = true
			return metrics, fmt.Errorf("failed to stream %s chat: %w", chatOpts.Config.APIType, err)
		}
		if err != nil {
			metrics.HadError = true
			_ = sseHandler.AiMsgFinish("")
			break
		}
		for _, msg := range rtnMessages {
			if msg != nil {
				if err := chatstore.DefaultChatStore.PostMessage(chatOpts.ChatId, &chatOpts.Config, msg); err != nil {
					log.Printf("Failed to post message: %v", err)
				}
			}
		}
		firstStep = false
		if stopReason != nil && stopReason.Kind == uctypes.StopKindPremiumRateLimit && chatOpts.Config.APIType == uctypes.APIType_OpenAIResponses && chatOpts.Config.Model == uctypes.PremiumOpenAIModel {
			log.Printf("Premium rate limit hit with %s, switching to %s\n", uctypes.PremiumOpenAIModel, uctypes.DefaultOpenAIModel)
			cont = &uctypes.GulinContinueResponse{
				Model:            uctypes.DefaultOpenAIModel,
				ContinueFromKind: uctypes.StopKindPremiumRateLimit,
			}
			continue
		}
		if stopReason != nil && stopReason.Kind == uctypes.StopKindToolUse {
			metrics.ToolUseCount += len(stopReason.ToolCalls)
			log.Printf("RunAIChat: processing %d tool calls...\n", len(stopReason.ToolCalls))
			processAllToolCalls(ctx, backend, stopReason, chatOpts, sseHandler, metrics)
			
			// SYNC FIX: Ensure the chat store has a moment to flush and that we are continuing from the right state
			log.Printf("RunAIChat: tool calls processed, continuing to next turn.\n")
			time.Sleep(100 * time.Millisecond)

			cont = &uctypes.GulinContinueResponse{
				Model:            chatOpts.Config.Model,
				ContinueFromKind: uctypes.StopKindToolUse,
			}
			continue
		}
		break
	}
	return metrics, nil
}

func ResolveToolCall(ctx context.Context, toolDef *uctypes.ToolDefinition, toolCall uctypes.GulinToolCall, chatOpts uctypes.GulinChatOpts) (result uctypes.AIToolResult) {
	result = uctypes.AIToolResult{
		ToolName:  toolCall.Name,
		ToolUseID: toolCall.ID,
	}

	defer func() {
		if r := recover(); r != nil {
			result.ErrorText = fmt.Sprintf("panic in tool execution: %v", r)
			result.Text = ""
		}
	}()

	if toolDef == nil {
		result.ErrorText = fmt.Sprintf("tool '%s' not found", toolCall.Name)
		return
	}

	// Try ToolTextCallback first, then ToolAnyCallback
	if toolDef.ToolTextCallback != nil {
		text, err := toolDef.ToolTextCallback(ctx, toolCall.Input)
		if err != nil {
			result.ErrorText = err.Error()
		} else {
			result.Text = text
			// Recompute tool description with the result
			if toolDef.ToolCallDesc != nil && toolCall.ToolUseData != nil {
				toolCall.ToolUseData.ToolDesc = toolDef.ToolCallDesc(toolCall.Input, text, toolCall.ToolUseData)
			}
		}
	} else if toolDef.ToolAnyCallback != nil {
		output, err := toolDef.ToolAnyCallback(ctx, toolCall.Input, toolCall.ToolUseData)
		if err != nil {
			result.ErrorText = err.Error()
		} else {
			// Marshal the result to JSON
			jsonBytes, marshalErr := json.Marshal(output)
			if marshalErr != nil {
				result.ErrorText = fmt.Sprintf("failed to marshal tool output: %v", marshalErr)
			} else {
				result.Text = string(jsonBytes)
				// Recompute tool description with the result
				if toolDef.ToolCallDesc != nil && toolCall.ToolUseData != nil {
					toolCall.ToolUseData.ToolDesc = toolDef.ToolCallDesc(toolCall.Input, output, toolCall.ToolUseData)
				}
			}
		}
	} else {
		result.ErrorText = fmt.Sprintf("tool '%s' has no callback functions", toolCall.Name)
	}

	return
}

func GulinAIPostMessageWrap(ctx context.Context, sseHandler *sse.SSEHandlerCh, message *uctypes.AIMessage, chatOpts uctypes.GulinChatOpts) error {
	startTime := time.Now()

	// Convert AIMessage to native chat message using backend
	backend, err := GetBackendByAPIType(chatOpts.Config.APIType)
	if err != nil {
		return err
	}
	convertedMessage, err := backend.ConvertAIMessageToNativeChatMessage(*message)
	if err != nil {
		return fmt.Errorf("message conversion failed: %w", err)
	}

	// Post message to chat store
	if err := chatstore.DefaultChatStore.PostMessage(chatOpts.ChatId, &chatOpts.Config, convertedMessage); err != nil {
		return fmt.Errorf("failed to store message: %w", err)
	}

	metrics, err := RunAIChat(ctx, sseHandler, backend, chatOpts)
	if metrics != nil {
		metrics.RequestDuration = int(time.Since(startTime).Milliseconds())
		for _, part := range message.Parts {
			if part.Type == uctypes.AIMessagePartTypeText {
				metrics.TextLen += len(part.Text)
			} else if part.Type == uctypes.AIMessagePartTypeFile {
				mimeType := strings.ToLower(part.MimeType)
				if strings.HasPrefix(mimeType, "image/") {
					metrics.ImageCount++
				} else if mimeType == "application/pdf" {
					metrics.PDFCount++
				} else {
					metrics.TextDocCount++
				}
			}
		}
		log.Printf("GulinAI call metrics: requests=%d tools=%d premium=%d proxy=%d images=%d pdfs=%d textdocs=%d textlen=%d duration=%dms error=%v\n",
			metrics.RequestCount, metrics.ToolUseCount, metrics.PremiumReqCount, metrics.ProxyReqCount,
			metrics.ImageCount, metrics.PDFCount, metrics.TextDocCount, metrics.TextLen, metrics.RequestDuration, metrics.HadError)

		sendAIMetricsTelemetry(ctx, metrics)
	}
	return err
}

func sendAIMetricsTelemetry(ctx context.Context, metrics *uctypes.AIMetrics) {
	event := telemetrydata.MakeTEvent("gulinai:post", telemetrydata.TEventProps{
		GulinAIAPIType:              metrics.Usage.APIType,
		GulinAIModel:                metrics.Usage.Model,
		GulinAIChatId:               metrics.ChatId,
		GulinAIStepNum:              metrics.StepNum,
		GulinAIInputTokens:          metrics.Usage.InputTokens,
		GulinAIOutputTokens:         metrics.Usage.OutputTokens,
		GulinAINativeWebSearchCount: metrics.Usage.NativeWebSearchCount,
		GulinAIRequestCount:         metrics.RequestCount,
		GulinAIToolUseCount:         metrics.ToolUseCount,
		GulinAIToolUseErrorCount:    metrics.ToolUseErrorCount,
		GulinAIToolDetail:           metrics.ToolDetail,
		GulinAIPremiumReq:           metrics.PremiumReqCount,
		GulinAIProxyReq:             metrics.ProxyReqCount,
		GulinAIHadError:             metrics.HadError,
		GulinAIImageCount:           metrics.ImageCount,
		GulinAIPDFCount:             metrics.PDFCount,
		GulinAITextDocCount:         metrics.TextDocCount,
		GulinAITextLen:              metrics.TextLen,
		GulinAIFirstByteMs:          metrics.FirstByteLatency,
		GulinAIRequestDurMs:         metrics.RequestDuration,
		GulinAIWidgetAccess:         metrics.WidgetAccess,
		GulinAIThinkingLevel:        metrics.ThinkingLevel,
		GulinAIMode:                 metrics.AIMode,
		GulinAIProvider:             metrics.AIProvider,
		GulinAIIsLocal:              metrics.IsLocal,
	})
	_ = telemetry.RecordTEvent(ctx, event)
}

// PostMessageRequest represents the request body for posting a message
type PostMessageRequest struct {
	TabId        string            `json:"tabid,omitempty"`
	BuilderId    string            `json:"builderid,omitempty"`
	BuilderAppId string            `json:"builderappid,omitempty"`
	ChatID       string            `json:"chatid"`
	Msg          uctypes.AIMessage `json:"msg"`
	WidgetAccess bool              `json:"widgetaccess,omitempty"`
	AIMode       string            `json:"aimode"`
}

type BrainSummary struct {
	Filename   string `json:"filename"`
	Title      string `json:"title"`
	LastUpdate int64  `json:"lastupdate"`
	Snippet    string `json:"snippet"`
}

func GulinAIBrainListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	files, err := ListGulinMemoryFiles()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list brain files: %v", err), http.StatusInternalServerError)
		return
	}

	summaries := make([]BrainSummary, 0)
	for _, file := range files {
		path := filepath.Join(GetGulinMemoryDir(), file)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		content, err := ReadGulinMemoryFile(file)
		if err != nil {
			continue
		}
		snippet := content
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		title := strings.TrimSuffix(file, ".md")
		title = strings.ReplaceAll(title, "_", " ")
		title = strings.Title(title)

		summaries = append(summaries, BrainSummary{
			Filename:   file,
			Title:      title,
			LastUpdate: info.ModTime().UnixMilli(),
			Snippet:    snippet,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summaries)
}

func GulinAIDBSchemaHandler(w http.ResponseWriter, r *http.Request) {
	connName := r.URL.Query().Get("connection")
	if connName == "" {
		http.Error(w, "connection parameter is required", http.StatusBadRequest)
		return
	}

	val, exists, _ := secretstore.GetSecret(DBConnectionsSecretKey)
	if !exists {
		http.Error(w, "no connections registered", http.StatusNotFound)
		return
	}
	connections := make(map[string]DBRegisterInput)
	json.Unmarshal([]byte(val), &connections)

	connInfo, ok := connections[connName]
	if !ok {
		http.Error(w, fmt.Sprintf("connection '%s' not found", connName), http.StatusNotFound)
		return
	}

	if connInfo.Type != "sqlite" {
		http.Error(w, "only 'sqlite' is supported currently", http.StatusBadRequest)
		return
	}

	db, err := sql.Open("sqlite3", connInfo.URL)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to open db: %v", err), http.StatusInternalServerError)
		return
	}
	defer db.Close()

	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to query schema: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			tables = append(tables, name)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tables)
}

func GulinAIDBQueryHandler(w http.ResponseWriter, r *http.Request) {
	connName := r.URL.Query().Get("connection")
	sqlStr := r.URL.Query().Get("sql")
	tabId := r.URL.Query().Get("tabid")

	if connName == "" || sqlStr == "" || tabId == "" {
		http.Error(w, "connection, sql, and tabid parameters are required", http.StatusBadRequest)
		return
	}

	val, exists, _ := secretstore.GetSecret(DBConnectionsSecretKey)
	if !exists {
		http.Error(w, "no connections registered", http.StatusNotFound)
		return
	}
	connections := make(map[string]DBRegisterInput)
	json.Unmarshal([]byte(val), &connections)

	connInfo, ok := connections[connName]
	if !ok {
		http.Error(w, fmt.Sprintf("connection '%s' not found", connName), http.StatusNotFound)
		return
	}

	if connInfo.Type != "sqlite" {
		http.Error(w, "only 'sqlite' is supported currently", http.StatusBadRequest)
		return
	}

	db, err := sql.Open("sqlite3", connInfo.URL)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to open db: %v", err), http.StatusInternalServerError)
		return
	}
	defer db.Close()

	rows, err := db.QueryContext(r.Context(), sqlStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to execute query: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	var results []map[string]any

	for rows.Next() {
		columns := make([]any, len(cols))
		columnPointers := make([]any, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}

		if err := rows.Scan(columnPointers...); err != nil {
			http.Error(w, fmt.Sprintf("failed to scan row: %v", err), http.StatusInternalServerError)
			return
		}

		m := make(map[string]any)
		for i, colName := range cols {
			val := columns[i]
			if b, ok := val.([]byte); ok {
				m[colName] = string(b)
			} else {
				m[colName] = val
			}
		}
		results = append(results, m)
	}

	// Create the block in the UI
	rpcClient := wshclient.GetBareRpcClient()
	dataJson, _ := json.Marshal(results)
	_, err = wshclient.CreateBlockCommand(rpcClient, wshrpc.CommandCreateBlockData{
		TabId: tabId,
		BlockDef: &gulinobj.BlockDef{
			Meta: map[string]any{
				"view":          "db-explorer",
				"db:title":      fmt.Sprintf("Table: %s", connName),
				"db:connection": connName,
				"db:data":       string(dataJson),
			},
		},
	}, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create block: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func GulinAIDBListHandler(w http.ResponseWriter, r *http.Request) {
	val, exists, _ := secretstore.GetSecret(DBConnectionsSecretKey)
	if !exists {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]any{})
		return
	}
	connections := make(map[string]DBRegisterInput)
	json.Unmarshal([]byte(val), &connections)

	var result []DBConnectionInfo
	for name, conn := range connections {
		result = append(result, DBConnectionInfo{
			Name: name,
			Type: conn.Type,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func GulinAIGetChatListHandler(w http.ResponseWriter, r *http.Request) {
	// Only allow GET method
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	summaries, err := chatstore.GetChatListFromDB()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get chat list: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summaries)
}

func GulinAIPostMessageHandler(w http.ResponseWriter, r *http.Request) {
	// Only allow POST method
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req PostMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Validate chatid is present and is a UUID
	if req.ChatID == "" {
		http.Error(w, "chatid is required in request body", http.StatusBadRequest)
		return
	}
	if _, err := uuid.Parse(req.ChatID); err != nil {
		http.Error(w, "chatid must be a valid UUID", http.StatusBadRequest)
		return
	}

	// Get RTInfo from TabId or BuilderId
	var rtInfo *gulinobj.ObjRTInfo
	if req.TabId != "" {
		oref := gulinobj.MakeORef(gulinobj.OType_Tab, req.TabId)
		rtInfo = wstore.GetRTInfo(oref)
	} else if req.BuilderId != "" {
		oref := gulinobj.MakeORef(gulinobj.OType_Builder, req.BuilderId)
		rtInfo = wstore.GetRTInfo(oref)
	}
	if rtInfo == nil {
		rtInfo = &gulinobj.ObjRTInfo{}
	}

	// Get GulinAI settings
	premium := shouldUsePremium()
	builderMode := req.BuilderId != ""
	if req.AIMode == "" {
		http.Error(w, "aimode is required in request body", http.StatusBadRequest)
		return
	}
	aiOpts, err := getGulinAISettings(premium, builderMode, *rtInfo, req.AIMode)
	if err != nil {
		http.Error(w, fmt.Sprintf("GulinAI configuration error: %v", err), http.StatusInternalServerError)
		return
	}

	// Call the core GulinAIPostMessage function
	chatOpts := uctypes.GulinChatOpts{
		ChatId:               req.ChatID,
		ClientId:             wstore.GetClientId(),
		Config:               *aiOpts,
		WidgetAccess:         req.WidgetAccess,
		AllowNativeWebSearch: true,
		BuilderId:            req.BuilderId,
		BuilderAppId:         req.BuilderAppId,
	}
	chatOpts.SystemPrompt = getSystemPrompt(chatOpts.Config.APIType, chatOpts.Config.Model, chatOpts.BuilderId != "", chatOpts.Config.HasCapability(uctypes.AICapabilityTools), chatOpts.WidgetAccess, chatOpts.Config.AIMode)
	brainContext := GetGulinBrainContext(req.Msg.GetContent())
	if brainContext != "" {
		chatOpts.SystemPrompt = append(chatOpts.SystemPrompt, brainContext)
	}

	if req.TabId != "" {
		chatOpts.TabStateGenerator = func() (string, []uctypes.ToolDefinition, string, error) {
			tabState, tabTools, err := GenerateTabStateAndTools(r.Context(), req.TabId, req.WidgetAccess, &chatOpts)
			return tabState, tabTools, req.TabId, err
		}
	}

	if req.BuilderAppId != "" {
		chatOpts.BuilderAppGenerator = func() (string, string, string, error) {
			return generateBuilderAppData(req.BuilderAppId)
		}
	}

	if req.BuilderAppId != "" {
		chatOpts.Tools = append(chatOpts.Tools,
			GetBuilderWriteAppFileToolDefinition(req.BuilderAppId, req.BuilderId),
			GetBuilderEditAppFileToolDefinition(req.BuilderAppId, req.BuilderId),
			GetBuilderListFilesToolDefinition(req.BuilderAppId),
		)
	}

	// Validate the message
	if err := req.Msg.Validate(); err != nil {
		http.Error(w, fmt.Sprintf("Message validation failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Handle Interruption
	lastUserMsg := chatstore.DefaultChatStore.GetLastUserMessage(req.ChatID)
	// Check if the chat is already active
	activeCancel, ok := activeChats.GetEx(req.ChatID)
	isInterruption := false
	if ok && activeCancel != nil {
		log.Printf("Interrupting active chat %s to merge context\n", req.ChatID)
		CancelActiveChat(req.ChatID)
		isInterruption = true

		if lastUserMsg != nil {
			// Merge the new message into the last user message
			// We assume req.Msg has text content
			newContent := req.Msg.GetContent()
			if newContent != "" {
				// Cast GenAIMessage to *uctypes.AIMessage to access its parts
				aiMsg, ok := lastUserMsg.(*uctypes.AIMessage)
				if ok {
					currentParts := aiMsg.Parts
					// Find first text part and append or add new part
					merged := false
					for i := range currentParts {
						if currentParts[i].Type == uctypes.AIMessagePartTypeText {
							currentParts[i].Text += "\n(Contexto adicional: " + newContent + ")"
							merged = true
							break
						}
					}
					if !merged {
						currentParts = append(currentParts, uctypes.AIMessagePart{
							Type: uctypes.AIMessagePartTypeText,
							Text: "\n(Contexto adicional: " + newContent + ")",
						})
					}
					aiMsg.Parts = currentParts

					// Save the updated message and trim everything after it
					_ = chatstore.DefaultChatStore.PostMessage(req.ChatID, aiOpts, aiMsg)
					chatstore.DefaultChatStore.TrimMessagesAfter(req.ChatID, aiMsg.GetMessageId())
				}
			}
		}
	}

	// Create SSE handler and set up streaming
	sseHandler := sse.MakeSSEHandlerCh(w, r.Context())
	defer sseHandler.Close()

	if isInterruption && lastUserMsg != nil {
		// Restart with merged message
		if err := RunAIChatWrap(r.Context(), sseHandler, chatOpts); err != nil {
			http.Error(w, fmt.Sprintf("Failed to restart chat: %v", err), http.StatusInternalServerError)
			return
		}
	} else {
		// Normal post
		if err := GulinAIPostMessageWrap(r.Context(), sseHandler, &req.Msg, chatOpts); err != nil {
			log.Printf("GulinAIPostMessageWrap failed with error: %v", err)
			http.Error(w, fmt.Sprintf("Failed to post message: %v", err), http.StatusInternalServerError)
			return
		}
	}
}

func RunAIChatWrap(ctx context.Context, sseHandler *sse.SSEHandlerCh, chatOpts uctypes.GulinChatOpts) error {
	backend, err := GetBackendByAPIType(chatOpts.Config.APIType)
	if err != nil {
		return err
	}
	metrics, err := RunAIChat(ctx, sseHandler, backend, chatOpts)
	if metrics != nil {
		sendAIMetricsTelemetry(ctx, metrics)
	}
	return err
}

func GulinAIGetChatHandler(w http.ResponseWriter, r *http.Request) {
	// Only allow GET method
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get chatid from URL parameters
	chatID := r.URL.Query().Get("chatid")
	if chatID == "" {
		http.Error(w, "chatid parameter is required", http.StatusBadRequest)
		return
	}

	// Validate chatid is a UUID
	if _, err := uuid.Parse(chatID); err != nil {
		http.Error(w, "chatid must be a valid UUID", http.StatusBadRequest)
		return
	}

	// Get chat from store
	chat := chatstore.DefaultChatStore.Get(chatID)
	if chat == nil {
		http.Error(w, "chat not found", http.StatusNotFound)
		return
	}

	// Set response headers for JSON
	w.Header().Set("Content-Type", "application/json")

	// Encode and return the chat
	if err := json.NewEncoder(w).Encode(chat); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
		return
	}
}

// CreateWriteTextFileDiff generates a diff for write_text_file or edit_text_file tool calls.
// Returns the original content, modified content, and any error.
// For Anthropic, this returns an unimplemented error.
func CreateWriteTextFileDiff(ctx context.Context, chatId string, toolCallId string) ([]byte, []byte, error) {
	aiChat := chatstore.DefaultChatStore.Get(chatId)
	if aiChat == nil {
		return nil, nil, fmt.Errorf("chat not found: %s", chatId)
	}

	backend, err := GetBackendByAPIType(aiChat.APIType)
	if err != nil {
		return nil, nil, err
	}

	funcCallInput := backend.GetFunctionCallInputByToolCallId(*aiChat, toolCallId)
	if funcCallInput == nil {
		return nil, nil, fmt.Errorf("tool call not found: %s", toolCallId)
	}

	toolName := funcCallInput.Name
	if toolName != "write_text_file" && toolName != "edit_text_file" {
		return nil, nil, fmt.Errorf("tool call %s is not a write_text_file or edit_text_file (got: %s)", toolCallId, toolName)
	}

	var backupFileName string
	if funcCallInput.ToolUseData != nil {
		backupFileName = funcCallInput.ToolUseData.WriteBackupFileName
	}

	var parsedArguments any
	if err := json.Unmarshal([]byte(funcCallInput.Arguments), &parsedArguments); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal arguments: %w", err)
	}

	if toolName == "edit_text_file" {
		originalContent, modifiedContent, err := EditTextFileDryRun(parsedArguments, backupFileName)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to generate diff: %w", err)
		}
		return originalContent, modifiedContent, nil
	}

	params, err := parseWriteTextFileInput(parsedArguments)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse write_text_file input: %w", err)
	}

	var originalContent []byte
	if backupFileName != "" {
		originalContent, err = os.ReadFile(backupFileName)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read backup file: %w", err)
		}
	} else {
		expandedPath, err := gulinbase.ExpandHomeDir(params.Filename)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to expand path: %w", err)
		}
		originalContent, err = os.ReadFile(expandedPath)
		if err != nil && !os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("failed to read original file: %w", err)
		}
	}

	modifiedContent := []byte(params.Contents)
	return originalContent, modifiedContent, nil
}

type StaticFileInfo struct {
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	Modified     string `json:"modified"`
	ModifiedTime string `json:"modified_time"`
}

func generateBuilderAppData(appId string) (string, string, string, error) {
	appGoFile := ""
	fileData, err := gulinappstore.ReadAppFile(appId, "app.go")
	if err == nil {
		appGoFile = string(fileData.Contents)
	}

	staticFilesJSON := ""
	allFiles, err := gulinappstore.ListAllAppFiles(appId)
	if err == nil {
		var staticFiles []StaticFileInfo
		for _, entry := range allFiles.Entries {
			if strings.HasPrefix(entry.Name, "static/") {
				staticFiles = append(staticFiles, StaticFileInfo{
					Name:         entry.Name,
					Size:         entry.Size,
					Modified:     entry.Modified,
					ModifiedTime: entry.ModifiedTime,
				})
			}
		}

		if len(staticFiles) > 0 {
			staticFilesBytes, marshalErr := json.Marshal(staticFiles)
			if marshalErr == nil {
				staticFilesJSON = string(staticFilesBytes)
			}
		}
	}

	platformInfo := gulinbase.GetSystemSummary()
	if currentUser, userErr := user.Current(); userErr == nil && currentUser.Username != "" {
		platformInfo = fmt.Sprintf("Local Machine: %s, User: %s", platformInfo, currentUser.Username)
	} else {
		platformInfo = fmt.Sprintf("Local Machine: %s", platformInfo)
	}

	return appGoFile, staticFilesJSON, platformInfo, nil
}

func runExpertSubChat(ctx context.Context, backend UseChatBackend, chatOpts uctypes.GulinChatOpts, sseHandler *sse.SSEHandlerCh, expertID string, task string) (string, error) {
	expert, ok := Experts[AgentExpertType(expertID)]
	if !ok {
		return "", fmt.Errorf("experto desconocido: %s", expertID)
	}

	// 1. Configurar el contexto del experto usando un SubChatId efímero para aislamiento total
	expertSubChatId := "expert-" + uuid.New().String()
	expertOpts := chatOpts
	expertOpts.ChatId = expertSubChatId
	expertOpts.Config.AIMode = string(expert.ID)
	// Forzamos el uso del modelo más eficiente y barato para el experto (mini)
	expertOpts.Config.Model = "gpt-4o-mini"
	if expert.DefaultModel != "" {
		expertOpts.Config.Model = expert.DefaultModel
	}

	// 2. Obtener herramientas filtradas para el experto y su prompt específico
	tabState, tabTools, err := GenerateTabStateAndTools(ctx, chatOpts.TabId, chatOpts.WidgetAccess, &expertOpts)
	if err != nil {
		return "", fmt.Errorf("error generando herramientas para experto: %v", err)
	}
	expertOpts.TabTools = tabTools
	expertOpts.TabState = tabState
	expertOpts.SystemPrompt = getSystemPrompt(expertOpts.Config.APIType, expertOpts.Config.Model, false, true, chatOpts.WidgetAccess, expertOpts.Config.AIMode)

	// 3. Crear y guardar el mensaje para el experto en la base aislada
	expertTaskMsg := fmt.Sprintf("TAREA ESPECÍFICA (REGLA CRÍTICA: NO EMULAR RESULTADOS, OBTIENELOS USANDO TUS HERRAMIENTAS): %s\n\nResponde solo con el resultado técnico final.", task)
	aiMessage := uctypes.AIMessage{
		MessageId: uuid.New().String(),
		Role:      "user",
		Parts: []uctypes.AIMessagePart{
			{
				Type: uctypes.AIMessagePartTypeText,
				Text: expertTaskMsg,
			},
		},
	}
	nativeMsg, err := backend.ConvertAIMessageToNativeChatMessage(aiMessage)
	if err != nil {
		return "", fmt.Errorf("error convirtiendo mensaje de experto: %v", err)
	}
	if err := chatstore.DefaultChatStore.PostMessage(expertOpts.ChatId, &expertOpts.Config, nativeMsg); err != nil {
		return "", fmt.Errorf("falló al guardar mensaje del experto: %v", err)
	}

	// 4. Bucle de Ejecución del Experto (Tool Execution Loop)
	log.Printf("[MAS] Delegando a %s (Modelo: %s, SubChat: %s) la tarea: %s\n", expert.ID, expertOpts.Config.Model, expertSubChatId, task)

	_ = sseHandler.AiMsgData("data-expert-status", expertID, map[string]string{
		"status": "running",
		"task":   task,
	})

	// Informar al usuario en el chat principal mediante un bloque de pensamiento (Reasoning)
	reasoningID := "expert-reasoning-" + uuid.New().String()[:8]
	_ = sseHandler.AiMsgReasoningStart(reasoningID)
	_ = sseHandler.AiMsgReasoningDelta(reasoningID, fmt.Sprintf("Delegando a %s...\n", expert.Name))

	metrics := &uctypes.AIMetrics{
		ChatId:  expertSubChatId,
		AIMode:  expertOpts.Config.AIMode,
		Usage: uctypes.AIUsage{
			APIType: expertOpts.Config.APIType,
			Model:   expertOpts.Config.Model,
		},
	}

	var resultText string
	var cont *uctypes.GulinContinueResponse

	for {
		stopReason, nativeMsgs, rateLimitInfo, err := backend.RunChatStep(ctx, sseHandler, expertOpts, cont)
		updateRateLimit(rateLimitInfo)
		metrics.RequestCount++
		
		if len(nativeMsgs) > 0 {
			usage := getUsage(nativeMsgs)
			metrics.Usage.InputTokens += usage.InputTokens
			metrics.Usage.OutputTokens += usage.OutputTokens
			metrics.Usage.NativeWebSearchCount += usage.NativeWebSearchCount
		}

		if err != nil {
			resultText = fmt.Sprintf("Error en experto: %v", err)
			break
		}

		// Enviar mensajes resultantes al sub-chat log y retransmitir pensamientos al chat principal
		for _, msg := range nativeMsgs {
			if msg != nil {
				if err := chatstore.DefaultChatStore.PostMessage(expertOpts.ChatId, &expertOpts.Config, msg); err != nil {
					log.Printf("Error guardando respuesta del experto: %v\n", err)
				}
				content := msg.GetContent()
				if content != "" {
					log.Printf("[PENSAMIENTO DE %s]: %s\n", expert.Name, content)
					// Retransmitir al bloque de razonamiento en la UI
					_ = sseHandler.AiMsgReasoningDelta(reasoningID, content+"\n")
				}
			}
		}

		// Si el experto decidió usar herramientas, procesarlas iterativamente
		if stopReason != nil && stopReason.Kind == uctypes.StopKindToolUse {
			_ = sseHandler.AiMsgData("data-expert-status", expertID, map[string]string{
				"status": "tool_use",
			})
			metrics.ToolUseCount += len(stopReason.ToolCalls)
			processAllToolCalls(ctx, backend, stopReason, expertOpts, sseHandler, metrics)
			cont = &uctypes.GulinContinueResponse{
				Model:            expertOpts.Config.Model,
				ContinueFromKind: uctypes.StopKindToolUse,
			}
			continue
		}

		// Si el flujo terminó limpiamente o por otro motivo final
		if len(nativeMsgs) > 0 {
			resultText = nativeMsgs[0].GetContent()
		} else if stopReason != nil && stopReason.Kind != uctypes.StopKindDone {
			resultText = fmt.Sprintf("Experto se detuvo con motivo: %s", stopReason.Kind)
		} else {
			resultText = "Experto completó la tarea sin respuesta textual."
		}
		break
	}

	// Cerrar el bloque de pensamiento en la UI
	_ = sseHandler.AiMsgReasoningEnd(reasoningID)

	// Notificar conclusión y reportar la telemetría del experto
	_ = sseHandler.AiMsgData("data-expert-status", expertID, map[string]string{
		"status": "completed",
	})
	
	sendAIMetricsTelemetry(ctx, metrics)

	return resultText, nil
}
