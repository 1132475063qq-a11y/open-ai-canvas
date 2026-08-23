import { createEcommerceCanvasNode } from "@/lib/canvas/canvas-project-domain";
import { FRAME_COLLAPSED_HEIGHT, FRAME_COLLAPSED_WIDTH } from "@/lib/canvas/canvas-frame";
import type { EcommerceEvidence, EcommerceNodeKind, EcommerceProductionStatus, EcommerceState } from "@/ecommerce/domain/types";
import type {
    EcommerceArtifact,
    EcommerceRunView,
    EcommerceSlotView,
    EcommerceWorkspace,
    ProjectAsset,
    ProjectDetail,
} from "@/services/api/projects";
import { resolveResourceUrl } from "@/services/api/resources";
import { CanvasNodeType, type CanvasConnection, type CanvasNodeData, type CanvasNodeMetadata, type ViewportTransform } from "@/types/canvas";

type EcommerceCanvasProjection = Pick<ProjectDetail, "project" | "assets" | "ecommerceArtifacts">;

const FRAME_POSITION = { x: 80, y: 80 };
const FRAME_EXPANDED_WIDTH = 1120;
const ECOMMERCE_PROJECTION_LAYOUT_VERSION = 3;
const LEGACY_FRAME_EXPANDED_WIDTH = 1840;
const LEGACY_RESULT_GAP_X = 280;
const LEGACY_RESULT_GAP_Y = 220;
const LEGACY_RESULT_WIDTH = 240;
const LEGACY_RESULT_HEIGHT = 180;
const RESULT_COLUMNS = 6;
const RESULT_TOP = 300;
const RESULT_GAP_X = 190;
const RESULT_GAP_Y = 180;
const ASSET_GAP_X = 190;

type EcommerceAssetProjection = {
    kind: Extract<EcommerceNodeKind, "product_input" | "model_profile" | "scene_pack">;
    title: string;
    assets: ProjectAsset[];
};

export function createEcommerceCanvasProjection(
    detail: EcommerceCanvasProjection,
    workspaceState?: EcommerceWorkspace,
): { nodes: CanvasNodeData[]; connections: CanvasConnection[] } {
    const artifacts = workspaceState?.artifacts || detail.ecommerceArtifacts || [];
    const runView = workspaceState?.activeRun;
    const runId = runView?.run.id;
    const latestArtifacts = latestArtifactsByType(artifacts, runId);
    const frameId = ecommerceNodeId(detail.project.id, "production_frame", runId);
    const projectedSlots = slotsForProjection(runView);
    const resultRows = Math.max(1, Math.ceil(projectedSlots.length / RESULT_COLUMNS));
    const videoTop = RESULT_TOP + resultRows * RESULT_GAP_Y + 36;
    const expandedHeight = videoTop + 220;

    const frame = createProductionFrame(detail, frameId, runView, latestArtifacts, projectedSlots, expandedHeight);
    const assetGroups = ecommerceAssetGroups(detail.assets);
    const assetNodes = assetGroups.flatMap((group) => group.assets.map((asset, index) => createAssetReferenceNode(detail.project.id, asset, group, index)));
    const resultNodes = projectedSlots.map((slotView, index) => createResultNode(detail.project.id, frameId, slotView, runView, index));
    const videoNode = createVideoSequenceNode(detail.project.id, frameId, runView, latestArtifacts.video_sequence, resultNodes, videoTop);
    const nodes = [frame, ...assetNodes, ...resultNodes, videoNode];
    const connections: CanvasConnection[] = [];
    const generationNode = frame;
    for (const resultNode of resultNodes) {
        appendConnection(connections, detail.project.id, generationNode?.id, resultNode.id);
        if (resultNode.metadata?.content) appendConnection(connections, detail.project.id, resultNode.id, videoNode.id);
    }
    for (const assetNode of assetNodes) appendConnection(connections, detail.project.id, assetNode.id, frame.id);
    return { nodes, connections };
}

