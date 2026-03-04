// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { useAtomValue } from "jotai";
import { memo } from "react";
import { WaveAIModel } from "./waveai-model";
import { cn, formatRelativeTime } from "@/util/util";

export const AIHistorySidebar = memo(() => {
    const model = WaveAIModel.getInstance();
    const isOpen = useAtomValue(model.isSidebarOpen);
    const summaries = useAtomValue(model.chatSummaries);
    const isLoading = useAtomValue(model.isLoadingChatSummaries);
    const activeChatId = useAtomValue(model.chatId);

    if (!isOpen) {
        return null;
    }

    return (
        <div className="absolute inset-0 z-[100] flex min-w-0">
            {/* Backdrop */}
            <div
                className="absolute inset-0 bg-black/40 backdrop-blur-sm"
                onClick={() => model.toggleSidebar(false)}
            />

            {/* Sidebar Content */}
            <div className="relative w-[280px] h-full bg-zinc-900 border-r border-zinc-700 shadow-2xl flex flex-col p-0 transition-transform duration-200">
                <div className="p-4 border-b border-zinc-800 flex items-center justify-between">
                    <h3 className="text-primary font-bold flex items-center gap-2">
                        <i className="fa fa-history text-accent"></i>
                        Historial de Chats
                    </h3>
                    <button
                        onClick={() => model.toggleSidebar(false)}
                        className="text-muted hover:text-primary transition-colors cursor-pointer"
                    >
                        <i className="fa fa-times"></i>
                    </button>
                </div>

                <div className="flex-1 overflow-y-auto custom-scrollbar">
                    {isLoading && summaries.length === 0 ? (
                        <div className="p-8 text-center text-muted">
                            <i className="fa fa-spinner fa-spin text-2xl mb-2"></i>
                            <p className="text-xs">Cargando historial...</p>
                        </div>
                    ) : (
                        <div className="py-2">
                            {summaries.length === 0 ? (
                                <div className="p-8 text-center text-muted text-xs">
                                    No hay chats anteriores.
                                </div>
                            ) : (
                                summaries.map((s) => (
                                    <div
                                        key={s.chatid}
                                        className={cn(
                                            "group relative flex flex-col px-4 py-3 cursor-pointer hover:bg-zinc-800 transition-colors border-l-2",
                                            activeChatId === s.chatid ? "bg-zinc-800 border-accent" : "border-transparent"
                                        )}
                                        onClick={() => model.switchToChat(s.chatid)}
                                    >
                                        <div className="flex justify-between items-start mb-1">
                                            <span className="text-[10px] text-accent uppercase font-bold tracking-tight">
                                                {s.model || "Default Model"}
                                            </span>
                                            <div className="flex items-center gap-2">
                                                <button
                                                    onClick={(e) => {
                                                        e.stopPropagation();
                                                        // model.exportChatLog(s.chatid);
                                                    }}
                                                    className="opacity-0 group-hover:opacity-100 text-muted hover:text-accent transition-all p-1"
                                                    title="Exportar Log"
                                                >
                                                    <i className="fa fa-download"></i>
                                                </button>
                                                <span className="text-[10px] text-muted">
                                                    {formatRelativeTime(s.lastupdate)}
                                                </span>
                                            </div>
                                        </div>
                                        <p className="text-sm text-primary line-clamp-2 leading-snug">
                                            {s.snippet || "(Sin contenido)"}
                                        </p>
                                        <div className="mt-1 text-[10px] text-muted flex items-center gap-1">
                                            <i className="fa fa-message text-[8px]"></i>
                                            {s.messagecount} mensajes
                                        </div>
                                    </div>
                                ))
                            )}
                        </div>
                    )}
                </div>

                <div className="p-4 border-t border-zinc-800">
                    <button
                        onClick={() => {
                            model.clearChat();
                            model.toggleSidebar(false);
                        }}
                        className="w-full py-2 bg-accent hover:bg-accent-600 text-white rounded font-bold text-sm transition-colors flex items-center justify-center gap-2 cursor-pointer shadow-lg"
                    >
                        <i className="fa fa-plus"></i>
                        Nuevo Chat
                    </button>
                </div>
            </div>
        </div>
    );
});

AIHistorySidebar.displayName = "AIHistorySidebar";
