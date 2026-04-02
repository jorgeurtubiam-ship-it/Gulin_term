// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package aiusechat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/microsoft/go-mssqldb"
	_ "github.com/mattn/go-sqlite3"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/gulindev/gulin/pkg/aiusechat/uctypes"
	"github.com/gulindev/gulin/pkg/secretstore"
	"github.com/gulindev/gulin/pkg/gulinobj"
	"github.com/gulindev/gulin/pkg/wshrpc"
	"github.com/gulindev/gulin/pkg/wshrpc/wshclient"
)

// DBConnection info stored in public config
type DBConnectionInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

const DBConnectionsSecretKey = "gulin_db_connections"

type DBRegisterInput struct {
	Name string `json:"name"`
	Type string `json:"type"`
	URL  string `json:"url"` // Connection string / DSN
}

// supportedDBTypes lists the databases GuLiN can connect to.
var supportedDBTypes = []string{
	"postgres",  // lib/pq – pure Go
	"mysql",     // go-sql-driver/mysql – pure Go, also MariaDB, AuroraDB MySQL
	"mssql",     // go-mssqldb – pure Go, SQL Server, Azure SQL
	"sqlite",    // mattn/go-sqlite3 – embedded SQLite
	"mongodb",   // mongo-driver – NoSQL aggregation / find
}

// ──────────────────────────────────────────────────────────────────────────────
// Helper: open a standard database/sql connection based on type
// ──────────────────────────────────────────────────────────────────────────────

func openSQLDB(dbType, dsn string) (*sql.DB, error) {
	var driverName string
	switch dbType {
	case "postgres":
		// lib/pq: postgres://user:password@host:5432/dbname?sslmode=disable
		driverName = "postgres"
	case "mysql", "mariadb", "aurora-mysql":
		// go-sql-driver: user:password@tcp(host:3306)/dbname
		driverName = "mysql"
	case "mssql", "sqlserver", "azure-sql":
		// go-mssqldb: sqlserver://user:password@host:1433?database=mydb
		driverName = "sqlserver"
	case "sqlite":
		driverName = "sqlite3"
	default:
		return nil, fmt.Errorf("unsupported DB type %q. Supported: %v", dbType, supportedDBTypes)
	}
	return sql.Open(driverName, dsn)
}

// ──────────────────────────────────────────────────────────────────────────────
// Shared: read connections from secret store
// ──────────────────────────────────────────────────────────────────────────────

func loadDBConnections() (map[string]DBRegisterInput, error) {
	val, exists, _ := secretstore.GetSecret(DBConnectionsSecretKey)
	connections := make(map[string]DBRegisterInput)
	if exists && val != "" {
		json.Unmarshal([]byte(val), &connections)
	}
	return connections, nil
}

func saveDBConnections(connections map[string]DBRegisterInput) error {
	newVal, _ := json.Marshal(connections)
	return secretstore.SetSecret(DBConnectionsSecretKey, string(newVal))
}

// ──────────────────────────────────────────────────────────────────────────────
// Tool: db_register_connection
// ──────────────────────────────────────────────────────────────────────────────

