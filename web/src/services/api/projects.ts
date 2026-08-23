import { apiClient, request } from "@/services/api/request";
import type { GenerationTask } from "@/services/api/task-center";

const api = apiClient;

export type Project = {
    id: string;
    userId: string;
    name: string;
    type: string;
    aspectRatio: string;
    sourceType: string;
    description: string;
    stylePresetId: string;
    styleProfileJson?: string;
    status: "active" | "archived" | string;
    revision: number;
    createdAt: string;
    updatedAt: string;
};

export type ProjectCanvas = {
    id: string;
    projectId?: string;
    title: string;
    createdAt: string;
    updatedAt: string;
};

export type CanvasUnitLink = {
    id: string;
    projectId: string;
    canvasId: string;
    unitId: string;
    role: string;
    createdAt: string;
};

export type ProjectUnit = {
    id: string;
    projectId: string;
    kind: "chapter" | "episode" | string;
    title: string;
    sourceText: string;
    status: "draft" | "ready" | "completed" | string;
    position: number;
    createdAt: string;
    updatedAt: string;
};

export type ProjectAsset = {
    id: string;
    title: string;
    mediaType: string;
    category: string;
    projectRole?: string;
    status: string;
    primaryVersionId?: string;
    versionCount: number;
    usages: string[];
    folderId?: string;
    position: number;
    storageKey?: string;
    previewText?: string;
    updatedAt: string;
    character?: CharacterCardSummary;
};

export type EcommerceArtifact = {
    id: string;
    projectId: string;
    artifactKey: string;
    artifactType: string;
    schemaVersion: number;
    revision: number;
    lifecycle: "draft" | "review" | "finalized" | "superseded" | "archived" | string;
    evidence: "recorded" | "inferred" | "unknown" | string;
    responsibleAgentId?: string;
    skillRef?: string;
    payloadJson: string;
    sourceRefsJson: string;
    authorityRefsJson: string;
    createdAt: string;
    updatedAt: string;
};

export type EcommerceKernel = "MODEL_INTERACTION" | "STILL_LIFE";

export type EcommerceShotCameraSpec = {
    azimuth: string;
    elevation: string;
    cameraHeight: string;
    lens: string;
    distance: string;
    subjectRegion: string;
    subjectFill: string;
    pose: string;
    composition: string;
    avoidReuseOf?: string[];
};

export type EcommercePresetShotRole = {
    key: string;
    title: string;
    framing: string;
    direction: string;
    interaction: string;
    durationMs: number;
    camera: EcommerceShotCameraSpec;
};

export type EcommercePresetConstraints = {
    productFidelity: string[];
    identitySafety: string[];
    commercial: string[];
    cost: string[];
};

export type EcommercePresetDefinition = {
    schemaVersion: number;
    skillRef: string;
    kernel: EcommerceKernel;
    supportedCategories: string[];
    sceneTemplate: string;
    interactionTemplate: string;
    shotRoles: EcommercePresetShotRole[];
    requiredConstraints: EcommercePresetConstraints;
    negativePrompt: string;
};

export type EcommercePreset = {
    id: string;
    presetKey: string;
    version: number;
    name: string;
    kernel: EcommerceKernel;
    category: string;
    description: string;
    system: boolean;
    sourceId?: string;
    definition: EcommercePresetDefinition;
    updatedAt: string;
};

export type EcommercePresetCatalog = {
    schemaVersion: number;
    system: EcommercePreset[];
    custom: EcommercePreset[];
};

export type EcommerceProviderRoute = {
    channelId: string;
    channelName: string;
    channelModelId: string;
    model: string;
    modelDisplayName: string;
    protocol: string;
    billingMode: string;
    unitPriceMicrocredits?: number;
    priceVersion: number;
    capabilityVersion: number;
    maxReferenceImages: number;
    supportsImageEdit: boolean;
    providerReady: boolean;
    billingReady: boolean;
    routeReady: boolean;
    blockers: string[];
};

export type EcommerceQuote = {
    fingerprint: string;
    expiresAt: string;
    channelId: string;
    channelModelId: string;
    model: string;
    resolution?: string;
    pixelSize?: string;
    priceVersion: number;
    unitMicrocredits: number;
    count: number;
    totalMicrocredits: number;
};

