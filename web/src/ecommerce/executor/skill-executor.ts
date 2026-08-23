import type { CreativeDirection, ScenePlan } from "../domain/creative-direction";
import type { EcommerceArtifact } from "../domain/artifact";
import type { ProductDNA } from "../domain/product-dna";
import type { CreativeShotPlan } from "../domain/shot-plan";
import type { EcommerceSkillDefinition } from "../skills/types";

export type EcommerceSkillExecutionContext = {
    productDna: EcommerceArtifact<ProductDNA>;
    creativeDirection: EcommerceArtifact<CreativeDirection>;
    scenePlan: EcommerceArtifact<ScenePlan>;
    sourceAssetRefs: string[];
    artifactId: string;
    now: string;
};

export interface CreativeSkillExecutor {
    supports(skill: EcommerceSkillDefinition): boolean;
    execute(skill: EcommerceSkillDefinition, context: EcommerceSkillExecutionContext): EcommerceArtifact<CreativeShotPlan>;
}

export function assertExecutionContext(context: EcommerceSkillExecutionContext): void {
    const projectIds = [context.productDna.projectId, context.creativeDirection.projectId, context.scenePlan.projectId];
    if (new Set(projectIds).size !== 1) throw new Error("Ecommerce Skill execution cannot mix projects");
    if (context.sourceAssetRefs.length === 0) throw new Error("Ecommerce Skill execution requires source assets");
    if (!context.artifactId.trim()) throw new Error("Ecommerce Skill execution requires an artifact id");
}
