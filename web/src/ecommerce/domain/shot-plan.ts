import type { CreativeDirection, ScenePlan } from "./creative-direction";
import type { ExpectedRelationGraph } from "./relation-graph";
import type { EcommerceMode } from "./types";

export type CreativeShotVariation = {
    id: string;
    camera: string;
    lighting: string;
    composition: string;
    productPlacement: string;
    surface: string;
    props: string[];
    support: string;
    orientation: string;
    logoVisibility: string;
    textSafeZones: string[];
    constraints: string[];
    expectedRelations: ExpectedRelationGraph;
    promptPreview: string;
};

export type CreativeShotPlan = {
    mode: EcommerceMode;
    productRef: string;
    creativeDirectionRef: string;
    skillRef: string;
    sceneRef: string;
    camera: string;
    lighting: string;
    composition: string;
    constraints: string[];
    expectedRelations: ExpectedRelationGraph;
    variations: CreativeShotVariation[];
    sourceAssetRefs: string[];
};

export type StillLifePlannerInput = {
    productRef: string;
    creativeDirectionRef: string;
    skillRef: string;
    sceneRef: string;
    sourceAssetRefs: string[];
    direction: CreativeDirection;
    scene: ScenePlan;
    expectedRelations: ExpectedRelationGraph;
    variationCount: number;
};

export function assertVariationCount(plan: CreativeShotPlan, expected: number): void {
    if (plan.variations.length !== expected) {
        throw new Error(`CreativeShotPlan requires ${expected} variations, received ${plan.variations.length}`);
    }
}