export type EcommerceProductionRun = {
    id: string;
    userId: string;
    projectId: string;
    idempotencyKey: string;
    status: "planning" | "awaiting_review" | "awaiting_cost" | "generating" | "qa" | "needs_you" | "ready" | "failed" | "cancelled" | string;
    kernel: EcommerceKernel;
    category: string;
    presetId: string;
    presetVersion: number;
    targetChannel: string;
    aspectRatio: string;
    resolution?: string;
    pixelSize?: string;
    outputCount: number;
    reviewBeforeGeneration: boolean;
    productAssetIdsJson: string;
    supportingAssetIdsJson: string;
    modelAssetIdsJson: string;
    sceneAssetIdsJson: string;
    brandAssetIdsJson: string;
    userGoal: string;
    modelMode: string;
    modelBrief: string;
    sceneBrief: string;
    brandBrief: string;
    advancedJson: string;
    productDnaArtifactId: string;
    modelProfileArtifactId?: string;
    scenePackArtifactId: string;
    presetSnapshotArtifactId: string;
    generationRequestArtifactId: string;
    motionPlanArtifactId?: string;
    videoSequenceArtifactId?: string;
    quoteFingerprint?: string;
    quoteExpiresAt?: string;
    quoteChannelId?: string;
    quoteChannelModelId?: string;
    quoteModel?: string;
    quotePriceVersion?: number;
    quoteUnitMicrocredits?: number;
    quoteTotalMicrocredits?: number;
    error?: string;
    submittedAt?: string;
    completedAt?: string;
    createdAt: string;
    updatedAt: string;
};

export type EcommerceProductionSlot = {
    id: string;
    userId: string;
    projectId: string;
    runId: string;
    position: number;
    role: string;
    title: string;
    cameraJson: string;
    prompt: string;
    negativePrompt: string;
    status: "planned" | "scheduled" | "queued" | "running" | "qa" | "accepted" | "failed" | "cancelled" | string;
    qaStatus: "PENDING" | "PASS" | "UNCERTAIN" | "FAIL" | string;
    qaIssuesJson: string;
    qaNote: string;
    accepted: boolean;
    acceptedAttemptId?: string;
    acceptedAt?: string;
    activeAttemptId?: string;
    activeTaskId?: string;
    resultUrl?: string;
    resultPayloadJson?: string;
    compositionHash?: string;
    compositionHashAlgorithm?: string;
    duplicateOfSlotId?: string;
    compositionDistance?: number;
    regenerationReason?: string;
    generatedAssetId?: string;
    createdAt: string;
    updatedAt: string;
};

export type EcommerceProductionAttempt = {
    id: string;
    userId: string;
    projectId: string;
    runId: string;
    slotId: string;
    attemptNumber: number;
    kind: "initial" | "retry" | "repair" | "variation" | string;
    status: string;
    prompt: string;
    negativePrompt: string;
    quoteFingerprint?: string;
    quoteExpiresAt?: string;
    quoteChannelId?: string;
    quoteChannelModelId?: string;
    quoteModel?: string;
    quotePriceVersion?: number;
    quoteAmountMicrocredits?: number;
    taskId?: string;
    billingOrderId?: string;
    providerRequestId?: string;
    resultId?: string;
    generatedAssetArtifactId?: string;
    resultUrl?: string;
    resultPayloadJson?: string;
    error?: string;
    startedAt?: string;
    completedAt?: string;
    createdAt: string;
    updatedAt: string;
};

export type EcommerceQADimensionAssessment = {
    key: string;
    score?: number | null;
    verdict: "PASS" | "UNCERTAIN" | "FAIL" | "NOT_APPLICABLE" | string;
    note?: string;
};

export type EcommerceQARuntimeEvidence = {
    source: string;
    attemptId: string;
    attemptStatus: string;
    taskId?: string;
    taskStatus?: string;
    resultId: string;
    providerRequestRecorded: boolean;
    billingRecorded: boolean;
    latencyMs?: number;
    apiStabilityScore: number;
    apiStabilityVerdict: "PASS" | "UNCERTAIN" | "FAIL" | string;
};

