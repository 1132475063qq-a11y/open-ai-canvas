import type { EcommerceArtifact, EcommerceProductionAttempt, EcommerceProductionSlot, ProjectAsset } from "@/services/api/projects";
import { ecommerceSelectionKeyForRole, emptyEcommerceWorkspaceAssetSelection, type EcommerceWorkspaceAssetSelection } from "@/ecommerce/canvas/ecommerce-workspace-entry";

export type EcommerceAssetSelectionDefaults = EcommerceWorkspaceAssetSelection;

export function ecommerceAssetSelectionDefaults(assets: Pick<ProjectAsset, "id" | "mediaType" | "projectRole">[]): EcommerceAssetSelectionDefaults {
    const defaults = emptyEcommerceWorkspaceAssetSelection();
    for (const asset of assets) {
        if (asset.mediaType !== "image") continue;
        const key = ecommerceSelectionKeyForRole(asset.projectRole);
        if (key && !defaults[key].includes(asset.id)) defaults[key].push(asset.id);
    }
    return defaults;
}

export const ecommerceCategoryOptions = [
    { value: "apparel", label: "服装" },
    { value: "shoes_bags", label: "鞋包" },
    { value: "jewelry", label: "珠宝" },
    { value: "beauty", label: "美妆" },
    { value: "electronics", label: "数码" },
    { value: "home", label: "家居" },
    { value: "food", label: "食品" },
    { value: "general", label: "通用商品" },
];

export const ecommerceChannelOptions = [
    { value: "taobao_jd", label: "淘宝 / 京东" },
    { value: "xiaohongshu", label: "小红书" },
    { value: "douyin", label: "抖音" },
];

export const ecommerceAspectOptions = ["1:1", "3:4", "4:3", "9:16", "16:9"].map((value) => ({ value, label: value }));

export const ecommerceResolutionOptions = [
    { value: "4k", label: "4K" },
    { value: "2k", label: "2K" },
    { value: "1k", label: "1K" },
] as const;

export function ecommercePixelSize(aspectRatio: string, resolution: string) {
    const sizes: Record<string, Record<string, string>> = {
        "1k": { "1:1": "1024x1024", "3:4": "768x1024", "4:3": "1024x768", "9:16": "576x1024", "16:9": "1024x576" },
        "2k": { "1:1": "2048x2048", "3:4": "1536x2048", "4:3": "2048x1536", "9:16": "1152x2048", "16:9": "2048x1152" },
        "4k": { "1:1": "4096x4096", "3:4": "3072x4096", "4:3": "4096x3072", "9:16": "2160x3840", "16:9": "3840x2160" },
    };
    return sizes[resolution.toLowerCase()]?.[aspectRatio] || "";
}

export const ecommerceIssueOptions = [
    { value: "product_structure", label: "商品结构" },
    { value: "color_material", label: "颜色 / 材质" },
    { value: "logo_text", label: "Logo / 文字" },
    { value: "model_identity", label: "模特身份" },
    { value: "anatomy_contact", label: "人体 / 接触" },
    { value: "scene_consistency", label: "场景一致性" },
    { value: "series_consistency", label: "系列一致性" },
    { value: "duplicate_composition", label: "重复机位 / 构图" },
    { value: "resolution_mismatch", label: "分辨率不足" },
    { value: "resolution_unverified", label: "分辨率待核验" },
];

export function parseJSONObject(raw?: string): Record<string, unknown> {
    if (!raw) return {};
    try {
        const value = JSON.parse(raw) as unknown;
        return value && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : {};
    } catch {
        return {};
    }
}

export function parseJSONArray(raw?: string): string[] {
    if (!raw) return [];
    try {
        const value = JSON.parse(raw) as unknown;
        return Array.isArray(value) ? value.filter((item): item is string => typeof item === "string") : [];
    } catch {
        return [];
    }
}

export function latestRunArtifact(artifacts: EcommerceArtifact[], runId: string, artifactType: string) {
    return artifacts
        .filter((artifact) => artifact.artifactType === artifactType && artifact.artifactKey.startsWith(`run:${runId}:`))
        .sort((left, right) => right.revision - left.revision)[0];
}

export function ecommerceResultURL(value: EcommerceProductionSlot | EcommerceProductionAttempt) {
    if (value.resultUrl) return value.resultUrl;
    const payload = parseJSONObject(value.resultPayloadJson);
    const images = Array.isArray(payload.images) ? payload.images : [];
    const first = images[0];
    if (first && typeof first === "object" && !Array.isArray(first)) {
        const image = first as Record<string, unknown>;
        for (const key of ["url", "dataUrl"] as const) {
            if (typeof image[key] === "string" && image[key]) return image[key] as string;
        }
    }
    return "";
}

export function formatCredits(microcredits?: number) {
    return ((microcredits || 0) / 1_000_000).toLocaleString("zh-CN", { maximumFractionDigits: 6 });
}

export function ecommerceRunStatusLabel(status: string) {
    return ({
        planning: "Agent 规划中",
        awaiting_review: "等待方案确认",
        awaiting_cost: "等待费用确认",
        generating: "系列生成中",
        qa: "等待质检",
        needs_you: "需要处理",
        ready: "系列已就绪",
        failed: "系列失败",
        cancelled: "已取消",
    } as Record<string, string>)[status] || status;
}

export function ecommerceSlotStatusLabel(status: string) {
    return ({
        planned: "已规划",
        awaiting_cost: "待确认费用",
        scheduled: "等待调度",
        queued: "排队中",
        running: "生成中",
        qa: "待质检",
        accepted: "已接受",
        failed: "生成失败",
        cancelled: "已取消",
        succeeded: "已生成",
    } as Record<string, string>)[status] || status;
}

export function shouldPollEcommerceRun(status?: string) {
    return status === "planning" || status === "generating";
}

export function routeKey(channelId: string, model: string) {
    return `${channelId}\u0000${model}`;
}

export function splitRouteKey(value: string) {
    const [channelId = "", model = ""] = value.split("\u0000");
    return { channelId, model };
}
