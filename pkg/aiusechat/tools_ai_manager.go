package aiusechat

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"path/filepath"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/gulindev/gulin/pkg/aiusechat/uctypes"
	"github.com/gulindev/gulin/pkg/gulinbase"
)

const GulinAPIEndpointsTable = "gulin_api_endpoints"

func getGulintermDB() (*sql.DB, error) {
	dataDir := gulinbase.GetGulinDataDir()
	dbPath := filepath.Join(dataDir, gulinbase.GulinDBDir, "gulin.db")
	return sql.Open("sqlite3", dbPath)
}

func GulinAIApiListHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("[API-MANAGER] GulinAIApiListHandler called\n")

	db, err := getGulintermDB()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to open db: %v", err), http.StatusInternalServerError)
		return
	}
	defer db.Close()

	rows, err := db.Query(fmt.Sprintf("SELECT id, name, url, username, password, token, created_at, updated_at FROM %s ORDER BY created_at DESC", GulinAPIEndpointsTable))
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to query endpoints: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var endpoints []uctypes.APIEndpointInfo
	for rows.Next() {
		var e uctypes.APIEndpointInfo
		err := rows.Scan(&e.ID, &e.Name, &e.URL, &e.Username, &e.Password, &e.Token, &e.CreatedAt, &e.UpdatedAt)
		if err == nil {
			endpoints = append(endpoints, e)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(endpoints)
}

func GulinAIApiRegisterHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("[API-MANAGER] GulinAIApiRegisterHandler called: %s\n", r.Method)
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req uctypes.APIEndpointInfo
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fmt.Printf("[API-MANAGER] error decoding body: %v\n", err)
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	fmt.Printf("[API-MANAGER] registering endpoint: %s (%s)\n", req.Name, req.URL)

	if req.Name == "" || req.URL == "" {
		http.Error(w, "name and url are required", http.StatusBadRequest)
		return
	}

	db, err := getGulintermDB()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to open db: %v", err), http.StatusInternalServerError)
		return
	}
	defer db.Close()

	now := time.Now().UnixMilli()
	if req.ID == "" {
		req.ID = uuid.New().String()
		req.CreatedAt = now
	}
	req.UpdatedAt = now

	query := fmt.Sprintf(`
		INSERT INTO %s (id, name, url, username, password, token, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,
			url=excluded.url,
			username=excluded.username,
			password=excluded.password,
			token=excluded.token,
			updated_at=excluded.updated_at
	`, GulinAPIEndpointsTable)

	_, err = db.Exec(query, req.ID, req.Name, req.URL, req.Username, req.Password, req.Token, req.CreatedAt, req.UpdatedAt)
	if err != nil {
		fmt.Printf("[API-MANAGER] error executing insert: %v\n", err)
		http.Error(w, fmt.Sprintf("failed to save endpoint: %v", err), http.StatusInternalServerError)
		return
	}
	fmt.Printf("[API-MANAGER] successfully saved endpoint: %s\n", req.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(req)
}
