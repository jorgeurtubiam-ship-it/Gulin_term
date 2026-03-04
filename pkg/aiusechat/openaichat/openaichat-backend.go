// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package openaichat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/launchdarkly/eventsource"
	"github.com/wavetermdev/waveterm/pkg/aiusechat/chatstore"
	"github.com/wavetermdev/waveterm/pkg/aiusechat/uctypes"
	"github.com/wavetermdev/waveterm/pkg/web/sse"
)

// RunChatStep executes a chat step using the chat completions API
func RunChatStep(
	ctx context.Context,
	sseHandler *sse.SSEHandlerCh,
	chatOpts uctypes.WaveChatOpts,
	cont *uctypes.WaveContinueResponse,
) (*uctypes.WaveStopReason, []*StoredChatMessage, *uctypes.RateLimitInfo, error) {
	if sseHandler == nil {
		return nil, nil, nil, errors.New("sse handler is nil")
	}

	chat := chatstore.DefaultChatStore.Get(chatOpts.ChatId)
	if chat == nil {
		return nil, nil, nil, fmt.Errorf("chat not found: %s", chatOpts.ChatId)
	}

	if chatOpts.Config.TimeoutMs > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(chatOpts.Config.TimeoutMs)*time.Millisecond)
		defer cancel()
	}

	// Convert stored messages to chat completions format
	var messages []ChatRequestMessage

	// Convert native messages
	for _, genMsg := range chat.NativeMessages {
		var chatMsg *StoredChatMessage
		var ok bool
		if chatMsg, ok = genMsg.(*StoredChatMessage); !ok {
			if aiMsg, ok := genMsg.(*uctypes.AIMessage); ok {
				var err error
				chatMsg, err = ConvertAIMessageToStoredChatMessage(*aiMsg)
				if err != nil {
					return nil, nil, nil, fmt.Errorf("failed to convert fallback AIMessage: %w", err)
				}
			} else {
				return nil, nil, nil, fmt.Errorf("expected StoredChatMessage or *uctypes.AIMessage, got %T", genMsg)
			}
		}
		messages = append(messages, *chatMsg.Message.clean())
	}

	req, err := buildChatHTTPRequest(ctx, messages, chatOpts)
	if err != nil {
		return nil, nil, nil, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, nil, nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Setup SSE if this is a new request (not a continuation)
	if cont == nil {
		if err := sseHandler.SetupSSE(); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to setup SSE: %w", err)
		}
	}

	// Stream processing
	stopReason, assistantMsg, err := processChatStream(ctx, resp.Body, sseHandler, chatOpts, cont)
	if err != nil {
		return nil, nil, nil, err
	}

	return stopReason, []*StoredChatMessage{assistantMsg}, nil, nil
}

func processChatStream(
	ctx context.Context,
	body io.Reader,
	sseHandler *sse.SSEHandlerCh,
	chatOpts uctypes.WaveChatOpts,
	cont *uctypes.WaveContinueResponse,
) (*uctypes.WaveStopReason, *StoredChatMessage, error) {
	decoder := eventsource.NewDecoder(body)
	var textBuilder strings.Builder
	msgID := uuid.New().String()
	textID := uuid.New().String()
	var finishReason string
	textStarted := false
	var toolCallsInProgress []ToolCall

	if cont == nil {
		_ = sseHandler.AiMsgStart(msgID)
	}
	_ = sseHandler.AiMsgStartStep()

	for {
		if err := ctx.Err(); err != nil {
			_ = sseHandler.AiMsgError("request cancelled")
			return &uctypes.WaveStopReason{
				Kind:      uctypes.StopKindCanceled,
				ErrorType: "cancelled",
				ErrorText: "request cancelled",
			}, nil, err
		}

		event, err := decoder.Decode()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			if sseHandler.Err() != nil {
				partialMsg := extractPartialTextMessage(msgID, textBuilder.String())
				return &uctypes.WaveStopReason{
					Kind:      uctypes.StopKindCanceled,
					ErrorType: "client_disconnect",
					ErrorText: "client disconnected",
				}, partialMsg, nil
			}
			_ = sseHandler.AiMsgError(err.Error())
			return &uctypes.WaveStopReason{
				Kind:      uctypes.StopKindError,
				ErrorType: "stream",
				ErrorText: err.Error(),
			}, nil, fmt.Errorf("stream decode error: %w", err)
		}

		data := event.Data()
		if data == "[DONE]" {
			break
		}

		var chunk StreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			log.Printf("openaichat: failed to parse chunk: %v\n", err)
			continue
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]
		if choice.Delta.Content != "" {
			if !textStarted {
				_ = sseHandler.AiMsgTextStart(textID)
				textStarted = true
			}
			textBuilder.WriteString(choice.Delta.Content)
			_ = sseHandler.AiMsgTextDelta(textID, choice.Delta.Content)
		}

		if len(choice.Delta.ToolCalls) > 0 {
			for _, tcDelta := range choice.Delta.ToolCalls {
				idx := tcDelta.Index
				for len(toolCallsInProgress) <= idx {
					toolCallsInProgress = append(toolCallsInProgress, ToolCall{Type: "function"})
				}

				tc := &toolCallsInProgress[idx]
				if tcDelta.ID != "" {
					tc.ID = tcDelta.ID
				}
				if tcDelta.Type != "" {
					tc.Type = tcDelta.Type
				}
				if tcDelta.Function != nil {
					if tcDelta.Function.Name != "" {
						tc.Function.Name = tcDelta.Function.Name
					}
					if tcDelta.Function.Arguments != "" {
						tc.Function.Arguments += tcDelta.Function.Arguments
					}
				}
			}
		}

		if choice.FinishReason != nil && *choice.FinishReason != "" {
			finishReason = *choice.FinishReason
		}
	}

	stopKind := uctypes.StopKindDone
	if finishReason == "length" {
		stopKind = uctypes.StopKindMaxTokens
	} else if finishReason == "tool_calls" {
		stopKind = uctypes.StopKindToolUse
	}

	if len(toolCallsInProgress) == 0 {
		extractedCalls, cleanText := extractToolCallsFromText(textBuilder.String())
		if len(extractedCalls) > 0 {
			toolCallsInProgress = extractedCalls
			stopKind = uctypes.StopKindToolUse
			finishReason = "tool_calls"
			// Replace the builder content with the clean text
			textBuilder.Reset()
			textBuilder.WriteString(cleanText)
		}
	}

	var validToolCalls []ToolCall
	for _, tc := range toolCallsInProgress {
		if tc.ID != "" && tc.Function.Name != "" {
			validToolCalls = append(validToolCalls, tc)
		}
	}

	var waveToolCalls []uctypes.WaveToolCall
	if len(validToolCalls) > 0 {
		for _, tc := range validToolCalls {
			var inputJSON any
			if tc.Function.Arguments != "" {
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &inputJSON); err != nil {
					log.Printf("openaichat: failed to parse tool call arguments: %v\n", err)
					continue
				}
			}
			waveToolCalls = append(waveToolCalls, uctypes.WaveToolCall{
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: inputJSON,
			})
		}
	}

	stopReason := &uctypes.WaveStopReason{
		Kind:      stopKind,
		RawReason: finishReason,
		ToolCalls: waveToolCalls,
	}

	assistantMsg := &StoredChatMessage{
		MessageId: msgID,
		Message: ChatRequestMessage{
			Role: "assistant",
		},
	}

	assistantMsg.Message.Content = textBuilder.String()
	if len(validToolCalls) > 0 {
		assistantMsg.Message.ToolCalls = validToolCalls
	}

	if textStarted {
		_ = sseHandler.AiMsgTextEnd(textID)
	}
	_ = sseHandler.AiMsgFinishStep()
	if stopKind != uctypes.StopKindToolUse {
		_ = sseHandler.AiMsgFinish(finishReason, nil)
	}

	return stopReason, assistantMsg, nil
}

