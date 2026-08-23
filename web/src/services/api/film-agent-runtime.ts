import { createClientId } from "@/lib/client-id";
import { apiClient, request } from "@/services/api/request";

export type FilmAgentRunStatus = "planning" | "ready" | "running" | "awaiting_human" | "completed" | "failed" | "cancelled" | string;
export type FilmAgentStepStatus = "planned" | "ready" | "running" | "awaiting_human" | "completed" | "failed" | "cancelled" | "skipped" | string;
export type FilmArtifactStatus = "draft" | "review" | "locked" | "superseded" | "archived" | string;

export type FilmAgentIntentRoute = {
    id: string;
    name?: string;
    description?: string;
    primaryAgentId?: string;
    candidateAgentIds?: string[];
    skillIds?: string[];
    outputArtifactTypes?: string[];
    requiresDisambiguation?: boolean;
    triggerPhrases?: string[];
    [key: string]: unknown;
};

export type FilmAgentRuntimeCatalog = {
    registry: {
        id: string;
        version: string;
        domain: string;
        sourceDigest: string;
        agentCount: number;
        skillCount: number;
        intentRouteCount: number;
        handoffRouteCount: number;
    };
    agents: Array<{ id: string; description: string; skillIds: string[]; sandboxMode?: string }>;
    skills: Array<{ id: string; version: string; description: string; ownerAgentIds: string[] }>;
    intentRoutes: FilmAgentIntentRoute[];
    handoffRoutes: Array<Record<string, unknown>>;
    artifactTypes: Array<Record<string, unknown>>;
};

export type FilmAgentRun = {
    id: string;
    userId?: string;
    projectId: string;
    canvasId?: string;
    domain: string;
    registryId: string;
    registryVersion: string;
    registryDigest: string;
    routeKind: string;
    intentRouteId?: string;
    handoffRouteId?: string;
    rootRunId: string;
    parentRunId?: string;
    status: FilmAgentRunStatus;
    objective: string;
    inputJson?: string;
    currentStepId?: string;
    idempotencyKey?: string;
    revision: number;
    eventSequence: number;
    failureCode?: string;
    failure?: string;
    startedAt?: string;
    completedAt?: string;
    createdAt: string;
    updatedAt: string;
};

export type FilmAgentStep = {
    id: string;
    runId: string;
    stepKey: string;
    position: number;
    routeKind: string;
    routeId: string;
    agentId: string;
    skillIdsJson: string;
    status: FilmAgentStepStatus;
    dependsOnStepIdsJson: string;
    inputArtifactRefsJson: string;
    expectedOutputArtifactTypesJson: string;
    outputArtifactRefsJson: string;
    attemptSequence: number;
    revision: number;
    failureCode?: string;
    failure?: string;
    startedAt?: string;
    completedAt?: string;
    createdAt: string;
    updatedAt: string;
};

export type FilmProductionArtifact = {
    id: string;
    userId?: string;
    projectId: string;
    domain: string;
    artifactType: string;
    logicalKey: string;
    currentRevisionId?: string;
    revisionSequence: number;
    createdAt: string;
    updatedAt: string;
};

export type FilmProductionArtifactRevision = {
    id: string;
    artifactId: string;
    version: number;
    status: FilmArtifactStatus;
    contentJson?: string;
    contentText?: string;
    contentDigest: string;
    sourceRunId?: string;
    sourceStepId?: string;
    sourceAttemptId?: string;
    parentRevisionId?: string;
    sourceArtifactRefsJson?: string;
    authorityRefsJson?: string;
    createdByType?: string;
    createdById?: string;
    createdAt: string;
};

export type FilmAgentHumanDecision = {
    id: string;
    runId: string;
    stepId?: string;
    status: "pending" | "resolved" | "cancelled" | string;
    question: string;
    optionsJson: string;
    recommendation?: string;
    responseJson?: string;
    impactRefsJson?: string;
    revision: number;
    createdAt: string;
    updatedAt: string;
};

export type FilmAgentRunDetail = {
    run: FilmAgentRun;
    routingDecisions: Array<Record<string, unknown>>;
    steps: FilmAgentStep[];
    attempts: Array<Record<string, unknown>>;
    humanDecisions: FilmAgentHumanDecision[];
    events: Array<Record<string, unknown>>;
    artifacts: FilmProductionArtifact[];
    artifactRevisions: FilmProductionArtifactRevision[];
};

