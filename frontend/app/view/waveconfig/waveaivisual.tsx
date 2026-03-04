// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import type { WaveConfigViewModel } from "@/app/view/waveconfig/waveconfig-model";
import { cn } from "@/util/util";
import { useAtom, useAtomValue } from "jotai";
import { memo, useCallback, useMemo, useState, useEffect } from "react";

interface WaveAIVisualContentProps {
    model: WaveConfigViewModel;
}

const PROVIDERS = [
    "wave",
    "google",
    "groq",
    "openrouter",
    "nanogpt",
    "openai",
    "deepseek",
    "azure",
    "azure-legacy",
    "custom"
];

const API_TYPES = [
    "google-gemini",
    "openai-responses",
    "openai-chat"
];

function tryParseJSON(str: string): Record<string, any> {
    try {
        const parsed = JSON.parse(str);
        if (typeof parsed === "object" && parsed !== null && !Array.isArray(parsed)) {
            return parsed;
        }
    } catch (e) {
        // ignore
    }
    return {};
}

export const WaveAIVisualContent = memo(({ model }: WaveAIVisualContentProps) => {
    const [fileContent, setFileContent] = useAtom(model.fileContentAtom);
    const parsedConfig = useMemo(() => tryParseJSON(fileContent), [fileContent]);
    const configKeys = Object.keys(parsedConfig).sort();

    const [selectedKey, setSelectedKey] = useState<string | null>(null);

    // Auto-select first key if none is selected
    useEffect(() => {
        if (!selectedKey && configKeys.length > 0) {
            setSelectedKey(configKeys[0]);
        } else if (selectedKey && !configKeys.includes(selectedKey)) {
            setSelectedKey(configKeys.length > 0 ? configKeys[0] : null);
        }
    }, [configKeys, selectedKey]);

    const updateConfig = useCallback((newConfig: Record<string, any>) => {
        setFileContent(JSON.stringify(newConfig, null, 2));
        model.markAsEdited();
    }, [setFileContent, model]);

    const handleAddMode = () => {
        let newKey = "new-mode";
        let counter = 1;
        while (parsedConfig[newKey]) {
            newKey = `new-mode-${counter}`;
            counter++;
        }

        const newConfig = {
            ...parsedConfig,
            [newKey]: {
                "display:name": "New Gulin IA Mode",
                "ai:provider": "custom",
                "ai:model": "llama3",
            }
        };
        updateConfig(newConfig);
        setSelectedKey(newKey);
    };

    const handleDeleteMode = (keyToDelete: string) => {
        const newConfig = { ...parsedConfig };
        delete newConfig[keyToDelete];
        updateConfig(newConfig);
        if (selectedKey === keyToDelete) {
            setSelectedKey(null);
        }
    };

    const handleUpdateField = (field: string, value: any) => {
        if (!selectedKey) return;
        const currentMode = parsedConfig[selectedKey] || {};

        const newMode = { ...currentMode };
        if (value === "") {
            delete newMode[field];
        } else {
            newMode[field] = value;
        }

        const newConfig = {
            ...parsedConfig,
            [selectedKey]: newMode
        };
        updateConfig(newConfig);
    };

    const handleKeyChange = (newKey: string) => {
        if (!selectedKey || newKey === selectedKey || newKey.trim() === "") return;
        if (parsedConfig[newKey]) return; // Refuse if already exists

        const newConfig = { ...parsedConfig };
        newConfig[newKey] = newConfig[selectedKey];
        delete newConfig[selectedKey];

        updateConfig(newConfig);
        setSelectedKey(newKey);
    };

    const selectedData = selectedKey ? parsedConfig[selectedKey] : null;

    return (
        <div className="flex w-full h-full overflow-hidden text-sm">
            {/* Left Sidebar - List of Modes */}
            <div className="w-1/3 min-w-[200px] border-r border-border flex flex-col h-full bg-zinc-800/20">
                <div className="p-3 border-b border-border flex justify-between items-center bg-zinc-800/20">
                    <span className="font-semibold text-zinc-300">Mis Modelos</span>
                    <button
                        onClick={handleAddMode}
                        className="p-1 px-2 hover:bg-zinc-700/50 rounded flex items-center justify-center cursor-pointer transition-colors text-accent-500 text-xs font-medium"
                        title="Add Custom Model"
                    >
                        <i className="fa-sharp fa-solid fa-plus mr-1" />
                        Personalizado
                    </button>
                </div>
                <div className="p-2 border-b border-border bg-zinc-800/30 flex flex-col gap-1.5 justify-center">
                    <div className="text-[10px] text-zinc-400 font-medium uppercase px-1">Configuración Rápida</div>
                    <button
                        onClick={() => {
                            let newKey = "deepseek-api";
                            let counter = 1;
                            while (parsedConfig[newKey]) {
                                newKey = `deepseek-api-${counter}`;
                                counter++;
                            }
                            const newConfig = {
                                ...parsedConfig,
                                [newKey]: {
                                    "display:name": "DeepSeek (API)",
                                    "ai:provider": "deepseek",
                                    "ai:apitype": "openai-chat",
                                    "ai:model": "deepseek-chat",
                                    "ai:capabilities": ["tools"]
                                }
                            };
                            updateConfig(newConfig);
                            setSelectedKey(newKey);
                        }}
                        className="py-1 px-3 bg-indigo-500/10 hover:bg-indigo-500/20 border border-indigo-500/30 rounded text-[11px] text-indigo-300 cursor-pointer transition-colors flex items-center gap-2 w-full justify-start"
                    >
                        <i className="fa-solid fa-sparkles text-indigo-400" />
                        Añadir DeepSeek
                    </button>
                    <button
                        onClick={() => {
                            let newKey = "ollama-local";
                            let counter = 1;
                            while (parsedConfig[newKey]) {
                                newKey = `ollama-local-${counter}`;
                                counter++;
                            }
                            const newConfig = {
                                ...parsedConfig,
                                [newKey]: {
                                    "display:name": "Ollama (Local)",
                                    "ai:provider": "custom",
                                    "ai:apitype": "openai-chat",
                                    "ai:model": "llama3.2",
                                    "ai:endpoint": "http://127.0.0.1:11434/v1/chat/completions",
                                    "ai:capabilities": ["tools"]
                                }
                            };
                            updateConfig(newConfig);
                            setSelectedKey(newKey);
                        }}
                        className="py-1 px-3 bg-zinc-700/50 hover:bg-zinc-600 border border-zinc-600 rounded text-[11px] text-zinc-200 cursor-pointer transition-colors flex items-center gap-2 w-full justify-start mt-1"
                    >
                        <i className="fa-solid fa-bolt text-yellow-500" />
                        Añadir Ollama
                    </button>
                    <button
                        onClick={() => {
                            let newKey = "openai-api";
                            let counter = 1;
                            while (parsedConfig[newKey]) {
                                newKey = `openai-api-${counter}`;
                                counter++;
                            }
                            const newConfig = {
                                ...parsedConfig,
                                [newKey]: {
                                    "display:name": "OpenAI (API)",
                                    "ai:provider": "openai",
                                    "ai:apitype": "openai-chat",
                                    "ai:model": "gpt-4o",
                                    "ai:capabilities": ["tools"]
                                }
                            };
                            updateConfig(newConfig);
                            setSelectedKey(newKey);
                        }}
                        className="py-1 px-3 bg-green-500/10 hover:bg-green-500/20 border border-green-500/30 rounded text-[11px] text-green-300 cursor-pointer transition-colors flex items-center gap-2 w-full justify-start mt-1"
                    >
                        <i className="fa-solid fa-brain text-green-400" />
                        Añadir OpenAI
                    </button>
                    <button
                        onClick={() => {
                            let newKey = "gemini-api";
                            let counter = 1;
                            while (parsedConfig[newKey]) {
                                newKey = `gemini-api-${counter}`;
                                counter++;
                            }
                            const newConfig = {
                                ...parsedConfig,
                                [newKey]: {
                                    "display:name": "Google Gemini",
                                    "ai:provider": "google",
                                    "ai:apitype": "google-gemini",
                                    "ai:model": "gemini-2.5-flash",
                                    "ai:capabilities": ["tools"]
                                }
                            };
                            updateConfig(newConfig);
                            setSelectedKey(newKey);
                        }}
                        className="py-1 px-3 bg-blue-500/10 hover:bg-blue-500/20 border border-blue-500/30 rounded text-[11px] text-blue-300 cursor-pointer transition-colors flex items-center gap-2 w-full justify-start mt-1"
                    >
                        <i className="fa-brands fa-google text-blue-400" />
                        Añadir Gemini
                    </button>
                </div>
                <div className="text-[10px] text-zinc-400 font-medium uppercase px-3 pt-3">Tus Modelos Listos</div>
                <div className="flex-1 overflow-y-auto w-full">
                    {configKeys.length === 0 ? (
                        <div className="p-4 text-zinc-500 text-center text-xs">No hay configuraciones aún.</div>
                    ) : (
                        <div className="flex flex-col divide-y divide-zinc-700/50">
                            {configKeys.map(key => {
                                const m = parsedConfig[key];
                                const displayName = m["display:name"] || key;
                                const isSelected = selectedKey === key;

                                return (
                                    <div
                                        key={key}
                                        className={cn(
                                            "p-3 cursor-pointer hover:bg-zinc-700/30 transition-colors flex justify-between items-center",
                                            isSelected ? "bg-zinc-700/50 border-l-2 border-accent-500" : "border-l-2 border-transparent"
                                        )}
                                        onClick={() => setSelectedKey(key)}
                                    >
                                        <div className="flex flex-col overflow-hidden">
                                            <span className="font-medium truncate">{displayName}</span>
                                            <span className="text-xs text-zinc-500 font-mono truncate">{key}</span>
                                        </div>
                                    </div>
                                );
                            })}
                        </div>
                    )}
                </div>
            </div>

            {/* Right Side - Details Editor */}
            <div className="w-2/3 flex-1 flex flex-col h-full overflow-y-auto p-6 bg-zinc-900/50">
                {!selectedKey || !selectedData ? (
                    <div className="flex h-full items-center justify-center text-zinc-500">
                        Select a Gulin IA mode to edit its properties
                    </div>
                ) : (
                    <div className="flex flex-col gap-6 max-w-2xl w-full mx-auto">
                        <div className="flex justify-between items-center">
                            <h2 className="text-xl font-semibold">Edit Mode Configuration</h2>
                            <button
                                onClick={() => handleDeleteMode(selectedKey)}
                                className="px-3 py-1.5 bg-red-500/10 text-red-500 hover:bg-red-500/20 rounded cursor-pointer transition-colors flex items-center gap-2"
                            >
                                <i className="fa-sharp fa-solid fa-trash" />
                                Delete
                            </button>
                        </div>

                        <div className="flex flex-col gap-4">
                            {/* Unique Key Editor */}
                            <div className="flex flex-col gap-1.5">
                                <label className="font-medium text-zinc-300">Mode ID (Key)</label>
                                <input
                                    type="text"
                                    className="px-3 py-2 bg-zinc-800 border fill-border border-zinc-600 rounded focus:outline-none focus:border-accent-500 font-mono text-xs"
                                    value={selectedKey}
                                    onChange={(e) => handleKeyChange(e.target.value)}
                                    placeholder="my-custom-model"
                                />
                                <span className="text-xs text-zinc-500">Must be unique, letters/numbers/hyphens only.</span>
                            </div>

                            {/* Display Name */}
                            <div className="flex flex-col gap-1.5">
                                <label className="font-medium text-zinc-300">Display Name</label>
                                <input
                                    type="text"
                                    className="px-3 py-2 bg-zinc-800 border fill-border border-zinc-600 rounded focus:outline-none focus:border-accent-500"
                                    value={selectedData["display:name"] || ""}
                                    onChange={(e) => handleUpdateField("display:name", e.target.value)}
                                    placeholder="E.g. Ollama - Llama 3"
                                />
                            </div>

                            {/* Provider & APIType */}
                            <div className="flex gap-4">
                                <div className="flex flex-col gap-1.5 flex-1">
                                    <label className="font-medium text-zinc-300">Provider</label>
                                    <select
                                        className="px-3 py-2 bg-zinc-800 border border-zinc-600 rounded focus:outline-none focus:border-accent-500"
                                        value={selectedData["ai:provider"] || ""}
                                        onChange={(e) => handleUpdateField("ai:provider", e.target.value)}
                                    >
                                        <option value="">-- Select Provider --</option>
                                        {PROVIDERS.map(p => <option key={p} value={p}>{p}</option>)}
                                    </select>
                                </div>
                                <div className="flex flex-col gap-1.5 flex-1">
                                    <label className="font-medium text-zinc-300">API Type</label>
                                    <select
                                        className="px-3 py-2 bg-zinc-800 border border-zinc-600 rounded focus:outline-none focus:border-accent-500"
                                        value={selectedData["ai:apitype"] || ""}
                                        onChange={(e) => handleUpdateField("ai:apitype", e.target.value)}
                                    >
                                        <option value="">-- Default --</option>
                                        {API_TYPES.map(p => <option key={p} value={p}>{p}</option>)}
                                    </select>
                                </div>
                            </div>

                            {/* Model */}
                            <div className="flex flex-col gap-1.5">
                                <label className="font-medium text-zinc-300">Model Name</label>
                                <input
                                    type="text"
                                    className="px-3 py-2 bg-zinc-800 border border-zinc-600 rounded focus:outline-none focus:border-accent-500 font-mono text-xs"
                                    value={selectedData["ai:model"] || ""}
                                    onChange={(e) => handleUpdateField("ai:model", e.target.value)}
                                    placeholder="llama3, gpt-4o, etc."
                                />
                            </div>

                            {/* Endpoint URL */}
                            <div className="flex flex-col gap-1.5">
                                <label className="font-medium text-zinc-300">API Endpoint (Optional)</label>
                                <input
                                    type="text"
                                    className="px-3 py-2 bg-zinc-800 border border-zinc-600 rounded focus:outline-none focus:border-accent-500 font-mono text-xs"
                                    value={selectedData["ai:endpoint"] || ""}
                                    onChange={(e) => handleUpdateField("ai:endpoint", e.target.value)}
                                    placeholder="http://localhost:11434/v1/chat/completions"
                                />
                                <span className="text-xs text-zinc-500">Required for custom providers or local Ollama / LM Studio.</span>
                            </div>

                            {/* API Token Secret Name */}
                            <div className="flex flex-col gap-1.5">
                                <label className="font-medium text-zinc-300">API Token Secret Name</label>
                                <input
                                    type="text"
                                    className="px-3 py-2 bg-zinc-800 border border-zinc-600 rounded focus:outline-none focus:border-accent-500 font-mono text-xs"
                                    value={selectedData["ai:apitokensecretname"] || ""}
                                    onChange={(e) => handleUpdateField("ai:apitokensecretname", e.target.value)}
                                    placeholder="Provide the name of a secret configured in 'Secrets'"
                                />
                            </div>

                            {/* API Token Inline */}
                            <div className="flex flex-col gap-1.5">
                                <label className="font-medium text-zinc-300">Inline API Token (Insecure)</label>
                                <input
                                    type="password"
                                    className="px-3 py-2 bg-zinc-800 border border-zinc-600 rounded focus:outline-none focus:border-accent-500 font-mono text-xs"
                                    value={selectedData["ai:apitoken"] || ""}
                                    onChange={(e) => handleUpdateField("ai:apitoken", e.target.value)}
                                    placeholder="Prefer using API Token Secret Name above"
                                />
                            </div>

                            <div className="pt-4 border-t border-zinc-700">
                                <p className="text-xs text-zinc-400">Note: Use the "Raw JSON" tab to view or edit advanced fields not shown here.</p>
                            </div>
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
});

WaveAIVisualContent.displayName = "WaveAIVisualContent";