// Copyright 2026, GuLiN Terminal
// SPDX-License-Identifier: Apache-2.0

package aiusechat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gulindev/gulin/pkg/aiusechat/uctypes"
	"github.com/gulindev/gulin/pkg/gulinbase"

	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

// APICallInput describes a call to a registered API endpoint.
type APICallInput struct {
	// Name of the API as registered in the API Manager.
	APIName string `json:"api_name"`
	// HTTP method: GET, POST, PUT, DELETE, PATCH. Defaults to GET.
	Method string `json:"method,omitempty"`
	// Path to append to the base URL, e.g. "/users/1". Leave empty for the root.
	Path string `json:"path,omitempty"`
	// Optional JSON body for POST/PUT/PATCH requests (as a string).
	Body string `json:"body,omitempty"`
	// Optional extra headers as key:value pairs, e.g. {"Accept": "application/json"}.
	Headers map[string]string `json:"headers,omitempty"`
}

// getAPIEndpointByName reads the API endpoint record from gulin.db by its name.
func getAPIEndpointByName(name string) (*uctypes.APIEndpointInfo, error) {
	dataDir := gulinbase.GetGulinDataDir()
	dbPath := filepath.Join(dataDir, gulinbase.GulinDBDir, "gulin.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open gulin db: %w", err)
	}
	defer db.Close()

	row := db.QueryRow(
		fmt.Sprintf("SELECT id, name, url, username, password, token FROM %s WHERE name = ? LIMIT 1", GulinAPIEndpointsTable),
		name,
	)
	var ep uctypes.APIEndpointInfo
	if err := row.Scan(&ep.ID, &ep.Name, &ep.URL, &ep.Username, &ep.Password, &ep.Token); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("API '%s' not found in API Manager. Register it first using the API Manager widget", name)
		}
		return nil, fmt.Errorf("error reading API endpoint: %w", err)
	}
	return &ep, nil
}

// listAPIEndpointNames returns all registered API names, for error hints.
func listAPIEndpointNames() ([]string, error) {
	dataDir := gulinbase.GetGulinDataDir()
	dbPath := filepath.Join(dataDir, gulinbase.GulinDBDir, "gulin.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(fmt.Sprintf("SELECT name FROM %s ORDER BY name", GulinAPIEndpointsTable))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err == nil {
			names = append(names, n)
		}
	}
	return names, nil
}

