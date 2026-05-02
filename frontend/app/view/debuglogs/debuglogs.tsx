// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import * as React from "react";
import * as jotai from "jotai";
import { BlockNodeModel } from "@/app/block/blocktypes";
import { TabModel } from "@/app/store/tab-model";
import { WOS, globalStore } from "@/store/global";
import { GulinAIModel } from "@/app/aipanel/gulinai-model";
import { cn } from "@/util/util";
import { OverlayScrollbarsComponent } from "overlayscrollbars-react";

const LOG_COLORS: Record<string, string> = {
    API: "text-blue-400 border-blue-500/30 bg-blue-500/5",
    TERM: "text-green-400 border-green-500/30 bg-green-500/5",
    FILE: "text-amber-400 border-amber-500/30 bg-amber-500/5",
    DB: "text-purple-400 border-purple-500/30 bg-purple-500/5",
    AI: "text-emerald-400 border-emerald-500/30 bg-emerald-500/5",
    PLAI: "text-rose-400 border-rose-500/30 bg-rose-500/5",
};

const DEFAULT_COLOR = "text-gray-400 border-gray-500/30 bg-gray-500/5";

function maskSensitiveData(text: string): string {
    return text.replace(/(token|key|auth|password|secret|pwd)=([^&\s]+)/gi, (match, p1, p2) => {
        return `${p1}=${p2.substring(0, 4)}***${p2.substring(p2.length - 4)}`;
    });
}

class DebugLogsViewModel implements ViewModel {
    viewType: string;
    nodeModel: BlockNodeModel;
    tabModel: TabModel;
    blockId: string;
    blockAtom: jotai.Atom<Block>;
    viewIcon: jotai.Atom<string>;
    viewName: jotai.Atom<string>;

    constructor(blockId: string, nodeModel: BlockNodeModel, tabModel: TabModel) {
        this.nodeModel = nodeModel;
        this.tabModel = tabModel;
        this.viewType = "debug-logs";
        this.blockId = blockId;
        this.blockAtom = WOS.getGulinObjectAtom<Block>(`block:${blockId}`);
        this.viewIcon = jotai.atom("bug");
        this.viewName = jotai.atom("Logs de Depuración");
    }

    get viewComponent(): ViewComponent {
        return DebugLogsView;
    }
}

function DebugLogsView({ model }: { model: DebugLogsViewModel }) {
    const aiModel = GulinAIModel.getInstance();
    const logs = jotai.useAtomValue(aiModel.debugLogs);
    const [filters, setFilters] = jotai.useAtom(aiModel.debugFilters);
    const scrollRef = React.useRef<any>(null);
    const [autoScroll, setAutoScroll] = React.useState(true);

    const filteredLogs = logs.filter(log => filters.includes(log.category));

    const toggleFilter = (cat: string) => {
        if (filters.includes(cat)) {
            setFilters(filters.filter(f => f !== cat));
        } else {
            setFilters([...filters, cat]);
        }
    };

    const categories = ["API", "TERM", "FILE", "DB", "AI", "PLAI"];

    return (
        <div className="flex flex-col h-full bg-zinc-950 text-white font-sans overflow-hidden">
            <header className="p-4 border-b border-white/10 flex items-center justify-between bg-zinc-900/50">
                <div className="flex items-center gap-3">
                    <i className="fa fa-bug text-rose-500 text-xl"></i>
                    <div>
                        <h1 className="text-lg font-bold">Logs de Depuración Universal</h1>
                        <p className="text-[10px] text-muted uppercase tracking-widest opacity-50">Diagnostic Console v1.0</p>
                    </div>
                </div>
                <div className="flex items-center gap-2">
                    <button 
                        onClick={() => aiModel.clearDebugLogs()}
                        className="px-3 py-1.5 bg-white/5 hover:bg-white/10 rounded-md border border-white/10 text-xs transition-all flex items-center gap-2"
                    >
                        <i className="fa fa-trash-can"></i> Limpiar
                    </button>
                </div>
            </header>

            <div className="p-3 flex flex-wrap gap-2 bg-zinc-900/30 border-b border-white/5">
                <button
                    onClick={() => setFilters(categories)}
                    className="px-3 py-1 rounded-md text-[10px] font-bold border border-zinc-700 text-zinc-300 bg-zinc-800 hover:bg-zinc-700 transition-all"
                >
                    TODOS
                </button>
                {categories.map(cat => (
                    <button
                        key={cat}
                        onClick={() => toggleFilter(cat)}
                        className={cn(
                            "px-3 py-1 rounded-md text-[10px] font-bold border transition-all",
                            filters.includes(cat) 
                                ? (LOG_COLORS[cat] || DEFAULT_COLOR) + " opacity-100 border-white/20 shadow-[0_0_10px_rgba(255,255,255,0.1)]"
                                : "text-zinc-500 border-zinc-800 bg-transparent opacity-60 hover:opacity-100"
                        )}
                    >
                        {cat}
                    </button>
                ))}
            </div>

            <OverlayScrollbarsComponent
                className="flex-1 overflow-y-auto"
                options={{ scrollbars: { autoHide: "leave" } }}
                ref={scrollRef}
            >
                <div className="p-4 space-y-3">
                    {filteredLogs.length === 0 ? (
                        <div className="h-64 flex flex-col items-center justify-center opacity-20">
                            <i className="fa fa-terminal text-6xl mb-4"></i>
                            <p className="italic">No hay eventos registrados bajo los filtros actuales.</p>
                        </div>
                    ) : (
                        filteredLogs.map((log) => (
                            <div 
                                key={log.id} 
                                className={cn(
                                    "p-3 rounded-lg border font-mono text-xs transition-all hover:bg-white/[0.02]",
                                    LOG_COLORS[log.category] || DEFAULT_COLOR
                                )}
                            >
                                <div className="flex items-center justify-between mb-2 opacity-60">
                                    <span className="font-bold uppercase tracking-tighter bg-white/10 px-1.5 py-0.5 rounded">{log.category}</span>
                                    <span>{new Date(log.ts).toLocaleString()}</span>
                                </div>
                                <div className="whitespace-pre-wrap break-all leading-relaxed">
                                    {maskSensitiveData(log.message)}
                                </div>
                            </div>
                        ))
                    )}
                </div>
            </OverlayScrollbarsComponent>

            <footer className="p-2 border-t border-white/10 bg-zinc-950 flex justify-between items-center text-[10px] text-zinc-500">
                <span>{filteredLogs.length} eventos en total</span>
                <div className="flex items-center gap-3">
                    <span className="flex items-center gap-1">
                        <div className="w-1.5 h-1.5 rounded-full bg-emerald-500 shadow-[0_0_5px_rgba(16,185,129,0.5)]" />
                        Sistema Activo
                    </span>
                </div>
            </footer>
        </div>
    );
}

export { DebugLogsViewModel };
