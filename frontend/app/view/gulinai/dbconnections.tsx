// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import * as React from "react";
import * as jotai from "jotai";
import { BlockNodeModel } from "@/app/block/blocktypes";
import { TabModel } from "@/app/store/tab-model";
import { WOS, globalStore } from "@/store/global";
import { DBConnectionInfo } from "@/app/aipanel/aitypes";
import { OverlayScrollbarsComponent } from "overlayscrollbars-react";
import { getApi } from "@/store/global";
import { getWebServerEndpoint } from "@/util/endpoints";
import "./dbconnections.scss";

class DBConnectionsViewModel implements ViewModel {
    viewType: string;
    nodeModel: BlockNodeModel;
    tabModel: TabModel;
    blockId: string;
    blockAtom: jotai.Atom<Block>;
    viewIcon: jotai.Atom<string>;
    viewText: jotai.Atom<string>;
    viewName: jotai.Atom<string>;

    dbsAtom = jotai.atom<DBConnectionInfo[]>([]);
    loadingAtom = jotai.atom<boolean>(true);
    selectedConnAtom = jotai.atom<string | null>(null);
    tablesAtom = jotai.atom<string[]>([]);
    loadingTablesAtom = jotai.atom<boolean>(false);

    constructor(blockId: string, nodeModel: BlockNodeModel, tabModel: TabModel) {
        this.nodeModel = nodeModel;
        this.tabModel = tabModel;
        this.viewType = "db-connections";
        this.blockId = blockId;
        this.blockAtom = WOS.getGulinObjectAtom<Block>(`block:${blockId}`);
        this.viewIcon = jotai.atom<string>("database") as jotai.WritableAtom<string, [string], unknown>;
        this.viewName = jotai.atom<string>("DB Explorer") as jotai.WritableAtom<string, [string], unknown>;
        this.viewText = jotai.atom<string>("DB Explorer") as jotai.WritableAtom<string, [string], unknown>;
        this.loadData();
    }

    async loadData() {
        globalStore.set(this.loadingAtom, true);
        try {
            const endpoint = getWebServerEndpoint();
            const headers = { "X-AuthKey": getApi().getAuthKey() };
            const resp = await fetch(`${endpoint}/gulin/db-list`, { headers });
            if (!resp.ok) return;
            const dbs = await resp.json();
            globalStore.set(this.dbsAtom, dbs || []);
        } catch (e) {
            console.error("Error loading db connections", e);
        } finally {
            globalStore.set(this.loadingAtom, false);
        }
    }

    async selectConnection(connName: string | null) {
        globalStore.set(this.selectedConnAtom, connName);
        if (!connName) {
            globalStore.set(this.tablesAtom, []);
            return;
        }

        globalStore.set(this.loadingTablesAtom, true);
        try {
            const endpoint = getWebServerEndpoint();
            const headers = { "X-AuthKey": getApi().getAuthKey() };
            const resp = await fetch(`${endpoint}/gulin/db-schema?connection=${encodeURIComponent(connName)}`, { headers });
            if (!resp.ok) return;
            const tables = await resp.json();
            globalStore.set(this.tablesAtom, tables || []);
        } catch (e) {
            console.error("Error loading tables", e);
        } finally {
            globalStore.set(this.loadingTablesAtom, false);
        }
    }

    async exploreTable(connName: string, tableName: string) {
        try {
            const sql = `SELECT * FROM ${tableName} LIMIT 100`;
            const endpoint = getWebServerEndpoint();
            const headers = { "X-AuthKey": getApi().getAuthKey() };
            const url = `${endpoint}/gulin/db-query?connection=${encodeURIComponent(connName)}&sql=${encodeURIComponent(sql)}&tabid=${encodeURIComponent(this.tabModel.tabId)}`;

            // Execute query and create block directly
            await fetch(url, { headers });
        } catch (e) {
            console.error("Error exploring table", e);
        }
    }

    async testConnection(connName: string) {
        try {
            const endpoint = getWebServerEndpoint();
            const headers = { "X-AuthKey": getApi().getAuthKey() };
            const resp = await fetch(`${endpoint}/gulin/db-test?connection=${encodeURIComponent(connName)}`, { headers });
            if (!resp.ok) {
                const text = await resp.text();
                throw new Error(text || "Error testing connection");
            }
            alert(`Conexión '${connName}' exitosa.`);
        } catch (e: any) {
            console.error("Error testing connection", e);
            alert(`Error al probar conexión:\n${e.message}`);
        }
    }

