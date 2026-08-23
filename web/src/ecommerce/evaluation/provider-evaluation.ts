import { isEcommerceMode, type EcommerceEvidence, type EcommerceMode } from "../domain/types";

export const PROVIDER_EVALUATION_SCHEMA_VERSION = 1 as const;

export type ProviderEvaluationVariant = "basic" | "expert" | "skill";
export type ProviderEvaluationDecision = "pending" | "go" | "modify" | "stop";
export type ProviderEvaluationAvailability = "available" | "unavailable" | "unknown";
export type ProviderEvaluationAttemptStatus = "queued" | "running" | "succeeded" | "failed" | "cancelled" | "needs_you";
export type ProviderEvaluationEvidence = EcommerceEvidence;

export type ProviderCapabilities = {
    referenceImage: boolean;
    imageEdit: boolean;
    asyncJob: boolean;
    polling: boolean;
    callback: boolean;
    costReporting: boolean;
};

/**
 * A candidate is configuration metadata, not a hard-coded provider allowlist.
 * The evaluator can be populated from the channels available in the current deployment.
 */
export type ProviderEvaluationCandidate = {
    candidateId: string;
    adapterId: string;
    modelRef: string;
    displayName?: string;
    availability: ProviderEvaluationAvailability;
    capabilities: ProviderCapabilities;
    notes?: string;
};

export type ProviderEvaluationSettings = {
    mode: EcommerceMode;
    width: number;
    height: number;
    quality: string;
    outputCount: number;
    referenceAssetIds: string[];
    promptTemplateVersion: string;
    seedPolicy: "fixed" | "provider_default" | "unavailable";
};

export type ProviderEvaluationCase = {
    caseId: string;
    projectId: string;
    fixtureRevision: string;
    sourceAssetIds: string[];
    productImageId: string;
    userGoal: string;
    constraints: string[];
    createdAt: string;
};

export type ProviderUsage = {
    inputUnits?: number;
    outputUnits?: number;
    unitName?: string;
    raw?: Record<string, unknown>;
};

export type ProviderCost = {
    amount?: number;
    currency?: string;
    status: "recorded" | "estimated" | "unavailable";
    raw?: Record<string, unknown>;
};

export type ProviderFailure = {
    code: string;
    stage: "validation" | "submit" | "poll" | "callback" | "normalize" | "storage" | "unknown";
    retryable: boolean;
    summary: string;
};

export type ProviderEvaluationAttempt = {
    attemptId: string;
    planId: string;
    caseId: string;
    candidateId: string;
    variant: ProviderEvaluationVariant;
    status: ProviderEvaluationAttemptStatus;
    settingsFingerprint: string;
    requestFingerprint: string;
    providerJobId?: string;
    resultRefs: string[];
    startedAt: string;
    completedAt?: string;
    latencyMs?: number;
    usage?: ProviderUsage;
    cost?: ProviderCost;
    failure?: ProviderFailure;
    evidence: ProviderEvaluationEvidence;
};

export type ProviderEvaluationDimensions = {
    referenceFidelity: number;
    productIdentity: number;
    commercialQuality: number;
    instructionFollowing: number;
    variationAbility: number;
    physicalPlausibility: number;
    aiArtifactSeverity: number;
    apiStability: number;
};

export type ProviderEvaluationScore = {
    scoreId: string;
    attemptId: string;
    evaluatorRef: string;
    dimensions: ProviderEvaluationDimensions;
    notes?: string;
    evidence: ProviderEvaluationEvidence;
    recordedAt: string;
};

export type BlindPreferenceRecord = {
    preferenceId: string;
    caseId: string;
    resultRefs: string[];
    preferredResultRef?: string;
    tie: boolean;
    voterRef: string;
    blinded: true;
    evidence: ProviderEvaluationEvidence;
    recordedAt: string;
};

export type ProviderEvaluationDecisionRecord = {
    decision: Exclude<ProviderEvaluationDecision, "pending">;
    reviewerRef: string;
    rationale: string;
    nextAction: string;
    evidence: ProviderEvaluationEvidence;
    recordedAt: string;
};

export type ProviderEvaluationPlan = {
    schemaVersion: typeof PROVIDER_EVALUATION_SCHEMA_VERSION;
    planId: string;
    projectId: string;
    skillRef: string;
    settings: ProviderEvaluationSettings;
    cases: ProviderEvaluationCase[];
    candidates: ProviderEvaluationCandidate[];
    attempts: ProviderEvaluationAttempt[];
    scores: ProviderEvaluationScore[];
    blindPreferences: BlindPreferenceRecord[];
    decision?: ProviderEvaluationDecisionRecord;
    createdAt: string;
    updatedAt: string;
};

