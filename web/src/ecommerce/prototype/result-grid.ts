import type { EcommerceArtifact } from "../domain/artifact";
import type { CreativeShotPlan } from "../domain/shot-plan";

export type EcommerceResultSlotStatus = "waiting" | "running" | "ready" | "failed" | "needs_you";

export type EcommerceResultSlot = {
    id: string;
    variationId: string;
    status: EcommerceResultSlotStatus;
    generationJobId?: string;
    resultId?: string;
    qaReportId?: string;
    evidence: "recorded" | "inferred" | "unknown";
    needsYouReason?: string;
};

export type EcommerceResultGrid = {
    id: string;
    projectId: string;
    shotPlanArtifactId: string;
    slots: EcommerceResultSlot[];
};

export function createProviderFreeResultGrid(id: string, shotPlan: EcommerceArtifact<CreativeShotPlan>): EcommerceResultGrid {
    if (!id.trim()) throw new Error("Result Grid requires an id");
    return {
        id,
        projectId: shotPlan.projectId,
        shotPlanArtifactId: shotPlan.id,
        slots: shotPlan.payload.variations.map((variation) => ({
            id: `${id}:${variation.id}`,
            variationId: variation.id,
            status: "waiting",
            evidence: "unknown",
            needsYouReason: "Provider generation has not been selected or executed",
        })),
    };
}
