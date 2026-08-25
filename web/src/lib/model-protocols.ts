export type ModelProtocol = string;
export type ProtocolCapability = "text" | "image" | "video" | "audio";
export type ModelProtocolDefinition = { value: ModelProtocol; label: string; vendor?: string; capability: ProtocolCapability; create: string; contentType: string; poll?: string; media: string; enabled?: boolean };

export function protocolGroups(protocols: ModelProtocolDefinition[]) {
    return (["text", "image", "video", "audio"] as ProtocolCapability[]).map((capability) => ({ label: { text: "文本", image: "图片", video: "视频", audio: "音频" }[capability], options: protocols.filter((item) => item.capability === capability && item.enabled !== false).map((item) => ({ label: `${item.label} · ${item.create.replace(/^POST /, "")}`, value: item.value })) }));
}
export function modelProtocolDefinition(value: string | undefined, definitions: ModelProtocolDefinition[] = []) { return definitions.find((item) => item.value === value); }
export function modelProtocolLabel(value: string | undefined, definitions: ModelProtocolDefinition[] = []) { return modelProtocolDefinition(value, definitions)?.label || (value ? value : "未安装协议"); }
export function modelProtocolCapability(value: string | undefined, definitions: ModelProtocolDefinition[] = []) {
    return modelProtocolDefinition(value, definitions)?.capability || (value ? BUILTIN_PROTOCOL_CAPABILITIES[value] : undefined);
}
export function modelProtocolSupportsTokenBilling(capability?: string, protocol?: string) {
    return capability === "text" || (capability === "video" && protocol === "volcengine-ark-video");
}

export function protocolForModelCatalog(endpointTypes: string[] = []): ModelProtocol | undefined {
    const aliases: Record<string, ModelProtocol> = {
        "chat": "chat-completion",
        "chat-completion": "chat-completion",
        "openai-chat": "chat-completion",
        "responses": "openai-response",
        "openai-response": "openai-response",
        "claude": "claude-api",
        "claude-api": "claude-api",
        "anthropic-messages": "claude-api",
        "image": "openai-image",
        "openai-image": "openai-image",
        "grok-image": "grok-image",
        "volcengine-ark-image": "volcengine-ark-image",
        "volcengine-jimeng-image": "volcengine-jimeng-image",
        "gemini-image": "gemini-image",
        "gemini-images": "gemini-image",
        "audio": "openai-audio",
        "openai-audio": "openai-audio",
        "async-audio": "async-audio",
        "video": "newapi-channel-2",
        "openai-video": "newapi-channel-2",
        "openai-videos": "newapi-channel-2",
        "newapi": "newapi",
        "newapi-channel-1": "newapi-channel-1",
        "newapi-channel-2": "newapi-channel-2",
        "xai-video": "xai-video",
        "volcengine-ark-video": "volcengine-ark-video",
        "volcengine-jimeng-video": "volcengine-jimeng-video",
        "gemini-veo": "gemini-veo",
        "gemini-video": "gemini-veo",
        "veo": "gemini-veo",
        "novita-video": "novita-video",
        "minimax-video": "minimax-video",
        "agnes-video": "agnes-video",
    };
    for (const endpointType of endpointTypes) {
        const normalized = endpointType.trim().toLowerCase();
        const protocol = aliases[normalized];
        if (protocol) return protocol;
    }
    return undefined;
}
export function modelProtocolSummary(value: string | undefined, definitions: ModelProtocolDefinition[] = []) { const protocol = modelProtocolDefinition(value, definitions); return protocol ? [protocol.create, protocol.contentType, protocol.poll, protocol.media].filter(Boolean).join(" · ") : "当前协议未安装或尚未选择。"; }
export function normalizeModelProtocol(value: unknown): ModelProtocol | undefined { return typeof value === "string" && value.trim() ? value.trim() : undefined; }

const BUILTIN_PROTOCOL_CAPABILITIES: Record<string, ProtocolCapability> = {
    "chat-completion": "text",
    "openai-response": "text",
    "claude-api": "text",
    "openai-image": "image",
    "grok-image": "image",
    "volcengine-ark-image": "image",
    "volcengine-jimeng-image": "image",
    "gemini-image": "image",
    "openai-audio": "audio",
    "async-audio": "audio",
    "newapi": "video",
    "newapi-channel-1": "video",
    "newapi-channel-2": "video",
    "xai-video": "video",
    "volcengine-ark-video": "video",
    "volcengine-jimeng-video": "video",
    "gemini-veo": "video",
    "novita-video": "video",
    "minimax-video": "video",
    "agnes-video": "video",
};