export type ProviderEvaluationPlanInput = Omit<ProviderEvaluationPlan, "schemaVersion" | "attempts" | "scores" | "blindPreferences" | "decision" | "updatedAt">;

export type ProviderEvaluationCandidateSummary = {
    candidateId: string;
    attempts: number;
    succeeded: number;
    failed: number;
    needsYou: number;
    successRate: number | null;
    averageLatencyMs: number | null;
    knownCost: { amount: number; currency: string } | null;
    scoreCount: number;
    dimensionAverages: Partial<ProviderEvaluationDimensions>;
    failureModes: Record<string, number>;
    evidence: ProviderEvaluationEvidence;
};

export type ProviderEvaluationSummary = {
    planId: string;
    candidateSummaries: ProviderEvaluationCandidateSummary[];
    blindPreferenceCount: number;
    decision: ProviderEvaluationDecision;
    hasRecordedResults: boolean;
    hasUnknowns: boolean;
};

export function createProviderEvaluationPlan(input: ProviderEvaluationPlanInput): ProviderEvaluationPlan {
    const planId = required(input.planId, "Provider Evaluation planId");
    const projectId = required(input.projectId, "Provider Evaluation projectId");
    const skillRef = required(input.skillRef, "Provider Evaluation skillRef");
    validateSettings(input.settings);
    assertUnique(input.cases.map((item) => item.caseId), "Provider Evaluation caseId");
    assertUnique(input.candidates.map((item) => item.candidateId), "Provider Evaluation candidateId");
    if (input.cases.length === 0) throw new Error("Provider Evaluation requires at least one case");
    if (input.candidates.length === 0) throw new Error("Provider Evaluation requires at least one candidate");
    required(input.createdAt, "Provider Evaluation createdAt");
    for (const item of input.cases) validateCase(item, projectId);
    for (const item of input.candidates) validateCandidate(item);
    return {
        schemaVersion: PROVIDER_EVALUATION_SCHEMA_VERSION,
        planId,
        projectId,
        skillRef,
        settings: normalizeSettings(input.settings),
        cases: input.cases.map((item) => ({ ...item, sourceAssetIds: uniqueStrings(item.sourceAssetIds), constraints: uniqueStrings(item.constraints) })),
        candidates: input.candidates.map((item) => ({ ...item, candidateId: item.candidateId.trim(), adapterId: item.adapterId.trim(), modelRef: item.modelRef.trim() })),
        attempts: [],
        scores: [],
        blindPreferences: [],
        createdAt: input.createdAt,
        updatedAt: input.createdAt,
    };
}

export function appendProviderEvaluationAttempt(plan: ProviderEvaluationPlan, attempt: ProviderEvaluationAttempt): ProviderEvaluationPlan {
    if (plan.attempts.some((item) => item.attemptId === attempt.attemptId)) throw new Error("Provider Evaluation attemptId must be unique");
    if (!plan.cases.some((item) => item.caseId === attempt.caseId)) throw new Error("Provider Evaluation attempt references an unknown case");
    if (!plan.candidates.some((item) => item.candidateId === attempt.candidateId)) throw new Error("Provider Evaluation attempt references an unknown candidate");
    if (attempt.planId !== plan.planId) throw new Error("Provider Evaluation attempt belongs to a different plan");
    required(attempt.requestFingerprint, "Provider Evaluation requestFingerprint");
    required(attempt.settingsFingerprint, "Provider Evaluation settingsFingerprint");
    if (attempt.settingsFingerprint !== evaluationSettingsFingerprint(plan.settings)) throw new Error("Provider Evaluation attempt settingsFingerprint does not match the plan settings");
    required(attempt.startedAt, "Provider Evaluation attempt startedAt");
    if (attempt.latencyMs !== undefined && (!Number.isFinite(attempt.latencyMs) || attempt.latencyMs < 0)) throw new Error("Provider Evaluation latencyMs must be non-negative");
    if (attempt.status === "succeeded" && attempt.resultRefs.length === 0) throw new Error("Succeeded Provider Evaluation attempt requires resultRefs");
    if (attempt.status === "failed" && !attempt.failure) throw new Error("Failed Provider Evaluation attempt requires failure");
    return { ...plan, attempts: [...plan.attempts, { ...attempt, resultRefs: uniqueStrings(attempt.resultRefs) }], updatedAt: attempt.completedAt || attempt.startedAt };
}

