
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gulindev/gulin/pkg/aiusechat/chatstore"
	"github.com/gulindev/gulin/pkg/gulinbase"
	"github.com/gulindev/gulin/pkg/wstore"
)

func main() {
	home, _ := os.UserHomeDir()
	os.Setenv("GULIN_DATA_HOME", filepath.Join(home, "Library/Application Support/gulin-dev"))
	os.Setenv("GULIN_CONFIG_HOME", filepath.Join(home, ".config/gulin-dev"))
	os.Setenv("GULIN_DEV", "1")

	gulinbase.CacheAndRemoveEnvVars()
	wstore.InitWStore()

	summaries, err := chatstore.GetChatListFromDB()
	if err != nil {
		fmt.Printf("Error getting chat list: %v\n", err)
		return
	}
	
	if len(summaries) == 0 {
		fmt.Println("No chats found in DB")
		return
	}

	fmt.Printf("Found %d chats in DB\n", len(summaries))
	
	// Load the most recent chat
	recentSummary := summaries[0]
	chat, err := chatstore.LoadChatFromDB(recentSummary.ChatID)
	if err != nil {
		fmt.Printf("Error loading chat %s: %v\n", recentSummary.ChatID, err)
		return
	}

	if chat != nil {
		fmt.Printf("Dumping last 20 messages from chat: %s\n", chat.ChatId)
		start := 0
		if len(chat.NativeMessages) > 20 {
			start = len(chat.NativeMessages) - 20
		}
		for i := start; i < len(chat.NativeMessages); i++ {
			msg := chat.NativeMessages[i]
			fmt.Printf("[%d] Role: %s | Content: %s\n", i, msg.GetRole(), msg.GetContent())
			// Try to find tool calls or results
			data, _ := json.Marshal(msg)
			fmt.Printf("    Raw: %s\n", string(data))
		}
	}
}
