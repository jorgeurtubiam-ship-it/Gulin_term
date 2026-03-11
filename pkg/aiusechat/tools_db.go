// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package aiusechat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/wavetermdev/waveterm/pkg/aiusechat/uctypes"
	"github.com/wavetermdev/waveterm/pkg/secretstore"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
	"github.com/wavetermdev/waveterm/pkg/wshrpc/wshclient"
)

// DBConnection info stored in public config
type DBConnectionInfo struct {
	Name string `json:"name"`
	Type string `json:"type"` // "sqlite", "postgres", etc.
}

const DBConnectionsSecretKey = "gulin_db_connections"

type DBRegisterInput struct {
	Name string `json:"name"`
	Type string `json:"type"`
	URL  string `json:"url"` // Connection string
}

func GetDBRegisterToolDefinition(tabId string) uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "db_register_connection",
		DisplayName: "Register DB Connection",
		Description: "Register a new database connection. The URL/connection string will be stored securely. Supported types: 'sqlite'.",
		ToolLogName: "db:register",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string", "description": "Unique name for this connection"},
				"type": map[string]any{"type": "string", "enum": []string{"sqlite"}, "description": "Database type"},
				"url":  map[string]any{"type": "string", "description": "Connection URL or file path for SQLite"},
			},
			"required": []string{"name", "type", "url"},
		},
		ToolAnyCallback: func(ctx context.Context, input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
			var parsed DBRegisterInput
			b, _ := json.Marshal(input)
			json.Unmarshal(b, &parsed)

			// Get existing connections from secrets
			val, exists, _ := secretstore.GetSecret(DBConnectionsSecretKey)
			connections := make(map[string]DBRegisterInput)
			if exists {
				json.Unmarshal([]byte(val), &connections)
			}

			connections[parsed.Name] = parsed
			newVal, _ := json.Marshal(connections)
			err := secretstore.SetSecret(DBConnectionsSecretKey, string(newVal))
			if err != nil {
				return nil, err
			}

			return fmt.Sprintf("Connection '%s' registered successfully.", parsed.Name), nil
		},
	}
}

type DBQueryInput struct {
	ConnectionName string `json:"connection_name"`
	SQL            string `json:"sql"`
}

func GetDBListConnectionsToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "db_list_connections",
		DisplayName: "List DB Connections",
		Description: "List all registered database connections.",
		ToolLogName: "db:list",
		InputSchema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
		ToolAnyCallback: func(ctx context.Context, input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
			val, exists, _ := secretstore.GetSecret(DBConnectionsSecretKey)
			if !exists {
				return []any{}, nil
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
			return result, nil
		},
	}
}

func GetDBQueryToolDefinition(tabId string) uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "db_query",
		DisplayName: "Execute SQL Query",
		Description: "Execute a SQL query on a registered connection and return results as a JSON array suitable for the Grid widget.",
		ToolLogName: "db:query",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"connection_name": map[string]any{"type": "string"},
				"sql":             map[string]any{"type": "string", "description": "SQL SELECT statement"},
			},
			"required": []string{"connection_name", "sql"},
		},
		ToolAnyCallback: func(ctx context.Context, input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
			var parsed DBQueryInput
			b, _ := json.Marshal(input)
			json.Unmarshal(b, &parsed)

			// Get connection info
			val, exists, _ := secretstore.GetSecret(DBConnectionsSecretKey)
			if !exists {
				return nil, fmt.Errorf("no connections registered")
			}
			connections := make(map[string]DBRegisterInput)
			json.Unmarshal([]byte(val), &connections)

			connInfo, ok := connections[parsed.ConnectionName]
			if !ok {
				return nil, fmt.Errorf("connection '%s' not found", parsed.ConnectionName)
			}

			if connInfo.Type != "sqlite" {
				return nil, fmt.Errorf("only 'sqlite' is supported currently")
			}

			db, err := sql.Open("sqlite3", connInfo.URL)
			if err != nil {
				return nil, err
			}
			defer db.Close()

			rows, err := db.QueryContext(ctx, parsed.SQL)
			if err != nil {
				return nil, err
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
					return nil, err
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
				BlockDef: &waveobj.BlockDef{
					Meta: map[string]any{
						"view":          "db-explorer",
						"db:title":      fmt.Sprintf("Query: %s", parsed.SQL),
						"db:connection": parsed.ConnectionName,
						"db:data":       string(dataJson),
					},
				},
			}, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create db-explorer block: %w", err)
			}

			return fmt.Sprintf("Query executed. %d rows returned. DB Explorer widget opened.", len(results)), nil
		},
	}
}
