import { createClientId } from "@/lib/client-id";
import { apiClient, request } from "@/services/api/request";
import type { EcommerceAgentRuntimeCatalog } from "@/services/api/projects";

export type EcommerceAgentRunStatus = "planning" | "ready" | "running" | "completed" | "failed" | "cancelled" | string;
export type EcommerceAgentStepStatus = "planned" | "ready" | "running" | "completed" | "failed" | "cancelled" | "skipped" | string;

export type EcommerceAgentRuntimeRun = {
    id: string;
    userId: string;
    projectId: string;
    domain: "ecommerce" | string;
    registryId: string;
    registryVersion: string;
    registryDigest: string;
    routeKind: string;
    intentRouteId?: string;
    rootRunId: string;
    status: EcommerceAgentRunStatus;
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

export type EcommerceAgentRuntimeStep = {
    id: string;
    runId: string;
    stepKey: string;
    position: number;
    routeKind: string;
    routeId: string;
    agentId: string;
    skillIdsJson: string;
    status: EcommerceAgentStepStatus;
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

export type EcommerceAgentRuntimeArtifact = {
    id: string;
    userId?: string;
    projectId: string;
    domain: "ecommerce" | string;
    artifactType: string;
    logicalKey: string;
    currentRevisionId?: string;
    revisionSequence: number;
    createdAt: string;
    updatedAt: string;
};

export type EcommerceAgentRuntimeArtifactRevision = {
    id: string;
    artifactId: string;
    version: number;
    status: "draft" | "review" | "locked" | "superseded" | "archived" | string;
    contentJson?: string;
    contentText?: string;
    contentDigest: string;
    sourceRunId?: string;
    sourceStepId?: string;
    sourceAttemptId?: string;
    createdByType?: string;
    createdById?: string;
    createdAt: string;
};

export type EcommerceAgentRuntimeDetail = {
    run: EcommerceAgentRuntimeRun;
    routingDecisions: Array<Record<string, unknown>>;
    steps: EcommerceAgentRuntimeStep[];
    attempts: Array<Record<string, unknown>>;
    humanDecisions: Array<Record<string, unknown>>;
    events: Array<Record<string, unknown>>;
    artifacts: EcommerceAgentRuntimeArtifact[];
    artifactRevisions: EcommerceAgentRuntimeArtifactRevision[];
};

export type EcommerceAgentRunCreateInput = {
    objective: string;
    input?: Record<string, unknown>;
    productAssetIds?: string[];
    productFacts?: Record<string, unknown>;
    inputArtifactRevisionIds?: string[];
    idempotencyKey?: string;
};

const api = apiClient;

function projectPath(projectId: string) {
    return `/projects/${encodeURIComponent(projectId)}/ecommerce`;
}

function idempotencyHeaders(value?: string) {
    return { "X-Idempotency-Key": value?.trim() || createClientId() };
}

export function getEcommerceAgentRuntimeCatalog(projectId: string) {
    return request<EcommerceAgentRuntimeCatalog>(api.get(`${projectPath(projectId)}/agent-runtime/catalog`));
}

export function listEcommerceAgentRuns(projectId: string, limit = 50) {
    return request<{ runs: EcommerceAgentRuntimeRun[] }>(api.get(`${projectPath(projectId)}/agent-runs`, { params: { limit } }));
}

export function createEcommerceAgentRun(projectId: string, input: EcommerceAgentRunCreateInput, idempotencyKey?: string) {
    return request<{ detail: EcommerceAgentRuntimeDetail; idempotent: boolean }>(api.post(`${projectPath(projectId)}/agent-runs`, input, { headers: idempotencyHeaders(idempotencyKey) }));
}

export function getEcommerceAgentRun(projectId: string, runId: string) {
    return request<EcommerceAgentRuntimeDetail>(api.get(`${projectPath(projectId)}/agent-runs/${encodeURIComponent(runId)}`));
}