func extractPartialTextMessage(msgID string, text string) *StoredChatMessage {
	if text == "" {
		return nil
	}

	return &StoredChatMessage{
		MessageId: msgID,
		Message: ChatRequestMessage{
			Role:    "assistant",
			Content: text,
		},
	}
}

func extractToolCallsFromText(text string) ([]ToolCall, string) {
	var toolCalls []ToolCall
	cleanText := text
	extractedIndices := [][]int{}

	// 1. Try to find content inside ```json ... ``` blocks
	reMarkdown := regexp.MustCompile("(?s)```(?:json)?\n?(.*?)\n?```")
	markdownMatches := reMarkdown.FindAllStringSubmatchIndex(text, -1)
	for _, m := range markdownMatches {
		candidate := strings.TrimSpace(text[m[2]:m[3]])
		if tc := parseSingleToolCall(candidate); tc != nil {
			toolCalls = append(toolCalls, *tc)
			extractedIndices = append(extractedIndices, []int{m[0], m[1]})
		}
	}

	// 2. Look for raw JSON objects outside of markdown blocks if we haven't found everything
	// Use regex to find { "name": "...", "parameters": { ... } }
	reRaw := regexp.MustCompile(`(?s)\{\s*"name":\s*"([^"]+)"\s*,\s*"parameters":\s*(\{.*?\})\s*\}`)
	rawMatches := reRaw.FindAllStringSubmatchIndex(text, -1)
	for _, m := range rawMatches {
		// Check if this match overlaps with any already extracted markdown block
		overlaps := false
		for _, existing := range extractedIndices {
			if (m[0] >= existing[0] && m[0] < existing[1]) || (m[1] > existing[0] && m[1] <= existing[1]) {
				overlaps = true
				break
			}
		}
		if overlaps {
			continue
		}

		name := text[m[2]:m[3]]
		args := text[m[4]:m[5]]
		toolCalls = append(toolCalls, ToolCall{
			ID:   "call_" + uuid.New().String()[:8],
			Type: "function",
			Function: ToolFunctionCall{
				Name:      name,
				Arguments: args,
			},
		})
		extractedIndices = append(extractedIndices, []int{m[0], m[1]})
	}

	// Clean up the text by removing all extracted parts
	if len(extractedIndices) > 0 {
		// Sort indices from end to start to remove correctly
		slices.SortFunc(extractedIndices, func(a, b []int) int {
			return b[0] - a[0]
		})
		for _, idxPair := range extractedIndices {
			cleanText = cleanText[:idxPair[0]] + cleanText[idxPair[1]:]
		}
	}

	return toolCalls, strings.TrimSpace(cleanText)
}

func parseSingleToolCall(candidate string) *ToolCall {
	var simple struct {
		Name       string         `json:"name"`
		Parameters map[string]any `json:"parameters"`
	}
	if err := json.Unmarshal([]byte(candidate), &simple); err == nil && simple.Name != "" {
		args, _ := json.Marshal(simple.Parameters)
		return &ToolCall{
			ID:   "call_" + uuid.New().String()[:8],
			Type: "function",
			Function: ToolFunctionCall{
				Name:      simple.Name,
				Arguments: string(args),
			},
		}
	}
	return nil
}
