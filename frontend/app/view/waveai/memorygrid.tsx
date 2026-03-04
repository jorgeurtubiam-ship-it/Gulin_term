// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import * as React from "react";
import * as jotai from "jotai";
import { BlockNodeModel } from "@/app/block/blocktypes";
import { TabModel } from "@/app/store/tab-model";
import { WOS, globalStore } from "@/store/global";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import { ChatSummary, BrainSummary } from "@/app/aipanel/aitypes";
import { OverlayScrollbarsComponent } from "overlayscrollbars-react";
import clsx from "clsx";
import dayjs from "dayjs";
import relativeTime from "dayjs/plugin/relativeTime";
import "./memorygrid.scss";

import { WaveAIModel } from "@/app/aipanel/waveai-model";
import { getApi } from "@/store/global";
import { getWebServerEndpoint } from "@/util/endpoints";

dayjs.extend(relativeTime);

class MemoryGridViewModel implements ViewModel {
    viewType: string;
    nodeModel: BlockNodeModel;
    tabModel: TabModel;
    blockId: string;
    blockAtom: jotai.Atom<Block>;
    viewIcon: jotai.Atom<string>;
    viewText: jotai.Atom<string>;
    viewName: jotai.Atom<string>;

    chatsAtom = jotai.atom<ChatSummary[]>([]);
    brainsAtom = jotai.atom<BrainSummary[]>([]);
    loadingAtom = jotai.atom<boolean>(true);

    constructor(blockId: string, nodeModel: BlockNodeModel, tabModel: TabModel) {
        this.nodeModel = nodeModel;
        this.tabModel = tabModel;
        this.viewType = "memory-grid";
        this.blockId = blockId;
        this.blockAtom = WOS.getWaveObjectAtom<Block>(`block:${blockId}`);
        this.viewIcon = jotai.atom("brain");
        this.viewName = jotai.atom("Memoria y Chats");
        this.loadData();
    }

    async loadData() {
        globalStore.set(this.loadingAtom, true);
        try {
            const endpoint = getWebServerEndpoint();
            const headers = {
                "X-AuthKey": getApi().getAuthKey()
            };
            const [chatsResp, brainsResp] = await Promise.all([
                fetch(`${endpoint}/wave/chat-list`, { headers }),
                fetch(`${endpoint}/wave/brain-list`, { headers })
            ]);
            if (!chatsResp.ok || !brainsResp.ok) {
                console.error("Failed to load memory grid data", chatsResp.status, brainsResp.status);
                return;
            }
            const chats = await chatsResp.json();
            const brains = await brainsResp.json();
            globalStore.set(this.chatsAtom, chats || []);
            globalStore.set(this.brainsAtom, brains || []);
        } catch (e) {
            console.error("Error loading memory grid data", e);
        } finally {
            globalStore.set(this.loadingAtom, false);
        }
    }

    handleChatClick(chatId: string) {
        console.log("MemoryGrid: Switching to chat", chatId);
        const aiModel = WaveAIModel.getInstance();
        aiModel.switchToChat(chatId);
        aiModel.toggleSidebar(false);
        aiModel.focusInput();
    }

    async handleBrainClick(filename: string) {
        const aiModel = WaveAIModel.getInstance();
        // For now, let's just open the AI panel and ask about this memory
        aiModel.focusInput();
        aiModel.appendText(`@Gulin recuérdame lo que sabes sobre este tema del archivo: ${filename}`, true);
    }

    get viewComponent(): ViewComponent {
        return MemoryGridView;
    }
}

