import type { EcommerceResultGrid } from "./result-grid";
import type { EcommerceArtifact } from "../domain/artifact";
import type { CreativeShotPlan } from "../domain/shot-plan";

export type BasicQAOutcome = "PASS" | "UNCERTAIN" | "FAIL";

export type BasicQACheck = {
    key: "variation_count" | "variation_refs" | "provider_result";
    outcome: BasicQAOutcome;
    detail: string;
};

export type BasicQAReport = {
    id: string;
    projectId: string;
    shotPlanArtifactId: string;
    outcome: BasicQAOutcome;
    checks: BasicQACheck[];
    needsYou: boolean;
};

export function runProviderFreeBasicQA(id: string, shotPlan: EcommerceArtifact<CreativeShotPlan>, resultGrid: EcommerceResultGrid, expectedVariationCount = 4): BasicQAReport {
    const checks: BasicQACheck[] = [];
    checks.push({
        key: "variation_count",
        outcome: shotPlan.payload.variations.length === expectedVariationCount ? "PASS" : "FAIL",
        detail: `Expected ${expectedVariationCount} variations; received ${shotPlan.payload.variations.length}`,
    });

    const expectedIds = new Set(shotPlan.payload.variations.map((variation) => variation.id));
    const refsAreComplete = resultGrid.slots.length === expectedIds.size && resultGrid.slots.every((slot) => expectedIds.has(slot.variationId));
    checks.push({
        key: "variation_refs",
        outcome: refsAreComplete ? "PASS" : "FAIL",
        detail: refsAreComplete ? "Every Result Grid slot points to a CreativeShotPlan variation" : "Result Grid has an incomplete variation reference",
    });

    const hasProviderResult = resultGrid.slots.every((slot) => slot.status === "ready" && slot.resultId);
    checks.push({
        key: "provider_result",
        outcome: hasProviderResult ? "PASS" : "UNCERTAIN",
        detail: hasProviderResult ? "Provider results are present" : "Provider generation has not been executed; media quality is unproven",
    });

    const outcome = checks.some((check) => check.outcome === "FAIL") ? "FAIL" : checks.some((check) => check.outcome === "UNCERTAIN") ? "UNCERTAIN" : "PASS";
    return { id, projectId: shotPlan.projectId, shotPlanArtifactId: shotPlan.id, outcome, checks, needsYou: outcome === "UNCERTAIN" };
}