export type EcommerceQAReview = {
    artifactId: string;
    revision: number;
    schemaVersion: number;
    reviewId: string;
    runId: string;
    slotId: string;
    attemptId: string;
    resultId: string;
    decision: "PASS" | "UNCERTAIN" | "FAIL" | string;
    action: "accept" | "hold" | "reject" | string;
    issueCodes: string[];
    note?: string;
    dimensions: EcommerceQADimensionAssessment[];
    runtimeEvidence: EcommerceQARuntimeEvidence;
    reviewerUserId: string;
    source: string;
    reviewedAt: string;
};

export type EcommerceAttemptView = { attempt: EcommerceProductionAttempt; task?: GenerationTask };
export type EcommerceSlotView = { slot: EcommerceProductionSlot; attempts: EcommerceAttemptView[]; reviews: EcommerceQAReview[] };
export type EcommerceRunView = { run: EcommerceProductionRun; quote?: EcommerceQuote; slots: EcommerceSlotView[]; routes: EcommerceProviderRoute[] };
export type EcommerceWorkspace = {
    schemaVersion: number;
    artifacts: EcommerceArtifact[];
    presets: EcommercePresetCatalog;
    providerRoutes: EcommerceProviderRoute[];
    runs: EcommerceProductionRun[];
    activeRun?: EcommerceRunView;
};

export type CreateEcommerceRunInput = {
    idempotencyKey: string;
    productAssetIds: string[];
    supportingAssetIds?: string[];
    modelAssetIds?: string[];
    sceneAssetIds?: string[];
    brandAssetIds?: string[];
    presetId: string;
    category: string;
    targetChannel: string;
    aspectRatio: string;
    resolution: "1k" | "2k" | "4k";
    outputCount: number;
    reviewBeforeGeneration: boolean;
    userGoal?: string;
    modelMode?: "ai" | "uploaded" | "none";
    modelBrief?: string;
    sceneBrief?: string;
    brandBrief?: string;
    productFacts?: Record<string, unknown>;
    advanced?: Record<string, unknown>;
    channelId?: string;
    model?: string;
};

export type EcommerceRetryResponse = { attempt: EcommerceProductionAttempt; quote?: EcommerceQuote; run?: EcommerceRunView };

export type ProjectAssetFolder = {
    id: string;
    projectId: string;
    parentId?: string;
    name: string;
    style: "glass" | "stacked" | "midnight" | "paper" | "cinema" | "compact" | string;
    theme: "aurora" | "obsidian" | "ember" | "pearl" | string;
    position: number;
    createdAt: string;
    updatedAt: string;
};

export type CharacterRepresentation = {
    id: string;
    resourceId: string;
    mediaType: string;
    role: "primary" | "front" | "side" | "back" | "turnaround_sheet" | "expression_sheet" | string;
};

export type VoiceProfile = {
    id: string;
    name: string;
    provider: string;
    voiceKey: string;
    language: string;
    timbre: string;
    sampleResourceId?: string;
    compatibleModels: string[];
    status: string;
};

export type CharacterCardSummary = {
    versionId: string;
    version: number;
    definition: Record<string, unknown>;
    representations: CharacterRepresentation[];
    voice?: { profile: VoiceProfile; instructions: string };
    visualStatus: "missing" | "partial" | "ready" | string;
    voiceStatus: "missing" | "ready" | "unavailable" | string;
};

export type ProjectCharacterDetail = {
    asset: ProjectAsset;
    character: CharacterCardSummary;
};

export type ProjectAssetCandidate = {
    id: string;
    projectId: string;
    unitId?: string;
    shotId?: string;
    name: string;
    category: string;
    status: "pending_confirmation" | "confirmed" | "ignored" | string;
    detailsJson: string;
    resolvedAssetId?: string;
    createdAt: string;
    updatedAt: string;
};

export type ProjectShot = {
    id: string;
    projectId: string;
    unitId?: string;
    title: string;
    description: string;
    position: number;
    durationMs: number;
    status: string;
    createdAt: string;
    updatedAt: string;
};

