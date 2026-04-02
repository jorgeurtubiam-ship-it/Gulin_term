// Copyright 2026, GuLiN Terminal
// SPDX-License-Identifier: Apache-2.0

import { getApi } from "@/app/store/global";
import { getWebServerEndpoint } from "@/util/endpoints";
import { fetch } from "@/util/fetchutil";
import { makeIconClass } from "@/util/util";
import clsx from "clsx";
import { atom, useAtom } from "jotai";
import { useEffect, useState } from "react";
import { OverlayScrollbarsComponent } from "overlayscrollbars-react";
import { BlockNodeModel } from "@/app/block/blocktypes";
import { TabModel } from "@/app/store/tab-model";
import "./widgetbuilder.scss";

// ──────────────────────────────────────────────
// Types
// ──────────────────────────────────────────────

interface WidgetItem {
    id: string;
    label: string;
    icon: string;
    color: string;
    description: string;
    blockdef: {
        meta?: Record<string, unknown>;
    };
    "display:order"?: number;
}

type ViewType = "term" | "preview" | "web" | "gulinai" | "sysinfo" | "dashboard" | "codeeditor";

const VIEW_OPTIONS: { value: ViewType; label: string; icon: string }[] = [
    { value: "term", label: "Terminal", icon: "terminal" },
    { value: "preview", label: "Preview / Archivo", icon: "file" },
    { value: "web", label: "Navegador Web", icon: "globe" },
    { value: "gulinai", label: "GuLiN AI Chat", icon: "sparkles" },
    { value: "sysinfo", label: "Info del Sistema", icon: "microchip" },
    { value: "dashboard", label: "Dashboard", icon: "chart-bar" },
    { value: "codeeditor", label: "Editor de Código", icon: "code" },
];

const POPULAR_ICONS = [
    "terminal", "globe", "file", "folder", "sparkles", "code", "server",
    "database", "chart-bar", "chart-line", "microchip", "network-wired",
    "shield-alt", "key", "lock", "cloud", "robot", "brain", "bolt",
    "fire", "star", "heart", "bookmark", "tag", "wrench", "gear",
    "boxes", "cube", "layer-group", "sitemap", "code-branch",
    "docker", "python", "js", "react", "house", "magnifying-glass",
];

const PRESET_COLORS = [
    "#6366f1", "#8b5cf6", "#a855f7", "#ec4899", "#ef4444",
    "#f97316", "#eab308", "#22c55e", "#14b8a6", "#06b6d4",
    "#3b82f6", "#ffffff", "#94a3b8",
];

// ──────────────────────────────────────────────
// Atoms
// ──────────────────────────────────────────────

const widgetsAtom = atom<WidgetItem[]>([]);
const loadingAtom = atom<boolean>(false);

// ──────────────────────────────────────────────
// ViewModel
// ──────────────────────────────────────────────

export class WidgetBuilderViewModel implements ViewModel {
    viewType = "widget-builder";
    viewIcon = atom("wand-magic-sparkles");
    viewName = atom("Widget Builder");
    viewComponent = WidgetBuilderView;
    blockId: string;
    nodeModel: BlockNodeModel;
    tabModel: TabModel;

    constructor(blockId: string, nodeModel: BlockNodeModel, tabModel: TabModel) {
        this.blockId = blockId;
        this.nodeModel = nodeModel;
        this.tabModel = tabModel;
    }

    getHeaders() {
        return {
            "X-AuthKey": getApi().getAuthKey(),
            "Content-Type": "application/json",
        };
    }

    async fetchWidgets(setWidgets: (w: WidgetItem[]) => void, setLoading: (v: boolean) => void) {
        setLoading(true);
        try {
            const base = getWebServerEndpoint();
            const resp = await fetch(`${base}/gulin/widgets-list`, { headers: { "X-AuthKey": getApi().getAuthKey() } });
            if (resp.ok) {
                const data = await resp.json();
                setWidgets(data?.data || []);
            }
        } catch (e) {
            console.error("Widget fetch error", e);
        } finally {
            setLoading(false);
        }
    }

    async saveWidget(widget: Partial<WidgetItem>) {
        const base = getWebServerEndpoint();
        const resp = await fetch(`${base}/gulin/widgets-save`, {
            method: "POST",
            headers: this.getHeaders(),
            body: JSON.stringify(widget),
        });
        if (!resp.ok) throw new Error(await resp.text());
        return await resp.json();
    }

