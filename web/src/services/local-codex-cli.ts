import type { LocalRuntimeTransport } from "@/services/local-runtime";
import { LocalRuntimeClientError } from "@/services/local-runtime-session";

const MAX_RESPONSE_BYTES = 2 * 1024 * 1024;

export type CodexCliStatus = {
    provider: "openai-codex-cli";
    state: "installed" | "missing" | "error";
    installed: boolean;
    executable?: string;
    version?: string;
    message: string;
};

export type CodexLocalModel = {
    provider: "openai-codex-cli";
    id: string;
    displayName: string;
    modality: "text";
    source: "local-settings";
};

export type LocalCodexTextInput = {
    model: `local:codex-cli:${string}`;
    prompt: string;
    textHistory?: Array<{ role: "user" | "assistant"; content: string }>;
};

export async function getCodexCliStatus(client: LocalRuntimeTransport, signal?: AbortSignal): Promise<CodexCliStatus> {
    const response = await client.request("/agent/codex/status", { method: "GET", signal });
    const value = await readJson(response);
    if (!response.ok) throw runtimeError(response.status, "GPT CLI 状态读取失败");
    return parseStatus(value);
}

export async function getCodexCliModels(client: LocalRuntimeTransport, signal?: AbortSignal): Promise<string[]> {
    const response = await client.request("/agent/codex/models", { method: "GET", signal });
    const value = await readJson(response);
    if (!response.ok) throw runtimeError(response.status, "GPT CLI 模型目录读取失败");
    if (!isRecord(value) || value.ok !== true || value.provider !== "openai-codex-cli" || !Array.isArray(value.models)) throw new Error("GPT CLI 模型目录无效");
    return value.models.filter((model): model is string => typeof model === "string" && /^[A-Za-z0-9][A-Za-z0-9._:-]{0,119}$/.test(model));
}

export async function runLocalCodexText(input: LocalCodexTextInput, client: LocalRuntimeTransport, signal?: AbortSignal) {
    if (!/^local:codex-cli:[A-Za-z0-9][A-Za-z0-9._:-]{0,119}$/.test(input.model) || !input.prompt.trim()) throw new Error("GPT CLI 文本请求无效");
    const response = await client.request("/agent/codex/text", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
            model: input.model.slice("local:codex-cli:".length),
            prompt: input.prompt.trim(),
            textHistory: input.textHistory || [],
        }),
        signal,
    });
    const value = await readJson(response);
    if (!response.ok) throw runtimeError(response.status, "GPT CLI 文本生成失败");
    if (!isRecord(value) || value.ok !== true || value.mode !== "text" || typeof value.text !== "string") throw new Error("GPT CLI 文本结果无效");
    return { mode: "text" as const, text: value.text, ...(value.usage === undefined ? {} : { usage: value.usage }) };
}

function parseStatus(value: unknown): CodexCliStatus {
    if (!isRecord(value) || value.ok !== true || value.provider !== "openai-codex-cli" || !["installed", "missing", "error"].includes(String(value.state)) || typeof value.installed !== "boolean" || typeof value.message !== "string") {
        throw new Error("GPT CLI 状态无效");
    }
    return {
        provider: "openai-codex-cli",
        state: value.state as CodexCliStatus["state"],
        installed: value.installed,
        ...(typeof value.executable === "string" ? { executable: value.executable } : {}),
        ...(typeof value.version === "string" ? { version: value.version } : {}),
        message: value.message,
    };
}

async function readJson(response: Response) {
    const declared = Number(response.headers.get("content-length"));
    if (Number.isFinite(declared) && declared > MAX_RESPONSE_BYTES) throw new Error("GPT CLI 响应过大");
    const text = await response.text();
    if (text.length > MAX_RESPONSE_BYTES) throw new Error("GPT CLI 响应过大");
    try {
        return JSON.parse(text) as unknown;
    } catch {
        throw new Error("GPT CLI 响应格式无效");
    }
}

function runtimeError(status: number, message: string) {
    return new LocalRuntimeClientError(status === 401 ? "session_required" : status === 403 ? "scope_denied" : "runtime_request_failed", message, status);
}

function isRecord(value: unknown): value is Record<string, unknown> {
    return Boolean(value && typeof value === "object" && !Array.isArray(value));
}
