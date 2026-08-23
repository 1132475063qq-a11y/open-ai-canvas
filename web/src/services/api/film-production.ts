import { createClientId } from "@/lib/client-id";
import { apiClient, request } from "@/services/api/request";

export type FilmProductionQuoteStatus = "pending" | "submitted" | "expired" | "rejected" | "consumed" | string;
export type FilmProductionAttemptStatus = "queued" | "running" | "succeeded" | "failed" | "cancelled" | "uncertain" | string;
export type FilmProductionQCDecision = "PASS" | "UNCERTAIN" | "FAIL" | string;
export type FilmProductionQCAction = "accept" | "hold" | "retry" | string;
export type FilmProductionTaskStatus = "queued" | "running" | "succeeded" | "failed" | "cancelled" | string;

export type FilmProductionArtifactRef = {
    artifactId: string;
    revisionId: string;
    type: string;
    version: number;
    digest: string;
    status: string;
};

export type FilmProductionImageOptions = {
    size?: string;
    quality?: string;
    transparentBackground?: boolean;
};

export type FilmProductionImageQuoteInput = {
    rootRunId: string;
    shotId: string;
    storyboardArtifactRevisionId: string;
    promptArtifactRevisionId: string;
    feasibilityArtifactRevisionId: string;
    logicalModelId: string;
    retryOfAttemptId?: string;
    referenceResourceIds?: string[];
    options?: FilmProductionImageOptions;
};

export type FilmProductionCost = {
    required: boolean;
    billingOrderId?: string;
    billingMode?: string;
    priceVersion?: number;
    quantity?: number;
    amountMicrocredits: number;
    reservedAmountMicrocredits?: number;
    actualAmountMicrocredits?: number;
    refundedAmountMicrocredits?: number;
    status?: string;
};

export type FilmProductionImageQuote = {
    id: string;
    projectId: string;
    rootRunId: string;
    shotId: string;
    retryOfAttemptId?: string;
    artifacts: FilmProductionArtifactRef[];
    logicalModelId: string;
    model: string;
    prompt: string;
    referenceResourceIds: string[];
    options: FilmProductionImageOptions;
    count: number;
    cost: FilmProductionCost;
    quoteFingerprint: string;
    requestFingerprint: string;
    status: FilmProductionQuoteStatus;
    expiresAt: string;
    createdAt: string;
    idempotent: boolean;
};

export type FilmProductionQuoteSubmitInput = {
    quoteFingerprint: string;
};

export type FilmProductionTaskSummary = {
    id: string;
    projectId?: string;
    type: string;
    status: FilmProductionTaskStatus;
    stage: string;
    progress: number;
    prompt: string;
    operation?: string;
    provider?: string;
    model?: string;
    providerRequestId?: string;
    errorCode?: string;
    previewUrl?: string;
    previewKind?: string;
    attempts: number;
    startedAt?: string;
    completedAt?: string;
    createdAt: string;
    updatedAt: string;
    billing?: { amountMicrocredits: number; status: string };
};

export type FilmProductionResult = {
    id: string;
    userId?: string;
    taskId: string;
    attemptId?: string;
    domainProjectId?: string;
    artifactId?: string;
    artifactRevisionId?: string;
    kind: string;
    availability?: string;
    url?: string;
    payload?: string;
    createdAt: string;
};

export type FilmProductionQCReport = {
    id: string;
    attemptId: string;
    resultId: string;
    decision: FilmProductionQCDecision;
    action: FilmProductionQCAction;
    issueCodes: string[];
    evidence: Record<string, unknown>;
    note: string;
    source: string;
    assessmentKind?: string;
    modelAttemptId?: string;
    mediaState?: "structured_only" | "available" | string;
    reworkEventId?: string;
    reviewerUserId?: string;
    artifactId: string;
    revisionId: string;
    createdAt: string;
};

export type FilmProductionAttempt = {
    id: string;
    userId?: string;
    projectId: string;
    rootRunId: string;
    shotId: string;
    number: number;
    retryOfAttemptId?: string;
    quoteId: string;
    taskId: string;
    billingOrderId?: string;
    logicalModelId: string;
    model: string;
    providerModel?: string;
    attemptArtifactId: string;
    attemptRevisionId: string;
    resultId?: string;
    resultArtifactId?: string;
    resultRevisionId?: string;
    status: FilmProductionAttemptStatus;
    error?: string;
    startedAt?: string;
    completedAt?: string;
    createdAt: string;
    updatedAt: string;
};

