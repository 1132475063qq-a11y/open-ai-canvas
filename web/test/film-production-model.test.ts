import { describe, expect, test } from "bun:test";

import { filmRunLogicalModelId, filmRunTextModelReadiness, formatFilmImageOptions, formatFilmImageQuality, resolveFilmImageOptions } from "../src/lib/canvas/film-production-model";
import type { PublicLogicalModel } from "../src/services/api/logical-models";

function imageModel(options: Record<string, unknown[]>, defaults: Record<string, unknown> = {}) {
    return {
        capabilitySpec: {
            version: 1,
            capability: "image",
            options: Object.fromEntries(Object.entries(options).map(([key, values]) => [key, { values }])),
        },
        defaultOptions: defaults,
    } as PublicLogicalModel;
}

describe("Film production model options", () => {
    test("prefers vertical 4K without escalating to 8K and keeps quality separate", () => {
        const options = resolveFilmImageOptions(
            imageModel({
                size: ["auto", "9:16", "1080x1920", "2160x3840", "4320x7680", "3840x2160"],
                quality: ["auto", "low", "medium", "high"],
            }),
        );
        expect(options).toEqual({ size: "2160x3840", quality: "high" });
        expect(formatFilmImageOptions(options)).toBe("9:16 · 2160×3840 · 高质量");
        expect(formatFilmImageQuality("high")).not.toBe("4K");
    });

    test("uses the best supported portrait size and quality even when route defaults are lower", () => {
        const options = resolveFilmImageOptions(imageModel({ size: ["1080x1920", "2160x3840"], quality: ["low", "high"] }, { size: "1080x1920", quality: "low" }));
        expect(options).toEqual({ size: "2160x3840", quality: "high" });
    });

    test("does not silently submit an oversized portrait when only 8K is declared", () => {
        const options = resolveFilmImageOptions(imageModel({ size: ["4320x7680"], quality: ["high"] }));
        expect(options).toEqual({ quality: "high" });
        expect(formatFilmImageOptions(options)).toBe("需配置比例");
    });

    test("falls back to the portrait ratio for wildcard capability", () => {
        expect(resolveFilmImageOptions(imageModel({ size: ["*"], quality: ["*"] }))).toEqual({ size: "9:16" });
    });

    test("reads the pinned text model from the immutable Run input envelope", () => {
        expect(filmRunLogicalModelId({ inputJson: '{"logicalModelId":"MODEL_TEXT"}' })).toBe("MODEL_TEXT");
        expect(filmRunLogicalModelId({ inputJson: "not-json" })).toBe("");
        expect(filmRunLogicalModelId({})).toBe("");
    });

    test("projects missing and unavailable pinned text models before approval", () => {
        expect(filmRunTextModelReadiness({ inputJson: "{}" }, [{ id: "MODEL_TEXT" }])).toBe("missing");
        expect(filmRunTextModelReadiness({ inputJson: '{"logicalModelId":"MODEL_TEXT"}' }, [])).toBe("unavailable");
        expect(filmRunTextModelReadiness({ inputJson: '{"logicalModelId":"MODEL_TEXT"}' }, [{ id: "MODEL_TEXT" }])).toBe("available");
    });
});