export type FilmAgentRunCreateInput = {
    canvasId?: string;
    objective: string;
    intentRouteId?: string;
    agentId?: string;
    logicalModelId?: string;
    input?: Record<string, unknown>;
    inputArtifactRevisionIds?: string[];
    reviewBeforeExecution?: boolean;
};

export type ResolveFilmAgentDecisionInput = {
    expectedRunRevision: number;
    expectedStepRevision: number;
    expectedDecisionRevision: number;
    action: "approve" | "cancel";
    response?: Record<string, unknown>;
};

export type RetryFilmAgentStepInput = {
    expectedRunRevision: number;
    expectedStepRevision: number;
    reason?: string;
};

export type LockFilmAgentArtifactInput = {
    expectedRunRevision: number;
    expectedArtifactSequence: number;
    expectedRevisionId: string;
};

export type RollbackFilmAgentArtifactInput = {
    expectedRunRevision: number;
    expectedArtifactSequence: number;
    expectedCurrentRevisionId: string;
    targetRevisionId: string;
};

const api = apiClient;

function projectPath(projectId: string) {
    return `/projects/${encodeURIComponent(projectId)}/film`;
}

function idempotencyHeaders(value?: string) {
    return { "X-Idempotency-Key": value?.trim() || createClientId() };
}

export function getFilmAgentRuntimeCatalog(projectId: string) {
    return request<FilmAgentRuntimeCatalog>(api.get(`${projectPath(projectId)}/agent-runtime/catalog`));
}

export function listFilmAgentRuns(projectId: string, limit = 50) {
    return request<{ runs: FilmAgentRun[] }>(api.get(`${projectPath(projectId)}/agent-runs`, { params: { limit } }));
}

export function createFilmAgentRun(projectId: string, input: FilmAgentRunCreateInput, idempotencyKey?: string) {
    return request<{ detail: FilmAgentRunDetail; idempotent: boolean }>(api.post(`${projectPath(projectId)}/agent-runs`, input, { headers: idempotencyHeaders(idempotencyKey) }));
}

export function getFilmAgentRun(projectId: string, runId: string) {
    return request<FilmAgentRunDetail>(api.get(`${projectPath(projectId)}/agent-runs/${encodeURIComponent(runId)}`));
}

export function resolveFilmAgentDecision(projectId: string, runId: string, decisionId: string, input: ResolveFilmAgentDecisionInput) {
    return request<FilmAgentRunDetail>(api.post(`${projectPath(projectId)}/agent-runs/${encodeURIComponent(runId)}/decisions/${encodeURIComponent(decisionId)}/resolve`, input));
}

export function retryFilmAgentStep(projectId: string, runId: string, stepId: string, input: RetryFilmAgentStepInput) {
    return request<FilmAgentRunDetail>(api.post(`${projectPath(projectId)}/agent-runs/${encodeURIComponent(runId)}/steps/${encodeURIComponent(stepId)}/retry`, input));
}

export function lockFilmAgentArtifact(projectId: string, runId: string, artifactId: string, input: LockFilmAgentArtifactInput) {
    return request<{ run: FilmAgentRun; artifact: FilmProductionArtifact; sourceRevision: FilmProductionArtifactRevision; lockedRevision: FilmProductionArtifactRevision; trigger: Record<string, unknown> }>(
        api.post(`${projectPath(projectId)}/agent-runs/${encodeURIComponent(runId)}/artifacts/${encodeURIComponent(artifactId)}/lock`, input),
    );
}

export function rollbackFilmAgentArtifact(projectId: string, runId: string, artifactId: string, input: RollbackFilmAgentArtifactInput, idempotencyKey?: string) {
    return request<{
        run: FilmAgentRun;
        artifact: FilmProductionArtifact;
        targetRevision: FilmProductionArtifactRevision;
        rollbackRevision: FilmProductionArtifactRevision;
        idempotent: boolean;
    }>(api.post(`${projectPath(projectId)}/agent-runs/${encodeURIComponent(runId)}/artifacts/${encodeURIComponent(artifactId)}/rollback`, input, { headers: idempotencyHeaders(idempotencyKey) }));
}