export type FilmProductionAttemptView = {
    attempt: FilmProductionAttempt;
    task: FilmProductionTaskSummary;
    result?: FilmProductionResult;
    qcReports: FilmProductionQCReport[];
    currentQc?: FilmProductionQCReport;
    visualQcAttempts: FilmVisualQCAttemptView[];
    cost: FilmProductionCost;
    accepted: boolean;
    retryAllowed: boolean;
};

export type FilmProductionQuoteSubmitResult = {
    attempt: FilmProductionAttemptView;
    idempotent: boolean;
};

export type FilmProductionQCInput = {
    decision: FilmProductionQCDecision;
    action: FilmProductionQCAction;
    issueCodes?: string[];
    evidence?: Record<string, unknown>;
    note?: string;
};

export type FilmProductionQCResult = {
    report: FilmProductionQCReport;
    attempt: FilmProductionAttemptView;
    idempotent: boolean;
};

export type FilmVisualQCQuoteInput = {
    sourceAttemptId: string;
    logicalModelId: string;
    referenceResourceIds?: string[];
};

export type FilmVisualQCQuote = {
    id: string;
    projectId: string;
    rootRunId: string;
    shotId: string;
    sourceAttemptId: string;
    sourceResultId: string;
    logicalModelId: string;
    model: string;
    referenceResourceIds: string[];
    dimensions: string[];
    cost: FilmProductionCost;
    quoteFingerprint: string;
    requestFingerprint: string;
    status: FilmProductionQuoteStatus;
    expiresAt: string;
    createdAt: string;
    idempotent: boolean;
};

export type FilmVisualQCAttempt = {
    id: string;
    projectId: string;
    rootRunId: string;
    shotId: string;
    sourceAttemptId: string;
    sourceResultId: string;
    sourceResourceId?: string;
    sourceResultArtifactId: string;
    sourceResultRevisionId: string;
    sourceResultDigest: string;
    number: number;
    quoteId: string;
    taskId: string;
    billingOrderId?: string;
    registryId: string;
    registryVersion: string;
    registryDigest: string;
    logicalModelId: string;
    logicalModelRevisionId: string;
    model: string;
    capabilityVersion: number;
    channelPriceVersion: number;
    requestFingerprint: string;
    reportId?: string;
    reportArtifactId?: string;
    reportRevisionId?: string;
    status: FilmProductionAttemptStatus;
    error?: string;
    startedAt?: string;
    completedAt?: string;
    createdAt: string;
    updatedAt: string;
};

export type FilmVisualQCAttemptView = {
    attempt: FilmVisualQCAttempt;
    task: FilmProductionTaskSummary;
    report?: FilmProductionQCReport;
    cost: FilmProductionCost;
};

export type FilmVisualQCQuoteSubmitResult = {
    attempt: FilmVisualQCAttemptView;
    idempotent: boolean;
};

export type FilmVideoSequenceStatus = "ready" | "generating" | "needs_review" | "completed" | string;
export type FilmVideoSlotStatus = "ready" | "queued" | "running" | "needs_review" | "accepted" | "failed" | "cancelled" | "uncertain" | string;

export type FilmVideoOptions = {
    resolution?: string;
    generateAudio?: boolean;
    watermark?: boolean;
};

export type FilmVideoSequenceInput = {
    rootRunId: string;
    promptArtifactRevisionId: string;
    title?: string;
    aspectRatio: string;
    targetDurationMs: number;
    slots: Array<{
        shotId: string;
        sourceImageAttemptId: string;
        durationMs: number;
    }>;
};

export type FilmVideoSequence = {
    id: string;
    projectId: string;
    rootRunId: string;
    title: string;
    aspectRatio: string;
    targetDurationMs: number;
    promptArtifactId: string;
    promptArtifactRevisionId: string;
    promptArtifactDigest: string;
    registryId: string;
    registryVersion: string;
    registryDigest: string;
    requestFingerprint: string;
    artifactId: string;
    artifactRevisionId: string;
    status: FilmVideoSequenceStatus;
    revision: number;
    createdAt: string;
    updatedAt: string;
};

