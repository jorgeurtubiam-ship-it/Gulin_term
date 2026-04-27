
package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, "Library/Application Support/gulin-dev/db/gulin.db")
	
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		fmt.Printf("Error opening DB: %v\n", err)
		return
	}
	defer db.Close()

	// Update dremio to use the root URL as base, and add auth instructions
	_, err = db.Exec(`
		UPDATE gulin_api_endpoints 
		SET url = 'http://127.0.0.1:9047', 
		    auth_instructions = 'Use POST /apiv2/login for tokens. Use /api/v3/ for data.' 
		WHERE name = 'dremio'
	`)
	if err != nil {
		fmt.Printf("Error updating Dremio: %v\n", err)
		return
	}

	fmt.Println("Dremio API configuration updated successfully.")
}
