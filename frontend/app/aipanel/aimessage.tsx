// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { useTranslation } from "@/app/store/i18n";
import { GulinStreamdown } from "@/app/element/streamdown";
import { cn } from "@/util/util";
import { memo, useEffect, useRef } from "react";
import { getFileIcon } from "./ai-utils";
import { AIFeedbackButtons } from "./aifeedbackbuttons";
import { AIToolUseGroup } from "./aitooluse";
import { GulinUIMessage, GulinUIMessagePart } from "./aitypes";
import { GulinAIModel } from "./gulinai-model";

const AIThinking = memo(
    ({
        message = "AI is thinking...",
        reasoningText,
        isWaitingApproval = false,
    }: {
        message?: string;
        reasoningText?: string;
        isWaitingApproval?: boolean;
    }) => {
        const { t } = useTranslation();
        const thinkingMessage = message || t("gulin.ai.message.thinking");
        const scrollRef = useRef<HTMLDivElement>(null);

        useEffect(() => {
            if (scrollRef.current && reasoningText) {
                scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
            }
        }, [reasoningText]);

        const displayText = reasoningText
            ? (() => {
                  const lastDoubleNewline = reasoningText.lastIndexOf("\n\n");
                  return lastDoubleNewline !== -1 ? reasoningText.substring(lastDoubleNewline + 2) : reasoningText;
              })()
            : "";

        return (
            <div className="flex flex-col gap-1">
                <div className="flex items-center gap-2">
                    {isWaitingApproval ? (
                        <i className="fa fa-clock text-base text-yellow-500"></i>
                    ) : (
                        <div className="animate-pulse flex items-center">
                            <i className="fa fa-circle text-[10px]"></i>
                            <i className="fa fa-circle text-[10px] mx-1"></i>
                            <i className="fa fa-circle text-[10px]"></i>
                        </div>
                    )}
                    {thinkingMessage && <span className="text-sm text-gray-400">{thinkingMessage}</span>}
                </div>
                <div ref={scrollRef} className="text-sm text-gray-500 overflow-y-auto h-[3lh] max-w-[600px] pl-9">
                    {displayText}
                </div>
            </div>
        );
    }
);

AIThinking.displayName = "AIThinking";

interface UserMessageFilesProps {
    fileParts: Array<GulinUIMessagePart & { type: "data-userfile" }>;
}

const UserMessageFiles = memo(({ fileParts }: UserMessageFilesProps) => {
    const { t } = useTranslation();
    if (fileParts.length === 0) return null;

    return (
        <div className="mt-2 pt-2 border-t border-gray-600">
            <div className="flex gap-2 overflow-x-auto pb-1">
                {fileParts.map((file, index) => (
                    <div key={index} className="relative bg-zinc-700 rounded-lg p-2 min-w-20 flex-shrink-0">
                        <div className="flex flex-col items-center text-center">
                            <div className="w-12 h-12 mb-1 flex items-center justify-center bg-zinc-600 rounded overflow-hidden">
                                {file.data?.previewurl ? (
                                    <img
                                        src={file.data.previewurl}
                                        alt={file.data?.filename || t("gulin.ai.message.file")}
                                        className="w-full h-full object-cover"
                                    />
                                ) : (
                                    <i
                                        className={cn(
                                            "fa text-lg text-gray-300",
                                            getFileIcon(file.data?.filename || "", file.data?.mimetype || "")
                                        )}
                                    ></i>
                                )}
                            </div>
                            <div
                                className="text-[10px] text-gray-200 truncate w-full max-w-16"
                                title={file.data?.filename || t("gulin.ai.message.file")}
                            >
                                {file.data?.filename || t("gulin.ai.message.file")}
                            </div>
                        </div>
                    </div>
                ))}
            </div>
        </div>
    );
});

UserMessageFiles.displayName = "UserMessageFiles";

interface AIMessagePartProps {
    part: GulinUIMessagePart;
    role: string;
    isStreaming: boolean;
}

const AIMessagePart = memo(({ part, role, isStreaming }: AIMessagePartProps) => {
    const model = GulinAIModel.getInstance();

    if (!part || typeof part !== "object") return null;

    if (part.type === "text") {
        const content = (part as any)?.text || (part as any)?.content || "";

        if (role === "user") {
            return <div className="whitespace-pre-wrap break-words">{content}</div>;
        } else {
            return (
                <GulinStreamdown
                    text={content}
                    parseIncompleteMarkdown={isStreaming}
                    className="text-gray-100"
                    codeBlockMaxWidthAtom={model.codeBlockMaxWidth}
                />
            );
        }
    }

    if (part.type === "reasoning") {
        const reasoning = (part as any)?.reasoning || (part as any)?.text || (part as any)?.content || "";
        if (!reasoning) return null;
        
        // SAFE ACCESS: Access providerMetadata only safely with optional chaining
        const metadata = (part as any)?.providerMetadata;
        
        return (
            <div className="text-gray-400 italic text-sm border-l-2 border-gray-600 pl-2 my-1">
                {reasoning}
            </div>
        );
    }

    return null;
});

AIMessagePart.displayName = "AIMessagePart";

interface AIMessageProps {
    message: GulinUIMessage;
    isStreaming: boolean;
}

const isDisplayPart = (part: GulinUIMessagePart): boolean => {
    if (!part || typeof part.type !== "string") return false;
    return (
        part.type === "text" ||
        part.type === "reasoning" || // Permitir renderizado de razonamiento
        part.type === "data-tooluse" ||
        part.type === "data-toolprogress" ||
        (part.type.startsWith("tool-") && "state" in part && part.state === "input-available")
    );
};

