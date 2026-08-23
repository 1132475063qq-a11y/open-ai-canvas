import type { NextFunction, Request, RequestHandler, Response } from "express";

import {
    archiveCodexThread,
    listCodexThreads,
    readCodexThread,
    resumeCodexThread,
    runClaudeTurn,
    runCodexTurn,
    startCodexThread,
    summarizeCodexThread,
    verifyCodexThreadWorkspace,
    withAgentPrompt,
    getCodexCliStatus,
    runCodexText,
    CODEX_CLI_DEFAULT_MODELS,
} from "../agents.js";
import { CanvasSession } from "../canvas-session.js";
import {
    ensureCanvasWorkspace,
    updateCanvasWorkspace,
    type LocalRuntimeConfig,
} from "../config.js";
import type { LocalRuntimeModule, LocalRuntimeProtectedRoute } from "../local-runtime.js";
import { assertExactKeys } from "../local-runtime-security.js";
import type { AgentAttachment } from "../types.js";

export type CanvasAgentSession = Pick<
    CanvasSession,
    "health" | "openEvents" | "updateState" | "resolveResult" | "emitAll" | "callTool" | "closeRuntimeSession" | "dispose"
>;

export function createCanvasAgentHttpModule(
    config: LocalRuntimeConfig,
    session: CanvasAgentSession = new CanvasSession(),
): LocalRuntimeModule {
    const emit = (type: string, payload: unknown) => session.emitAll(type, payload);
    const routes: LocalRuntimeProtectedRoute[] = [
        canvasRoute("GET", "/events", (req, res) => {
            session.openEvents(
                new URL(req.originalUrl || req.url, config.url),
                res,
                runtimeSessionId(res),
            );
        }, { queryKeys: ["clientId"], lastEventId: true }),
        canvasRoute("POST", "/canvas/state", (req, res) => {
            session.updateState(jsonBody(req), queryValue(req, "clientId") || undefined);
            res.json({ ok: true });
        }, { queryKeys: ["clientId"] }),
        canvasRoute("POST", "/canvas/result", (req, res) => {
            session.resolveResult(jsonBody(req) as { requestId?: string; error?: string; result?: unknown });
            res.json({ ok: true });
        }, { queryKeys: ["clientId"] }),
        canvasRoute("POST", "/api/tools", async (req, res) => {
            const body = jsonRecord(req);
            res.json({ ok: true, result: await session.callTool(body.name, body.input || {}) });
        }),
        canvasRoute("GET", "/agent/codex/workspace", (req, res) => {
            const workspace = ensureCanvasWorkspace(config, queryValue(req, "canvasId"));
            res.json({ ok: true, workspace });
        }, { queryKeys: ["canvasId"] }),
        canvasRoute("GET", "/agent/codex/threads", async (req, res) => {
            const workspace = ensureCanvasWorkspace(config, queryValue(req, "canvasId"));
            const result = await listCodexThreads(emit, {
                cwd: workspace.workspacePath,
                searchTerm: queryValue(req, "searchTerm"),
            });
            res.json({ ok: true, workspace, ...result });
        }, { queryKeys: ["canvasId", "searchTerm"] }),
        canvasRoute("POST", "/agent/codex/threads/new", async (req, res) => {
            const body = jsonRecord(req);
            const workspace = ensureCanvasWorkspace(config, String(body.canvasId || ""));
            const thread = await startCodexThread(emit, workspace.workspacePath);
            const activeThreadId = String((thread as Record<string, unknown>).id || "");
            updateCanvasWorkspace(config, workspace.canvasId, { activeThreadId });
            res.json({
                ok: true,
                workspace: { ...workspace, activeThreadId },
                thread: summarizeCodexThread(thread),
                messages: [],
            });
        }),
        canvasRoute("GET", "/agent/codex/threads/:threadId", async (req, res) => {
            const workspace = ensureCanvasWorkspace(config, queryValue(req, "canvasId"));
            const threadId = routeParam(req.params.threadId);
            res.json({
                ok: true,
                workspace,
                ...(await readCodexThread(emit, threadId, workspace.workspacePath)),
            });
        }, { queryKeys: ["canvasId"] }),
        canvasRoute("POST", "/agent/codex/threads/:threadId/resume", async (req, res) => {
            const body = jsonRecord(req);
            const workspace = ensureCanvasWorkspace(config, String(body.canvasId || ""));
            const threadId = routeParam(req.params.threadId);
            const result = await resumeCodexThread(emit, threadId, workspace.workspacePath);
            updateCanvasWorkspace(config, workspace.canvasId, { activeThreadId: threadId });
            res.json({
                ok: true,
                workspace: { ...workspace, activeThreadId: threadId },
                ...result,
            });
        }),
        canvasRoute("POST", "/agent/codex/threads/:threadId/delete", async (req, res) => {
            const body = jsonRecord(req);
            const workspace = ensureCanvasWorkspace(config, String(body.canvasId || ""));
            const threadId = routeParam(req.params.threadId);
            await archiveCodexThread(emit, threadId, workspace.workspacePath);
            if (workspace.activeThreadId === threadId) {
                updateCanvasWorkspace(config, workspace.canvasId, { activeThreadId: undefined });
            }
            res.json({ ok: true });
        }),
        canvasRoute("POST", "/agent/codex/turn", (req, res) => {
            const body = jsonRecord(req);
            const attachments = Array.isArray(body.attachments)
                ? body.attachments as AgentAttachment[]
                : [];
            const workspace = ensureCanvasWorkspace(config, String(body.canvasId || ""));
            let threadId = String(body.threadId || workspace.activeThreadId || "");
            void (async () => {
                if (!threadId) {
                    const thread = await startCodexThread(emit, workspace.workspacePath);
                    threadId = String((thread as Record<string, unknown>).id || "");
                    updateCanvasWorkspace(config, workspace.canvasId, { activeThreadId: threadId });
                } else if (threadId !== workspace.activeThreadId) {
                    await verifyCodexThreadWorkspace(emit, threadId, workspace.workspacePath);
                    updateCanvasWorkspace(config, workspace.canvasId, { activeThreadId: threadId });
                }
                void runCodexTurn(
                    withAgentPrompt(String(body.prompt || "")),
                    emit,
                    attachments,
                    {
                        threadId,
                        cwd: workspace.workspacePath,
                        onThreadId: (nextThreadId) => updateCanvasWorkspace(
                            config,
                            workspace.canvasId,
                            { activeThreadId: nextThreadId },
                        ),
                    },
                );
                if (!res.headersSent) res.json({ ok: true, threadId });
            })().catch((error) => {
                if (!res.headersSent) res.status(500).json({ ok: false, error: publicCanvasError(error) });
            });
        }),
        canvasRoute("GET", "/agent/codex/status", async (_req, res) => {
            res.json({ ok: true, ...(await getCodexCliStatus()) });
        }),
        canvasRoute("GET", "/agent/codex/models", (_req, res) => {
            res.json({ ok: true, provider: "openai-codex-cli", models: [...CODEX_CLI_DEFAULT_MODELS] });
        }),
        canvasRoute("POST", "/agent/codex/text", async (req, res) => {
            const body = jsonRecord(req);
            assertExactKeys(body, ["model", "prompt", "textHistory"]);
            const model = String(body.model || "").trim();
            const prompt = String(body.prompt || "").trim();
            const textHistory = body.textHistory;
            if (!Array.isArray(textHistory) || !/^[A-Za-z0-9][A-Za-z0-9._:-]{0,119}$/.test(model) || !prompt || prompt.length > 60_000 || textHistory.length > 40) {
                throw new Error("GPT CLI 文本请求格式无效");
            }
            const history = textHistory
                .map((item) => {
                    const value = item && typeof item === "object" ? (item as { role?: unknown; content?: unknown }) : {};
                    if ((value.role !== "user" && value.role !== "assistant") || typeof value.content !== "string") throw new Error("GPT CLI 文本历史格式无效");
                    return `${value.role}: ${value.content}`;
                })
                .join("\n\n");
            const requestPrompt = history ? `对话上下文：\n${history}\n\n当前请求：\n${prompt}` : prompt;
            if (requestPrompt.length > 60_000) throw new Error("GPT CLI 文本上下文过长");
            const result = await runCodexText(requestPrompt, model);
            res.json({ ok: true, mode: "text", model, text: result.text, ...(result.usage === undefined ? {} : { usage: result.usage }) });
        }),
        canvasRoute("POST", "/agent/claude/turn", (req, res) => {
            const body = jsonRecord(req);
            runClaudeTurn(withAgentPrompt(String(body.prompt || "")), emit);
            res.json({ ok: true });
        }),
    ];

    return {
        descriptor: {
            id: "canvas-agent",
            displayName: "Canvas Agent",
            apiVersion: 1,
            scopes: ["canvas:connect"],
        },
        routes,
        onRuntimeSessionRevoked: (sessionId) => session.closeRuntimeSession(sessionId),
        publicHealth: () => {
            const { ok: _ok, ...health } = session.health();
            return health;
        },
        dispose: () => session.dispose(),
    };
}

