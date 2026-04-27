// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import React, { memo, useRef, useMemo } from "react";
import {
    BarChart, Bar, LineChart, Line, AreaChart, Area, PieChart, Pie, Cell, RadarChart, Radar, PolarGrid, PolarAngleAxis, PolarRadiusAxis, ComposedChart, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer
} from 'recharts';
import { toPng } from 'html-to-image';
import { useAtomValue } from "jotai";
import { DashboardViewModel } from "./dashboard-model";
import { ErrorBoundary } from "@/element/errorboundary";
import { getGulinObjectAtom, makeORef } from "@/store/wos";
import { IconButton } from "@/element/iconbutton";

/**
 * DashboardView: Widget interactivo de alto rendimiento para la visualización de datos.
 * 
 * Este componente implementa una estrategia de "bloqueo de renderizado" (Ref Lock).
 * Captura la primera ráfaga de datos válida recibida desde el backend y 'congela' el estado interno.
 * Esto previene bucles de renderizado infinito causados por las actualizaciones constantes
 * del flujo de chat de Gulin mientras mantiene una visualización estática y estable para el usuario.
 * 
 * Soporta múltiples tipos de gráficos de Recharts: bar, line, area, pie, radar, composed y grid.
 * Incluye funcionalidad de exportación a PNG mediante la librería html-to-image.
 * 
 * @param model - El ViewModel que gestiona la lógica de negocio del dashboard.
 * @param blockId - El identificador único del bloque/widget.
 */