    async deleteWidget(id: string) {
        const base = getWebServerEndpoint();
        const resp = await fetch(`${base}/gulin/widgets-delete?id=${encodeURIComponent(id)}`, {
            method: "DELETE",
            headers: { "X-AuthKey": getApi().getAuthKey() },
        });
        if (!resp.ok) throw new Error(await resp.text());
    }
}

// ──────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────

function generateId(label: string) {
    return (
        "custom@" +
        label
            .toLowerCase()
            .replace(/\s+/g, "-")
            .replace(/[^a-z0-9-]/g, "") +
        "-" +
        Date.now().toString(36)
    );
}

function defaultBlockDef(view: ViewType, extra: Record<string, string>): Record<string, unknown> {
    const meta: Record<string, unknown> = { view };
    if (view === "web" && extra.url) meta.url = extra.url;
    if (view === "preview" && extra.file) meta.file = extra.file;
    if (view === "term" && extra.cmd) {
        meta.cmd = extra.cmd;
        meta["cmd:interactive"] = true;
    }
    return { meta };
}

// ──────────────────────────────────────────────
// Sub-components
// ──────────────────────────────────────────────

function LivePreviewTile({ label, icon, color }: { label: string; icon: string; color: string }) {
    return (
        <div className="wb-preview-container">
            <span className="wb-preview-label">Preview del tile</span>
            <div className="wb-preview-tile">
                <div style={{ color: color || "#6366f1" }}>
                    <i className={makeIconClass(icon || "cube", true, { defaultIcon: "cube" })} />
                </div>
                {label && <span className="wb-preview-tile-label">{label}</span>}
            </div>
        </div>
    );
}

function IconPicker({ value, onChange }: { value: string; onChange: (v: string) => void }) {
    const [search, setSearch] = useState("");
    const filtered = search
        ? POPULAR_ICONS.filter((i) => i.includes(search.toLowerCase()))
        : POPULAR_ICONS;

    return (
        <div className="wb-icon-picker">
            <input
                className="wb-input"
                placeholder="Buscar icono o escribir nombre..."
                value={search || value}
                onChange={(e) => {
                    setSearch(e.target.value);
                    onChange(e.target.value);
                }}
            />
            <div className="wb-icon-grid">
                {filtered.map((ic) => (
                    <button
                        key={ic}
                        title={ic}
                        onClick={() => { onChange(ic); setSearch(""); }}
                        className={clsx("wb-icon-btn", { "wb-icon-btn-active": value === ic })}
                    >
                        <i className={makeIconClass(ic, true, { defaultIcon: "cube" })} />
                    </button>
                ))}
            </div>
        </div>
    );
}

function ColorPicker({ value, onChange }: { value: string; onChange: (v: string) => void }) {
    return (
        <div className="wb-color-picker">
            {PRESET_COLORS.map((c) => (
                <button
                    key={c}
                    onClick={() => onChange(c)}
                    className={clsx("wb-color-btn", { "wb-color-btn-active": value === c })}
                    style={{ backgroundColor: c }}
                    title={c}
                />
            ))}
            <input
                type="color"
                value={value || "#6366f1"}
                onChange={(e) => onChange(e.target.value)}
                className="wb-color-input-native"
                title="Color personalizado"
            />
        </div>
    );
}

function WidgetCard({
    widget,
    onEdit,
    onDelete,
}: {
    widget: WidgetItem;
    onEdit: (w: WidgetItem) => void;
    onDelete: (id: string) => void;
}) {
    return (
        <div className="wb-card">
            <div className="wb-card-icon" style={{ color: widget.color || "#6366f1" }}>
                <i className={makeIconClass(widget.icon || "cube", true, { defaultIcon: "cube" })} />
            </div>
            <div className="wb-card-info">
                <span className="wb-card-label">{widget.label || widget.id}</span>
                {widget.description && (
                    <span className="wb-card-desc">{widget.description}</span>
                )}
                <span className="wb-card-view">
                    Vista: {(widget.blockdef?.meta as Record<string, unknown>)?.["view"] as string || "—"}
                </span>
            </div>
            <div className="wb-card-actions">
                <button className="wb-btn-icon" title="Editar" onClick={() => onEdit(widget)}>
                    <i className="fa fa-edit" />
                </button>
                <button className="wb-btn-icon wb-btn-danger" title="Eliminar" onClick={() => onDelete(widget.id)}>
                    <i className="fa fa-trash" />
                </button>
            </div>
        </div>
    );
}

// ──────────────────────────────────────────────
// Main View
// ──────────────────────────────────────────────