export type ShotAssetReference = {
    id: string;
    shotId: string;
    assetVersionId: string;
    role: "reference" | "start_frame" | "end_frame" | "keyframe" | "storyboard" | "output" | string;
    status: string;
    createdAt: string;
};

export type WorkflowStep = {
    id: string;
    workflowInstanceId: string;
    stepKey: string;
    name: string;
    position: number;
    status: "pending" | "ready" | "running" | "review" | "completed" | "failed" | "skipped" | string;
    error?: string;
    updatedAt: string;
};

export type ProjectWorkflow = {
    instance: { id: string; projectId: string; unitId?: string; scope: string; status: string; revision: number };
    steps: WorkflowStep[];
};

export type ProjectSummary = {
    project: Project;
    canvasCount: number;
    assetCount: number;
    unitCount: number;
    completedUnitCount: number;
};

export type ProjectDetail = {
    project: Project;
    units: ProjectUnit[];
    canvases: ProjectCanvas[];
    canvasUnitLinks: CanvasUnitLink[];
    assets: ProjectAsset[];
    assetFolders: ProjectAssetFolder[];
    workflows: ProjectWorkflow[];
    shots: ProjectShot[];
    ecommerceArtifacts?: EcommerceArtifact[];
    shotReferences: ShotAssetReference[];
    assetCandidates: ProjectAssetCandidate[];
};

export function listProjects() {
    return request<{ projects: ProjectSummary[] }>(api.get("/projects"));
}

export function getProject(id: string) {
    return request<ProjectDetail>(api.get(`/projects/${encodeURIComponent(id)}`));
}

export function createProject(input: { name: string; type: string; aspectRatio: string; sourceType: string; description?: string; stylePresetId?: string; styleProfileJson?: string }) {
    return request<{ project: Project }>(api.post("/projects", input));
}

export function updateProject(projectId: string, input: Partial<Pick<Project, "name" | "type" | "aspectRatio" | "sourceType" | "description" | "stylePresetId" | "styleProfileJson" | "status">>) {
    return request<{ project: Project }>(api.patch(`/projects/${encodeURIComponent(projectId)}`, input));
}

export function deleteProject(projectId: string) {
    return request<{ id: string }>(api.delete(`/projects/${encodeURIComponent(projectId)}`));
}

export function createProjectUnit(projectId: string, input: { kind: string; title: string; sourceText?: string; position?: number }) {
    return request<{ unit: ProjectUnit }>(api.post(`/projects/${encodeURIComponent(projectId)}/units`, input));
}

export function getProjectUnit(projectId: string, unitId: string) {
    return request<{ unit: ProjectUnit }>(api.get(`/projects/${encodeURIComponent(projectId)}/units/${encodeURIComponent(unitId)}`));
}

export function importProjectUnits(projectId: string, units: Array<{ kind: string; title: string; sourceText?: string }>) {
    return request<{ units: ProjectUnit[] }>(api.post(`/projects/${encodeURIComponent(projectId)}/units/import`, { units }));
}

export function reorderProjectUnits(projectId: string, unitIds: string[]) {
    return request<{ unitIds: string[] }>(api.patch(`/projects/${encodeURIComponent(projectId)}/units/reorder`, { unitIds }));
}

export function updateProjectUnit(projectId: string, unitId: string, input: { title?: string; sourceText: string; status?: ProjectUnit["status"] }) {
    return request<{ unit: ProjectUnit }>(api.patch(`/projects/${encodeURIComponent(projectId)}/units/${encodeURIComponent(unitId)}`, input));
}

export function deleteProjectUnit(projectId: string, unitId: string) {
    return request<{ id: string }>(api.delete(`/projects/${encodeURIComponent(projectId)}/units/${encodeURIComponent(unitId)}`));
}

export function linkCanvasUnit(projectId: string, input: { canvasId: string; unitId: string; role?: string }) {
    return request<{ link: { id: string; projectId: string; canvasId: string; unitId: string; role: string } }>(api.post(`/projects/${encodeURIComponent(projectId)}/canvas-links`, input));
}

