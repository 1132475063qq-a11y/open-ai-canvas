import { expect, test } from "bun:test";

import { getCodexCliModels, getCodexCliStatus, runLocalCodexText } from "../src/services/local-codex-cli";

test("GPT CLI status and model catalog accept only the bounded local contract", async () => {
    const calls: Array<{ path: string; method: string; body: string; headers: Headers }> = [];
    const client = {
        async request(path: string, init: RequestInit = {}) {
            calls.push({ path, method: String(init.method || "GET"), body: String(init.body || ""), headers: new Headers(init.headers) });
            return path.endsWith("/status")
                ? jsonResponse({ ok: true, provider: "openai-codex-cli", state: "installed", installed: true, executable: "/Applications/ChatGPT.app/Contents/Resources/codex", version: "codex-cli 0.148.0-alpha.21", message: "ready" })
                : jsonResponse({ ok: true, provider: "openai-codex-cli", models: ["gpt-5.5", "bad model", "gpt-5.5"] });
        },
    };

    await expect(getCodexCliStatus(client)).resolves.toMatchObject({ state: "installed", installed: true });
    await expect(getCodexCliModels(client)).resolves.toEqual(["gpt-5.5", "gpt-5.5"]);
    expect(calls.map((call) => [call.method, call.path])).toEqual([
        ["GET", "/agent/codex/status"],
        ["GET", "/agent/codex/models"],
    ]);
    expect(JSON.stringify(calls)).not.toContain("apiKey");
    expect(JSON.stringify(calls)).not.toContain("authorization");
});

test("GPT CLI text requests use the local model id and preserve bounded history", async () => {
    let request: { path: string; init: RequestInit } | undefined;
    const client = {
        async request(path: string, init: RequestInit = {}) {
            request = { path, init };
            return jsonResponse({ ok: true, mode: "text", text: "本机结果", usage: { output_tokens: 3 } });
        },
    };

    await expect(runLocalCodexText({ model: "local:codex-cli:gpt-5.5", prompt: "写一句商品标题", textHistory: [{ role: "user", content: "上一轮" }] }, client)).resolves.toMatchObject({ mode: "text", text: "本机结果" });
    expect(request?.path).toBe("/agent/codex/text");
    expect(request?.init.method).toBe("POST");
    expect(request?.init.headers).toMatchObject({ "content-type": "application/json" });
    expect(JSON.parse(String(request?.init.body))).toEqual({
        model: "gpt-5.5",
        prompt: "写一句商品标题",
        textHistory: [{ role: "user", content: "上一轮" }],
    });
});

test("GPT CLI text requests reject non-local models before making a Runtime request", async () => {
    let calls = 0;
    const client = {
        async request() {
            calls++;
            return jsonResponse({});
        },
    };
    await expect(runLocalCodexText({ model: "channel:gpt-5.5" as `local:codex-cli:${string}`, prompt: "test" }, client)).rejects.toThrow("请求无效");
    expect(calls).toBe(0);
});

function jsonResponse(body: unknown, status = 200) {
    return new Response(JSON.stringify(body), {
        status,
        headers: { "content-type": "application/json", "cache-control": "no-store" },
    });
}