interface FormState {
    id: string;
    label: string;
    icon: string;
    color: string;
    description: string;
    view: ViewType;
    url: string;
    file: string;
    cmd: string;
}

const emptyForm = (): FormState => ({
    id: "",
    label: "",
    icon: "cube",
    color: "#6366f1",
    description: "",
    view: "term",
    url: "",
    file: "",
    cmd: "",
});

export function WidgetBuilderView({ model }: { model: WidgetBuilderViewModel }) {
    const [widgets, setWidgets] = useAtom(widgetsAtom);
    const [loading, setLoading] = useAtom(loadingAtom);
    const [showForm, setShowForm] = useState(false);
    const [form, setForm] = useState<FormState>(emptyForm());
    const [saving, setSaving] = useState(false);
    const [errorMsg, setErrorMsg] = useState("");
    const [successMsg, setSuccessMsg] = useState("");

    useEffect(() => {
        model.fetchWidgets(setWidgets, setLoading);
    }, []);

    function openNew() {
        setForm(emptyForm());
        setErrorMsg("");
        setSuccessMsg("");
        setShowForm(true);
    }

    function openEdit(w: WidgetItem) {
        const meta = (w.blockdef?.meta || {}) as Record<string, unknown>;
        setForm({
            id: w.id,
            label: w.label,
            icon: w.icon,
            color: w.color,
            description: w.description,
            view: (meta["view"] as ViewType) || "term",
            url: (meta["url"] as string) || "",
            file: (meta["file"] as string) || "",
            cmd: (meta["cmd"] as string) || "",
        });
        setErrorMsg("");
        setSuccessMsg("");
        setShowForm(true);
    }

    async function handleDelete(id: string) {
        if (!confirm("¿Eliminar este widget?")) return;
        try {
            await model.deleteWidget(id);
            setSuccessMsg("Widget eliminado.");
            model.fetchWidgets(setWidgets, setLoading);
        } catch (e: unknown) {
            setErrorMsg("Error al eliminar: " + (e as Error).message);
        }
    }

    function setField<K extends keyof FormState>(key: K, val: FormState[K]) {
        setForm((prev) => ({ ...prev, [key]: val }));
    }

    async function handleSave(e: React.FormEvent) {
        e.preventDefault();
        if (!form.label.trim()) {
            setErrorMsg("El nombre es obligatorio.");
            return;
        }
        setSaving(true);
        setErrorMsg("");
        try {
            const id = form.id || generateId(form.label);
            const blockdef = defaultBlockDef(form.view, {
                url: form.url,
                file: form.file,
                cmd: form.cmd,
            });
            await model.saveWidget({ id, label: form.label, icon: form.icon, color: form.color, description: form.description, blockdef });
            setSuccessMsg(`Widget "${form.label}" guardado. Reinicia el launcher para verlo.`);
            setShowForm(false);
            model.fetchWidgets(setWidgets, setLoading);
        } catch (e: unknown) {
            setErrorMsg("Error al guardar: " + (e as Error).message);
        } finally {
            setSaving(false);
        }
    }

    return (
        <div className="wb-root">
            {/* Header */}
            <div className="wb-header">
                <div className="wb-header-info">
                    <h2>
                        <i className="fa fa-wand-magic-sparkles" />
                        Widget Builder
                    </h2>
                    <p>Crea tus propios widgets personalizados para el Launcher</p>
                </div>
                <button className="wb-btn-primary" onClick={openNew}>
                    <i className="fa fa-plus" /> Nuevo Widget
                </button>
            </div>

            {/* Feedback */}
            {successMsg && (
                <div className="wb-alert wb-alert-success">
                    <i className="fa fa-circle-check" /> {successMsg}
                    <button onClick={() => setSuccessMsg("")}><i className="fa fa-times" /></button>
                </div>
            )}
            {errorMsg && (
                <div className="wb-alert wb-alert-error">
                    <i className="fa fa-circle-exclamation" /> {errorMsg}
                    <button onClick={() => setErrorMsg("")}><i className="fa fa-times" /></button>
                </div>
            )}

            <div className="wb-body">
                {/* Left: Widget List */}
                <div className="wb-list-panel">
                    <div className="wb-panel-title">Mis Widgets ({widgets.length})</div>
                    <OverlayScrollbarsComponent className="wb-scroll" options={{ scrollbars: { autoHide: "leave" } }}>
                        {loading && (
                            <div className="wb-empty">
                                <i className="fa fa-circle-notch fa-spin fa-2x" />
                            </div>
                        )}
                        {!loading && widgets.length === 0 && (
                            <div className="wb-empty">
                                <i className="fa fa-shapes fa-2x" />
                                <span>Aún no tienes widgets personalizados.</span>
                                <button className="wb-btn-ghost" onClick={openNew}>Crear el primero</button>
                            </div>
                        )}
                        {!loading && widgets.map((w) => (
                            <WidgetCard key={w.id} widget={w} onEdit={openEdit} onDelete={handleDelete} />
                        ))}
                    </OverlayScrollbarsComponent>
                </div>

                {/* Right: Form or empty state */}
                <div className="wb-form-panel">
                    {!showForm ? (
                        <div className="wb-form-placeholder">
                            <i className="fa fa-wand-magic-sparkles fa-3x" />
                            <span>Selecciona un widget para editar <br />o crea uno nuevo</span>
                            <button className="wb-btn-primary" onClick={openNew}>
                                <i className="fa fa-plus" /> Nuevo Widget
                            </button>
                        </div>
                    ) : (
                        <form className="wb-form" onSubmit={handleSave}>
                            <div className="wb-panel-title">
                                {form.id ? "Editar Widget" : "Nuevo Widget"}
                            </div>

                            {/* Preview */}
                            <LivePreviewTile label={form.label} icon={form.icon} color={form.color} />

                            {/* Nombre */}
                            <div className="wb-field">
                                <label>Nombre del Widget *</label>
                                <input
                                    className="wb-input"
                                    placeholder="Ej. Mi Terminal"
                                    value={form.label}
                                    onChange={(e) => setField("label", e.target.value)}
                                    required
                                />
                            </div>

                            {/* Tipo de Vista */}
                            <div className="wb-field">
                                <label>Tipo de Vista</label>
                                <div className="wb-view-grid">
                                    {VIEW_OPTIONS.map((v) => (
                                        <button
                                            type="button"
                                            key={v.value}
                                            className={clsx("wb-view-btn", { "wb-view-btn-active": form.view === v.value })}
                                            onClick={() => setField("view", v.value)}
                                        >
                                            <i className={makeIconClass(v.icon, true, { defaultIcon: "square" })} />
                                            <span>{v.label}</span>
                                        </button>
                                    ))}
                                </div>
                            </div>

                            {/* Opciones condicionales por tipo */}
                            {form.view === "web" && (
                                <div className="wb-field">
                                    <label>URL a abrir</label>
                                    <input
                                        className="wb-input"
                                        placeholder="https://ejemplo.com"
                                        value={form.url}
                                        onChange={(e) => setField("url", e.target.value)}
                                    />
                                </div>
                            )}
                            {form.view === "preview" && (
                                <div className="wb-field">
                                    <label>Ruta del archivo</label>
                                    <input
                                        className="wb-input"
                                        placeholder="~/documentos/notas.md"
                                        value={form.file}
                                        onChange={(e) => setField("file", e.target.value)}
                                    />
                                </div>
                            )}
                            {form.view === "term" && (
                                <div className="wb-field">
                                    <label>Comando al arrancar (opcional)</label>
                                    <input
                                        className="wb-input"
                                        placeholder='Ej. htop'
                                        value={form.cmd}
                                        onChange={(e) => setField("cmd", e.target.value)}
                                    />
                                </div>
                            )}

                            {/* Icono */}
                            <div className="wb-field">
                                <label>Icono</label>
                                <IconPicker value={form.icon} onChange={(v) => setField("icon", v)} />
                            </div>

                            {/* Color */}
                            <div className="wb-field">
                                <label>Color del icono</label>
                                <ColorPicker value={form.color} onChange={(v) => setField("color", v)} />
                            </div>

                            {/* Descripción */}
                            <div className="wb-field">
                                <label>Descripción (opcional)</label>
                                <textarea
                                    className="wb-input wb-textarea"
                                    placeholder="Descripción breve del widget"
                                    value={form.description}
                                    onChange={(e) => setField("description", e.target.value)}
                                    rows={2}
                                />
                            </div>

                            {/* Acciones */}
                            <div className="wb-form-actions">
                                <button type="submit" className="wb-btn-primary" disabled={saving}>
                                    {saving ? <><i className="fa fa-circle-notch fa-spin" /> Guardando...</> : <><i className="fa fa-check" /> Guardar Widget</>}
                                </button>
                                <button type="button" className="wb-btn-ghost" onClick={() => setShowForm(false)}>
                                    Cancelar
                                </button>
                            </div>
                        </form>
                    )}
                </div>
            </div>
        </div>
    );
}

export default WidgetBuilderView;
