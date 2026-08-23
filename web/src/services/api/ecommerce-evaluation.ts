import { createClientId } from "@/lib/client-id";
import { apiClient, request } from "@/services/api/request";
import type { BlindPreferenceRecord, ProviderEvaluationCandidate, ProviderEvaluationCase, ProviderEvaluationDimensions, ProviderEvaluationPlan, ProviderEvaluationSettings, ProviderEvaluationVariant } from "@/ecommerce/evaluation/provider-evaluation";

export type EcommerceProviderEvaluationPlanRow = {
    id: string;
    userId: string;
    projectId: string;
    skillRef: string;
    mode: string;
    status: string;
    requestFingerprint: string;
    decision?: string;
    decisionReviewerRef?: string;
    decisionRationale?: string;
    decisionNextAction?: string;
    decisionEvidence?: string;
    decisionRecordedAt?: string;
    createdAt: string;
    updatedAt: string;
};

export type EcommerceProviderEvaluationAttempt = {
    id: string;
    userId: string;
    projectId: string;
    planId: string;
    caseId: string;
    candidateId: string;
    variant: ProviderEvaluationVariant;
    status: string;
    settingsFingerprint: string;
    requestFingerprint: string;
    providerJobId?: string;
    startedAt: string;
    completedAt?: string;
    latencyMs?: number;
    evidence: string;
    createdAt: string;
    updatedAt: string;
};

export type EcommerceProviderEvaluationPlanView = {
    plan: EcommerceProviderEvaluationPlanRow;
    settings: ProviderEvaluationSettings;
    cases: ProviderEvaluationCase[];
    candidates: ProviderEvaluationCandidate[];
    attempts: Array<{
        attempt: EcommerceProviderEvaluationAttempt;
        resultRefs: string[];
        usage?: Record<string, unknown>;
        cost?: Record<string, unknown>;
        failure?: Record<string, unknown>;
    }>;
    scores: Array<{
        score: {
            id: string;
            planId: string;
            attemptId: string;
            evaluatorRef: string;
            notes?: string;
            evidence: string;
            recordedAt: string;
        };
        dimensions: ProviderEvaluationDimensions;
    }>;
    blindPreferences: Array<{
        preference: Omit<BlindPreferenceRecord, "preferenceId"> & { id: string; planId: string };
        resultRefs: string[];
    }>;
    summary: {
        planId: string;
        candidateSummaries: Array<Record<string, unknown>>;
        blindPreferenceCount: number;
        decision: string;
        hasRecordedResults: boolean;
        hasUnknowns: boolean;
    };
};

export type CreateEcommerceProviderEvaluationPlanInput = {
    planId?: string;
    idempotencyKey?: string;
    skillRef: string;
    settings: ProviderEvaluationSettings;
    cases: ProviderEvaluationCase[];
    candidates: ProviderEvaluationCandidate[];
};

export type RecordEcommerceProviderEvaluationAttemptInput = {
    attemptId?: string;
    idempotencyKey?: string;
    caseId: string;
    candidateId: string;
    variant: ProviderEvaluationVariant;
    status: "queued" | "running" | "succeeded" | "failed" | "cancelled" | "needs_you";
    settingsFingerprint: string;
    requestFingerprint: string;
    providerJobId?: string;
    resultRefs?: string[];
    startedAt: string;
    completedAt?: string;
    latencyMs?: number;
    usage?: Record<string, unknown>;
    cost?: Record<string, unknown>;
    failure?: Record<string, unknown>;
    evidence: "recorded" | "inferred" | "unknown";
};

function idempotencyHeaders(idempotencyKey?: string) {
    return { "X-Idempotency-Key": idempotencyKey || createClientId() };
}

function projectPath(projectId: string) {
    return `/projects/${encodeURIComponent(projectId)}/ecommerce/provider-evaluations`;
}

export function listEcommerceProviderEvaluations(projectId: string, limit = 20) {
    return request<{ plans: EcommerceProviderEvaluationPlanRow[] }>(apiClient.get(projectPath(projectId), { params: { limit } }));
}

export function getEcommerceProviderEvaluation(projectId: string, planId: string) {
    return request<EcommerceProviderEvaluationPlanView>(apiClient.get(`${projectPath(projectId)}/${encodeURIComponent(planId)}`));
}

export function createEcommerceProviderEvaluation(projectId: string, input: CreateEcommerceProviderEvaluationPlanInput) {
    const idempotencyKey = input.idempotencyKey || createClientId();
    return request<{ plan: EcommerceProviderEvaluationPlanView; idempotent: boolean }>(apiClient.post(projectPath(projectId), { ...input, idempotencyKey }, { headers: idempotencyHeaders(idempotencyKey) }));
}

export function recordEcommerceProviderEvaluationAttempt(projectId: string, planId: string, input: RecordEcommerceProviderEvaluationAttemptInput) {
    const idempotencyKey = input.idempotencyKey || createClientId();
    return request<{ plan: EcommerceProviderEvaluationPlanView; idempotent: boolean }>(apiClient.post(`${projectPath(projectId)}/${encodeURIComponent(planId)}/attempts`, { ...input, idempotencyKey }, { headers: idempotencyHeaders(idempotencyKey) }));
}

export function recordEcommerceProviderEvaluationScore(
    projectId: string,
    planId: string,
    input: {
        scoreId?: string;
        idempotencyKey?: string;
        attemptId: string;
        evaluatorRef: string;
        dimensions: ProviderEvaluationDimensions;
        notes?: string;
        evidence: "recorded" | "inferred" | "unknown";
        recordedAt?: string;
    },
) {
    const idempotencyKey = input.idempotencyKey || createClientId();
    return request<{ plan: EcommerceProviderEvaluationPlanView; idempotent: boolean }>(apiClient.post(`${projectPath(projectId)}/${encodeURIComponent(planId)}/scores`, { ...input, idempotencyKey }, { headers: idempotencyHeaders(idempotencyKey) }));
}

export function recordEcommerceProviderEvaluationPreference(projectId: string, planId: string, input: BlindPreferenceRecord & { idempotencyKey?: string }) {
    const idempotencyKey = input.idempotencyKey || createClientId();
    return request<{ plan: EcommerceProviderEvaluationPlanView; idempotent: boolean }>(
        apiClient.post(`${projectPath(projectId)}/${encodeURIComponent(planId)}/blind-preferences`, { ...input, idempotencyKey, preferenceId: input.preferenceId }, { headers: idempotencyHeaders(idempotencyKey) }),
    );
}

export function recordEcommerceProviderEvaluationDecision(
    projectId: string,
    planId: string,
    input: {
        decision: "go" | "modify" | "stop";
        reviewerRef: string;
        rationale: string;
        nextAction: string;
        evidence: "recorded" | "inferred" | "unknown";
    },
) {
    return request<EcommerceProviderEvaluationPlanView>(apiClient.post(`${projectPath(projectId)}/${encodeURIComponent(planId)}/decision`, input));
}

export type { ProviderEvaluationPlan };
