import { getApi } from "@/app/store/global";
import { fetch } from "@/util/fetchutil";
import { getWebServerEndpoint } from "@/util/endpoints";
import { atom, useAtom } from "jotai";
import { useEffect, useState } from "react";
import { OverlayScrollbarsComponent } from "overlayscrollbars-react";
import { BlockNodeModel } from "@/app/block/blocktypes";
import { TabModel } from "@/app/store/tab-model";
import { CodeEditor } from "@/app/view/codeeditor/codeeditor";
import "./gulinai.scss";

interface PluginInfo {
    name: string;
}

const pluginsAtom = atom<string[]>([]);
const loadingAtom = atom<boolean>(false);

export class PluginManagerViewModel implements ViewModel {
    viewType = "plugin-manager";
    viewComponent = PluginManagerView;
    blockId: string;
    nodeModel: BlockNodeModel;
    tabModel: TabModel;

    constructor(blockId: string, nodeModel: BlockNodeModel, tabModel: TabModel) {
        this.blockId = blockId;
        this.nodeModel = nodeModel;
        this.tabModel = tabModel;
    }

    async fetchPlugins(setPlugins: (val: string[]) => void, setLoading: (val: boolean) => void) {
        setLoading(true);
        try {
            const endpoint = getWebServerEndpoint();
            const headers = { "X-AuthKey": getApi().getAuthKey() };
            const resp = await fetch(`${endpoint}/gulin/plugin-list`, { headers });
            if (resp.ok) {
                const data = await resp.json();
                setPlugins(data || []);
            }
        } catch (e) {
            console.error("Error fetching plugins", e);
        } finally {
            setLoading(false);
        }
    }

    async readPlugin(name: string): Promise<string> {
        try {
            const endpoint = getWebServerEndpoint();
            const headers = { "X-AuthKey": getApi().getAuthKey() };
            const resp = await fetch(`${endpoint}/gulin/plugin-read?name=${name}`, { headers });
            if (resp.ok) {
                return await resp.text();
            }
        } catch (e) {
            console.error("Error reading plugin", e);
        }
        return "";
    }

    async savePlugin(name: string, content: string) {
        try {
            const endpoint = getWebServerEndpoint();
            const headers = {
                "X-AuthKey": getApi().getAuthKey(),
                "Content-Type": "application/json"
            };
            const resp = await fetch(`${endpoint}/gulin/plugin-save`, {
                method: "POST",
                headers,
                body: JSON.stringify({ name, content })
            });
            return resp.ok;
        } catch (e) {
            console.error("Error saving plugin", e);
            return false;
        }
    }

    async deletePlugin(name: string) {
        try {
            const endpoint = getWebServerEndpoint();
            const headers = {
                "X-AuthKey": getApi().getAuthKey(),
                "Content-Type": "application/json"
            };
            const resp = await fetch(`${endpoint}/gulin/plugin-delete`, {
                method: "POST",
                headers,
                body: JSON.stringify({ name })
            });
            return resp.ok;
        } catch (e) {
            console.error("Error deleting plugin", e);
            return false;
        }
    }
}