export function unlinkCanvasUnit(projectId: string, canvasId: string, unitId: string) {
    return request<{ canvasId: string; unitId: string }>(api.delete(`/projects/${encodeURIComponent(projectId)}/canvas-links/${encodeURIComponent(canvasId)}/units/${encodeURIComponent(unitId)}`));
}

export function unlinkCanvasProject(projectId: string, canvasId: string) {
    return request<{ canvasId: string }>(api.delete(`/projects/${encodeURIComponent(projectId)}/canvases/${encodeURIComponent(canvasId)}`));
}

export function linkProjectAsset(projectId: string, input: { assetId: string; category: string; folderId?: string; role?: string }, signal?: AbortSignal) {
    return request<{ asset: ProjectAsset }>(api.post(`/projects/${encodeURIComponent(projectId)}/assets`, input, { signal }));
}

export function unlinkProjectAsset(projectId: string, assetId: string) {
    return request<{ id: string }>(api.delete(`/projects/${encodeURIComponent(projectId)}/assets/${encodeURIComponent(assetId)}`));
}

export function updateProjectAssetCategory(projectId: string, assetId: string, category: string, signal?: AbortSignal) {
    return request<{ asset: ProjectAsset }>(api.patch(`/projects/${encodeURIComponent(projectId)}/assets/${encodeURIComponent(assetId)}`, { category }, { signal }));
}

export function updateProjectAssetRole(projectId: string, assetId: string, role: string) {
    return request<{ asset: ProjectAsset }>(api.patch(`/projects/${encodeURIComponent(projectId)}/assets/${encodeURIComponent(assetId)}`, { role }));
}

export function moveProjectAsset(projectId: string, assetId: string, folderId: string, signal?: AbortSignal) {
    return request<{ asset: ProjectAsset }>(api.patch(`/projects/${encodeURIComponent(projectId)}/assets/${encodeURIComponent(assetId)}`, { folderId }, { signal }));
}

export function listProjectAssetFolders(projectId: string, signal?: AbortSignal) {
    return request<{ folders: ProjectAssetFolder[] }>(api.get(`/projects/${encodeURIComponent(projectId)}/asset-folders`, { signal }));
}

export function createProjectAssetFolder(projectId: string, input: { name: string; parentId?: string; style?: ProjectAssetFolder["style"]; theme?: ProjectAssetFolder["theme"] }) {
    return request<{ folder: ProjectAssetFolder }>(api.post(`/projects/${encodeURIComponent(projectId)}/asset-folders`, input));
}

export function updateProjectAssetFolder(projectId: string, folderId: string, input: { name?: string; parentId?: string; style?: ProjectAssetFolder["style"]; theme?: ProjectAssetFolder["theme"] }) {
    return request<{ folder: ProjectAssetFolder }>(api.patch(`/projects/${encodeURIComponent(projectId)}/asset-folders/${encodeURIComponent(folderId)}`, input));
}

export function deleteProjectAssetFolder(projectId: string, folderId: string) {
    return request<{ id: string }>(api.delete(`/projects/${encodeURIComponent(projectId)}/asset-folders/${encodeURIComponent(folderId)}`));
}

export function createProjectAssetVersion(projectId: string, assetId: string, input: { prompt?: string; definitionJson?: string; note?: string }) {
    return request<{ version: { id: string; assetId: string; version: number; status: string } }>(api.post(`/projects/${encodeURIComponent(projectId)}/assets/${encodeURIComponent(assetId)}/versions`, input));
}

export function listVoiceProfiles() {
    return request<{ profiles: VoiceProfile[] }>(api.get("/voice-profiles"));
}

export function createProjectCharacter(projectId: string, input: { name: string; definition?: Record<string, unknown> }) {
    return request<ProjectCharacterDetail>(api.post(`/projects/${encodeURIComponent(projectId)}/characters`, input));
}

export function getProjectCharacter(projectId: string, assetId: string) {
    return request<ProjectCharacterDetail>(api.get(`/projects/${encodeURIComponent(projectId)}/characters/${encodeURIComponent(assetId)}`));
}

export function updateProjectCharacter(projectId: string, assetId: string, input: { name: string; definition: Record<string, unknown> }) {
    return request<ProjectCharacterDetail>(api.patch(`/projects/${encodeURIComponent(projectId)}/characters/${encodeURIComponent(assetId)}`, input));
}