func GetDBRegisterToolDefinition(tabId string) uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "db_register_connection",
		DisplayName: "Register DB Connection",
		Description: fmt.Sprintf(`Register a new database connection. The connection string will be stored securely.
Supported types: %v.

DSN format examples:
- postgres: postgres://user:pass@host:5432/dbname?sslmode=disable
- mysql   : user:pass@tcp(host:3306)/dbname
- mssql   : sqlserver://user:pass@host:1433?database=mydb
- sqlite  : /absolute/path/to/file.db
- mongodb : mongodb://user:pass@host:27017/dbname`, supportedDBTypes),
		ToolLogName: "db:register",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string", "description": "Unique name for this connection"},
				"type": map[string]any{"type": "string", "enum": supportedDBTypes, "description": "Database type"},
				"url":  map[string]any{"type": "string", "description": "Connection string / DSN"},
			},
			"required":             []string{"name", "type", "url"},
			"additionalProperties": false,
		},
		ToolAnyCallback: func(ctx context.Context, input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
			var parsed DBRegisterInput
			b, _ := json.Marshal(input)
			json.Unmarshal(b, &parsed)

			if parsed.Name == "" || parsed.Type == "" || parsed.URL == "" {
				return nil, fmt.Errorf("name, type, and url are required")
			}

			connections, _ := loadDBConnections()
			connections[parsed.Name] = parsed
			if err := saveDBConnections(connections); err != nil {
				return nil, err
			}
			return fmt.Sprintf("Connection '%s' (%s) registered successfully.", parsed.Name, parsed.Type), nil
		},
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Tool: db_list_connections
// ──────────────────────────────────────────────────────────────────────────────

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
			connections, _ := loadDBConnections()
			var result []DBConnectionInfo
			for name, conn := range connections {
				result = append(result, DBConnectionInfo{Name: name, Type: conn.Type})
			}
			if len(result) == 0 {
				return "No connections registered. Use db_register_connection first.", nil
			}
			return result, nil
		},
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Tool: db_query  (SQL databases + MongoDB)
// ──────────────────────────────────────────────────────────────────────────────

type DBQueryInput struct {
	ConnectionName string `json:"connection_name"`
	SQL            string `json:"sql"`             // For SQL databases
	Collection     string `json:"collection"`      // For MongoDB
	Filter         string `json:"filter"`          // For MongoDB: JSON filter
	Projection     string `json:"projection"`      // For MongoDB: JSON projection (optional)
	Limit          int64  `json:"limit"`           // For MongoDB: max documents (optional)
}

