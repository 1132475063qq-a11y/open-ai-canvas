import type { EcommerceMode } from "./types";

export type CreativeDirection = {
    mode: EcommerceMode;
    userGoal: string;
    visualDirection: string;
    commercialContext: string;
    constraints: string[];
    productDnaArtifactId: string;
};

export type ScenePlan = {
    mode: EcommerceMode;
    background: string;
    surface?: string;
    lighting: string;
    composition: string;
    propStrategy: string[];
    logoVisibility: string;
    textSafeZones: string[];
    constraints: string[];
    creativeDirectionArtifactId: string;
};

export function assertCreativeMode(value: CreativeDirection | ScenePlan, expected: EcommerceMode): void {
    if (value.mode !== expected) {
        throw new Error(`Ecommerce artifact mode mismatch: expected ${expected}, received ${value.mode}`);
    }
}