function runtimeSessionId(res: Response) {
    const value = (res.locals.runtimeSession as { sessionId?: unknown } | undefined)?.sessionId;
    return typeof value === "string" && value ? value : undefined;
}

function canvasRoute(
    method: "GET" | "POST",
    path: string,
    handler: (req: Request, res: Response) => void | Promise<void>,
    options: { queryKeys?: readonly string[]; lastEventId?: boolean } = {},
): LocalRuntimeProtectedRoute {
    return {
        method,
        path,
        scope: "canvas:connect",
        handler: route(handler),
        legacy: true,
        ...options,
    };
}

function route(handler: (req: Request, res: Response) => void | Promise<void>): RequestHandler {
    return (req, res, next) => void Promise.resolve(handler(req, res)).catch(next);
}

function jsonBody(req: Request) {
    if (!Buffer.isBuffer(req.body)) throw new Error("Canvas request body is invalid");
    return JSON.parse(req.body.toString("utf8")) as unknown;
}

function jsonRecord(req: Request) {
    const value = jsonBody(req);
    if (!value || typeof value !== "object" || Array.isArray(value)) {
        throw new Error("Canvas request body is invalid");
    }
    return value as Record<string, unknown>;
}

function queryValue(req: Request, key: string) {
    const value = req.query[key];
    return Array.isArray(value) ? String(value[0] ?? "") : String(value ?? "");
}

function routeParam(value: string | string[]) {
    return Array.isArray(value) ? value[0] || "" : value;
}

function publicCanvasError(_error: unknown) {
    return "Canvas Agent request failed";
}