export function replaceProjectCharacterRepresentations(projectId: string, assetId: string, representations: Array<{ role: string; resourceId: string; metadata?: Record<string, unknown> }>) {
    return request<ProjectCharacterDetail>(api.put(`/projects/${encodeURIComponent(projectId)}/characters/${encodeURIComponent(assetId)}/representations`, { representations }));
}

export function bindProjectCharacterVoice(projectId: string, assetId: string, input: { voiceProfileId: string; instructions?: string }) {
    return request<ProjectCharacterDetail>(api.put(`/projects/${encodeURIComponent(projectId)}/characters/${encodeURIComponent(assetId)}/voice`, input));
}

export function unbindProjectCharacterVoice(projectId: string, assetId: string) {
    return request<ProjectCharacterDetail>(api.delete(`/projects/${encodeURIComponent(projectId)}/characters/${encodeURIComponent(assetId)}/voice`));
}

export function createUnitWorkflow(projectId: string, unitId: string) {
    return request<{ workflow: ProjectWorkflow }>(api.post(`/projects/${encodeURIComponent(projectId)}/workflows`, { unitId }));
}

export function saveProjectShot(projectId: string, input: { id?: string; unitId?: string; title: string; description?: string; position?: number; durationMs?: number; status?: string }) {
    return request<{ shot: ProjectShot }>(api.post(`/projects/${encodeURIComponent(projectId)}/shots`, input));
}

export function replaceProjectUnitShots(projectId: string, unitId: string, shots: Array<{ title: string; description: string; durationMs: number }>) {
    return request<{ shots: ProjectShot[] }>(api.put(`/projects/${encodeURIComponent(projectId)}/units/${encodeURIComponent(unitId)}/shots`, { shots }));
}

export function linkShotAsset(projectId: string, shotId: string, input: { assetVersionId: string; role: ShotAssetReference["role"] }) {
    return request<{ reference: ShotAssetReference }>(api.post(`/projects/${encodeURIComponent(projectId)}/shots/${encodeURIComponent(shotId)}/assets`, input));
}

export function createProjectAssetCandidates(projectId: string, candidates: Array<{ unitId?: string; shotId?: string; name: string; category: string; details?: Record<string, unknown> }>) {
    return request<{ candidates: ProjectAssetCandidate[] }>(api.post(`/projects/${encodeURIComponent(projectId)}/asset-candidates`, { candidates }));
}

export function confirmProjectAssetCandidate(projectId: string, candidateId: string, assetId?: string) {
    return request<{ asset: ProjectAsset }>(api.post(`/projects/${encodeURIComponent(projectId)}/asset-candidates/${encodeURIComponent(candidateId)}/confirm`, { assetId: assetId || "" }));
}

export function updateWorkflowStep(projectId: string, stepId: string, input: { status: string; outputJson?: string; error?: string }) {
    return request<{ step: WorkflowStep }>(api.patch(`/projects/${encodeURIComponent(projectId)}/workflow-steps/${encodeURIComponent(stepId)}`, input));
}

export function registerProjectTaskOutput(projectId: string, stepId: string, input: { taskId: string; assetVersionId?: string; resourceId?: string; mediaType?: string; role?: string; metadataJson?: string; outputJson?: string }) {
    return request<{ step: WorkflowStep }>(api.post(`/projects/${encodeURIComponent(projectId)}/workflow-steps/${encodeURIComponent(stepId)}/task-output`, input));
}

export function listProjectEcommerceArtifacts(projectId: string) {
    return request<{ artifacts: EcommerceArtifact[] }>(api.get(`/projects/${encodeURIComponent(projectId)}/ecommerce-artifacts`));
}

export function saveProjectEcommerceArtifact(
    projectId: string,
    input: {
        artifactKey: string;
        artifactType: string;
        schemaVersion: number;
        lifecycle?: EcommerceArtifact["lifecycle"];
        evidence?: EcommerceArtifact["evidence"];
        responsibleAgentId?: string;
        skillRef?: string;
        payload: Record<string, unknown>;
        sourceRefs: string[];
        authorityRefs?: string[];
    },
) {
    return request<{ artifact: EcommerceArtifact }>(api.post(`/projects/${encodeURIComponent(projectId)}/ecommerce-artifacts`, input));
}

