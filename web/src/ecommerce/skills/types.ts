import type { RelationTemplate } from "../domain/relation-graph";
import type { EcommerceMode } from "../domain/types";

export type EcommerceSkillFamily = "STILL_LIFE" | "HUMAN_INTERACTION" | "MODEL_INTERACTION";

export type EcommerceSkillDefinition = {
    id: string;
    version: number;
    family: EcommerceSkillFamily;
    mode: EcommerceMode;
    name: string;
    applicability: string[];
    executionRules: string[];
    constraints: string[];
    variations: readonly {
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
    }[];
    relationTemplates: readonly RelationTemplate[];
    promptTemplate: string;
};

export function assertSkillDefinition(skill: EcommerceSkillDefinition): void {
    if (!skill.id.trim() || !Number.isInteger(skill.version) || skill.version < 1) throw new Error("Invalid Ecommerce Skill identity");
    if (skill.variations.length === 0) throw new Error("Ecommerce Skill must define variations");
    if (skill.relationTemplates.length === 0) throw new Error("Ecommerce Skill must define ExpectedRelationGraph templates");
    if (skill.family === "STILL_LIFE" && skill.mode !== "STILL_LIFE") throw new Error("Still-life Skill must use STILL_LIFE mode");
    if (skill.family !== "STILL_LIFE" && skill.mode !== "MODEL_INTERACTION") throw new Error("Model-interaction Skill must use MODEL_INTERACTION mode");
    if (!skill.promptTemplate.trim() || skill.constraints.length === 0 || skill.executionRules.length === 0) throw new Error("Ecommerce Skill is missing executable instructions");
    const variationIds = skill.variations.map((variation) => variation.id.trim());
    if (variationIds.some((id) => !id) || new Set(variationIds).size !== variationIds.length) throw new Error("Ecommerce Skill variation ids must be unique");
    const cameraRoles = skill.variations.map((variation) => variation.camera.trim().toLowerCase());
    if (cameraRoles.some((camera) => !camera) || new Set(cameraRoles).size !== cameraRoles.length) throw new Error("Ecommerce Skill camera roles must be unique");
}
