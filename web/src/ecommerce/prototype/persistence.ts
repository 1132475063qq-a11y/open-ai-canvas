import type { EcommerceArtifact } from "../domain/artifact";
import type { BasicQAReport } from "./basic-qa";
import type { PrototypeAChain } from "./prototype-a";
import { createProviderFreeResultGrid, type EcommerceResultGrid } from "./result-grid";

export type EcommercePrototypeArtifactInput = {
    artifactKey: string;
    artifactType: "product_dna" | "creative_direction" | "scene_plan" | "creative_shot_plan" | "qa_report";
    schemaVersion: number;
    lifecycle: "draft";
    evidence: "recorded" | "inferred" | "unknown";
    skillRef?: string;
    payload: Record<string, unknown>;
    sourceRefs: string[];
    authorityRefs: string[];
};

/** Structural read model so the Ecommerce domain does not depend on the API client. */
export type PersistedEcommerceArtifact = {
    id: string;
    projectId: string;
    artifactKey: string;
    artifactType: string;
    schemaVersion: number;
    revision: number;
    lifecycle: string;
    evidence: string;
    payloadJson: string;
    sourceRefsJson: string;
    authorityRefsJson: string;
    createdAt: string;
    updatedAt: string;
};

/**
 * Convert the in-memory Prototype A chain into append-only API writes.
 *
 * The browser chain uses temporary artifact ids so it can be deterministic and
 * provider-free. Persistence uses stable artifact keys in sourceRefs instead of
 * those temporary ids; this keeps the backend graph meaningful across reruns.
 */
export function prototypeChainToArtifactInputs(chain: PrototypeAChain): EcommercePrototypeArtifactInput[] {
    const base = stableArtifactKeyPart(chain.productAssetId);
    const keys = {
        productDna: `product-dna:${base}`,
        creativeDirection: `creative-direction:${base}`,
        scenePlan: `scene-plan:${base}`,
        creativeShotPlan: `creative-shot-plan:${base}`,
        qaReport: `qa-report:${base}`,
    };
    const refMap = new Map<string, string>([
        [chain.productDna.id, keys.productDna],
        [chain.creativeDirection.id, keys.creativeDirection],
        [chain.scenePlan.id, keys.scenePlan],
        [chain.creativeShotPlan.id, keys.creativeShotPlan],
        [chain.basicQA.id, keys.qaReport],
    ]);
    const normalizeRefs = (refs: string[]) => [...new Set(refs.map((ref) => refMap.get(ref) || ref).filter(Boolean))];

    return [
        toInput(keys.productDna, "product_dna", chain.productDna, normalizeRefs(chain.productDna.sourceRefs), normalizeRefs(chain.productDna.authorityRefs)),
        toInput(keys.creativeDirection, "creative_direction", chain.creativeDirection, normalizeRefs(chain.creativeDirection.sourceRefs), normalizeRefs(chain.creativeDirection.authorityRefs)),
        toInput(keys.scenePlan, "scene_plan", chain.scenePlan, normalizeRefs(chain.scenePlan.sourceRefs), normalizeRefs(chain.scenePlan.authorityRefs)),
        {
            ...toInput(keys.creativeShotPlan, "creative_shot_plan", chain.creativeShotPlan, normalizeRefs(chain.creativeShotPlan.sourceRefs), normalizeRefs(chain.creativeShotPlan.authorityRefs)),
            skillRef: chain.creativeShotPlan.payload.skillRef,
        },
        {
            artifactKey: keys.qaReport,
            artifactType: "qa_report",
            schemaVersion: 1,
            lifecycle: "draft",
            evidence: "inferred",
            payload: chain.basicQA as unknown as Record<string, unknown>,
            sourceRefs: [keys.creativeShotPlan],
            authorityRefs: [keys.creativeShotPlan],
        },
    ];
}

export type HydratedPrototypeA = {
    creativeShotPlan?: EcommerceArtifact<PrototypeAChain["creativeShotPlan"]["payload"]>;
    resultGrid?: EcommerceResultGrid;
    basicQA?: BasicQAReport;
};

/** Rebuild the provider-free projection after a page refresh. */
export function hydratePrototypeAFromArtifacts(artifacts: PersistedEcommerceArtifact[], productAssetId?: string): HydratedPrototypeA {
    const scopedArtifacts = productAssetId?.trim()
        ? artifacts.filter((artifact) => artifact.artifactKey.endsWith(`:${stableArtifactKeyPart(productAssetId)}`))
        : artifacts;
    const latestShotPlan = latestByType(scopedArtifacts, "creative_shot_plan");
    const latestQA = latestByType(scopedArtifacts, "qa_report");
    if (!latestShotPlan) return {};

    const payload = parsePayload<PrototypeAChain["creativeShotPlan"]["payload"]>(latestShotPlan);
    if (!payload || !Array.isArray(payload.variations)) return {};
    const shotPlan = toDomainArtifact(latestShotPlan, payload);
    let basicQA: BasicQAReport | undefined;
    if (latestQA) {
        const parsedQA = parsePayload<BasicQAReport>(latestQA);
        if (parsedQA && Array.isArray(parsedQA.checks)) basicQA = parsedQA;
    }
    return {
        creativeShotPlan: shotPlan,
        resultGrid: createProviderFreeResultGrid(`result-grid:${latestShotPlan.artifactKey}`, shotPlan),
        basicQA,
    };
}

function toInput<TPayload>(key: string, artifactType: EcommercePrototypeArtifactInput["artifactType"], artifact: EcommerceArtifact<TPayload>, sourceRefs: string[], authorityRefs: string[]): EcommercePrototypeArtifactInput {
    return {
        artifactKey: key,
        artifactType,
        schemaVersion: artifact.schemaVersion,
        lifecycle: "draft",
        evidence: artifact.evidence,
        payload: artifact.payload as unknown as Record<string, unknown>,
        sourceRefs,
        authorityRefs,
    };
}

function latestByType(artifacts: PersistedEcommerceArtifact[], artifactType: string) {
    return artifacts.filter((artifact) => artifact.artifactType === artifactType).sort((left, right) => right.revision - left.revision)[0];
}

function parsePayload<T>(artifact: PersistedEcommerceArtifact): T | undefined {
    try {
        return JSON.parse(artifact.payloadJson) as T;
    } catch {
        return undefined;
    }
}

function toDomainArtifact<TPayload>(artifact: PersistedEcommerceArtifact, payload: TPayload): EcommerceArtifact<TPayload> {
    return {
        id: artifact.id,
        projectId: artifact.projectId,
        artifactType: artifact.artifactType as EcommerceArtifact<TPayload>["artifactType"],
        schemaVersion: artifact.schemaVersion,
        revision: artifact.revision,
        lifecycle: artifact.lifecycle as EcommerceArtifact<TPayload>["lifecycle"],
        payload,
        sourceRefs: parseStringArray(artifact.sourceRefsJson),
        authorityRefs: parseStringArray(artifact.authorityRefsJson),
        evidence: artifact.evidence as EcommerceArtifact<TPayload>["evidence"],
        createdAt: artifact.createdAt,
        updatedAt: artifact.updatedAt,
    };
}

function parseStringArray(value: string): string[] {
    try {
        const parsed = JSON.parse(value) as unknown;
        return Array.isArray(parsed) ? parsed.filter((item): item is string => typeof item === "string") : [];
    } catch {
        return [];
    }
}

export function stableArtifactKeyPart(value: string) {
    return value.trim().replace(/[^a-zA-Z0-9._:-]+/g, "-").replace(/^-+|-+$/g, "") || "product";
}
