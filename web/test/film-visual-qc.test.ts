import { expect, test } from "bun:test";

import { supportsFilmVisualQCModel } from "../src/lib/canvas/film-visual-qc";
import type { PublicLogicalModel } from "../src/services/api/logical-models";

function logicalModel(overrides: Partial<PublicLogicalModel> = {}): PublicLogicalModel {
    return {
        id: "model-1",
        code: "model-1",
        name: "Model 1",
        description: "",
        capability: "text",
        sortOrder: 0,
        pricePolicy: "unified",
        billingMode: "fixed_request",
        unitPriceMicrocredits: 100,
        inputPriceMicrocredits: 0,
        outputPriceMicrocredits: 0,
        cachedPriceMicrocredits: 0,
        capabilitySpec: { version: 1, capability: "text", inputs: { image: { min: 0, max: 0 } } },
        capabilityProfiles: [],
        defaultOptions: {},
        available: true,
        ...overrides,
    };
}

test("Film visual QC lists only available text models with image input", () => {
    expect(supportsFilmVisualQCModel(logicalModel())).toBe(false);
    expect(supportsFilmVisualQCModel(logicalModel({ available: false, capabilitySpec: { version: 1, capability: "text", inputs: { image: { min: 0, max: 8 } } } }))).toBe(false);
    expect(supportsFilmVisualQCModel(logicalModel({ capability: "image", capabilitySpec: { version: 1, capability: "image", inputs: { image: { min: 0, max: 8 } } } }))).toBe(false);
    expect(supportsFilmVisualQCModel(logicalModel({ capabilitySpec: { version: 1, capability: "text", inputs: { image: { min: 0, max: 1 } } } }))).toBe(true);
    expect(
        supportsFilmVisualQCModel(
            logicalModel({
                capabilityProfiles: [{ version: 1, capability: "text", inputs: { image: { min: 0, max: 16 } } }],
            }),
        ),
    ).toBe(true);
});