func GetDBQueryToolDefinition(tabId string) uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "db_query",
		DisplayName: "Execute DB Query",
		ToolLogName: "db:query",
		Description: `Execute a query on a registered database and display results.

For SQL databases (postgres, mysql, mssql, sqlite):
  Provide "sql" with a SELECT statement.

For MongoDB:
  Provide "collection" and optionally "filter" (JSON), "projection" (JSON), "limit".
  Example: collection="users", filter={"active": true}, limit=20
`,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"connection_name": map[string]any{"type": "string", "description": "Name of the registered connection"},
				"sql":             map[string]any{"type": "string", "description": "SQL SELECT statement (for SQL databases)"},
				"collection":      map[string]any{"type": "string", "description": "MongoDB collection name"},
				"filter":          map[string]any{"type": "string", "description": "MongoDB filter as JSON string, e.g. '{\"active\":true}'"},
				"projection":      map[string]any{"type": "string", "description": "MongoDB projection as JSON string (optional)"},
				"limit":           map[string]any{"type": "integer", "description": "Max documents for MongoDB (default 50)"},
			},
			"required":             []string{"connection_name"},
			"additionalProperties": false,
		},
		ToolAnyCallback: func(ctx context.Context, input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
			var parsed DBQueryInput
			b, _ := json.Marshal(input)
			json.Unmarshal(b, &parsed)

			connections, _ := loadDBConnections()
			connInfo, ok := connections[parsed.ConnectionName]
			if !ok {
				var names []string
				for n := range connections {
					names = append(names, n)
				}
				return nil, fmt.Errorf("connection '%s' not found. Available connections: %v", parsed.ConnectionName, names)
			}

			// ── MongoDB ──────────────────────────────────────────────────────
			if connInfo.Type == "mongodb" {
				return runMongoQuery(ctx, connInfo, parsed, tabId)
			}

			// ── SQL databases ────────────────────────────────────────────────
			if parsed.SQL == "" {
				return nil, fmt.Errorf("'sql' field is required for %s databases", connInfo.Type)
			}

			db, err := openSQLDB(connInfo.Type, connInfo.URL)
			if err != nil {
				return nil, fmt.Errorf("failed to open connection: %w", err)
			}
			defer db.Close()

			if err := db.PingContext(ctx); err != nil {
				return nil, fmt.Errorf("cannot connect to %s '%s': %w. Check the connection string and that the database is reachable", connInfo.Type, parsed.ConnectionName, err)
			}

			rows, err := db.QueryContext(ctx, parsed.SQL)
			if err != nil {
				return nil, fmt.Errorf("query error: %w", err)
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

			return openDBExplorerBlock(ctx, tabId, parsed, results)
		},
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// MongoDB query runner
// ──────────────────────────────────────────────────────────────────────────────

func runMongoQuery(ctx context.Context, connInfo DBRegisterInput, parsed DBQueryInput, tabId string) (any, error) {
	if parsed.Collection == "" {
		// List collections if no collection provided
		client, err := mongo.Connect(ctx, options.Client().ApplyURI(connInfo.URL))
		if err != nil {
			return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
		}
		defer client.Disconnect(ctx)
		dbName := parsed.ConnectionName
		names, err := client.Database(dbName).ListCollectionNames(ctx, bson.D{})
		if err != nil {
			// try default db from URI
			return nil, fmt.Errorf("specify a 'collection'. MongoDB error: %w", err)
		}
		return map[string]any{"collections": names}, nil
	}

	clientOpts := options.Client().ApplyURI(connInfo.URL)
	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}
	defer client.Disconnect(ctx)

	// Extract database name from URI or use connection name as fallback
	dbName := client.Database("") .Name()
	if dbName == "" {
		dbName = parsed.ConnectionName
	}

	coll := client.Database(dbName).Collection(parsed.Collection)

	// Build filter
	filter := bson.D{}
	if parsed.Filter != "" {
		if err := bson.UnmarshalExtJSON([]byte(parsed.Filter), true, &filter); err != nil {
			return nil, fmt.Errorf("invalid filter JSON: %w", err)
		}
	}

	// Build projection
	findOpts := options.Find()
	if parsed.Projection != "" {
		var proj bson.D
		if err := bson.UnmarshalExtJSON([]byte(parsed.Projection), true, &proj); err != nil {
			return nil, fmt.Errorf("invalid projection JSON: %w", err)
		}
		findOpts.SetProjection(proj)
	}
	limit := parsed.Limit
	if limit <= 0 {
		limit = 50
	}
	findOpts.SetLimit(limit)

	cursor, err := coll.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, fmt.Errorf("MongoDB find error: %w", err)
	}
	defer cursor.Close(ctx)

	var results []map[string]any
	for cursor.Next(ctx) {
		var doc map[string]any
		if err := cursor.Decode(&doc); err == nil {
			results = append(results, doc)
		}
	}

	return openDBExplorerBlock(ctx, tabId, parsed, results)
}

// ──────────────────────────────────────────────────────────────────────────────
// Helper: open DB Explorer widget with query results
// ──────────────────────────────────────────────────────────────────────────────

func openDBExplorerBlock(ctx context.Context, tabId string, parsed DBQueryInput, results []map[string]any) (any, error) {
	rpcClient := wshclient.GetBareRpcClient()
	dataJson, _ := json.Marshal(results)

	queryLabel := parsed.SQL
	if queryLabel == "" {
		queryLabel = fmt.Sprintf("MongoDB %s", parsed.Collection)
	}

	_, err := wshclient.CreateBlockCommand(rpcClient, wshrpc.CommandCreateBlockData{
		TabId: tabId,
		BlockDef: &gulinobj.BlockDef{
			Meta: map[string]any{
				"view":          "db-explorer",
				"db:title":      fmt.Sprintf("Query: %s", queryLabel),
				"db:connection": parsed.ConnectionName,
				"db:data":       string(dataJson),
			},
		},
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open DB Explorer: %w", err)
	}
	return fmt.Sprintf("Query executed. %d rows returned. DB Explorer widget opened.", len(results)), nil
}

