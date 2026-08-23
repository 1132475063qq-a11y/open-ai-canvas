import type { RelationTemplate } from "../domain/relation-graph";
import type { EcommerceMode } from "../domain/types";

export type EcommerceSkillFamily = "STILL_LIFE" | "HUMAN_INTERACTION";

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
}