export function getProjectEcommerceWorkspace(projectId: string) {
    return request<{ workspace: EcommerceWorkspace }>(api.get(`/projects/${encodeURIComponent(projectId)}/ecommerce/workspace`));
}

export function listProjectEcommercePresets(projectId: string) {
    return request<{ presets: EcommercePresetCatalog }>(api.get(`/projects/${encodeURIComponent(projectId)}/ecommerce/presets`));
}

export function saveProjectEcommercePreset(projectId: string, input: { sourceId: string; presetKey?: string; name?: string; description?: string; definition?: EcommercePresetDefinition }) {
    return request<{ preset: EcommercePreset }>(api.post(`/projects/${encodeURIComponent(projectId)}/ecommerce/presets`, input));
}

export function createProjectEcommerceRun(projectId: string, input: CreateEcommerceRunInput) {
    return request<{ run: EcommerceRunView }>(api.post(`/projects/${encodeURIComponent(projectId)}/ecommerce/runs`, input));
}

export function getProjectEcommerceRun(projectId: string, runId: string) {
    return request<{ run: EcommerceRunView }>(api.get(`/projects/${encodeURIComponent(projectId)}/ecommerce/runs/${encodeURIComponent(runId)}`));
}

export function refreshProjectEcommerceRunQuote(projectId: string, runId: string, input: { channelId: string; model: string }) {
    return request<{ run: EcommerceRunView }>(api.post(`/projects/${encodeURIComponent(projectId)}/ecommerce/runs/${encodeURIComponent(runId)}/quote`, input));
}

export function approveProjectEcommerceRun(projectId: string, runId: string) {
    return request<{ run: EcommerceRunView }>(api.post(`/projects/${encodeURIComponent(projectId)}/ecommerce/runs/${encodeURIComponent(runId)}/approve`));
}

export function submitProjectEcommerceRun(projectId: string, runId: string, quoteFingerprint: string) {
    return request<{ run: EcommerceRunView }>(api.post(`/projects/${encodeURIComponent(projectId)}/ecommerce/runs/${encodeURIComponent(runId)}/submit`, { quoteFingerprint }));
}

export function cancelProjectEcommerceRun(projectId: string, runId: string) {
    return request<{ run: EcommerceRunView }>(api.post(`/projects/${encodeURIComponent(projectId)}/ecommerce/runs/${encodeURIComponent(runId)}/cancel`));
}

export function retryProjectEcommerceSlot(
    projectId: string,
    runId: string,
    slotId: string,
    input: { attemptId?: string; quoteFingerprint?: string; kind?: "retry" | "repair" | "variation"; promptPatch?: string; channelId?: string; model?: string },
) {
    return request<{ retry: EcommerceRetryResponse }>(api.post(`/projects/${encodeURIComponent(projectId)}/ecommerce/runs/${encodeURIComponent(runId)}/slots/${encodeURIComponent(slotId)}/retry`, input));
}

export function reviewProjectEcommerceSlot(
    projectId: string,
    runId: string,
    slotId: string,
    input: {
        reviewId: string;
        attemptId: string;
        resultId: string;
        decision: "PASS" | "UNCERTAIN" | "FAIL";
        action: "accept" | "hold" | "reject";
        issueCodes?: string[];
        note?: string;
        dimensions: EcommerceQADimensionAssessment[];
    },
) {
    return request<{ run: EcommerceRunView }>(api.post(`/projects/${encodeURIComponent(projectId)}/ecommerce/runs/${encodeURIComponent(runId)}/slots/${encodeURIComponent(slotId)}/qa`, input));
}

export function createProjectEcommerceVideoSequence(
    projectId: string,
    runId: string,
    input: { slotIds: string[]; title?: string; aspectRatio?: string; durationSeconds?: number; musicResourceId?: string },
) {
    return request<{ run: EcommerceRunView }>(api.post(`/projects/${encodeURIComponent(projectId)}/ecommerce/runs/${encodeURIComponent(runId)}/video-sequences`, input));
}
