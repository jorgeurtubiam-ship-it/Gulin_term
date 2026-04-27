
package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type APIEndpointInfo struct {
	URL      string
	Username string
	Password string
}

func main() {
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, "Library/Application Support/gulin-dev/db/gulin.db")
	
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		fmt.Printf("Error opening DB: %v\n", err)
		return
	}
	defer db.Close()

	var ep APIEndpointInfo
	err = db.QueryRow("SELECT url, username, password FROM gulin_api_endpoints WHERE name = 'dremio'").Scan(&ep.URL, &ep.Username, &ep.Password)
	if err != nil {
		fmt.Printf("Error fetching Dremio config: %v\n", err)
		return
	}

	fmt.Printf("Testing Dremio login at %s/apiv2/login...\n", ep.URL)
	
	loginBody := map[string]string{
		"userName": ep.Username,
		"password": ep.Password,
	}
	bodyBytes, _ := json.Marshal(loginBody)
	
	req, _ := http.NewRequest("POST", ep.URL+"/apiv2/login", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Request failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("Status: %d\n", resp.StatusCode)
	
	// Masking token for display
	bodyStr := string(respBody)
	if strings.Contains(bodyStr, "token") {
		fmt.Println("Success! Received a token.")
	} else {
		fmt.Printf("Response Body: %s\n", bodyStr)
	}
}