export function reconcileEcommerceCanvasProjection(
    currentNodes: CanvasNodeData[],
    currentConnections: CanvasConnection[],
    projection: { nodes: CanvasNodeData[]; connections: CanvasConnection[] },
    projectId: string,
): { nodes: CanvasNodeData[]; connections: CanvasConnection[] } {
    const currentProjectionNodes = currentNodes.filter((node) => isProjectEcommerceProjectionNode(node, projectId));
    const currentProjectionIds = new Set(currentProjectionNodes.map((node) => node.id));
    const nextProjectionIds = new Set(projection.nodes.map((node) => node.id));
    const currentById = new Map(currentProjectionNodes.map((node) => [node.id, node]));
    // Run ID changes are data revisions, not a reason to reset the user's canvas layout.
    const currentBySemanticKey = new Map(
        currentProjectionNodes
            .map((node) => [projectionSemanticKey(node), node] as const)
            .filter(([key]) => Boolean(key)),
    );
    const currentFrame = currentProjectionNodes.find((node) => node.ecommerceKind === "production_frame");
    const nextFrame = projection.nodes.find((node) => node.ecommerceKind === "production_frame");
    const projectionOffset = currentFrame && nextFrame
        ? { x: currentFrame.position.x - nextFrame.position.x, y: currentFrame.position.y - nextFrame.position.y }
        : { x: 0, y: 0 };
    const legacyResultGridOffset = detectLegacyResultGridOffset(currentById, currentBySemanticKey, projection.nodes);
    const projectedNodes = projection.nodes.map((node) => preserveProjectionLayout(node, currentById.get(node.id) || currentBySemanticKey.get(projectionSemanticKey(node)), projectionOffset, legacyResultGridOffset));
    const userNodes = currentNodes.filter((node) => !currentProjectionIds.has(node.id));
    const userConnections = currentConnections.filter((connection) => (
        !currentProjectionIds.has(connection.fromNodeId)
        && !currentProjectionIds.has(connection.toNodeId)
        && !nextProjectionIds.has(connection.fromNodeId)
        && !nextProjectionIds.has(connection.toNodeId)
    ));
    return {
        nodes: [...projectedNodes, ...userNodes],
        connections: [...projection.connections, ...userConnections],
    };
}

export function shouldFitEcommerceProductionFrame(
    frame: CanvasNodeData,
    viewport: ViewportTransform,
    viewportSize: { width: number; height: number },
) {
    const screenWidth = frame.width * viewport.k;
    const screenHeight = frame.height * viewport.k;
    const centerX = (frame.position.x + frame.width / 2) * viewport.k + viewport.x;
    const centerY = (frame.position.y + frame.height / 2) * viewport.k + viewport.y;
    const edgePadding = 24;
    const fitPadding = 88;
    const targetScale = Math.min(
        1,
        Math.max(
            0.05,
            Math.min(
                Math.max(1, viewportSize.width - fitPadding * 2) / Math.max(1, frame.width),
                Math.max(1, viewportSize.height - fitPadding * 2) / Math.max(1, frame.height),
            ),
        ),
    );
    const centerVisible = centerX >= edgePadding
        && centerX <= viewportSize.width - edgePadding
        && centerY >= edgePadding
        && centerY <= viewportSize.height - edgePadding;
    const fitsViewport = screenWidth <= Math.max(1, viewportSize.width - edgePadding * 2)
        && screenHeight <= Math.max(1, viewportSize.height - edgePadding * 2);
    const materiallyTooSmall = viewport.k < targetScale * 0.65;
    return !centerVisible || !fitsViewport || materiallyTooSmall;
}

function isProjectEcommerceProjectionNode(node: CanvasNodeData, projectId: string) {
    return node.ecommerceRef?.projectId === projectId || (Boolean(node.ecommerceKind) && node.id.startsWith(`${projectId}:ecommerce:`));
}

function preserveProjectionLayout(
    next: CanvasNodeData,
    current: CanvasNodeData | undefined,
    projectionOffset: { x: number; y: number },
    legacyResultGridOffset: { x: number; y: number } | null,
): CanvasNodeData {
    if (!current) return next;
    const currentFrame = current.metadata?.frame;
    const nextFrame = next.metadata?.frame;
    if (next.ecommerceKind === "generated_asset" && legacyResultGridOffset) {
        return { ...next, position: translatedProjectionPosition(next, legacyResultGridOffset) };
    }
    if (next.ecommerceKind === "video_sequence" && legacyResultGridOffset && usesLegacyVideoLayout(current, next, legacyResultGridOffset)) {
        return { ...next, position: translatedProjectionPosition(next, legacyResultGridOffset) };
    }
    if (currentFrame && nextFrame && (legacyResultGridOffset || usesLegacyFrameLayout(current, currentFrame, nextFrame))) {
        return {
            ...next,
            position: current.position,
            width: currentFrame.collapsed ? FRAME_COLLAPSED_WIDTH : nextFrame.expandedWidth,
            height: currentFrame.collapsed ? FRAME_COLLAPSED_HEIGHT : nextFrame.expandedHeight,
            metadata: { ...next.metadata, frame: { ...nextFrame, collapsed: currentFrame.collapsed } },
        };
    }
    return {
        ...next,
        position: current.position,
        width: current.width,
        height: current.height,
        metadata: currentFrame && nextFrame
            ? { ...next.metadata, frame: { ...currentFrame } }
            : next.metadata,
    };
}