export function recordProviderEvaluationScore(plan: ProviderEvaluationPlan, score: ProviderEvaluationScore): ProviderEvaluationPlan {
    if (plan.scores.some((item) => item.scoreId === score.scoreId)) throw new Error("Provider Evaluation scoreId must be unique");
    const attempt = plan.attempts.find((item) => item.attemptId === score.attemptId);
    if (!attempt) throw new Error("Provider Evaluation score references an unknown attempt");
    if (attempt.status !== "succeeded") throw new Error("Only succeeded Provider Evaluation attempts can be scored");
    validateDimensions(score.dimensions);
    required(score.evaluatorRef, "Provider Evaluation evaluatorRef");
    return { ...plan, scores: [...plan.scores, score], updatedAt: score.recordedAt };
}

export function recordBlindPreference(plan: ProviderEvaluationPlan, preference: BlindPreferenceRecord): ProviderEvaluationPlan {
    if (plan.blindPreferences.some((item) => item.preferenceId === preference.preferenceId)) throw new Error("Blind preferenceId must be unique");
    if (!preference.blinded) throw new Error("Blind preference must be recorded blinded");
    if (!plan.cases.some((item) => item.caseId === preference.caseId)) throw new Error("Blind preference references an unknown case");
    const resultRefs = uniqueStrings(preference.resultRefs);
    if (resultRefs.length < 2) throw new Error("Blind preference requires at least two resultRefs");
    const recordedResultRefs = new Set(
        plan.attempts
            .filter((attempt) => attempt.caseId === preference.caseId && attempt.status === "succeeded")
            .flatMap((attempt) => attempt.resultRefs),
    );
    if (resultRefs.some((resultRef) => !recordedResultRefs.has(resultRef))) throw new Error("Blind preference resultRefs must reference succeeded results for the same case");
    const preferredResultRef = preference.preferredResultRef?.trim() || undefined;
    if (!preference.tie && (!preferredResultRef || !resultRefs.includes(preferredResultRef))) throw new Error("Non-tie blind preference requires a preferred resultRef");
    if (preference.tie && preferredResultRef) throw new Error("Tie blind preference cannot have a preferred resultRef");
    required(preference.voterRef, "Blind preference voterRef");
    return { ...plan, blindPreferences: [...plan.blindPreferences, { ...preference, resultRefs, preferredResultRef }], updatedAt: preference.recordedAt };
}

/** Decisions are explicitly human-recorded; this function never infers GO/MODIFY/STOP. */
export function recordProviderEvaluationDecision(plan: ProviderEvaluationPlan, decision: ProviderEvaluationDecisionRecord): ProviderEvaluationPlan {
    if (plan.attempts.length === 0) throw new Error("Provider Evaluation decision requires recorded attempts");
    required(decision.reviewerRef, "Provider Evaluation reviewerRef");
    required(decision.rationale, "Provider Evaluation decision rationale");
    required(decision.nextAction, "Provider Evaluation nextAction");
    return { ...plan, decision, updatedAt: decision.recordedAt };
}

export function summarizeProviderEvaluation(plan: ProviderEvaluationPlan): ProviderEvaluationSummary {
    const candidateSummaries = plan.candidates.map((candidate) => summarizeCandidate(plan, candidate));
    const hasRecordedResults = plan.attempts.some((attempt) => attempt.status === "succeeded" && attempt.resultRefs.length > 0);
    const hasUnknowns = plan.attempts.some((attempt) => attempt.evidence === "unknown" || attempt.cost?.status === "unavailable")
        || candidateSummaries.some((candidate) => candidate.evidence === "unknown")
        || plan.candidates.some((candidate) => candidate.availability === "unknown");
    return {
        planId: plan.planId,
        candidateSummaries,
        blindPreferenceCount: plan.blindPreferences.length,
        decision: plan.decision?.decision || "pending",
        hasRecordedResults,
        hasUnknowns,
    };
}

export function evaluationSettingsFingerprint(settings: ProviderEvaluationSettings): string {
    return JSON.stringify(normalizeSettings(settings));
}

