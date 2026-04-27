
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

	rows, err := db.Query("SELECT name, url, username FROM gulin_api_endpoints")
	if err != nil {
		fmt.Printf("Error querying APIs: %v\n", err)
		return
	}
	defer rows.Close()

	fmt.Println("Registered APIs:")
	for rows.Next() {
		var name, url, username string
		rows.Scan(&name, &url, &username)
		fmt.Printf("- Name: %s | URL: %s | User: %s\n", name, url, username)
	}
}