function translatedProjectionPosition(next: CanvasNodeData, projectionOffset: { x: number; y: number }) {
    return { x: next.position.x + projectionOffset.x, y: next.position.y + projectionOffset.y };
}

function detectLegacyResultGridOffset(currentById: Map<string, CanvasNodeData>, currentBySemanticKey: Map<string, CanvasNodeData>, nextNodes: CanvasNodeData[]) {
    const pairs = nextNodes
        .filter((node) => node.ecommerceKind === "generated_asset")
        .map((next) => ({ next, current: currentById.get(next.id) || currentBySemanticKey.get(projectionSemanticKey(next)) }))
        .filter((pair): pair is { next: CanvasNodeData; current: CanvasNodeData } => Boolean(pair.current));
    if (!pairs.length || pairs.length !== nextNodes.filter((node) => node.ecommerceKind === "generated_asset").length) return null;
    const firstLegacyPosition = legacyResultPosition(pairs[0].next);
    const offset = {
        x: pairs[0].current.position.x - firstLegacyPosition.x,
        y: pairs[0].current.position.y - firstLegacyPosition.y,
    };
    const followsLegacyGrid = pairs.every(({ current, next }) => {
        const legacyPosition = legacyResultPosition(next);
        const legacySize = (current.width === LEGACY_RESULT_WIDTH && current.height === LEGACY_RESULT_HEIGHT)
            || (current.width === 300 && current.height === 225);
        return legacySize
            && nearlyEqual(current.position.x, legacyPosition.x + offset.x)
            && nearlyEqual(current.position.y, legacyPosition.y + offset.y);
    });
    return followsLegacyGrid ? offset : null;
}

function projectionSemanticKey(node: CanvasNodeData) {
    if (!node.ecommerceKind) return "";
    const slotId = node.ecommerceRef?.slotId;
    return slotId ? `${node.ecommerceKind}:slot:${slotId}` : `kind:${node.ecommerceKind}`;
}

function legacyResultPosition(next: CanvasNodeData) {
    const column = Math.round((next.position.x - 150) / RESULT_GAP_X);
    const row = Math.round((next.position.y - RESULT_TOP) / RESULT_GAP_Y);
    return { x: 150 + column * LEGACY_RESULT_GAP_X, y: RESULT_TOP + row * LEGACY_RESULT_GAP_Y };
}

function usesLegacyVideoLayout(current: CanvasNodeData, next: CanvasNodeData, projectionOffset: { x: number; y: number }) {
    const resultRows = Math.max(1, Math.round((next.position.y - RESULT_TOP - 30) / RESULT_GAP_Y));
    return nearlyEqual(current.position.x, next.position.x + projectionOffset.x)
        && nearlyEqual(current.position.y, RESULT_TOP + resultRows * LEGACY_RESULT_GAP_Y + 30 + projectionOffset.y)
        && current.width === next.width
        && current.height === next.height;
}

function usesLegacyFrameLayout(node: CanvasNodeData, current: NonNullable<CanvasNodeMetadata["frame"]>, next: NonNullable<CanvasNodeMetadata["frame"]>) {
    const resultRows = Math.max(1, Math.round((next.expandedHeight - RESULT_TOP - 30 - 260) / RESULT_GAP_Y));
    const legacyExpandedHeight = RESULT_TOP + resultRows * LEGACY_RESULT_GAP_Y + 30 + 260;
    const expandedWidth = current.collapsed ? current.expandedWidth : node.width;
    const expandedHeight = current.collapsed ? current.expandedHeight : node.height;
    return expandedWidth === LEGACY_FRAME_EXPANDED_WIDTH && expandedHeight === legacyExpandedHeight;
}

function nearlyEqual(left: number, right: number) {
    return Math.abs(left - right) < 0.01;
}