export type FilmVideoSlot = {
    id: string;
    projectId: string;
    rootRunId: string;
    sequenceId: string;
    position: number;
    shotId: string;
    prompt: string;
    durationMs: number;
    sourceImageAttemptId: string;
    sourceImageResultId: string;
    sourceImageResourceId: string;
    sourceImageArtifactId: string;
    sourceImageRevisionId: string;
    currentAttemptId?: string;
    resultId?: string;
    resultArtifactId?: string;
    resultRevisionId?: string;
    status: FilmVideoSlotStatus;
    revision: number;
    createdAt: string;
    updatedAt: string;
};

export type FilmVideoAttempt = Omit<FilmProductionAttempt, "shotId"> & {
    sequenceId: string;
    slotId: string;
    shotId?: string;
    sourceImageAttemptId: string;
    sourceImageResultId: string;
    sourceImageResourceId: string;
    sourceImageRevisionId: string;
    promptArtifactId: string;
    promptRevisionId: string;
    promptDigest: string;
};

export type FilmVideoVisualQCSample = {
    timeMs: number;
    resourceId: string;
    mimeType: string;
    size: number;
    width: number;
    height: number;
    etag: string;
};

export type FilmVideoVisualQCAttempt = {
    id: string;
    projectId: string;
    rootRunId: string;
    sequenceId: string;
    slotId: string;
    shotId: string;
    sourceAttemptId: string;
    sourceResultId: string;
    sourceResourceId: string;
    sourceResultArtifactId: string;
    sourceResultRevisionId: string;
    sourceResultDigest: string;
    number: number;
    quoteId: string;
    taskId: string;
    billingOrderId?: string;
    registryId: string;
    registryVersion: string;
    registryDigest: string;
    logicalModelId: string;
    logicalModelRevisionId: string;
    model: string;
    capabilityVersion: number;
    channelPriceVersion: number;
    requestFingerprint: string;
    reportId?: string;
    reportArtifactId?: string;
    reportRevisionId?: string;
    status: FilmProductionAttemptStatus;
    error?: string;
    startedAt?: string;
    completedAt?: string;
    createdAt: string;
    updatedAt: string;
};

export type FilmVideoVisualQCAttemptView = {
    attempt: FilmVideoVisualQCAttempt;
    task: FilmProductionTaskSummary;
    report?: FilmProductionQCReport;
    sampleFrames: FilmVideoVisualQCSample[];
    cost: FilmProductionCost;
    valid: boolean;
};

export type FilmVideoAttemptView = Omit<FilmProductionAttemptView, "attempt" | "visualQcAttempts"> & {
    attempt: FilmVideoAttempt;
    visualQcAttempts: FilmVideoVisualQCAttemptView[];
};

export type FilmVideoSequenceView = {
    sequence: FilmVideoSequence;
    slots: Array<{ slot: FilmVideoSlot; attempts: FilmVideoAttemptView[] }>;
    continuity?: FilmContinuityLedgerView;
    sequenceReview?: FilmVideoSequenceReview;
    sequenceVisualQcAttempts: FilmVideoSequenceVisualQCAttemptView[];
    reworkEvents: FilmReworkEvent[];
    idempotent?: boolean;
};

export type FilmVideoSequenceReview = {
    id: string;
    sequenceId: string;
    ledgerId: string;
    decision: "PASS" | "UNCERTAIN" | "FAIL" | string;
    action: "accept" | "hold" | "retry" | string;
    issueCodes: string[];
    evidence: Record<string, unknown>;
    note: string;
    source: string;
    assessmentKind?: string;
    modelAttemptId?: string;
    reviewerUserId?: string;
    scopeFingerprint: string;
    artifactId: string;
    revisionId: string;
    createdAt: string;
    valid: boolean;
};

export type FilmVideoSequenceReviewInput = {
    decision: "PASS" | "UNCERTAIN" | "FAIL";
    action: "accept" | "hold" | "retry";
    issueCodes?: string[];
    evidence?: Record<string, unknown>;
    note?: string;
};

export type FilmVideoSequenceReviewResult = {
    review: FilmVideoSequenceReview;
    sequence: FilmVideoSequenceView;
    idempotent: boolean;
};

