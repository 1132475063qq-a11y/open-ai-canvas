import { createEcommerceArtifact, type EcommerceArtifact } from "../domain/artifact";
import { type CreativeDirection, type ScenePlan } from "../domain/creative-direction";
import { createProductDNA, type ProductDNA, type ProductDNAInput } from "../domain/product-dna";
import type { CreativeShotPlan } from "../domain/shot-plan";
import { STILL_LIFE_LIFESTYLE_TABLETOP } from "../skills/still-life-lifestyle-tabletop";
import { StillLifeSkillExecutor } from "../executor/still-life-executor";
import { runProviderFreeBasicQA, type BasicQAReport } from "./basic-qa";
import { createProviderFreeResultGrid, type EcommerceResultGrid } from "./result-grid";

export type PrototypeAInput = {
    projectId: string;
    productAssetId: string;
    productDna: Omit<ProductDNAInput, "sourceAssets">;
    userGoal?: string;
    now: string;
    ids?: Partial<{
        productDna: string;
        creativeDirection: string;
        scenePlan: string;
        creativeShotPlan: string;
        resultGrid: string;
        qaReport: string;
    }>;
};

export type PrototypeAChain = {
    productAssetId: string;
    productDna: EcommerceArtifact<ProductDNA>;
    creativeDirection: EcommerceArtifact<CreativeDirection>;
    scenePlan: EcommerceArtifact<ScenePlan>;
    creativeShotPlan: EcommerceArtifact<CreativeShotPlan>;
    resultGrid: EcommerceResultGrid;
    basicQA: BasicQAReport;
};

export function runProviderFreePrototypeA(input: PrototypeAInput): PrototypeAChain {
    if (!input.projectId.trim() || !input.productAssetId.trim()) throw new Error("Prototype A requires projectId and productAssetId");
    const ids = {
        productDna: `ecom-product-dna:${input.projectId}`,
        creativeDirection: `ecom-creative-direction:${input.projectId}`,
        scenePlan: `ecom-scene-plan:${input.projectId}`,
        creativeShotPlan: `ecom-shot-plan:${input.projectId}`,
        resultGrid: `ecom-result-grid:${input.projectId}`,
        qaReport: `ecom-qa:${input.projectId}`,
        ...input.ids,
    };

    const productDna = createEcommerceArtifact({
        id: ids.productDna,
        projectId: input.projectId,
        artifactType: "product_dna",
        schemaVersion: 1,
        payload: createProductDNA({ ...input.productDna, sourceAssets: [input.productAssetId] }),
        sourceRefs: [input.productAssetId],
        authorityRefs: [input.productAssetId],
        evidence: "recorded",
        now: input.now,
    });

    const creativeDirection = createEcommerceArtifact<CreativeDirection>({
        id: ids.creativeDirection,
        projectId: input.projectId,
        artifactType: "creative_direction",
        schemaVersion: 1,
        payload: {
            mode: "STILL_LIFE",
            userGoal: input.userGoal ?? "Create a premium but believable lifestyle tabletop product image",
            visualDirection: "warm, editorial, restrained commercial photography",
            commercialContext: "product detail and campaign exploration",
            constraints: ["preserve confirmed product facts", "keep the logo visible", "leave a text-safe area"],
            productDnaArtifactId: productDna.id,
        },
        sourceRefs: [productDna.id],
        authorityRefs: [productDna.id],
        evidence: "inferred",
        now: input.now,
    });

    const scenePlan = createEcommerceArtifact<ScenePlan>({
        id: ids.scenePlan,
        projectId: input.projectId,
        artifactType: "scene_plan",
        schemaVersion: 1,
        payload: {
            mode: "STILL_LIFE",
            background: "soft warm neutral studio background",
            surface: "matte stone tabletop",
            lighting: "soft directional window light",
            composition: "single product hero with restrained context",
            propStrategy: ["one or two material-matched accents"],
            logoVisibility: "unobstructed from camera",
            textSafeZones: ["upper right", "lower third"],
            constraints: ["props do not occlude product", "product maintains stable tabletop contact"],
            creativeDirectionArtifactId: creativeDirection.id,
        },
        sourceRefs: [creativeDirection.id, productDna.id],
        authorityRefs: [creativeDirection.id],
        evidence: "inferred",
        now: input.now,
    });

    const creativeShotPlan = new StillLifeSkillExecutor().execute(STILL_LIFE_LIFESTYLE_TABLETOP, {
        productDna,
        creativeDirection,
        scenePlan,
        sourceAssetRefs: [input.productAssetId],
        artifactId: ids.creativeShotPlan,
        now: input.now,
    });

    const resultGrid = createProviderFreeResultGrid(ids.resultGrid, creativeShotPlan);
    const basicQA = runProviderFreeBasicQA(ids.qaReport, creativeShotPlan, resultGrid, 4);

    return { productAssetId: input.productAssetId, productDna, creativeDirection, scenePlan, creativeShotPlan, resultGrid, basicQA };
}