function createProductionFrame(
    detail: EcommerceCanvasProjection,
    frameId: string,
    runView: EcommerceRunView | undefined,
    artifacts: Record<string, EcommerceArtifact>,
    slots: EcommerceSlotView[],
    expandedHeight: number,
) {
    const resultCount = slots.filter(({ slot }) => Boolean(resultURL(slot))).length;
    const acceptedCount = slots.filter(({ slot }) => slot.accepted).length;
    const totalCount = runView?.run.outputCount || slots.length;
    const presetLabel = artifactString(artifacts.preset_snapshot, "name") || (runView ? `${runView.run.kernel} · v${runView.run.presetVersion}` : "尚未选择商拍预设");
    const inputLabel = inputSummary(detail.assets);
    const statusLabel = ecommerceRunStatusLabel(runView?.run.status);
    const progress = ecommerceRunProgress(runView);
    const metadata: CanvasNodeMetadata = {
        workflowKind: "ecommerce_production",
        workflowTitle: "电商系列生产",
        workflowDescription: "电商项目专属生产 Frame",
        skillDomain: "ecommerce",
        ecommerceProjectionLayoutVersion: ECOMMERCE_PROJECTION_LAYOUT_VERSION,
        content: `${inputLabel}\n${presetLabel}\n${statusLabel}\n结果 ${resultCount}/${totalCount}`,
        frame: { collapsed: true, expandedWidth: FRAME_EXPANDED_WIDTH, expandedHeight },
        ecommerceFrame: { inputLabel, presetLabel, statusLabel, progress, resultCount, acceptedCount, totalCount },
    };
    const node = createEcommerceCanvasNode(
        CanvasNodeType.Frame,
        "production_frame",
        FRAME_POSITION,
        {
            projectId: detail.project.id,
            runId: runView?.run.id,
            presetId: runView?.run.presetId,
            presetVersion: runView?.run.presetVersion,
        },
        metadata,
        runNodeState(runView),
    );
    node.id = frameId;
    node.title = `${detail.project.name || "电商项目"} · 系列生产`;
    node.position = FRAME_POSITION;
    node.width = FRAME_COLLAPSED_WIDTH;
    node.height = FRAME_COLLAPSED_HEIGHT;
    return node;
}

function ecommerceAssetGroups(assets: ProjectAsset[]): EcommerceAssetProjection[] {
    const productAssets = assets.filter((asset) => {
        const role = asset.projectRole || "";
        return role.startsWith("product_") || role === "packaging" || role === "logo";
    });
    const modelAssets = assets.filter((asset) => asset.projectRole === "model_reference");
    const sceneAssets = assets.filter((asset) => asset.projectRole === "scene_reference" || asset.projectRole === "brand_reference");
    return [
        { kind: "product_input", title: "商品素材", assets: productAssets },
        { kind: "model_profile", title: "模特素材", assets: modelAssets },
        { kind: "scene_pack", title: "场景素材", assets: sceneAssets },
    ].filter((group) => group.assets.length) as EcommerceAssetProjection[];
}

function createAssetReferenceNode(projectId: string, asset: ProjectAsset, group: EcommerceAssetProjection, index: number) {
    const content = resolveResourceUrl(asset.storageKey);
    const visual = Boolean(content) && asset.mediaType.startsWith("image");
    const node = createEcommerceCanvasNode(
        visual ? CanvasNodeType.Image : CanvasNodeType.Text,
        group.kind,
        { x: -180, y: 100 + index * 190 },
        { projectId, assetId: asset.id, assetVersionId: asset.primaryVersionId },
        {
            workflowKind: "ecommerce_production",
            workflowTitle: group.title,
            workflowDescription: `${assetRoleLabel(asset.projectRole || "unassigned")} · AI 商拍输入素材`,
            skillDomain: "ecommerce",
            ecommerceProjectionLayoutVersion: ECOMMERCE_PROJECTION_LAYOUT_VERSION,
            content: content || asset.previewText || asset.title,
            storageKey: asset.storageKey,
            assetId: asset.id,
            status: "success",
        },
        { lifecycle: "draft", production: "ready", evidence: "recorded", attention: "none" },
    );
    node.id = `${projectId}:ecommerce:asset:${asset.id}`;
    node.title = asset.title || group.title;
    node.width = visual ? 168 : 220;
    node.height = visual ? 168 : 120;
    node.position = { x: -180, y: 100 + index * 190 };
    return node;
}