type MessagePart =
    | { type: "single"; part: GulinUIMessagePart }
    | { type: "toolgroup"; parts: Array<GulinUIMessagePart & { type: "data-tooluse" | "data-toolprogress" }> };

const groupMessageParts = (parts: GulinUIMessagePart[]): MessagePart[] => {
    const grouped: MessagePart[] = [];
    if (!Array.isArray(parts)) return grouped;
    let currentToolGroup: Array<GulinUIMessagePart & { type: "data-tooluse" | "data-toolprogress" }> = [];

    for (const part of parts) {
        if (!part) continue;
        if (part.type === "data-tooluse" || part.type === "data-toolprogress") {
            currentToolGroup.push(part as GulinUIMessagePart & { type: "data-tooluse" | "data-toolprogress" });
        } else {
            if (currentToolGroup.length > 0) {
                grouped.push({ type: "toolgroup", parts: currentToolGroup });
                currentToolGroup = [];
            }
            grouped.push({ type: "single", part });
        }
    }

    if (currentToolGroup.length > 0) {
        grouped.push({ type: "toolgroup", parts: currentToolGroup });
    }

    return grouped;
};

const getThinkingMessage = (
    parts: GulinUIMessagePart[],
    isStreaming: boolean,
    role: string,
    t: (key: string) => string
): { message: string; reasoningText?: string; isWaitingApproval?: boolean } | null => {
    if (!isStreaming || role !== "assistant") {
        return null;
    }

    if (!Array.isArray(parts) || parts.length === 0) return { message: t("gulin.ai.message.thinking") };
    const lastPart = parts[parts.length - 1];

    if (!lastPart || typeof lastPart !== "object") return { message: t("gulin.ai.message.thinking") };

    if (lastPart.type === "data-tooluse" && (lastPart as any)?.data?.approval === "needs-approval") {
        return { message: t("gulin.ai.message.waiting_approval"), isWaitingApproval: true };
    }

    if (lastPart.type === "reasoning") {
        const reasoningContent = (lastPart as any)?.reasoning || (lastPart as any)?.text || (lastPart as any)?.content || "";
        // Extreme safety for providerMetadata access which can cause crashes in some SDK versions
        const metadata = (lastPart as any)?.providerMetadata;
        return { message: t("gulin.ai.message.thinking"), reasoningText: reasoningContent };
    }

    if (lastPart.type === "text" && ((lastPart as any)?.text || (lastPart as any)?.content)) {
        return null;
    }

    return { message: t("gulin.ai.message.thinking") };
};

export const AIMessage = memo(({ message, isStreaming }: AIMessageProps) => {
    // Seguridad extrema en el acceso a 'message' y 'parts'
    if (!message) return null;
    const parts = Array.isArray(message.parts) ? message.parts : [];
    
    // Filtrar partes válidas con guarda de tipo
    const validParts = parts.filter(p => p && typeof p.type === "string");
    const displayParts = validParts.filter(isDisplayPart);
    
    const fileParts = validParts.filter((part): part is GulinUIMessagePart & { type: "data-userfile" } => 
        part.type === "data-userfile" && part.data !== undefined
    );
    
    const { t } = useTranslation();

    const thinkingData = getThinkingMessage(validParts, isStreaming, message.role, t);
    const groupedParts = groupMessageParts(displayParts);
    const seenBlockIds = new Set<string>();

    const allText = validParts
        .filter((p) => p && (p.type === "text" || p.type === "reasoning"))
        .map((p) => {
            const anyP = p as any;
            return anyP?.text || anyP?.reasoning || anyP?.content || "";
        })
        .filter(t => t != null)
        .join("\n\n");

    return (
        <div className={cn("flex", message.role === "user" ? "justify-end" : "justify-start")}>
            <div
                className={cn(
                    "px-2 rounded-lg [&>*:first-child]:!mt-0",
                    message.role === "user"
                        ? "py-2 bg-zinc-700/60 text-white max-w-[calc(100%-50px)]"
                        : "min-w-[min(100%,500px)]"
                )}
            >
                {displayParts.length === 0 && !isStreaming && !thinkingData ? (
                    <div className="whitespace-pre-wrap break-words">{t("gulin.ai.message.no_content")}</div>
                ) : (
                    <>
                        {groupedParts.map((group, index: number) => {
                            if (group.type === "toolgroup") {
                                return <AIToolUseGroup key={index} parts={group.parts} isStreaming={isStreaming} seenBlockIds={seenBlockIds} />;
                            }
                            if (!group.part) return null;
                            return (
                                <div key={index} className="mt-2">
                                    <AIMessagePart part={group.part} role={message.role} isStreaming={isStreaming} />
                                </div>
                            );
                        })}
                        {thinkingData != null && thinkingData.message && (
                            <div className="mt-2">
                                <AIThinking 
                                    message={thinkingData.message} 
                                    reasoningText={thinkingData.reasoningText} 
                                    isWaitingApproval={thinkingData.isWaitingApproval} 
                                />
                            </div>
                        )}
                        {message.role === "assistant" && !isStreaming && (
                            <AIFeedbackButtons messageText={allText} />
                        )}
                    </>
                )}

                {message.role === "user" && fileParts.length > 0 && <UserMessageFiles fileParts={fileParts} />}
            </div>
        </div>
    );
});

AIMessage.displayName = "AIMessage";
