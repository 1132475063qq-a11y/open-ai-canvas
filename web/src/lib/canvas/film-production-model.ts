import type { FilmAgentRun } from "@/services/api/film-agent-runtime";
import type { FilmProductionImageOptions } from "@/services/api/film-production";
import type { PublicLogicalModel } from "@/services/api/logical-models";

type ParsedImageSize = {
    raw: string;
    width: number;
    height: number;
};

const FILM_VERTICAL_4K_PIXELS = 2160 * 3840;

export function resolveFilmImageOptions(model?: PublicLogicalModel): FilmProductionImageOptions {
    const sizeConstraint = model?.capabilitySpec.options?.size;
    const size = sizeConstraint ? resolvePortraitSize(optionStringValues(sizeConstraint.values), model?.defaultOptions?.size) : undefined;
    const qualityConstraint = model?.capabilitySpec.options?.quality;
    const quality = qualityConstraint ? chooseFilmQuality(optionStringValues(qualityConstraint.values), model?.defaultOptions?.quality) : undefined;
    return { ...(size ? { size } : {}), ...(quality ? { quality } : {}) };
}

export function formatFilmImageOptions(options: FilmProductionImageOptions) {
    const size = formatFilmImageSize(options.size);
    if (!size) return "需配置比例";
    return [size, formatFilmImageQuality(options.quality)].filter(Boolean).join(" · ");
}

export function formatFilmImageSize(value?: string) {
    const normalized = value?.trim();
    if (!normalized) return "";
    const dimensions = parseImageSize(normalized);
    if (!dimensions) return normalized;
    return isNineSixteen(dimensions.width, dimensions.height) ? `9:16 · ${dimensions.width}×${dimensions.height}` : `${dimensions.width}×${dimensions.height}`;
}

export function formatFilmImageQuality(value?: string) {
    switch (value?.trim().toLowerCase()) {
        case "high":
            return "高质量";
        case "medium":
        case "hd":
            return "中等质量";
        case "low":
        case "standard":
            return "标准质量";
        case "4k":
            return "4K";
        case "2k":
            return "2K";
        case "1k":
            return "1K";
        case "auto":
            return "自动质量";
        default:
            return value?.trim() || "";
    }
}

export function filmRunLogicalModelId(run?: Pick<FilmAgentRun, "inputJson">) {
    const input = run?.inputJson?.trim();
    if (!input) return "";
    try {
        const envelope = JSON.parse(input) as { logicalModelId?: unknown };
        return typeof envelope.logicalModelId === "string" ? envelope.logicalModelId.trim() : "";
    } catch {
        return "";
    }
}

function optionStringValues(values?: unknown[]) {
    return (values || []).filter((value): value is string => typeof value === "string" && value.trim() !== "").map((value) => value.trim());
}

function resolvePortraitSize(values: string[], defaultValue: unknown) {
    const requestedDefault = typeof defaultValue === "string" ? defaultValue.trim() : "";
    const supportedDefault = values.find((value) => value.toLowerCase() === requestedDefault.toLowerCase());
    const parsedDefault = supportedDefault ? parseImageSize(supportedDefault) : undefined;

    const explicitPortraitSizes = values
        .map(parseImageSize)
        .filter((value): value is ParsedImageSize => Boolean(value && isNineSixteen(value.width, value.height) && value.width * value.height <= FILM_VERTICAL_4K_PIXELS))
        .sort((left, right) => right.width * right.height - left.width * left.height);
    if (explicitPortraitSizes.length) return explicitPortraitSizes[0].raw;

    if (parsedDefault && isNineSixteen(parsedDefault.width, parsedDefault.height) && parsedDefault.width * parsedDefault.height <= FILM_VERTICAL_4K_PIXELS) return parsedDefault.raw;
    const portraitRatio = values.find((value) => value.toLowerCase().replaceAll(" ", "") === "9:16");
    if (portraitRatio) return portraitRatio;
    if (values.includes("*")) return "9:16";
    return undefined;
}

function parseImageSize(value: string): ParsedImageSize | undefined {
    const normalized = value.toLowerCase().replaceAll("×", "x").replace(/\s+/g, "");
    const match = normalized.match(/^(\d+)x(\d+)$/);
    if (!match) return undefined;
    const width = Number(match[1]);
    const height = Number(match[2]);
    if (!Number.isSafeInteger(width) || !Number.isSafeInteger(height) || width <= 0 || height <= 0) return undefined;
    return { raw: value, width, height };
}

function isNineSixteen(width: number, height: number) {
    return Math.abs(width / height - 9 / 16) < 0.0005;
}

function chooseFilmQuality(values: string[], defaultValue: unknown) {
    if (values.length === 0) return undefined;
    const requestedDefault = typeof defaultValue === "string" ? defaultValue.trim() : "";
    if (values.includes("*")) return requestedDefault && requestedDefault.toLowerCase() !== "auto" ? requestedDefault : undefined;

    const preferred = ["4k", "high", "2k", "medium", "hd", "1k", "low", "standard", "auto"];
    for (const candidate of preferred) {
        const match = values.find((value) => value.toLowerCase() === candidate);
        if (match) return match;
    }
    return values.find((value) => value.toLowerCase() === requestedDefault.toLowerCase());
}
