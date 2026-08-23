import type { EcommerceEvidence, EcommerceLifecycle } from "./types";

export type EcommerceArtifactType = "product_upload" | "product_dna" | "creative_direction" | "scene_plan" | "creative_shot_plan" | "generation_job" | "generated_asset" | "qa_report";

export type EcommerceArtifact<TPayload> = {
    id: string;
    projectId: string;
    artifactType: EcommerceArtifactType;
    schemaVersion: number;
    revision: number;
    lifecycle: EcommerceLifecycle;
    payload: TPayload;
    sourceRefs: string[];
    authorityRefs: string[];
    evidence: EcommerceEvidence;
    createdAt: string;
    updatedAt: string;
};

export type CreateEcommerceArtifactInput<TPayload> = Omit<EcommerceArtifact<TPayload>, "revision" | "lifecycle" | "createdAt" | "updatedAt"> & {
    now: string;
};

export function createEcommerceArtifact<TPayload>(input: CreateEcommerceArtifactInput<TPayload>): EcommerceArtifact<TPayload> {
    if (!input.id.trim() || !input.projectId.trim()) throw new Error("Ecommerce Artifact requires id and projectId");
    if (!Number.isInteger(input.schemaVersion) || input.schemaVersion < 1) throw new Error("Ecommerce Artifact schemaVersion must be positive");
    if (input.sourceRefs.length === 0) throw new Error("Ecommerce Artifact requires sourceRefs");
    return {
        id: input.id,
        projectId: input.projectId,
        artifactType: input.artifactType,
        schemaVersion: input.schemaVersion,
        revision: 1,
        lifecycle: "draft",
        payload: input.payload,
        sourceRefs: [...new Set(input.sourceRefs)],
        authorityRefs: [...new Set(input.authorityRefs)],
        evidence: input.evidence,
        createdAt: input.now,
        updatedAt: input.now,
    };
}

export function finalizeEcommerceArtifact<TPayload>(artifact: EcommerceArtifact<TPayload>, now: string): EcommerceArtifact<TPayload> {
    if (artifact.lifecycle === "archived" || artifact.lifecycle === "superseded") {
        throw new Error("Archived or superseded Ecommerce Artifact cannot be finalized");
    }
    return { ...artifact, lifecycle: "finalized", updatedAt: now };
}

export function createEcommerceArtifactRevision<TPayload>(artifact: EcommerceArtifact<TPayload>, payload: TPayload, now: string, newId: string, sourceRefs: string[] = artifact.sourceRefs): EcommerceArtifact<TPayload> {
    if (artifact.lifecycle !== "finalized") throw new Error("Only finalized Ecommerce Artifact can create a new revision");
    if (!newId.trim() || newId === artifact.id) throw new Error("Ecommerce Artifact revision requires a new id");
    if (sourceRefs.length === 0) throw new Error("Ecommerce Artifact revision requires sourceRefs");
    return {
        ...artifact,
        id: newId,
        revision: artifact.revision + 1,
        lifecycle: "draft",
        payload,
        sourceRefs: [...new Set(sourceRefs)],
        updatedAt: now,
    };
}
