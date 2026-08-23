export const ECOMMERCE_PROJECT_TYPE = "ecommerce" as const;
export type EcommerceProjectType = typeof ECOMMERCE_PROJECT_TYPE;

export const ECOMMERCE_MODES = ["STILL_LIFE", "MODEL_INTERACTION"] as const;
export type EcommerceMode = (typeof ECOMMERCE_MODES)[number];

export const ECOMMERCE_NODE_KINDS = [
    "production_frame",
    "product_input",
    "product_dna",
    "model_profile",
    "scene_pack",
    "preset_snapshot",
    "creative_direction",
    "shot_plan",
    "generation_request",
    "generated_asset",
    "qa_report",
    "motion_plan",
    "video_sequence",
    "needs_you",
    // Kept readable so an existing local canvas can still be opened while its
    // production projection is rebuilt from the ecommerce workspace.
    "presentation_mode",
    "scene_plan",
    "creative_shot_plan",
    "generation",
    "result",
    "qc",
] as const;
export type EcommerceNodeKind = (typeof ECOMMERCE_NODE_KINDS)[number];

export type EcommerceLifecycle = "draft" | "review" | "finalized" | "superseded" | "archived";
export type EcommerceProductionStatus = "not_started" | "ready" | "scheduled" | "queued" | "running" | "generated" | "qa" | "qc_failed" | "approved" | "failed" | "cancelled";
export type EcommerceEvidence = "recorded" | "inferred" | "unknown";
export type EcommerceAttention = "none" | "warning" | "error" | "human_required";

export type EcommerceState = {
    lifecycle: EcommerceLifecycle;
    production: EcommerceProductionStatus;
    evidence: EcommerceEvidence;
    attention: EcommerceAttention;
};

export type EcommerceDomainRef = {
    projectId: string;
    runId?: string;
    slotId?: string;
    attemptId?: string;
    presetId?: string;
    presetVersion?: number;
    assetId?: string;
    assetVersionId?: string;
    artifactId?: string;
    artifactRevision?: number;
    skillId?: string;
    skillVersion?: number;
    taskId?: string;
    generationJobId?: string;
    resultId?: string;
    qaReportId?: string;
    sourceResourceIds?: string[];
};

export const DEFAULT_ECOMMERCE_STATE: EcommerceState = {
    lifecycle: "draft",
    production: "not_started",
    evidence: "unknown",
    attention: "none",
};

export function createEcommerceState(patch: Partial<EcommerceState> = {}): EcommerceState {
    return { ...DEFAULT_ECOMMERCE_STATE, ...patch };
}

export function isEcommerceMode(value: unknown): value is EcommerceMode {
    return typeof value === "string" && (ECOMMERCE_MODES as readonly string[]).includes(value);
}

export function isEcommerceNodeKind(value: unknown): value is EcommerceNodeKind {
    return typeof value === "string" && (ECOMMERCE_NODE_KINDS as readonly string[]).includes(value);
}