    async deleteConnection(connName: string) {
        if (!confirm(`¿Estás seguro de que quieres eliminar la conexión '${connName}'?`)) return;
        try {
            const endpoint = getWebServerEndpoint();
            const headers = { "X-AuthKey": getApi().getAuthKey() };
            const resp = await fetch(`${endpoint}/gulin/db-delete?connection=${encodeURIComponent(connName)}`, { headers });
            if (!resp.ok) {
                const text = await resp.text();
                throw new Error(text || "Error deleting connection");
            }
            await this.loadData();
            if (globalStore.get(this.selectedConnAtom) === connName) {
                globalStore.set(this.selectedConnAtom, null);
                globalStore.set(this.tablesAtom, []);
            }
        } catch (e: any) {
            console.error("Error deleting connection", e);
            alert(`Error al eliminar conexión:\n${e.message}`);
        }
    }

    async saveConnection(connName: string, type: string, url: string) {
        try {
            const endpoint = getWebServerEndpoint();
            const headers = { 
                "X-AuthKey": getApi().getAuthKey(),
                "Content-Type": "application/json"
            };
            const resp = await fetch(`${endpoint}/gulin/db-save`, { 
                method: "POST",
                headers,
                body: JSON.stringify({ name: connName, type, url })
            });
            if (!resp.ok) {
                const text = await resp.text();
                throw new Error(text || "Error saving connection");
            }
            await this.loadData();
        } catch (e: any) {
            console.error("Error saving connection", e);
            alert(`Error al guardar conexión:\n${e.message}`);
            throw e;
        }
    }

    get viewComponent(): ViewComponent {
        return DBConnectionsView;
    }
}

