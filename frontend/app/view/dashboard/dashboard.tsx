// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import React, { memo, useRef, useMemo } from "react";
import {
    BarChart, Bar, LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer
} from 'recharts';
import { useAtomValue } from "jotai";
import { DashboardViewModel } from "./dashboard-model";
import { ErrorBoundary } from "@/element/errorboundary";
import { getWaveObjectAtom, makeORef } from "@/store/wos";

/**
 * DashboardView: Widget interactivo con bloqueo de renderizado único.
 * Captura la primera ráfaga de datos válida y se 'congela' para evitar loops de React 
 * producidos por las actualizaciones constantes del chat de Gulin.
 */
export const DashboardView = memo(({ model, blockId }: { model: DashboardViewModel, blockId: string }) => {
    // Suscripción al átomo del bloque (necesario para recibir la primera data)
    // Memoizamos el átomo para que tenga una referencia de memoria ESTABLE y no cause loops en Jotai
    const blockDataAtom = useMemo(() => getWaveObjectAtom<Block>(makeORef("block", blockId)), [blockId]);
    const blockData = useAtomValue(blockDataAtom);

    // Memoria persistente fuera del ciclo de estado de React (Ref Lock)
    // Esto garantiza que NO se disparen re-renders por cambios locales.
    const lockedData = useRef<any[] | null>(null);
    const lockedTitle = useRef<string | null>(null);
    const lockedType = useRef<string | null>(null);

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
    const chartType = lockedType.current || "bar";

    const renderChart = () => {
        if (chartData.length === 0) {
            return <div className="flex h-full w-full items-center justify-center text-zinc-500 italic">Esperando datos finales de Gulin...</div>
        }

        if (chartType === "grid") {
            const ROW_LIMIT = 1000;
            const allKeys = Array.from(new Set(chartData.flatMap(item => Object.keys(item))));
            const displayData = chartData.slice(0, ROW_LIMIT);
            const isTruncated = chartData.length > ROW_LIMIT;

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
                                        <th key={key} className="px-4 py-3 text-[10px] font-bold text-violet-400 uppercase tracking-widest border-b border-zinc-800">
                                            {key}
                                        </th>
                                    ))}
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-zinc-800/30">
                                {displayData.map((row, i) => (
                                    <tr key={i} className="hover:bg-violet-500/5 transition-colors group">
                                        {allKeys.map(key => (
                                            <td key={key} className="px-4 py-2.5 text-xs text-zinc-300 font-mono group-hover:text-zinc-100">
                                                {String(row[key] ?? "-")}
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
        const colors = ["#8b5cf6", "#10b981", "#3b82f6", "#f59e0b", "#ec4899", "#06b6d4"];

        // Usamos dimensiones fijas en el wrapper o ResponsiveContainer estable para evitar Loops de ResizeObserver
        return (
            <ResponsiveContainer width="100%" height="100%" debounce={50}>
                {chartType === "line" ? (
                    <LineChart data={chartData} margin={{ top: 20, right: 30, left: 10, bottom: 20 }}>
                        <CartesianGrid strokeDasharray="3 3" stroke="#3f3f46" vertical={false} />
                        <XAxis dataKey={xAxisKey} stroke="#a1a1aa" fontSize={11} tickLine={false} axisLine={false} />
                        <YAxis stroke="#a1a1aa" fontSize={11} tickLine={false} axisLine={false} />
                        <Tooltip contentStyle={{ backgroundColor: '#18181b', borderRadius: '8px', border: '1px solid #3f3f46' }} />
                        <Legend iconType="circle" wrapperStyle={{ fontSize: '11px', paddingTop: '10px' }} />
                        {keys.map((key, i) => (
                            <Line type="monotone" key={key} dataKey={key} stroke={colors[i % colors.length]} strokeWidth={3} dot={{ r: 4 }} activeDot={{ r: 6 }} isAnimationActive={false} />
                        ))}
                    </LineChart>
                ) : (
                    <BarChart data={chartData} margin={{ top: 20, right: 30, left: 10, bottom: 20 }}>
                        <CartesianGrid strokeDasharray="3 3" stroke="#3f3f46" vertical={false} />
                        <XAxis dataKey={xAxisKey} stroke="#a1a1aa" fontSize={11} tickLine={false} axisLine={false} />
                        <YAxis stroke="#a1a1aa" fontSize={11} tickLine={false} axisLine={false} />
                        <Tooltip contentStyle={{ backgroundColor: '#18181b', borderRadius: '8px', border: '1px solid #3f3f46' }} cursor={{ fill: 'rgba(255, 255, 255, 0.05)' }} />
                        <Legend iconType="rect" wrapperStyle={{ fontSize: '11px', paddingTop: '10px' }} />
                        {keys.map((key, i) => (
                            <Bar key={key} dataKey={key} fill={colors[i % colors.length]} radius={[4, 4, 0, 0]} barSize={30} isAnimationActive={false} />
                        ))}
                    </BarChart>
                )}
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
                    <span className="px-2 py-0.5 bg-zinc-800 text-zinc-400 text-[9px] font-bold rounded border border-zinc-700 uppercase">
                        {chartType}
                    </span>
                </div>

                <div className="flex-1 min-h-0 w-full relative">
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
