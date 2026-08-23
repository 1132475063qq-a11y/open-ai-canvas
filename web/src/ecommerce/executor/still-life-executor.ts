import { createEcommerceArtifact, type EcommerceArtifact } from "../domain/artifact";
import { assertCreativeMode } from "../domain/creative-direction";
import { bindExpectedRelationGraph } from "../domain/relation-graph";
import type { CreativeShotPlan, CreativeShotVariation } from "../domain/shot-plan";
import type { EcommerceSkillDefinition } from "../skills/types";
import { assertSkillDefinition } from "../skills/types";
import { assertExecutionContext, type CreativeSkillExecutor, type EcommerceSkillExecutionContext } from "./skill-executor";

export class StillLifeSkillExecutor implements CreativeSkillExecutor {
    supports(skill: EcommerceSkillDefinition): boolean {
        return skill.id === "still-life.lifestyle-tabletop" && skill.mode === "STILL_LIFE" && skill.family === "STILL_LIFE";
    }

    execute(skill: EcommerceSkillDefinition, context: EcommerceSkillExecutionContext): EcommerceArtifact<CreativeShotPlan> {
        assertSkillDefinition(skill);
        assertExecutionContext(context);
        if (!this.supports(skill)) throw new Error(`Unsupported Still-Life Skill: ${skill.id}`);
        assertCreativeMode(context.creativeDirection.payload, "STILL_LIFE");
        assertCreativeMode(context.scenePlan.payload, "STILL_LIFE");

        const variations: CreativeShotVariation[] = skill.variations.map((variation) => {
            const expectedRelations = bindExpectedRelationGraph(skill.id, skill.version, skill.relationTemplates, {
                product: context.productDna.id,
                surface: variation.surface,
                props: variation.props.join(", ") || "none",
                logo: `${context.productDna.id}:logo`,
                camera: variation.camera,
            });
            const promptPreview = fillPromptTemplate(skill.promptTemplate, {
                product: context.productDna.id,
                logo: context.productDna.payload.logo || "the confirmed product logo",
                camera: variation.camera,
                lighting: variation.lighting,
                composition: variation.composition,
                surface: variation.surface,
                textSafeZone: variation.textSafeZones.join(", "),
            });
            return {
                ...variation,
                id: variation.id,
                expectedRelations,
                constraints: [...skill.constraints],
                promptPreview,
            };
        });

        const plan: CreativeShotPlan = {
            mode: "STILL_LIFE",
            productRef: context.productDna.id,
            creativeDirectionRef: context.creativeDirection.id,
            skillRef: `${skill.id}@${skill.version}`,
            sceneRef: context.scenePlan.id,
            camera: variations[0]?.camera ?? "",
            lighting: variations[0]?.lighting ?? "",
            composition: variations[0]?.composition ?? "",
            constraints: [...skill.constraints],
            expectedRelations: variations[0]?.expectedRelations ?? bindExpectedRelationGraph(skill.id, skill.version, skill.relationTemplates, {}),
            variations,
            sourceAssetRefs: [...new Set(context.sourceAssetRefs)],
        };

        return createEcommerceArtifact({
            id: context.artifactId,
            projectId: context.productDna.projectId,
            artifactType: "creative_shot_plan",
            schemaVersion: 1,
            payload: plan,
            sourceRefs: [context.productDna.id, context.creativeDirection.id, context.scenePlan.id, ...context.sourceAssetRefs],
            authorityRefs: [context.creativeDirection.id, context.scenePlan.id],
            evidence: "inferred",
            now: context.now,
        });
    }
}

function fillPromptTemplate(template: string, values: Record<string, string>): string {
    return template.replace(/\{\{([a-zA-Z0-9_.-]+)\}\}/g, (_, key: string) => values[key] ?? `{{${key}}}`);
}