function createResultNode(projectId: string, frameId: string, slotView: EcommerceSlotView, runView: EcommerceRunView | undefined, index: number) {
    const { slot } = slotView;
    const activeAttempt = slotView.attempts.find(({ attempt }) => attempt.id === slot.activeAttemptId)?.attempt || slotView.attempts.at(-1)?.attempt;
    const url = resultURL(slot) || (activeAttempt ? resultURL(activeAttempt) : "");
    const column = index % RESULT_COLUMNS;
    const row = Math.floor(index / RESULT_COLUMNS);
    const metadata: CanvasNodeMetadata = {
        workflowKind: "generated_asset",
        workflowTitle: slot.title,
        workflowDescription: `${slot.role} · ${slotStatusLabel(slot.status)} · QA ${slot.qaStatus}`,
        skillDomain: "ecommerce",
        ecommerceProjectionLayoutVersion: ECOMMERCE_PROJECTION_LAYOUT_VERSION,
        content: url || undefined,
        prompt: activeAttempt?.prompt || slot.prompt,
        taskId: activeAttempt?.taskId || slot.activeTaskId,
        taskStatus: activeAttempt?.status || slot.status,
        status: slot.status === "failed" ? "error" : ["scheduled", "queued", "running"].includes(slot.status) ? "loading" : url ? "success" : "idle",
    };
    const node = createEcommerceCanvasNode(
        CanvasNodeType.Image,
        "generated_asset",
        { x: 150 + column * RESULT_GAP_X, y: RESULT_TOP + row * RESULT_GAP_Y },
        {
            projectId,
            runId: runView?.run.id,
            slotId: slot.id,
            attemptId: activeAttempt?.id,
            taskId: activeAttempt?.taskId || slot.activeTaskId,
            resultId: activeAttempt?.resultId,
            artifactId: activeAttempt?.generatedAssetArtifactId || slot.generatedAssetId,
            qaReportId: slot.qaStatus !== "PENDING" ? `slot:${slot.id}:qa` : undefined,
        },
        metadata,
        slotNodeState(slot.status, slot.qaStatus, slot.accepted, Boolean(url)),
    );
    node.id = `${projectId}:ecommerce:${runView?.run.id || "draft"}:slot:${slot.id}`;
    node.title = `${slot.position}. ${slot.title}`;
    node.parentId = frameId;
    node.width = 300;
    node.height = 225;
    node.position = { x: 150 + column * RESULT_GAP_X, y: RESULT_TOP + row * RESULT_GAP_Y };
    return node;
}

function createVideoSequenceNode(
    projectId: string,
    frameId: string,
    runView: EcommerceRunView | undefined,
    artifact: EcommerceArtifact | undefined,
    resultNodes: CanvasNodeData[],
    y: number,
) {
    const sequence = parsePayload(artifact);
    const segmentCount = Array.isArray(sequence.segments) ? sequence.segments.length : 0;
    const node = createEcommerceCanvasNode(
        CanvasNodeType.Video,
        "video_sequence",
        { x: 150, y },
        {
            projectId,
            runId: runView?.run.id,
            artifactId: artifact?.id,
            artifactRevision: artifact?.revision,
            sourceResourceIds: resultNodes.flatMap((item) => item.ecommerceRef?.artifactId ? [item.ecommerceRef.artifactId] : []),
        },
        {
            workflowKind: "video_sequence",
            workflowTitle: "9:16 电商短片编排",
            workflowDescription: artifact ? `${segmentCount} 个已接受镜头 · FFmpeg 时间线` : "接受图片后生成动作、顺序和时间线",
            skillDomain: "ecommerce",
            ecommerceProjectionLayoutVersion: ECOMMERCE_PROJECTION_LAYOUT_VERSION,
            generationMode: "video",
            seconds: String(Math.max(1, Math.round(numberValue(sequence.durationMs) / 1000) || 15)),
            prompt: artifact ? videoSequencePrompt(sequence) : "",
            references: resultNodes.filter((item) => item.metadata?.content).map((item) => item.id),
        },
        artifact
            ? { lifecycle: "finalized", production: "ready", evidence: evidence(artifact.evidence), attention: "none" }
            : { lifecycle: "draft", production: "not_started", evidence: "unknown", attention: "human_required" },
    );
    node.id = ecommerceNodeId(projectId, "video_sequence", runView?.run.id);
    node.title = "Video Sequence · 图片转视频";
    node.parentId = frameId;
    node.width = 520;
    node.height = 190;
    node.position = { x: 150, y };
    return node;
}

function slotsForProjection(runView?: EcommerceRunView): EcommerceSlotView[] {
    if (runView?.slots.length) return runView.slots;
    const titles = ["主视觉", "环境全景", "中景关系", "动作 / 使用", "商品特写", "补充镜头"];
    return titles.map((title, index) => ({
        slot: {
            id: `placeholder-${index + 1}`,
            userId: "",
            projectId: "",
            runId: "",
            position: index + 1,
            role: ["hero", "environment", "mid", "action", "detail", "supporting"][index],
            title,
            cameraJson: "",
            prompt: "等待 Agent 根据商品、场景、渠道和商拍预设编译逐镜 Prompt。",
            negativePrompt: "",
            status: "planned",
            qaStatus: "PENDING",
            qaIssuesJson: "[]",
            qaNote: "",
            accepted: false,
            createdAt: "",
            updatedAt: "",
        },
        attempts: [],
        reviews: [],
    }));
}

function latestArtifactsByType(artifacts: EcommerceArtifact[], runId?: string) {
    const scoped = runId ? artifacts.filter((artifact) => artifact.artifactKey.startsWith(`run:${runId}:`)) : artifacts;
    return scoped.reduce<Record<string, EcommerceArtifact>>((result, artifact) => {
        const current = result[artifact.artifactType];
        if (!current || artifact.revision > current.revision || artifact.updatedAt > current.updatedAt) result[artifact.artifactType] = artifact;
        return result;
    }, {});
}