function summarizeCandidate(plan: ProviderEvaluationPlan, candidate: ProviderEvaluationCandidate): ProviderEvaluationCandidateSummary {
    const attempts = plan.attempts.filter((attempt) => attempt.candidateId === candidate.candidateId);
    const succeeded = attempts.filter((attempt) => attempt.status === "succeeded");
    const failed = attempts.filter((attempt) => attempt.status === "failed");
    const needsYou = attempts.filter((attempt) => attempt.status === "needs_you");
    const scores = plan.scores.filter((score) => attempts.some((attempt) => attempt.attemptId === score.attemptId));
    const dimensionAverages = averageDimensions(scores);
    const recordedCosts = attempts.filter((attempt) => attempt.cost?.status === "recorded" && typeof attempt.cost.amount === "number" && attempt.cost.currency);
    const currencies = new Set(recordedCosts.map((attempt) => attempt.cost!.currency));
    const currency = currencies.size === 1 ? [...currencies][0] : undefined;
    const knownCost = currency ? { amount: recordedCosts.reduce((sum, attempt) => sum + (attempt.cost!.amount || 0), 0), currency } : null;
    const failureModes = failed.reduce<Record<string, number>>((result, attempt) => { const key = attempt.failure?.code || "unknown"; result[key] = (result[key] || 0) + 1; return result; }, {});
    const latencies = succeeded.map((attempt) => attempt.latencyMs).filter((value): value is number => typeof value === "number");
    const evidence = attempts.length === 0 || attempts.some((attempt) => attempt.evidence === "unknown")
        ? "unknown"
        : attempts.some((attempt) => attempt.evidence === "inferred")
            ? "inferred"
            : "recorded";
    return {
        candidateId: candidate.candidateId,
        attempts: attempts.length,
        succeeded: succeeded.length,
        failed: failed.length,
        needsYou: needsYou.length,
        successRate: attempts.length ? succeeded.length / attempts.length : null,
        averageLatencyMs: latencies.length ? latencies.reduce((sum, value) => sum + value, 0) / latencies.length : null,
        knownCost,
        scoreCount: scores.length,
        dimensionAverages,
        failureModes,
        evidence,
    };
}

function averageDimensions(scores: ProviderEvaluationScore[]): Partial<ProviderEvaluationDimensions> {
    if (!scores.length) return {};
    const keys: Array<keyof ProviderEvaluationDimensions> = ["referenceFidelity", "productIdentity", "commercialQuality", "instructionFollowing", "variationAbility", "physicalPlausibility", "aiArtifactSeverity", "apiStability"];
    return Object.fromEntries(keys.map((key) => [key, scores.reduce((sum, score) => sum + score.dimensions[key], 0) / scores.length])) as Partial<ProviderEvaluationDimensions>;
}

function validateSettings(settings: ProviderEvaluationSettings): void {
    if (!settings || !Number.isInteger(settings.width) || settings.width < 1 || !Number.isInteger(settings.height) || settings.height < 1) throw new Error("Provider Evaluation requires positive integer dimensions");
    if (!isEcommerceMode(settings.mode)) throw new Error("Provider Evaluation mode is invalid");
    if (!Number.isInteger(settings.outputCount) || settings.outputCount < 1) throw new Error("Provider Evaluation outputCount must be positive");
    required(settings.quality, "Provider Evaluation quality");
    required(settings.promptTemplateVersion, "Provider Evaluation promptTemplateVersion");
    if (!settings.referenceAssetIds.length) throw new Error("Provider Evaluation requires referenceAssetIds");
}

function normalizeSettings(settings: ProviderEvaluationSettings): ProviderEvaluationSettings {
    return { ...settings, quality: settings.quality.trim(), promptTemplateVersion: settings.promptTemplateVersion.trim(), referenceAssetIds: uniqueStrings(settings.referenceAssetIds) };
}

function validateCase(item: ProviderEvaluationCase, projectId: string): void {
    required(item.caseId, "Provider Evaluation caseId");
    if (item.projectId !== projectId) throw new Error("Provider Evaluation cases must belong to the plan project");
    required(item.fixtureRevision, "Provider Evaluation fixtureRevision");
    required(item.productImageId, "Provider Evaluation productImageId");
    if (!item.sourceAssetIds.includes(item.productImageId)) throw new Error("Provider Evaluation productImageId must be in sourceAssetIds");
    required(item.createdAt, "Provider Evaluation case createdAt");
}

function validateCandidate(item: ProviderEvaluationCandidate): void {
    required(item.candidateId, "Provider Evaluation candidateId");
    required(item.adapterId, "Provider Evaluation adapterId");
    required(item.modelRef, "Provider Evaluation modelRef");
}

function validateDimensions(dimensions: ProviderEvaluationDimensions): void {
    const values = Object.values(dimensions);
    if (values.some((value) => !Number.isInteger(value) || value < 1 || value > 5)) throw new Error("Provider Evaluation dimensions must be integer scores from 1 to 5");
}

function required(value: string, label: string): string {
    const normalized = value.trim();
    if (!normalized) throw new Error(`${label} is required`);
    return normalized;
}

function assertUnique(values: string[], label: string): void {
    if (values.some((value) => !value.trim())) throw new Error(`${label} is required`);
    if (new Set(values.map((value) => value.trim())).size !== values.length) throw new Error(`${label} must be unique`);
}

function uniqueStrings(values: string[]): string[] {
    return [...new Set(values.map((value) => value.trim()).filter(Boolean))];
}
