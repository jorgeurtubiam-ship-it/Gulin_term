// Copyright 2026, GuLiN Terminal
// SPDX-License-Identifier: Apache-2.0

package aiusechat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gulindev/gulin/pkg/aiusechat/uctypes"
	"github.com/gulindev/gulin/pkg/gulinbase"
)

func GetPluginSaveToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "plugin_save",
		DisplayName: "Save Plugin",
		Description: "Save a new dynamic plugin or update an existing one. Use .js extension.",
		ToolLogName: "plugin:save",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"filename": map[string]any{
					"type":        "string",
					"description": "Name of the plugin file (e.g., 'oci_helper.js')",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "The Javascript code for the plugin, including @name, @description and @param tags.",
				},
			},
			"required":             []string{"filename", "content"},
			"additionalProperties": false,
		},
		ToolAnyCallback: func(ctx context.Context, input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
			m, _ := input.(map[string]any)
			filename := m["filename"].(string)
			content := m["content"].(string)

			if !strings.HasSuffix(filename, ".js") {
				filename += ".js"
			}

			pluginsDir := filepath.Join(gulinbase.GetGulinConfigDir(), PluginsDirName)
			os.MkdirAll(pluginsDir, 0755)

			path := filepath.Join(pluginsDir, filename)
			err := os.WriteFile(path, []byte(content), 0644)
			if err != nil {
				return nil, err
			}

			return fmt.Sprintf("Plugin %s guardado exitosamente. Estará disponible en el próximo turno o al refrescar.", filename), nil
		},
	}
}

func GetPluginListToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "plugin_list",
		DisplayName: "List Plugins",
		Description: "List all dynamic plugins installed in the system.",
		ToolLogName: "plugin:list",
		InputSchema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
		ToolAnyCallback: func(ctx context.Context, input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
			pluginsDir := filepath.Join(gulinbase.GetGulinConfigDir(), PluginsDirName)
			files, err := os.ReadDir(pluginsDir)
			if err != nil {
				return nil, err
			}

			var result []string
			for _, f := range files {
				if !f.IsDir() && strings.HasSuffix(f.Name(), ".js") {
					result = append(result, f.Name())
				}
			}
			if len(result) == 0 {
				return "No hay plugins instalados.", nil
			}
			return result, nil
		},
	}
}

func GetPluginDeleteToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "plugin_delete",
		DisplayName: "Delete Plugin",
		Description: "Permanently delete a dynamic plugin file.",
		ToolLogName: "plugin:delete",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"filename": map[string]any{
					"type":        "string",
					"description": "Name of the plugin file to delete.",
				},
			},
			"required":             []string{"filename"},
			"additionalProperties": false,
		},
		ToolAnyCallback: func(ctx context.Context, input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
			m, _ := input.(map[string]any)
			filename := m["filename"].(string)

			if !strings.HasSuffix(filename, ".js") {
				filename += ".js"
			}

			path := filepath.Join(gulinbase.GetGulinConfigDir(), PluginsDirName, filename)
			err := os.Remove(path)
			if err != nil {
				return nil, err
			}

			return fmt.Sprintf("Plugin %s eliminado exitosamente.", filename), nil
		},
	}
}

func GetPluginDebugToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "plugin_debug",
		DisplayName: "Debug Plugins",
		Description: "Diagnostic tool to check plugin loading state and directories.",
		ToolLogName: "plugin:debug",
		InputSchema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
		ToolAnyCallback: func(ctx context.Context, input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
			pluginsDir := filepath.Join(gulinbase.GetGulinConfigDir(), PluginsDirName)
			info, err := os.Stat(pluginsDir)
			
			res := make(map[string]any)
			res["config_dir"] = gulinbase.GetGulinConfigDir()
			res["plugins_dir"] = pluginsDir
			if err != nil {
				res["plugins_dir_error"] = err.Error()
			} else {
				res["plugins_dir_exists"] = true
				res["plugins_dir_is_dir"] = info.IsDir()
				
				files, _ := os.ReadDir(pluginsDir)
				var fileList []string
				for _, f := range files {
					fileList = append(fileList, f.Name())
				}
				res["files"] = fileList
			}
			
			return res, nil
		},
	}
}