function nodeState(kind: EcommerceNodeKind, artifact: EcommerceArtifact | undefined, runView: EcommerceRunView | undefined, hasAssets: boolean): Partial<EcommerceState> {
    if (kind === "product_input" && hasAssets) return { lifecycle: "draft", production: "ready", evidence: "recorded", attention: "none" };
    if (kind === "generation_request") return runNodeState(runView);
    if (kind === "qa_report") {
        const unresolved = runView?.slots.some(({ slot }) => !slot.accepted);
        return { lifecycle: artifact ? lifecycle(artifact.lifecycle) : "draft", production: artifact ? "qa" : "not_started", evidence: artifact ? evidence(artifact.evidence) : "unknown", attention: unresolved ? "human_required" : "none" };
    }
    if (artifact) return { lifecycle: lifecycle(artifact.lifecycle), production: "ready", evidence: evidence(artifact.evidence), attention: "none" };
    if (kind === "model_profile" && runView?.run.kernel === "STILL_LIFE") return { lifecycle: "draft", production: "ready", evidence: "recorded", attention: "none" };
    return { lifecycle: "draft", production: "not_started", evidence: "unknown", attention: hasAssets ? "warning" : "human_required" };
}

function runNodeState(runView?: EcommerceRunView): Partial<EcommerceState> {
    if (!runView) return { lifecycle: "draft", production: "not_started", evidence: "unknown", attention: "human_required" };
    const status = runView.run.status;
    const production: EcommerceProductionStatus = status === "ready"
        ? "approved"
        : status === "qa"
            ? "qa"
            : status === "generating"
                ? "running"
                : status === "failed"
                    ? "failed"
                    : status === "cancelled"
                        ? "cancelled"
                        : "ready";
    return {
        lifecycle: status === "ready" ? "finalized" : "draft",
        production,
        evidence: "recorded",
        attention: status === "failed" ? "error" : status === "needs_you" || status === "awaiting_review" || status === "awaiting_cost" ? "human_required" : "none",
    };
}

function slotNodeState(status: string, qaStatus: string, accepted: boolean, hasResult: boolean): Partial<EcommerceState> {
    return {
        lifecycle: hasResult ? "finalized" : "draft",
        production: slotProductionStatus(status, accepted, hasResult),
        evidence: hasResult ? "recorded" : "unknown",
        attention: accepted ? "none" : status === "failed" || qaStatus === "FAIL" ? "error" : qaStatus === "UNCERTAIN" || status === "qa" ? "human_required" : "warning",
    };
}

function slotProductionStatus(status: string, accepted: boolean, hasResult: boolean): EcommerceProductionStatus {
    if (accepted || status === "accepted") return "approved";
    if (status === "scheduled") return "scheduled";
    if (status === "queued") return "queued";
    if (status === "running") return "running";
    if (status === "qa") return "qa";
    if (status === "failed") return "failed";
    if (status === "cancelled") return "cancelled";
    return hasResult ? "generated" : "not_started";
}

function nodeSummary(kind: EcommerceNodeKind, assets: ProjectAsset[], artifact: EcommerceArtifact | undefined, runView?: EcommerceRunView) {
    if (kind === "product_input") return inputDetail(assets);
    if (kind === "model_profile" && runView?.run.kernel === "STILL_LIFE") return "静物内核无需模特。切换人物互动预设后，可使用授权真人参考或创建 AI 模特身份。";
    if (kind === "generation_request") {
        if (!runView) return "等待在电商创意工作台选择资产、商拍预设、渠道、比例和数量。";
        const quote = runView.quote ? `\n模型 ${runView.quote.model} · ${runView.quote.count} 张 · 预计 ${formatCredits(runView.quote.totalMicrocredits)} 积分` : "\n尚无可确认报价";
        const output = runView.run.resolution ? ` · ${runView.run.resolution.toUpperCase()} ${runView.run.pixelSize || ""}` : "";
        return `${ecommerceRunStatusLabel(runView.run.status)} · ${runView.run.targetChannel} · ${runView.run.aspectRatio}${output}${quote}`;
    }
    if (kind === "qa_report") {
        if (!runView) return "等待真实结果；逐张检查商品、Logo、身份、人体接触、场景和系列一致性。";
        const counts = runView.slots.reduce<Record<string, number>>((result, { slot }) => { result[slot.qaStatus] = (result[slot.qaStatus] || 0) + 1; return result; }, {});
        return `PASS ${counts.PASS || 0} · UNCERTAIN ${counts.UNCERTAIN || 0} · FAIL ${counts.FAIL || 0}\n已接受 ${runView.slots.filter(({ slot }) => slot.accepted).length}/${runView.slots.length}`;
    }
    if (kind === "motion_plan") return artifact ? summarizePayload(artifact, ["aspectRatio", "durationMs", "agentId"]) : "只有已接受图片才能进入 Motion Director 与视频顺序规划。";
    if (!artifact) return missingArtifactSummary(kind);
    if (kind === "product_dna") return summarizePayload(artifact, ["category", "recordedFacts", "mustPreserve"]);
    if (kind === "model_profile") return summarizePayload(artifact, ["mode", "status", "brief"]);
    if (kind === "scene_pack") return summarizePayload(artifact, ["brief", "wideView", "lighting", "palette"]);
    if (kind === "preset_snapshot") return summarizePayload(artifact, ["name", "presetVersion", "definition"]);
    if (kind === "creative_direction") return summarizePayload(artifact, ["goal", "visualDirection", "seriesRules"]);
    if (kind === "shot_plan") {
        const slots = parsePayload(artifact).slots;
        return `${Array.isArray(slots) ? slots.length : 0} 个镜头槽位 · Prompt 已确定性编译\nArtifact r${artifact.revision}${artifact.skillRef ? ` · ${artifact.skillRef}` : ""}`;
    }
    return `电商 Artifact r${artifact.revision}`;
}