// GetAPICallToolDefinition returns the tool definition for calling registered REST APIs.
func GetAPICallToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "apimanager_call",
		DisplayName: "Call API Manager Endpoint",
		ToolLogName: "apimanager:call",
		Description: strings.TrimSpace(`
Call an HTTP REST API that has been registered in the GuLiN API Manager widget.
The API Manager ALREADY stores the base URL and credentials (token, username/password).
CRITICAL: You DO NOT need to look up passwords, tokens, or search the SQLite database to use this tool. You NEVER need to expose credentials. The tool handles authentication automatically behind the scenes.
You only need to provide the API name (as it appears in the API Manager), the HTTP method, and optionally a path and body.
Use this tool whenever the user mentions "api manager", an API registered in the app (e.g. "test"), or asks to connect to an API securely.
Do NOT use db_query or terminal commands for REST APIs — use this tool instead.
`),
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"api_name": map[string]any{
					"type":        "string",
					"description": "Name of the API as saved in the GuLiN API Manager (case-sensitive).",
				},
				"method": map[string]any{
					"type":        "string",
					"enum":        []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
					"description": "HTTP method. Defaults to GET.",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Path to append to the base URL, e.g. '/users' or '/v1/models'. Leave empty to call the base URL.",
				},
				"body": map[string]any{
					"type":        "string",
					"description": "JSON request body as a string (only for POST/PUT/PATCH).",
				},
				"headers": map[string]any{
					"type":        "object",
					"description": "Extra HTTP headers to include (e.g. {\"Accept\": \"application/json\"}).",
					"additionalProperties": map[string]any{
						"type": "string",
					},
				},
			},
			"required":             []string{"api_name"},
			"additionalProperties": false,
		},
		ToolAnyCallback: func(ctx context.Context, input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
			b, _ := json.Marshal(input)
			var parsed APICallInput
			json.Unmarshal(b, &parsed)

			if parsed.APIName == "" {
				return nil, fmt.Errorf("api_name is required")
			}

			// Resolve the API endpoint
			ep, err := getAPIEndpointByName(parsed.APIName)
			if err != nil {
				// Provide a helpful list of available APIs
				names, listErr := listAPIEndpointNames()
				if listErr == nil && len(names) > 0 {
					return nil, fmt.Errorf("%v. Available APIs: %s", err, strings.Join(names, ", "))
				}
				return nil, err
			}

			// Build URL
			method := strings.ToUpper(parsed.Method)
			if method == "" {
				method = "GET"
			}
			baseURL := strings.TrimRight(ep.URL, "/")
			path := parsed.Path
			if path != "" && !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			fullURL := baseURL + path

			// Build request
			var bodyReader io.Reader
			if parsed.Body != "" {
				bodyReader = strings.NewReader(parsed.Body)
			}
			req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
			if err != nil {
				return nil, fmt.Errorf("failed to build request: %w", err)
			}

			// Auth: prefer token, then basic auth
			if ep.Token != "" {
				// Try to detect if it's already prefixed
				tokenVal := ep.Token
				if !strings.HasPrefix(strings.ToLower(tokenVal), "bearer ") {
					tokenVal = "Bearer " + tokenVal
				}
				req.Header.Set("Authorization", tokenVal)
			} else if ep.Username != "" && ep.Password != "" {
				req.SetBasicAuth(ep.Username, ep.Password)
			}

			// Default content type for body requests
			if parsed.Body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			req.Header.Set("Accept", "application/json")

			// Extra headers
			for k, v := range parsed.Headers {
				req.Header.Set(k, v)
			}

			// Execute
			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return nil, fmt.Errorf("request failed: %w", err)
			}
			defer resp.Body.Close()

			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to read response: %w", err)
			}

			// Try to pretty-print JSON responses and TRUNCATE if it's too huge, saving to file
			var prettyJSON any
			if err := json.Unmarshal(respBody, &prettyJSON); err == nil {
				prettyBytes, _ := json.MarshalIndent(prettyJSON, "", "  ")
				bodyStr := string(prettyBytes)
				
				// Limit output length to prevent LLM context overflow (max ~15KB per API call)
				const MaxBodyLen = 15000
				if len(bodyStr) > MaxBodyLen {
					// Save to a temporary file so the AI can use python/bash to process it if needed
					tmpFile := filepath.Join(gulinbase.GetGulinDataDir(), "tmp_api_response.json")
					os.WriteFile(tmpFile, prettyBytes, 0644)
					
					truncMsg := fmt.Sprintf("\n\n... [RESPONSE TRUNCATED: The API returned a huge JSON payload (%d bytes) which exceeds the context limit. Only showing the first %d bytes.]\n>>> CRITICAL INSTRUCTION: The FULL JSON response has been saved to the local file: %s\n>>> DO NOT try to read this file into chat (no cat, no read_file) or you will crash. Instead, use term_run_command to run a Python script that reads this file, performs the necessary data aggregation/counting, and prints a small summary. Then use that summary to generate the chart.", len(bodyStr), MaxBodyLen, tmpFile)
					bodyStr = bodyStr[:MaxBodyLen] + truncMsg
				}
				
				return map[string]any{
					"status":  resp.StatusCode,
					"url":     fullURL,
					"method":  method,
					"body":    bodyStr,
				}, nil
			}

			// Non-JSON response truncation
			bodyStr := string(respBody)
			const MaxStringLen = 15000
			if len(bodyStr) > MaxStringLen {
				tmpFile := filepath.Join(gulinbase.GetGulinDataDir(), "tmp_api_response.txt")
				os.WriteFile(tmpFile, respBody, 0644)
				bodyStr = bodyStr[:MaxStringLen] + fmt.Sprintf("\n\n... [TRUNCATED: %d bytes. Full response saved to %s. CRITICAL: DO NOT read this file into chat. Use python/bash to process it and print a summary.]", len(bodyStr), tmpFile)
			}

			return map[string]any{
				"status": resp.StatusCode,
				"url":    fullURL,
				"method": method,
				"body":   bodyStr,
			}, nil
		},
	}
}

// GetAPIListToolDefinition returns a tool to list all registered APIs (so the AI knows what's available).
func GetAPIListToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "apimanager_list",
		DisplayName: "List API Manager Endpoints",
		ToolLogName: "apimanager:list",
		Description: "List all APIs registered securely in the GuLiN API Manager. Use this before calling apimanager_call if you are unsure which APIs are available or if the user tells you to check the api manager.",
		InputSchema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
		ToolAnyCallback: func(ctx context.Context, input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
			dataDir := gulinbase.GetGulinDataDir()
			dbPath := filepath.Join(dataDir, gulinbase.GulinDBDir, "gulin.db")
			db, err := sql.Open("sqlite3", dbPath)
			if err != nil {
				return nil, fmt.Errorf("failed to open db: %w", err)
			}
			defer db.Close()

			rows, err := db.Query(
				fmt.Sprintf("SELECT name, url FROM %s ORDER BY name", GulinAPIEndpointsTable),
			)
			if err != nil {
				return nil, fmt.Errorf("failed to query APIs: %w", err)
			}
			defer rows.Close()

			type APIInfo struct {
				Name string `json:"name"`
				URL  string `json:"url"`
			}
			var apis []APIInfo
			for rows.Next() {
				var a APIInfo
				rows.Scan(&a.Name, &a.URL)
				apis = append(apis, a)
			}
			if len(apis) == 0 {
				return "No APIs registered. Use the API Manager widget to add one.", nil
			}
			return apis, nil
		},
	}
}