export type FilmContinuityLedger = {
    id: string;
    projectId: string;
    rootRunId: string;
    sequenceId: string;
    mediaState: "structured_only" | "available" | string;
    status: "ready" | "needs_you" | string;
    issueCount: number;
    artifactId: string;
    artifactRevisionId: string;
    createdAt: string;
};

export type FilmContinuityShotState = {
    id: string;
    shotId: string;
    position: number;
    readIn: Record<string, unknown>;
    writeOut: Record<string, unknown>;
    dimensions: Record<string, string>;
    referenceLock: Record<string, unknown>;
    status: "ready" | "needs_review" | string;
};

export type FilmContinuityIssue = {
    id: string;
    shotId?: string;
    dimension: string;
    severity: string;
    code: string;
    message: string;
    authority: string;
    owner: string;
    repairStatus: string;
    sourceRefs: Array<Record<string, unknown>>;
    createdAt: string;
};

export type FilmContinuityLedgerView = {
    ledger: FilmContinuityLedger;
    shots: FilmContinuityShotState[];
    issues: FilmContinuityIssue[];
};

export type FilmReworkEvent = {
    id: string;
    projectId: string;
    rootRunId: string;
    sequenceId?: string;
    slotId?: string;
    shotId?: string;
    attemptId: string;
    resultId: string;
    qcReportId: string;
    mediaType: string;
    source: string;
    severity: string;
    reasonCode: string;
    owner: string;
    repairScope: Record<string, unknown>;
    recheckGate: Record<string, unknown>;
    status: "open" | "resolved" | "superseded" | string;
    artifactId: string;
    artifactRevisionId: string;
    createdAt: string;
};

export type FilmVideoQuoteInput = {
    sequenceId: string;
    slotId: string;
    logicalModelId: string;
    retryOfAttemptId?: string;
    options?: FilmVideoOptions;
};

export type FilmVideoQuote = {
    id: string;
    projectId: string;
    rootRunId: string;
    sequenceId: string;
    slotId: string;
    retryOfAttemptId?: string;
    logicalModelId: string;
    model: string;
    prompt: string;
    sourceImageResourceId: string;
    durationMs: number;
    aspectRatio: string;
    options: FilmVideoOptions;
    cost: FilmProductionCost;
    quoteFingerprint: string;
    requestFingerprint: string;
    status: FilmProductionQuoteStatus;
    expiresAt: string;
    createdAt: string;
    idempotent: boolean;
};

export type FilmVideoQuoteSubmitResult = {
    attempt: FilmVideoAttemptView;
    idempotent: boolean;
};

export type FilmVideoVisualQCQuoteInput = {
    sourceAttemptId: string;
    logicalModelId: string;
    sampleFrames: Array<{ timeMs: number; resourceId: string }>;
};

export type FilmVideoVisualQCQuote = {
    id: string;
    projectId: string;
    rootRunId: string;
    sequenceId: string;
    slotId: string;
    shotId: string;
    sourceAttemptId: string;
    sourceResultId: string;
    logicalModelId: string;
    model: string;
    sampleFrames: FilmVideoVisualQCSample[];
    dimensions: string[];
    evidenceLimitation: string;
    cost: FilmProductionCost;
    quoteFingerprint: string;
    requestFingerprint: string;
    status: FilmProductionQuoteStatus;
    expiresAt: string;
    createdAt: string;
    idempotent: boolean;
};

export type FilmVideoVisualQCQuoteSubmitResult = {
    attempt: FilmVideoVisualQCAttemptView;
    idempotent: boolean;
};

export type FilmVideoSequenceVisualQCSample = FilmVideoVisualQCSample;

export type FilmVideoSequenceVisualQCSlotEvidence = {
    position: number;
    slotId: string;
    slotRevision: number;
    shotId: string;
    durationMs: number;
    sourceAttemptId: string;
    sourceResultId: string;
    sourceResourceId: string;
    sourceResultArtifactId: string;
    sourceResultRevisionId: string;
    sourceResultDigest: string;
    sourceImageResourceId: string;
    sourceImageArtifactId: string;
    sourceImageRevisionId: string;
    sourceImageRevisionDigest: string;
    samples: FilmVideoSequenceVisualQCSample[];
};