function MemoryGridView({ model }: { model: MemoryGridViewModel }) {
    const chats = jotai.useAtomValue(model.chatsAtom) || [];
    const brains = jotai.useAtomValue(model.brainsAtom) || [];
    const loading = jotai.useAtomValue(model.loadingAtom);

    if (loading) {
        return (
            <div className="flex items-center justify-center h-full text-muted">
                <i className="fa fa-spinner fa-spin mr-2"></i> Cargando memorias...
            </div>
        );
    }

    return (
        <OverlayScrollbarsComponent
            className="memory-grid-container h-full p-6 bg-zinc-950 text-secondary overflow-y-auto"
            options={{ scrollbars: { autoHide: "leave" } }}
        >
            <div className="max-w-6xl mx-auto">
                <header className="mb-10 text-center">
                    <h1 className="text-4xl font-bold text-white mb-2 flex items-center justify-center gap-3">
                        <i className="fa fa-brain text-accent"></i>
                        Gulin Dashboard
                    </h1>
                    <p className="text-muted text-lg">
                        Centro de control de tus memorias y conversaciones pasadas.
                    </p>
                </header>

                <section className="mb-12">
                    <div className="flex items-center justify-between mb-6">
                        <h2 className="text-xl font-semibold text-white flex items-center gap-2">
                            <i className="fa fa-comments text-blue-400"></i>
                            Conversaciones Recientes
                        </h2>
                    </div>
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                        {chats.map(chat => (
                            <div
                                key={chat.chatid}
                                onClick={() => model.handleChatClick(chat.chatid)}
                                className="group bg-zinc-900/50 border border-zinc-800 p-5 rounded-xl hover:border-blue-500/50 hover:bg-zinc-800/80 transition-all cursor-pointer shadow-lg hover:shadow-blue-900/20 flex flex-col h-[180px]"
                            >
                                <div className="flex justify-between items-start mb-3">
                                    <span className="text-[10px] bg-blue-500/20 text-blue-300 px-2 py-0.5 rounded uppercase font-bold tracking-wider">
                                        {chat.model || "AI Chat"}
                                    </span>
                                    <span className="text-[11px] text-muted">
                                        {dayjs(chat.lastupdate).fromNow()}
                                    </span>
                                </div>
                                <p className="text-sm line-clamp-3 text-zinc-300 leading-relaxed flex-grow">
                                    {chat.snippet || "Sin contenido..."}
                                </p>
                                <div className="mt-4 pt-3 border-t border-zinc-800/50 flex items-center justify-between text-[11px] text-muted">
                                    <span className="flex items-center gap-1">
                                        <i className="fa fa-message text-[9px]"></i>
                                        {chat.messagecount} mensajes
                                    </span>
                                    <span className="text-blue-400 opacity-0 group-hover:opacity-100 transition-all flex items-center gap-1">
                                        Continuar <i className="fa fa-arrow-right text-[9px]"></i>
                                    </span>
                                </div>
                            </div>
                        ))}
                    </div>
                </section>

                <section>
                    <div className="flex items-center justify-between mb-6">
                        <h2 className="text-xl font-semibold text-white flex items-center gap-2">
                            <i className="fa fa-book text-amber-500"></i>
                            Memoria a Largo Plazo (Brain)
                        </h2>
                    </div>
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                        {brains.map(brain => (
                            <div
                                key={brain.filename}
                                onClick={() => model.handleBrainClick(brain.filename)}
                                className="group bg-zinc-900/50 border border-zinc-800 p-5 rounded-xl hover:border-amber-500/50 hover:bg-zinc-800/80 transition-all cursor-pointer shadow-lg hover:shadow-amber-900/20 flex flex-col h-[180px]"
                            >
                                <div className="flex justify-between items-start mb-3">
                                    <span className="text-sm font-bold text-amber-200">
                                        {brain.title}
                                    </span>
                                    <span className="text-[11px] text-muted">
                                        {dayjs(brain.lastupdate).fromNow()}
                                    </span>
                                </div>
                                <p className="text-sm line-clamp-3 text-zinc-400 leading-relaxed italic flex-grow">
                                    "{brain.snippet}..."
                                </p>
                                <div className="mt-4 pt-3 border-t border-zinc-800/50 flex items-center justify-between text-[11px] text-muted">
                                    <span className="flex items-center gap-1 uppercase tracking-tighter text-[9px] text-amber-500/50">
                                        Reflective Memory
                                    </span>
                                    <span className="text-amber-500 opacity-0 group-hover:opacity-100 transition-all flex items-center gap-1">
                                        Consultar <i className="fa fa-magnifying-glass text-[9px]"></i>
                                    </span>
                                </div>
                            </div>
                        ))}
                    </div>
                </section>
            </div>
        </OverlayScrollbarsComponent>
    );
}

export { MemoryGridViewModel };