function inputSummary(assets: ProjectAsset[]) {
    const products = assets.filter((asset) => (asset.projectRole || "").startsWith("product_") || asset.projectRole === "packaging").length;
    const models = assets.filter((asset) => asset.projectRole === "model_reference").length;
    const scenes = assets.filter((asset) => asset.projectRole === "scene_reference").length;
    return products ? `${products} 份商品 · ${models} 份模特 · ${scenes} 份场景` : "等待主商品资产";
}

function inputDetail(assets: ProjectAsset[]) {
    if (!assets.length) return "尚未关联电商资产。请先加入主商品正反面、细节、包装、模特、场景或品牌参考。";
    const grouped = new Map<string, string[]>();
    for (const asset of assets) {
        const role = asset.projectRole || "unassigned";
        grouped.set(role, [...(grouped.get(role) || []), asset.title]);
    }
    return [...grouped.entries()].map(([role, titles]) => `${assetRoleLabel(role)}：${titles.join("、")}`).join("\n");
}

function assetRoleLabel(role: string) {
    return ({
        product_primary: "主商品",
        product_front: "商品正面",
        product_back: "商品背面",
        product_detail: "商品细节",
        product_supporting: "搭配商品",
        packaging: "包装",
        logo: "Logo",
        model_reference: "授权模特",
        scene_reference: "场景参考",
        brand_reference: "品牌参考",
        unassigned: "未分组",
    } as Record<string, string>)[role] || role;
}

function missingArtifactSummary(kind: EcommerceNodeKind) {
    return ({
        product_dna: "选择商品后由 ProductIntelligenceAgent 提取事实，并保留人工纠正入口。",
        model_profile: "人物互动预设支持授权真人参考或 AI 模特身份卡。",
        scene_pack: "上传、选择或描述场景后形成宽景、局部、光线与色彩约束。",
        preset_snapshot: "运行时保存不可变预设版本；系统安全、保真和费用约束不可删除。",
        creative_direction: "CreativeDirectorAgent 将渠道目标编排成一组互补镜头角色。",
        shot_plan: "Skill Executor 确定性生成 Shot Plan、逐镜 Prompt 与排除项。",
    } as Partial<Record<EcommerceNodeKind, string>>)[kind] || "等待上游电商生产事实。";
}

function summarizePayload(artifact: EcommerceArtifact, keys: string[]) {
    const payload = parsePayload(artifact);
    const values = keys.flatMap((key) => {
        const value = payload[key];
        if (typeof value === "string" && value.trim()) return [`${displayKey(key)}：${value.trim()}`];
        if (typeof value === "number" || typeof value === "boolean") return [`${displayKey(key)}：${String(value)}`];
        if (Array.isArray(value) && value.length) return [`${displayKey(key)}：${value.slice(0, 3).map(stringifyCompact).join("；")}`];
        if (value && typeof value === "object") return [`${displayKey(key)}：${stringifyCompact(value)}`];
        return [];
    });
    return values.slice(0, 4).join("\n") || `已保存电商 Artifact r${artifact.revision}`;
}

function displayKey(key: string) {
    return ({ category: "品类", recordedFacts: "已确认事实", mustPreserve: "必须保留", mode: "模式", status: "状态", brief: "要求", wideView: "宽景", lighting: "光线", palette: "色彩", name: "预设", presetVersion: "版本", definition: "结构", goal: "目标", visualDirection: "视觉方向", seriesRules: "系列规则", aspectRatio: "比例", resolution: "分辨率", pixelSize: "像素尺寸", durationMs: "时长", agentId: "Agent" } as Record<string, string>)[key] || key;
}

