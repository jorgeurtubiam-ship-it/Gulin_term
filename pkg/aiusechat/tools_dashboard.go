// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package aiusechat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gulindev/gulin/pkg/aiusechat/uctypes"
	"github.com/gulindev/gulin/pkg/gulinobj"
	"github.com/gulindev/gulin/pkg/wshrpc"
	"github.com/gulindev/gulin/pkg/wshrpc/wshclient"
)

type TermCreateDashboardInput struct {
	Title string `json:"title"`
	Type  string `json:"type"` // "bar", "line", etc.
	Data  string `json:"data"` // JSON array string of the chart data
}

func parseCreateDashboardInput(input any) (*TermCreateDashboardInput, error) {
	result := &TermCreateDashboardInput{}

	if input == nil {
		return nil, fmt.Errorf("title, type and data are required")
	}

	inputBytes, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}

	if err := json.Unmarshal(inputBytes, result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal input: %w", err)
	}

	if result.Title == "" {
		result.Title = "AI Generated Dashboard"
	}
	if result.Type == "" {
		result.Type = "bar"
	}

	return result, nil
}

func GetCreateDashboardToolDefinition(tabId string) uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "term_create_dashboard",
		DisplayName: "Create Interactive Dashboard",
		Description: "Generate a beautiful interactive data dashboard widget based on statistical or performance data. Use this when the user asks to see metrics, tables, costs, or status in a graph format. Provide a JSON array with the data. CRITICAL: If the user asks for a dashboard on a general topic, DO NOT search the local file system. IMMEDIATELY generate hypothetical or real data from your pre-training knowledge and call this tool. ONLY search the file system if the user EXPLICITLY asks to read a specific local file or folder.",
		ToolLogName: "widget:createdashboard",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title": map[string]any{
					"type":        "string",
					"description": "The title of the dashboard widget (e.g. 'Server Load Analysis')",
				},
				"type": map[string]any{
					"type":        "string",
					"enum":        []string{"bar", "line", "grid"},
					"description": "Chart type. Default is 'bar'. Use 'grid' for tabular data like database results or excel-like spreadsheets.",
				},
				"data": map[string]any{
					"type":        "string",
					"description": "A JSON array string containing the data set. For charts: first key is X axis. For 'grid': all keys become columns. E.g. '[{\"ID\":1,\"User\":\"Admin\",\"Status\":\"Active\"}]'",
				},
			},
			"required":             []string{"data"},
			"additionalProperties": false,
		},
		ToolCallDesc: func(input any, output any, toolUseData *uctypes.UIMessageDataToolUse) string {
			parsed, err := parseCreateDashboardInput(input)
			if err != nil {
				return fmt.Sprintf("error parsing input: %v", err)
			}
			return fmt.Sprintf("creating %s dashboard: %s", parsed.Type, parsed.Title)
		},
		ToolAnyCallback: func(ctx context.Context, input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
			parsed, err := parseCreateDashboardInput(input)
			if err != nil {
				return nil, err
			}

			// Validate JSON array format basically
			var dummy []map[string]any
			if err := json.Unmarshal([]byte(parsed.Data), &dummy); err != nil {
				return nil, fmt.Errorf("data must be a valid JSON array string: %v", err)
			}

			rpcClient := wshclient.GetBareRpcClient()
			_, err = wshclient.CreateBlockCommand(rpcClient, wshrpc.CommandCreateBlockData{
				TabId: tabId,
				BlockDef: &gulinobj.BlockDef{
					Meta: map[string]any{
						"view":            "dashboard",
						"dashboard:title": parsed.Title,
						"dashboard:type":  parsed.Type,
						"dashboard:data":  parsed.Data, // Store the json string directly
					},
				},
			}, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create dashboard block: %w", err)
			}

			return "Dashboard created successfully in the user's current workspace.", nil
		},
	}
}