export function PluginManagerView({ model, blockId }: { model: PluginManagerViewModel, blockId: string }) {
    const [plugins, setPlugins] = useAtom(pluginsAtom);
    const [loading, setLoading] = useAtom(loadingAtom);
    const [selectedPlugin, setSelectedPlugin] = useState<string | null>(null);
    const [content, setContent] = useState("");
    const [isEditing, setIsEditing] = useState(false);
    const [pluginName, setPluginName] = useState("");

    useEffect(() => {
        model.fetchPlugins(setPlugins, setLoading);
    }, []);

    const handleSelect = async (name: string) => {
        setSelectedPlugin(name);
        setPluginName(name);
        const text = await model.readPlugin(name);
        setContent(text);
        setIsEditing(true);
    };

    const handleNew = () => {
        setSelectedPlugin(null);
        setPluginName("nuevo_plugin.js");
        setContent("// @name: mi_herramienta\n// @description: Descripción aquí\n\nfunction execute(args) {\n    return \"Hola Mundo\";\n}");
        setIsEditing(true);
    };

    const handleSave = async () => {
        const success = await model.savePlugin(pluginName, content);
        if (success) {
            setIsEditing(false);
            model.fetchPlugins(setPlugins, setLoading);
        } else {
            alert("Error al guardar el plugin");
        }
    };

    return (
        <div className="plugin-manager-view bg-[#1a1a1a] h-full flex text-slate-200 font-sans">
            {/* Sidebar Lista */}
            <div className="w-64 border-r border-white/5 flex flex-col">
                <div className="p-4 border-b border-white/5 bg-gradient-to-b from-blue-900/10 to-transparent">
                    <h2 className="text-sm font-bold uppercase tracking-widest text-blue-400">Plugins Dinámicos</h2>
                    <button 
                        onClick={handleNew}
                        className="w-full mt-4 bg-blue-600 hover:bg-blue-500 text-white py-2 rounded-lg text-xs font-bold transition-all shadow-lg shadow-blue-900/20 active:scale-95"
                    >
                        + Nuevo Plugin
                    </button>
                </div>
                <OverlayScrollbarsComponent className="flex-1" options={{ scrollbars: { autoHide: "leave" } }}>
                    <div className="p-2 space-y-1">
                        {plugins.map(p => (
                            <div 
                                key={p} 
                                onClick={() => handleSelect(p)}
                                className={`p-3 rounded-xl cursor-pointer transition-all flex items-center justify-between group ${selectedPlugin === p ? 'bg-blue-600/20 border border-blue-500/30' : 'hover:bg-white/5 border border-transparent'}`}
                            >
                                <div className="flex items-center gap-2 overflow-hidden">
                                    <i className={`fa fa-bolt ${selectedPlugin === p ? 'text-blue-400' : 'text-slate-500'}`}></i>
                                    <span className="text-sm truncate font-medium">{p}</span>
                                </div>
                                <button 
                                    onClick={(e) => { e.stopPropagation(); if(confirm("¿Borrar plugin?")) model.deletePlugin(p).then(() => model.fetchPlugins(setPlugins, setLoading)); }}
                                    className="opacity-0 group-hover:opacity-100 p-1.5 hover:bg-red-500/20 rounded-md text-red-400 transition-all"
                                >
                                    <i className="fa fa-trash text-[10px]"></i>
                                </button>
                            </div>
                        ))}
                    </div>
                </OverlayScrollbarsComponent>
            </div>

            {/* Editor Area */}
            <div className="flex-1 flex flex-col bg-[#0d0d0d]">
                {isEditing ? (
                    <>
                        <div className="p-4 border-b border-white/5 flex items-center justify-between bg-black/40 backdrop-blur-md">
                            <div className="flex items-center gap-4">
                                <input 
                                    value={pluginName}
                                    onChange={(e) => setPluginName(e.target.value)}
                                    className="bg-white/5 border border-white/10 rounded-lg px-3 py-1.5 text-sm font-mono text-blue-300 focus:ring-1 focus:ring-blue-500 outline-none w-64"
                                />
                                <span className="text-[10px] bg-white/5 px-2 py-1 rounded text-slate-500 uppercase tracking-tighter font-bold border border-white/5">Javascript Runtime</span>
                            </div>
                            <div className="flex gap-2">
                                <button onClick={() => setIsEditing(false)} className="px-4 py-1.5 rounded-lg text-sm font-medium text-slate-400 hover:bg-white/5 transition-all">Cancelar</button>
                                <button onClick={handleSave} className="bg-blue-600 hover:bg-blue-500 text-white px-6 py-1.5 rounded-lg text-sm font-bold transition-all shadow-lg shadow-blue-900/20 active:scale-95">Guardar Cambios</button>
                            </div>
                        </div>
                        <div className="flex-1 relative">
                            <CodeEditor 
                                blockId={blockId}
                                text={content}
                                language="javascript"
                                fileName={pluginName}
                                readonly={false}
                                onChange={(t) => setContent(t)}
                            />
                        </div>
                    </>
                ) : (
                    <div className="flex-1 flex flex-col items-center justify-center text-slate-500 bg-[radial-gradient(circle_at_center,_var(--tw-gradient-stops))] from-blue-900/5 via-transparent to-transparent">
                        <div className="w-20 h-20 rounded-3xl bg-white/5 flex items-center justify-center mb-6 border border-white/5 shadow-2xl shadow-blue-900/10">
                            <i className="fa fa-bolt fa-3x text-blue-500/20"></i>
                        </div>
                        <h3 className="text-xl font-bold text-white mb-2">Selecciona un plugin para editar</h3>
                        <p className="text-sm text-slate-400 max-w-xs text-center leading-relaxed">Aquí podrás administrar las "manos" de Gulin. Los plugins dinámicos permiten que la IA aprenda nuevas APIs sin reiniciar.</p>
                    </div>
                )}
            </div>
        </div>
    );
}

export const PluginManagerViewModel_Prop = PluginManagerViewModel;