export type FilmVideoSequenceVisualQCQuoteInput = {
    sequenceId: string;
    logicalModelId: string;
    slots: Array<{ slotId: string; sampleFrames: Array<{ timeMs: number; resourceId: string }> }>;
};

export type FilmVideoSequenceVisualQCQuote = {
    id: string;
    projectId: string;
    rootRunId: string;
    sequenceId: string;
    ledgerId: string;
    logicalModelId: string;
    model: string;
    slotEvidence: FilmVideoSequenceVisualQCSlotEvidence[];
    dimensions: string[];
    evidenceLimitation: string;
    cost: FilmProductionCost;
    quoteFingerprint: string;
    requestFingerprint: string;
    status: FilmProductionQuoteStatus;
    expiresAt: string;
    createdAt: string;
    idempotent: boolean;
};

export type FilmVideoSequenceVisualQCAttempt = {
    id: string;
    projectId: string;
    rootRunId: string;
    sequenceId: string;
    sequenceRevision: number;
    ledgerId: string;
    ledgerArtifactId: string;
    ledgerRevisionId: string;
    ledgerDigest: string;
    scopeFingerprint: string;
    promptArtifactId: string;
    promptRevisionId: string;
    promptDigest: string;
    number: number;
    quoteId: string;
    taskId: string;
    billingOrderId?: string;
    registryId: string;
    registryVersion: string;
    registryDigest: string;
    logicalModelId: string;
    logicalModelRevisionId: string;
    model: string;
    capabilityVersion: number;
    channelPriceVersion: number;
    requestFingerprint: string;
    reportId?: string;
    reportArtifactId?: string;
    reportRevisionId?: string;
    status: FilmProductionAttemptStatus;
    error?: string;
    startedAt?: string;
    completedAt?: string;
    createdAt: string;
    updatedAt: string;
};

export type FilmVideoSequenceVisualQCAttemptView = {
    attempt: FilmVideoSequenceVisualQCAttempt;
    task: FilmProductionTaskSummary;
    report?: FilmVideoSequenceReview;
    slotEvidence: FilmVideoSequenceVisualQCSlotEvidence[];
    cost: FilmProductionCost;
    valid: boolean;
};

export type FilmVideoSequenceVisualQCQuoteSubmitResult = {
    attempt: FilmVideoSequenceVisualQCAttemptView;
    idempotent: boolean;
};

export type FilmVideoQCResult = {
    report: FilmProductionQCReport;
    attempt: FilmVideoAttemptView;
    idempotent: boolean;
};

export type ListFilmProductionAttemptsOptions = {
    rootRunId?: string;
    shotId?: string;
    limit?: number;
};

const api = apiClient;

function projectPath(projectId: string) {
    return `/projects/${encodeURIComponent(projectId)}/film/production`;
}

function idempotencyHeaders(value?: string) {
    return { "X-Idempotency-Key": value?.trim() || createClientId() };
}

export function createFilmProductionImageQuote(projectId: string, input: FilmProductionImageQuoteInput, idempotencyKey?: string) {
    return request<FilmProductionImageQuote>(api.post(`${projectPath(projectId)}/image-quotes`, input, { headers: idempotencyHeaders(idempotencyKey) }));
}

export function submitFilmProductionImageQuote(projectId: string, quoteId: string, input: FilmProductionQuoteSubmitInput, idempotencyKey?: string) {
    return request<FilmProductionQuoteSubmitResult>(api.post(`${projectPath(projectId)}/image-quotes/${encodeURIComponent(quoteId)}/submit`, input, { headers: idempotencyHeaders(idempotencyKey) }));
}

export function listFilmProductionAttempts(projectId: string, options: ListFilmProductionAttemptsOptions = {}) {
    return request<{ attempts: FilmProductionAttemptView[] }>(api.get(`${projectPath(projectId)}/attempts`, { params: options }));
}

export function createFilmProductionHumanQC(projectId: string, attemptId: string, input: FilmProductionQCInput, idempotencyKey?: string) {
    return request<FilmProductionQCResult>(api.post(`${projectPath(projectId)}/attempts/${encodeURIComponent(attemptId)}/qc`, input, { headers: idempotencyHeaders(idempotencyKey) }));
}