function DBConnectionsView({ model }: { model: DBConnectionsViewModel }) {
    const dbs = jotai.useAtomValue(model.dbsAtom) || [];
    const loading = jotai.useAtomValue(model.loadingAtom);
    const selectedConn = jotai.useAtomValue(model.selectedConnAtom);
    const tables = jotai.useAtomValue(model.tablesAtom);
    const loadingTables = jotai.useAtomValue(model.loadingTablesAtom);

    const [editingConn, setEditingConn] = React.useState<string | null>(null);
    const [editUrl, setEditUrl] = React.useState<string>("");
    const [editType, setEditType] = React.useState<string>("");

    if (loading) {
        return (
            <div className="flex items-center justify-center h-full text-zinc-500">
                <i className="fa fa-spinner fa-spin mr-3 text-purple-500"></i> Cargando...
            </div>
        );
    }

    const renderConnections = () => (
        <div className="flex flex-col gap-3">
            {dbs.map(db => {
                const isEditing = editingConn === db.name;
                return (
                    <div
                        key={db.name}
                        className="group bg-zinc-900/60 border border-zinc-800 p-4 rounded-xl flex flex-col gap-2 hover:bg-zinc-800/80 hover:border-purple-500/50 transition-all shadow-sm hover:shadow-purple-500/10"
                    >
                        {!isEditing ? (
                            <div className="flex items-center justify-between cursor-pointer" onClick={() => model.selectConnection(db.name)}>
                                <div className="flex items-center gap-4">
                                    <div className="bg-purple-500/10 w-10 h-10 flex items-center justify-center rounded-lg text-purple-400 group-hover:bg-purple-500/20 transition-colors">
                                        <i className="fa fa-database text-sm"></i>
                                    </div>
                                    <div>
                                        <div className="text-sm font-bold text-white group-hover:text-purple-300 transition-colors">{db.name}</div>
                                        <div className="text-[10px] text-zinc-500 uppercase tracking-widest font-semibold">{db.type}</div>
                                    </div>
                                </div>
                                <div className="flex items-center gap-2" onClick={(e) => e.stopPropagation()}>
                                    <button onClick={() => model.testConnection(db.name)} className="bg-zinc-800 hover:bg-blue-500/20 text-blue-400 p-1.5 rounded-md text-[10px] transition-all" title="Probar Conexión">
                                        <i className="fa fa-plug"></i>
                                    </button>
                                    <button onClick={() => { setEditingConn(db.name); setEditUrl(db.url || ""); setEditType(db.type); }} className="bg-zinc-800 hover:bg-green-500/20 text-green-400 p-1.5 rounded-md text-[10px] transition-all" title="Editar">
                                        <i className="fa fa-pencil"></i>
                                    </button>
                                    <button onClick={() => model.deleteConnection(db.name)} className="bg-zinc-800 hover:bg-red-500/20 text-red-400 p-1.5 rounded-md text-[10px] transition-all" title="Eliminar">
                                        <i className="fa fa-trash"></i>
                                    </button>
                                    <i className="fa fa-chevron-right text-[10px] text-zinc-700 group-hover:text-purple-500 group-hover:translate-x-1 transition-all ml-2"></i>

                                </div>
                            </div>
                        ) : (
                            <div className="flex flex-col gap-3 mt-2">
                                <div className="text-sm font-bold text-white">{db.name}</div>
                                <select 
                                    value={editType} 
                                    onChange={(e) => setEditType(e.target.value)}
                                    className="bg-zinc-950 border border-zinc-800 rounded p-2 text-sm text-white focus:outline-none focus:border-purple-500"
                                >
                                    <option value="postgres">Postgres</option>
                                    <option value="mysql">MySQL</option>
                                    <option value="mssql">SQL Server</option>
                                    <option value="sqlite">SQLite</option>
                                    <option value="mongodb">MongoDB</option>
                                </select>
                                <input 
                                    type="text" 
                                    value={editUrl} 
                                    onChange={(e) => setEditUrl(e.target.value)} 
                                    placeholder="URL de Conexión (ej. postgres://user:pass@localhost:5432/db)"
                                    className="bg-zinc-950 border border-zinc-800 rounded p-2 text-sm text-white focus:outline-none focus:border-purple-500 w-full"
                                />
                                <div className="flex gap-2 justify-end mt-2">
                                    <button onClick={() => setEditingConn(null)} className="px-3 py-1.5 text-xs text-zinc-400 hover:text-white bg-zinc-800 hover:bg-zinc-700 rounded transition-colors">Cancelar</button>
                                    <button onClick={async () => { await model.saveConnection(db.name, editType, editUrl); setEditingConn(null); }} className="px-3 py-1.5 text-xs text-white bg-purple-600 hover:bg-purple-500 rounded transition-colors shadow-sm shadow-purple-500/20">Guardar</button>
                                </div>
                            </div>
                        )}
                    </div>
                );
            })}
            {dbs.length === 0 && (
                <div className="text-center py-12 border-2 border-dashed border-zinc-800 rounded-2xl bg-zinc-900/20">
                    <i className="fa fa-plus-circle text-zinc-700 text-3xl mb-3"></i>
                    <p className="text-sm text-zinc-500 font-medium">No hay bases de datos.</p>
                    <p className="text-[10px] text-zinc-700 mt-2 uppercase tracking-[0.2em]">Usa db_register_connection</p>
                </div>
            )}
        </div>
    );

    const renderTables = () => (
        <div className="flex flex-col gap-2 animate-in fade-in slide-in-from-right-4 duration-300">
            <div className="flex items-center gap-2 mb-4">
                <button
                    onClick={() => model.selectConnection(null)}
                    className="p-2 hover:bg-zinc-800 rounded-lg text-zinc-500 hover:text-white transition-colors"
                >
                    <i className="fa fa-arrow-left text-xs"></i>
                </button>
                <div className="flex flex-col">
                    <span className="text-[9px] text-zinc-500 uppercase font-bold tracking-tighter">Explorando</span>
                    <span className="text-sm font-bold text-purple-400">{selectedConn}</span>
                </div>
            </div>

            {loadingTables ? (
                <div className="flex flex-col items-center justify-center py-12 gap-3 text-zinc-600">
                    <i className="fa fa-circle-notch fa-spin text-xl text-purple-500"></i>
                    <span className="text-[10px] uppercase font-bold tracking-widest">Leyendo esquema...</span>
                </div>
            ) : (
                <div className="grid grid-cols-1 gap-2">
                    {tables.map(table => (
                        <div
                            key={table}
                            className="bg-zinc-900/40 border border-zinc-800/50 p-3 rounded-lg flex items-center justify-between hover:bg-purple-500/5 hover:border-purple-500/30 transition-all cursor-default group"
                        >
                            <div className="flex items-center gap-3">
                                <i className="fa fa-table text-xs text-zinc-600 group-hover:text-purple-500 transition-colors"></i>
                                <span className="text-xs text-zinc-300 font-mono group-hover:text-white transition-colors">{table}</span>
                            </div>
                            <button
                                onClick={() => selectedConn && model.exploreTable(selectedConn, table)}
                                className="opacity-0 group-hover:opacity-100 bg-purple-500/10 hover:bg-purple-500/20 text-purple-400 p-1.5 rounded-md text-[10px] transition-all"
                                title="Ver registros"
                            >
                                <i className="fa fa-eye"></i>
                            </button>
                        </div>
                    ))}
                    {tables.length === 0 && <p className="text-center text-xs text-zinc-600 py-8 italic">No se encontraron tablas.</p>}
                </div>
            )}
        </div>
    );

    return (
        <OverlayScrollbarsComponent
            className="db-connections-container h-full p-6 bg-zinc-950 text-secondary overflow-y-auto"
            options={{ scrollbars: { autoHide: "leave" } }}
        >
            <div className="flex flex-col h-full">
                {!selectedConn && (
                    <h2 className="text-2xl font-black text-white flex items-center gap-3 mb-8 tracking-tight">
                        <i className="fa fa-database text-purple-600 size-8 flex items-center justify-center bg-purple-600/10 rounded-xl shadow-lg shadow-purple-900/20"></i>
                        DB Explorer
                    </h2>
                )}

                {selectedConn ? renderTables() : renderConnections()}
            </div>
        </OverlayScrollbarsComponent>
    );
}

export { DBConnectionsViewModel };
