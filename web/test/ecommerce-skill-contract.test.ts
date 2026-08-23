import { describe, expect, test } from "bun:test";

import { createEcommerceArtifact } from "../src/ecommerce/domain/artifact";
import type { CreativeDirection, ScenePlan } from "../src/ecommerce/domain/creative-direction";
import { createProductDNA } from "../src/ecommerce/domain/product-dna";
import { ModelInteractionSkillExecutor } from "../src/ecommerce/executor/model-interaction-executor";
import { StillLifeSkillExecutor } from "../src/ecommerce/executor/still-life-executor";
import { MODEL_INTERACTION_TOP_WEAR } from "../src/ecommerce/skills/model-interaction-top-wear";
import { STILL_LIFE_LIFESTYLE_TABLETOP } from "../src/ecommerce/skills/still-life-lifestyle-tabletop";
import { assertSkillDefinition } from "../src/ecommerce/skills/types";

const now = "2026-08-23T00:00:00Z";

test("top-wear Skill is versioned, executable, and produces six distinct camera roles", () => {
    assertSkillDefinition(MODEL_INTERACTION_TOP_WEAR);
    expect(MODEL_INTERACTION_TOP_WEAR.family).toBe("MODEL_INTERACTION");
    expect(MODEL_INTERACTION_TOP_WEAR.variations).toHaveLength(6);
    expect(new Set(MODEL_INTERACTION_TOP_WEAR.variations.map((variation) => variation.camera)).size).toBe(6);
    expect(MODEL_INTERACTION_TOP_WEAR.relationTemplates).toHaveLength(5);
    expect(MODEL_INTERACTION_TOP_WEAR.constraints).toContain("do not deform hands, neck, shoulders, sleeves or garment-to-body contact");
});

test("lifestyle tabletop Skill compiles six non-repeating commercial roles", () => {
    assertSkillDefinition(STILL_LIFE_LIFESTYLE_TABLETOP);
    expect(STILL_LIFE_LIFESTYLE_TABLETOP.family).toBe("STILL_LIFE");
    expect(STILL_LIFE_LIFESTYLE_TABLETOP.variations).toHaveLength(6);
    expect(new Set(STILL_LIFE_LIFESTYLE_TABLETOP.variations.map((variation) => variation.id)).size).toBe(6);
    expect(new Set(STILL_LIFE_LIFESTYLE_TABLETOP.variations.map((variation) => variation.camera)).size).toBe(6);

    const productDna = createEcommerceArtifact({
        id: "tabletop-product-dna",
        projectId: "project-tabletop",
        artifactType: "product_dna",
        schemaVersion: 1,
        payload: createProductDNA({ category: "home", sourceAssets: ["product-tabletop"], logo: "North Star" }),
        sourceRefs: ["product-tabletop"],
        authorityRefs: ["product-tabletop"],
        evidence: "recorded",
        now,
    });
    const creativeDirection = createEcommerceArtifact<CreativeDirection>({
        id: "tabletop-creative",
        projectId: "project-tabletop",
        artifactType: "creative_direction",
        schemaVersion: 1,
        payload: { mode: "STILL_LIFE", userGoal: "生活方式桌面套图", visualDirection: "自然居家桌面", commercialContext: "social commerce", constraints: ["商品保真"], productDnaArtifactId: productDna.id },
        sourceRefs: [productDna.id],
        authorityRefs: [productDna.id],
        evidence: "inferred",
        now,
    });
    const scenePlan = createEcommerceArtifact<ScenePlan>({
        id: "tabletop-scene",
        projectId: "project-tabletop",
        artifactType: "scene_plan",
        schemaVersion: 1,
        payload: {
            mode: "STILL_LIFE",
            background: "自然居家桌面",
            lighting: "柔和窗光",
            composition: "六个不同商业信息角色",
            propStrategy: ["克制生活道具"],
            logoVisibility: "自然可见",
            textSafeZones: ["upper left"],
            constraints: ["保持系列连续性"],
            creativeDirectionArtifactId: creativeDirection.id,
        },
        sourceRefs: [creativeDirection.id, productDna.id],
        authorityRefs: [creativeDirection.id],
        evidence: "inferred",
        now,
    });
    const plan = new StillLifeSkillExecutor().execute(STILL_LIFE_LIFESTYLE_TABLETOP, {
        productDna,
        creativeDirection,
        scenePlan,
        sourceAssetRefs: ["product-tabletop", "scene-tabletop"],
        artifactId: "tabletop-shot-plan",
        now,
    });

    expect(plan.payload.mode).toBe("STILL_LIFE");
    expect(plan.payload.variations).toHaveLength(6);
    expect(new Set(plan.payload.variations.map((variation) => variation.camera)).size).toBe(6);
});

describe("ModelInteractionSkillExecutor", () => {
    test("compiles a six-slot plan without mixing project references", () => {
        const productDna = createEcommerceArtifact({
            id: "product-dna-1",
            projectId: "project-1",
            artifactType: "product_dna",
            schemaVersion: 1,
            payload: createProductDNA({ category: "apparel", sourceAssets: ["product-1"], logo: "North Star" }),
            sourceRefs: ["product-1"],
            authorityRefs: ["product-1"],
            evidence: "recorded",
            now,
        });
        const creativeDirection = createEcommerceArtifact<CreativeDirection>({
            id: "creative-1",
            projectId: "project-1",
            artifactType: "creative_direction",
            schemaVersion: 1,
            payload: { mode: "MODEL_INTERACTION", userGoal: "生活化上装套图", visualDirection: "自然街区日光", commercialContext: "social commerce", constraints: ["商品保真"], productDnaArtifactId: productDna.id },
            sourceRefs: [productDna.id],
            authorityRefs: [productDna.id],
            evidence: "inferred",
            now,
        });
        const scenePlan = createEcommerceArtifact<ScenePlan>({
            id: "scene-1",
            projectId: "project-1",
            artifactType: "scene_plan",
            schemaVersion: 1,
            payload: {
                mode: "MODEL_INTERACTION",
                background: "城市街区",
                lighting: "柔和日光",
                composition: "自然行走",
                propStrategy: ["帆布包"],
                logoVisibility: "自然可见",
                textSafeZones: ["upper left"],
                constraints: ["保持连续性"],
                creativeDirectionArtifactId: creativeDirection.id,
            },
            sourceRefs: [creativeDirection.id, productDna.id],
            authorityRefs: [creativeDirection.id],
            evidence: "inferred",
            now,
        });
        const plan = new ModelInteractionSkillExecutor().execute(MODEL_INTERACTION_TOP_WEAR, {
            productDna,
            creativeDirection,
            scenePlan,
            sourceAssetRefs: ["product-1", "model-1", "scene-1"],
            artifactId: "shot-plan-1",
            now,
        });

        expect(plan.payload.mode).toBe("MODEL_INTERACTION");
        expect(plan.payload.variations).toHaveLength(6);
        expect(plan.payload.sourceAssetRefs).toEqual(["product-1", "model-1", "scene-1"]);
        expect(plan.payload.variations.every((variation) => variation.promptPreview.includes("model-1"))).toBe(true);
    });
});