export const DashboardView = memo(({ model, blockId }: { model: DashboardViewModel, blockId: string }) => {
    // Suscripción al átomo del bloque (necesario para recibir la primera data)
    // Memoizamos el átomo para que tenga una referencia de memoria ESTABLE y no cause loops en Jotai
    const blockDataAtom = useMemo(() => getGulinObjectAtom<Block>(makeORef("block", blockId)), [blockId]);
    const blockData = useAtomValue(blockDataAtom);

    // Memoria persistente fuera del ciclo de estado de React (Ref Lock)
    // Esto garantiza que NO se disparen re-renders por cambios locales.
    const lockedData = useRef<any[] | null>(null);
    const lockedTitle = useRef<string | null>(null);
    const lockedType = useRef<string | null>(null);
    const chartContainerRef = useRef<HTMLDivElement>(null);

    const rawData = blockData?.meta?.["dashboard:data"];

    // Lógica de "Una sola actualización":
    // Si aún no tenemos datos bloqueados y llega algo válido, lo guardamos para siempre.
    if (!lockedData.current && rawData) {
        try {
            const strData = typeof rawData === "string" ? rawData : JSON.stringify(rawData);
            const parsed = JSON.parse(strData);
            if (Array.isArray(parsed) && parsed.length > 0) {
                lockedData.current = parsed;
                lockedTitle.current = (blockData?.meta?.["dashboard:title"] as string) || "Interactive Dashboard";
                lockedType.current = (blockData?.meta?.["dashboard:type"] as string) || "bar";
            }
        } catch (e) {
            console.error("Dashboard parse error:", e);
        }
    }

    // Datos finales a renderizar (estáticos una vez bloqueados)
    const chartData = lockedData.current || [];
    const chartTitle = lockedTitle.current || "Interactive Dashboard";
    const chartType = (lockedType.current || "bar").toLowerCase();

    const handleDownload = async () => {
        if (!chartContainerRef.current) return;
        try {
            const dataUrl = await toPng(chartContainerRef.current, {
                backgroundColor: "#111111",
                style: {
                    padding: "20px"
                }
            });
            const link = document.createElement("a");
            link.download = `gulin-chart-${chartTitle.toLowerCase().replace(/\s+/g, "-")}.png`;
            link.href = dataUrl;
            link.click();
        } catch (err) {
            console.error("Error downloading chart:", err);
        }
    };

    const renderChart = () => {
        if (chartData.length === 0) {
            return <div className="flex h-full w-full items-center justify-center text-zinc-500 italic">Esperando datos finales de Gulin...</div>
        }

        if (chartType === "grid") {
            const ROW_LIMIT = 1000;
            const allKeys = Array.from(new Set(chartData.flatMap(item => Object.keys(item))));
            const displayData = chartData.slice(0, ROW_LIMIT);
            const isTruncated = chartData.length > ROW_LIMIT;

            const formatHeader = (key: string) => {
                const result = key.replace(/([A-Z])/g, " $1");
                return result.charAt(0).toUpperCase() + result.slice(1);
            };

            return (
                <div className="flex flex-col w-full h-full">
                    {isTruncated && (
                        <div className="mb-2 px-3 py-1 bg-amber-500/10 border border-amber-500/20 rounded text-[10px] text-amber-500 flex justify-between items-center italic">
                            <span>⚠️ Rendimiento optimizado: Mostrando primeras {ROW_LIMIT} de {chartData.length} filas.</span>
                        </div>
                    )}
                    <div className="flex-1 w-full overflow-auto rounded-md border border-zinc-800/50 bg-zinc-950/20 custom-scrollbar shadow-inner">
                        <table className="w-full text-left border-collapse min-w-max">
                            <thead className="sticky top-0 z-10 bg-zinc-900 shadow-sm">
                                <tr>
                                    {allKeys.map(key => (
                                        <th key={key} className="px-4 py-3 text-[10px] font-bold text-violet-400 tracking-wider border-b border-zinc-800">
                                            {formatHeader(key)}
                                        </th>
                                    ))}
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-zinc-800/30">
                                {displayData.map((row, i) => (
                                    <tr key={i} className="hover:bg-violet-500/5 transition-colors group">
                                        {allKeys.map(key => (
                                            <td key={key} className="px-4 py-2.5 text-xs text-zinc-300 font-mono group-hover:text-zinc-100">
                                                {typeof row[key] === "object" && row[key] !== null 
                                                    ? JSON.stringify(row[key]) 
                                                    : String(row[key] ?? "-")}
                                            </td>
                                        ))}
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                </div>
            );
        }

        const sample = chartData[0];
        const keys = Object.keys(sample).filter(k => k !== "name" && k !== "label" && k !== "id" && k !== "Año" && k !== "month");
        const xAxisKey = Object.keys(sample).find(k => k === "name" || k === "label" || k === "month" || k === "Año" || k === "Nivel" || k === "Equipo") || Object.keys(sample)[0] || "name";
        const colors = ["#8b5cf6", "#10b981", "#3b82f6", "#f59e0b", "#ec4899", "#06b6d4", "#f87171", "#fb923c"];

        const commonProps = {
            data: chartData,
            margin: { top: 20, right: 30, left: 10, bottom: 20 }
        };

        const renderChartContent = () => {
            switch (chartType) {
                case "line":
                    return (
                        <LineChart {...commonProps}>
                            <CartesianGrid strokeDasharray="3 3" stroke="#3f3f46" vertical={false} />
                            <XAxis dataKey={xAxisKey} stroke="#a1a1aa" fontSize={11} tickLine={false} axisLine={false} />
                            <YAxis stroke="#a1a1aa" fontSize={11} tickLine={false} axisLine={false} />
                            <Tooltip contentStyle={{ backgroundColor: "#18181b", borderRadius: "8px", border: "1px solid #3f3f46" }} />
                            <Legend iconType="circle" wrapperStyle={{ fontSize: "11px", paddingTop: "10px" }} />
                            {keys.map((key, i) => (
                                <Line type="monotone" key={key} dataKey={key} stroke={colors[i % colors.length]} strokeWidth={3} dot={{ r: 4 }} activeDot={{ r: 6 }} isAnimationActive={false} />
                            ))}
                        </LineChart>
                    );
                case "area":
                    return (
                        <AreaChart {...commonProps}>
                            <CartesianGrid strokeDasharray="3 3" stroke="#3f3f46" vertical={false} />
                            <XAxis dataKey={xAxisKey} stroke="#a1a1aa" fontSize={11} tickLine={false} axisLine={false} />
                            <YAxis stroke="#a1a1aa" fontSize={11} tickLine={false} axisLine={false} />
                            <Tooltip contentStyle={{ backgroundColor: "#18181b", borderRadius: "8px", border: "1px solid #3f3f46" }} />
                            <Legend iconType="rect" wrapperStyle={{ fontSize: "11px", paddingTop: "10px" }} />
                            {keys.map((key, i) => (
                                <Area type="monotone" key={key} dataKey={key} fill={colors[i % colors.length]} stroke={colors[i % colors.length]} fillOpacity={0.3} isAnimationActive={false} />
                            ))}
                        </AreaChart>
                    );
                case "pie":
                    return (
                        <PieChart>
                            <Pie
                                data={chartData}
                                cx="50%"
                                cy="50%"
                                labelLine={false}
                                label={({ name, percent }) => `${name} ${(percent * 100).toFixed(0)}%`}
                                outerRadius="80%"
                                fill="#8884d8"
                                dataKey={keys[0]}
                                isAnimationActive={false}
                            >
                                {chartData.map((_entry, index) => (
                                    <Cell key={`cell-${index}`} fill={colors[index % colors.length]} />
                                ))}
                            </Pie>
                            <Tooltip contentStyle={{ backgroundColor: "#18181b", borderRadius: "8px", border: "1px solid #3f3f46" }} />
                            <Legend wrapperStyle={{ fontSize: "11px" }} />
                        </PieChart>
                    );
                case "radar":
                    return (
                        <RadarChart cx="50%" cy="50%" outerRadius="80%" data={chartData}>
                            <PolarGrid stroke="#3f3f46" />
                            <PolarAngleAxis dataKey={xAxisKey} stroke="#a1a1aa" fontSize={11} />
                            <PolarRadiusAxis stroke="#a1a1aa" fontSize={10} />
                            {keys.map((key, i) => (
                                <Radar key={key} name={key} dataKey={key} stroke={colors[i % colors.length]} fill={colors[i % colors.length]} fillOpacity={0.6} isAnimationActive={false} />
                            ))}
                            <Tooltip contentStyle={{ backgroundColor: "#18181b", borderRadius: "8px", border: "1px solid #3f3f46" }} />
                            <Legend wrapperStyle={{ fontSize: "11px" }} />
                        </RadarChart>
                    );
                case "composed":
                    return (
                        <ComposedChart {...commonProps}>
                            <CartesianGrid strokeDasharray="3 3" stroke="#3f3f46" vertical={false} />
                            <XAxis dataKey={xAxisKey} stroke="#a1a1aa" fontSize={11} tickLine={false} axisLine={false} />
                            <YAxis stroke="#a1a1aa" fontSize={11} tickLine={false} axisLine={false} />
                            <Tooltip contentStyle={{ backgroundColor: "#18181b", borderRadius: "8px", border: "1px solid #3f3f46" }} />
                            <Legend wrapperStyle={{ fontSize: "11px", paddingTop: "10px" }} />
                            {keys.map((key, i) => (
                                i % 2 === 0 ? (
                                    <Bar key={key} dataKey={key} fill={colors[i % colors.length]} radius={[4, 4, 0, 0]} barSize={30} isAnimationActive={false} />
                                ) : (
                                    <Line type="monotone" key={key} dataKey={key} stroke={colors[i % colors.length]} strokeWidth={3} isAnimationActive={false} />
                                )
                            ))}
                        </ComposedChart>
                    );
                default:
                    return (
                        <BarChart {...commonProps}>
                            <CartesianGrid strokeDasharray="3 3" stroke="#3f3f46" vertical={false} />
                            <XAxis dataKey={xAxisKey} stroke="#a1a1aa" fontSize={11} tickLine={false} axisLine={false} />
                            <YAxis stroke="#a1a1aa" fontSize={11} tickLine={false} axisLine={false} />
                            <Tooltip contentStyle={{ backgroundColor: "#18181b", borderRadius: "8px", border: "1px solid #3f3f46" }} cursor={{ fill: "rgba(255, 255, 255, 0.05)" }} />
                            <Legend iconType="rect" wrapperStyle={{ fontSize: "11px", paddingTop: "10px" }} />
                            {keys.map((key, i) => (
                                <Bar key={key} dataKey={key} fill={colors[i % colors.length]} radius={[4, 4, 0, 0]} barSize={30} isAnimationActive={false} />
                            ))}
                        </BarChart>
                    );
            }
        };

        return (
            <ResponsiveContainer width="100%" height="100%" debounce={50}>
                {renderChartContent()}
            </ResponsiveContainer>
        );
    };

    return (
        <ErrorBoundary>
            <div className="flex flex-col w-full h-full bg-zinc-900/80 p-5 rounded-lg border border-zinc-800 shadow-2xl overflow-hidden self-stretch">
                <div className="flex justify-between items-center mb-6 pl-3 border-l-4 border-violet-500 shrink-0">
                    <div>
                        <h2 className="text-lg font-bold text-zinc-100 tracking-tight leading-none">{chartTitle}</h2>
                        <p className="text-[9px] text-zinc-500 uppercase tracking-widest mt-1 font-bold">Datos Congelados (Modo Estable)</p>
                    </div>
                    <div className="flex items-center gap-3">
                        <IconButton
                            decl={{
                                icon: "download",
                                click: handleDownload,
                                title: "Descargar Gráfico",
                            }}
                            className="text-zinc-400 hover:text-white transition-colors"
                        />
                        <span className="px-2 py-0.5 bg-zinc-800 text-zinc-400 text-[9px] font-bold rounded border border-zinc-700 uppercase">
                            {chartType}
                        </span>
                    </div>
                </div>

                <div className="flex-1 min-h-0 w-full relative" ref={chartContainerRef}>
                    {renderChart()}
                </div>

                <div className="mt-2 text-right border-t border-zinc-800/30 pt-2 shrink-0">
                    <span className="text-[8px] text-zinc-700 font-mono tracking-tighter">REF: {blockId.substring(0, 8)} | NO-REFRESH MODE</span>
                </div>
            </div>
        </ErrorBoundary>
    );
});

DashboardView.displayName = "DashboardView";