export function createFilmVisualQCQuote(projectId: string, input: FilmVisualQCQuoteInput, idempotencyKey?: string) {
    return request<FilmVisualQCQuote>(api.post(`${projectPath(projectId)}/visual-qc-quotes`, input, { headers: idempotencyHeaders(idempotencyKey) }));
}

export function submitFilmVisualQCQuote(projectId: string, quoteId: string, input: FilmProductionQuoteSubmitInput, idempotencyKey?: string) {
    return request<FilmVisualQCQuoteSubmitResult>(api.post(`${projectPath(projectId)}/visual-qc-quotes/${encodeURIComponent(quoteId)}/submit`, input, { headers: idempotencyHeaders(idempotencyKey) }));
}

export function createFilmVideoSequence(projectId: string, input: FilmVideoSequenceInput, idempotencyKey?: string) {
    return request<FilmVideoSequenceView>(api.post(`${projectPath(projectId)}/video-sequences`, input, { headers: idempotencyHeaders(idempotencyKey) }));
}

export function listFilmVideoSequences(projectId: string, options: { rootRunId?: string; limit?: number } = {}) {
    return request<{ sequences: FilmVideoSequenceView[] }>(api.get(`${projectPath(projectId)}/video-sequences`, { params: options }));
}

export function createFilmVideoSequenceReview(projectId: string, sequenceId: string, input: FilmVideoSequenceReviewInput, idempotencyKey?: string) {
    return request<FilmVideoSequenceReviewResult>(api.post(`${projectPath(projectId)}/video-sequences/${encodeURIComponent(sequenceId)}/review`, input, { headers: idempotencyHeaders(idempotencyKey) }));
}

export function createFilmVideoQuote(projectId: string, input: FilmVideoQuoteInput, idempotencyKey?: string) {
    return request<FilmVideoQuote>(api.post(`${projectPath(projectId)}/video-quotes`, input, { headers: idempotencyHeaders(idempotencyKey) }));
}

export function submitFilmVideoQuote(projectId: string, quoteId: string, input: FilmProductionQuoteSubmitInput, idempotencyKey?: string) {
    return request<FilmVideoQuoteSubmitResult>(api.post(`${projectPath(projectId)}/video-quotes/${encodeURIComponent(quoteId)}/submit`, input, { headers: idempotencyHeaders(idempotencyKey) }));
}

export function createFilmVideoVisualQCQuote(projectId: string, input: FilmVideoVisualQCQuoteInput, idempotencyKey?: string) {
    return request<FilmVideoVisualQCQuote>(api.post(`${projectPath(projectId)}/video-visual-qc-quotes`, input, { headers: idempotencyHeaders(idempotencyKey) }));
}

export function submitFilmVideoVisualQCQuote(projectId: string, quoteId: string, input: FilmProductionQuoteSubmitInput, idempotencyKey?: string) {
    return request<FilmVideoVisualQCQuoteSubmitResult>(api.post(`${projectPath(projectId)}/video-visual-qc-quotes/${encodeURIComponent(quoteId)}/submit`, input, { headers: idempotencyHeaders(idempotencyKey) }));
}

export function createFilmVideoSequenceVisualQCQuote(projectId: string, input: FilmVideoSequenceVisualQCQuoteInput, idempotencyKey?: string) {
    return request<FilmVideoSequenceVisualQCQuote>(api.post(`${projectPath(projectId)}/video-sequence-visual-qc-quotes`, input, { headers: idempotencyHeaders(idempotencyKey) }));
}

export function submitFilmVideoSequenceVisualQCQuote(projectId: string, quoteId: string, input: FilmProductionQuoteSubmitInput, idempotencyKey?: string) {
    return request<FilmVideoSequenceVisualQCQuoteSubmitResult>(api.post(`${projectPath(projectId)}/video-sequence-visual-qc-quotes/${encodeURIComponent(quoteId)}/submit`, input, { headers: idempotencyHeaders(idempotencyKey) }));
}

export function createFilmVideoHumanQC(projectId: string, attemptId: string, input: FilmProductionQCInput, idempotencyKey?: string) {
    return request<FilmVideoQCResult>(api.post(`${projectPath(projectId)}/video-attempts/${encodeURIComponent(attemptId)}/qc`, input, { headers: idempotencyHeaders(idempotencyKey) }));
}