function stringifyCompact(value: unknown) {
    if (typeof value === "string") return value;
    const encoded = JSON.stringify(value);
    return encoded.length > 150 ? `${encoded.slice(0, 147)}...` : encoded;
}

function resultURL(value: { resultUrl?: string; resultPayloadJson?: string }) {
    if (value.resultUrl) return value.resultUrl;
    const payload = parsePayloadJSON(value.resultPayloadJson);
    const images = Array.isArray(payload.images) ? payload.images : [];
    const first = images[0];
    if (!first || typeof first !== "object" || Array.isArray(first)) return "";
    const image = first as Record<string, unknown>;
    return typeof image.url === "string" ? image.url : typeof image.dataUrl === "string" ? image.dataUrl : "";
}

function videoSequencePrompt(sequence: Record<string, unknown>) {
    const segments = Array.isArray(sequence.segments) ? sequence.segments : [];
    return segments.map((segment, index) => {
        const value = segment && typeof segment === "object" && !Array.isArray(segment) ? segment as Record<string, unknown> : {};
        return `${index + 1}. ${String(value.motion || "restrained ecommerce motion")} · ${Math.round(numberValue(value.durationMs) / 1000)}s`;
    }).join("\n");
}

function ecommerceRunProgress(runView?: EcommerceRunView) {
    if (!runView) return 0;
    if (runView.run.status === "awaiting_review") return 20;
    if (runView.run.status === "awaiting_cost") return 30;
    if (runView.run.status === "cancelled") return 0;
    if (!runView.slots.length) return runView.run.status === "ready" ? 100 : 5;
    const weights: Record<string, number> = { planned: 0, scheduled: 8, queued: 15, running: 45, qa: 82, failed: 100, cancelled: 100, accepted: 100 };
    return Math.round(runView.slots.reduce((sum, item) => sum + (weights[item.slot.status] ?? 0), 0) / runView.slots.length);
}

function ecommerceRunStatusLabel(status?: string) {
    return ({
        planning: "Agent 规划中",
        awaiting_review: "等待方案确认",
        awaiting_cost: "等待费用确认",
        generating: "系列生成中",
        qa: "等待质检",
        needs_you: "需要处理",
        ready: "系列已就绪",
        failed: "系列失败",
        cancelled: "已取消",
    } as Record<string, string>)[status || ""] || "等待创建系列";
}

function slotStatusLabel(status: string) {
    return ({ planned: "已规划", scheduled: "等待调度", queued: "排队中", running: "生成中", qa: "待质检", accepted: "已接受", failed: "生成失败", cancelled: "已取消" } as Record<string, string>)[status] || status;
}

function activeTaskId(runView?: EcommerceRunView) {
    return runView?.slots.flatMap(({ attempts }) => attempts).find(({ task }) => task && ["scheduled", "queued", "running"].includes(task.status))?.task?.id;
}

function appendConnection(connections: CanvasConnection[], projectId: string, fromNodeId?: string, toNodeId?: string) {
    if (!fromNodeId || !toNodeId || fromNodeId === toNodeId) return;
    connections.push({ id: `${projectId}:ecommerce:edge:${connections.length + 1}`, fromNodeId, toNodeId });
}

function ecommerceNodeId(projectId: string, kind: EcommerceNodeKind, runId?: string) {
    return `${projectId}:ecommerce:${runId || "workspace"}:${kind}`;
}

function parsePayload(artifact?: EcommerceArtifact) {
    return parsePayloadJSON(artifact?.payloadJson);
}

function parsePayloadJSON(raw?: string) {
    try {
        const parsed = JSON.parse(raw || "{}") as unknown;
        return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? parsed as Record<string, unknown> : {};
    } catch {
        return {};
    }
}

function artifactString(artifact: EcommerceArtifact | undefined, key: string) {
    const value = parsePayload(artifact)[key];
    return typeof value === "string" ? value : "";
}

function numberValue(value: unknown) {
    return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function lifecycle(value: string): EcommerceState["lifecycle"] {
    return (["draft", "review", "finalized", "superseded", "archived"] as const).find((item) => item === value) || "draft";
}

function evidence(value: string): EcommerceEvidence {
    return value === "recorded" || value === "inferred" ? value : "unknown";
}

function skillId(value?: string) {
    return value?.split("@")[0] || undefined;
}

function formatCredits(microcredits?: number) {
    return ((microcredits || 0) / 1_000_000).toLocaleString("zh-CN", { maximumFractionDigits: 6 });
}
