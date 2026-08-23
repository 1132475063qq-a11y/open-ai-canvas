import { useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, Button, Image, Input, Modal, Segmented, Select, Spin, Tag } from "antd";
import { AlertCircle, Check, ChevronDown, Clapperboard, Coins, FileCheck2, ImagePlus, ListChecks, LockKeyhole, RotateCcw, Sparkles, X } from "lucide-react";

import { formatCredits } from "@/constant/credits";
import { canvasThemes } from "@/lib/canvas-theme";
import { createClientId } from "@/lib/client-id";
import {
    createFilmAgentRun,
    getFilmAgentRun,
    getFilmAgentRuntimeCatalog,
    listFilmAgentRuns,
    lockFilmAgentArtifact,
    retryFilmAgentStep,
    rollbackFilmAgentArtifact,
    resolveFilmAgentDecision,
    type FilmAgentIntentRoute,
    type FilmAgentRun,
    type FilmAgentRunDetail,
    type FilmAgentStep,
    type FilmProductionArtifact,
    type FilmProductionArtifactRevision,
} from "@/services/api/film-agent-runtime";
import {
    createFilmProductionHumanQC,
    createFilmProductionImageQuote,
    listFilmProductionAttempts,
    submitFilmProductionImageQuote,
    type FilmProductionAttemptView,
    type FilmProductionImageOptions,
    type FilmProductionImageQuote,
    type FilmVideoSequenceView,
} from "@/services/api/film-production";
import { listLogicalModels, type PublicLogicalModel } from "@/services/api/logical-models";
import type { ProjectDetail } from "@/services/api/projects";
import { latestFilmAttemptsByShot, normalizeFilmBatchShotIds, summarizeFilmBatch, type FilmBatchQuoteItem } from "@/lib/canvas/film-batch-production";
import { collectLockedFilmInputRevisionIds, FILM_AUTO_INTENT_ROUTE, filmIntentSelection } from "@/lib/canvas/film-intent-routing";
import { filmRunLogicalModelId, formatFilmImageOptions, resolveFilmImageOptions } from "@/lib/canvas/film-production-model";
import { useThemeStore } from "@/stores/use-theme-store";
import { FilmVideoProductionSection } from "@/components/canvas/film-video-production-section";
import { FilmVisualQCSection } from "@/components/canvas/film-visual-qc-section";
import { supportsFilmVisualQCModel } from "@/lib/canvas/film-visual-qc";

type FilmProductionPanelProps = {
    projectId: string;
    canvasId: string;
    project: ProjectDetail;
    referenceResourceIds?: string[];
    hidden?: boolean;
    onImportVideoSequence?: (sequence: FilmVideoSequenceView) => void;
};

const REQUIRED_ARTIFACTS = [
    { key: "storyboard", label: "分镜", types: ["storyboard"] },
    { key: "prompt", label: "图片 Prompt", types: ["image-prompt-pack", "prompt-manifest"] },
    { key: "feasibility", label: "制作可行性", types: ["production-feasibility-report"] },
] as const;

const VIDEO_PROMPT_ARTIFACT_TYPES = ["ai-video-prompts", "prompt-manifest"] as const;

export function FilmProductionPanel({ projectId, canvasId, project, referenceResourceIds = [], hidden = false, onImportVideoSequence }: FilmProductionPanelProps) {
    const theme = canvasThemes[useThemeStore((state) => state.theme)];
    const queryClient = useQueryClient();
    const [open, setOpen] = useState(false);
    const [runId, setRunId] = useState("");
    const [shotId, setShotId] = useState("");
    const [imageModelId, setImageModelId] = useState("");
    const [textModelId, setTextModelId] = useState("");
    const [routeId, setRouteId] = useState(FILM_AUTO_INTENT_ROUTE);
    const [agentId, setAgentId] = useState("");
    const [objective, setObjective] = useState("创作短片剧本，并形成后续分镜与制作可执行的故事基础");
    const [reviewBeforeExecution, setReviewBeforeExecution] = useState(true);
    const [generationMode, setGenerationMode] = useState<"single" | "batch">("batch");
    const [quote, setQuote] = useState<FilmProductionImageQuote | null>(null);
    const [confirmSubmit, setConfirmSubmit] = useState(false);
    const [batchShotIds, setBatchShotIds] = useState<string[]>([]);
    const [batchItems, setBatchItems] = useState<FilmBatchQuoteItem[]>([]);
    const [batchConfirmSubmit, setBatchConfirmSubmit] = useState(false);
    const [batchQuoting, setBatchQuoting] = useState(false);
    const [batchSubmitting, setBatchSubmitting] = useState(false);
    const [selectedAttemptId, setSelectedAttemptId] = useState("");
    const [qcNote, setQcNote] = useState("");
    const runKeyRef = useRef({ signature: "", key: "" });
    const quoteKeyRef = useRef({ signature: "", key: "" });
    const submitKeyRef = useRef({ signature: "", key: "" });
    const qcKeyRef = useRef({ signature: "", key: "" });
    const rollbackKeyRef = useRef({ signature: "", key: "" });
    const batchQuoteKeysRef = useRef(new Map<string, { signature: string; key: string }>());
    const batchSubmitKeysRef = useRef(new Map<string, { signature: string; key: string }>());
    const selectionVersionRef = useRef(0);

    const catalogQuery = useQuery({ queryKey: ["film-agent-catalog", projectId], queryFn: () => getFilmAgentRuntimeCatalog(projectId), enabled: open });
    const runsQuery = useQuery({ queryKey: ["film-agent-runs", projectId], queryFn: () => listFilmAgentRuns(projectId), enabled: open, refetchInterval: open ? 8_000 : false });
    const detailQuery = useQuery({ queryKey: ["film-agent-run", projectId, runId], queryFn: () => getFilmAgentRun(projectId, runId), enabled: open && Boolean(runId), refetchInterval: (query) => (isActiveRun(query.state.data) ? 5_000 : false) });
    const modelsQuery = useQuery({ queryKey: ["logical-models", "film-production"], queryFn: listLogicalModels, enabled: open });

    const runs = runsQuery.data?.runs || [];
    const detail = detailQuery.data;
    const activeRun = runs.find((run) => run.id === runId);
    const rootRunId = activeRun?.rootRunId || detail?.run.rootRunId || "";
    const attemptsQuery = useQuery({
        queryKey: ["film-production-attempts", projectId, rootRunId || "all"],
        queryFn: () => listFilmProductionAttempts(projectId, { rootRunId, limit: 100 }),
        enabled: open && Boolean(rootRunId),
        refetchInterval: (query) => (hasActiveAttempt(query.state.data?.attempts) ? 5_000 : false),
    });
    const lineageRuns = useMemo(() => {
        if (!rootRunId) return [];
        const candidates = runs.filter((run) => run.rootRunId === rootRunId);
        if (!candidates.some((run) => run.id === runId)) {
            const fallback = activeRun || detail?.run;
            if (fallback) candidates.push(fallback);
        }
        return candidates;
    }, [activeRun, detail?.run, rootRunId, runId, runs]);
    const lineageDetailQueries = useQueries({
        queries: lineageRuns.map((run) => ({
            queryKey: ["film-agent-run", projectId, run.id],
            queryFn: () => getFilmAgentRun(projectId, run.id),
            enabled: open,
            refetchInterval: (query: { state: { data?: FilmAgentRunDetail } }) => (isActiveRun(query.state.data) ? 5_000 : false),
        })),
    });
    const lineageDetails = useMemo(() => {
        const byRunID = new Map<string, FilmAgentRunDetail>();
        if (detail) byRunID.set(detail.run.id, detail);
        for (const query of lineageDetailQueries) {
            if (query.data) byRunID.set(query.data.run.id, query.data);
        }
        return [...byRunID.values()];
    }, [detail, lineageDetailQueries]);
    const shots = useMemo(() => project.shots.slice().sort((left, right) => left.position - right.position), [project.shots]);
    const imageModels = useMemo(() => (modelsQuery.data?.models || []).filter((model) => model.capability === "image" && model.available), [modelsQuery.data?.models]);
    const textModels = useMemo(() => (modelsQuery.data?.models || []).filter((model) => model.capability === "text" && model.available), [modelsQuery.data?.models]);
    const visualQCModels = useMemo(() => (modelsQuery.data?.models || []).filter(supportsFilmVisualQCModel), [modelsQuery.data?.models]);
    const videoModels = useMemo(() => (modelsQuery.data?.models || []).filter((model) => model.capability === "video" && model.available && supportsImageToVideo(model)), [modelsQuery.data?.models]);
    const selectedRoute = useMemo(() => catalogQuery.data?.intentRoutes.find((route) => route.id === routeId), [catalogQuery.data?.intentRoutes, routeId]);
    const attempts = useMemo(() => {
        const allAttempts = attemptsQuery.data?.attempts || [];
        return detail?.run.rootRunId ? allAttempts.filter((item) => item.attempt.rootRunId === detail.run.rootRunId) : allAttempts;
    }, [attemptsQuery.data?.attempts, detail?.run.rootRunId]);
    const selectedAttempt = useMemo(() => attempts.find((item) => item.attempt.id === selectedAttemptId) || null, [attempts, selectedAttemptId]);
    const latestAttemptsByShot = useMemo(() => latestFilmAttemptsByShot(attempts), [attempts]);
    const batchSummary = useMemo(
        () =>
            summarizeFilmBatch(
                batchItems.filter((item) => batchShotIds.includes(item.shotId)),
                attempts,
            ),
        [attempts, batchItems, batchShotIds],
    );
    const artifactChoices = useMemo(() => resolveRequiredArtifacts(lineageDetails), [lineageDetails]);
    const videoPromptRevision = useMemo(() => resolveVideoPromptRevision(lineageDetails), [lineageDetails]);
    const inputArtifactRevisionIds = useMemo(() => collectLockedFilmInputRevisionIds(lineageDetails), [lineageDetails]);
    const lineageLoading = lineageDetailQueries.some((query) => query.isLoading);
    const pendingDecision = detail?.humanDecisions.find((decision) => decision.status === "pending");
    const selectedImageModel = imageModels.find((model) => model.id === imageModelId);
    const imageOptions = useMemo(() => resolveFilmImageOptions(selectedImageModel), [selectedImageModel]);
    const imageOptionsLabel = formatFilmImageOptions(imageOptions);
    const canQuote = Boolean(rootRunId && shotId && imageModelId && imageOptions.size && artifactChoices.every((item) => item.revision?.status === "locked"));

    useEffect(() => {
        if (!runId && runs.length) setRunId(preferredRun(runs).id);
    }, [runId, runs]);
    useEffect(() => {
        if (!shotId && shots.length) setShotId(shots[0].id);
    }, [shotId, shots]);
    useEffect(() => {
        if (!imageModelId && imageModels.length) setImageModelId(imageModels[0].id);
        if (!textModelId && textModels.length) setTextModelId(textModels[0].id);
    }, [imageModelId, imageModels, textModelId, textModels]);
    useEffect(() => {
        if (!selectedRoute?.requiresDisambiguation || !selectedRoute.candidateAgentIds?.includes(agentId)) setAgentId("");
    }, [agentId, selectedRoute]);
    useEffect(() => {
        const availableShotIds = shots.map((shot) => shot.id);
        setBatchShotIds((current) => {
            const retained = normalizeFilmBatchShotIds(current.filter((shotId) => availableShotIds.includes(shotId)));
            return retained.length ? retained : normalizeFilmBatchShotIds(availableShotIds);
        });
    }, [shots]);
    useEffect(() => {
        if (selectedAttemptId && !selectedAttempt) setSelectedAttemptId("");
    }, [selectedAttempt, selectedAttemptId]);

    const invalidateProduction = () => {
        void queryClient.invalidateQueries({ queryKey: ["film-production-attempts", projectId] });
        void queryClient.invalidateQueries({ queryKey: ["film-agent-runs", projectId] });
        void queryClient.invalidateQueries({ queryKey: ["film-agent-run", projectId] });
    };

    const clearGenerationSelection = () => {
        selectionVersionRef.current += 1;
        setQuote(null);
        setConfirmSubmit(false);
        quoteKeyRef.current = { signature: "", key: "" };
        submitKeyRef.current = { signature: "", key: "" };
    };
    const clearBatchSelection = () => {
        setBatchItems([]);
        setBatchConfirmSubmit(false);
        batchQuoteKeysRef.current.clear();
        batchSubmitKeysRef.current.clear();
    };
    const handleRunChange = (nextRunId: string) => {
        setRunId(nextRunId);
        clearGenerationSelection();
        clearBatchSelection();
        setSelectedAttemptId("");
        setQcNote("");
    };
    const handleShotChange = (nextShotId: string) => {
        setShotId(nextShotId);
        clearGenerationSelection();
        setSelectedAttemptId("");
        setQcNote("");
    };
    const handleImageModelChange = (nextImageModelId: string) => {
        setImageModelId(nextImageModelId);
        clearGenerationSelection();
        clearBatchSelection();
    };

    useEffect(() => {
        if (!quote) return;
        const expiresAt = Date.parse(quote.expiresAt);
        const expire = () => {
            selectionVersionRef.current += 1;
            setQuote(null);
            setConfirmSubmit(false);
            quoteKeyRef.current = { signature: "", key: "" };
            submitKeyRef.current = { signature: "", key: "" };
        };
        if (!Number.isFinite(expiresAt) || expiresAt <= Date.now()) {
            expire();
            return;
        }
        const timer = window.setTimeout(expire, expiresAt - Date.now() + 50);
        return () => window.clearTimeout(timer);
    }, [quote]);

    const createRunMutation = useMutation({
        mutationFn: () => {
            const routing = filmIntentSelection(routeId, selectedRoute, agentId);
            return createFilmAgentRun(
                projectId,
                {
                    canvasId,
                    objective,
                    ...routing,
                    logicalModelId: textModelId || undefined,
                    reviewBeforeExecution,
                    input: { shotCount: shots.length },
                    inputArtifactRevisionIds,
                },
                stableOperationKey(runKeyRef, "film-run", JSON.stringify([canvasId, objective.trim(), routeId, agentId, textModelId, reviewBeforeExecution, shots.length, inputArtifactRevisionIds])),
            );
        },
        onSuccess: (result) => {
            setRunId(result.detail.run.id);
            runKeyRef.current = { signature: "", key: "" };
            invalidateProduction();
        },
    });
    const decisionMutation = useMutation({
        mutationFn: (action: "approve" | "cancel") =>
            resolveFilmAgentDecision(projectId, detail!.run.id, pendingDecision!.id, {
                expectedRunRevision: detail!.run.revision,
                expectedStepRevision: detail!.steps.find((step) => step.id === pendingDecision!.stepId)?.revision || 0,
                expectedDecisionRevision: pendingDecision!.revision,
                action,
            }),
        onSuccess: (next) => {
            queryClient.setQueryData(["film-agent-run", projectId, next.run.id], next);
            invalidateProduction();
        },
    });
    const lockMutation = useMutation({
        mutationFn: (artifact: FilmArtifactChoice) =>
            lockFilmAgentArtifact(projectId, artifact.runId || detail!.run.id, artifact.artifact!.id, {
                expectedRunRevision: artifact.runRevision ?? detail!.run.revision,
                expectedArtifactSequence: artifact.artifact!.revisionSequence,
                expectedRevisionId: artifact.revision!.id,
            }),
        onSuccess: () => invalidateProduction(),
    });
    const rollbackMutation = useMutation({
        mutationFn: ({ choice, targetRevisionId }: { choice: FilmArtifactChoice; targetRevisionId: string }) =>
            rollbackFilmAgentArtifact(
                projectId,
                choice.runId || detail!.run.id,
                choice.artifact!.id,
                {
                    expectedRunRevision: choice.runRevision ?? detail!.run.revision,
                    expectedArtifactSequence: choice.artifact!.revisionSequence,
                    expectedCurrentRevisionId: choice.artifact!.currentRevisionId || choice.revision!.id,
                    targetRevisionId,
                },
                stableOperationKey(rollbackKeyRef, "film-artifact-rollback", JSON.stringify([choice.artifact!.id, choice.artifact!.currentRevisionId, targetRevisionId])),
            ),
        onSuccess: () => {
            rollbackKeyRef.current = { signature: "", key: "" };
            invalidateProduction();
        },
    });
    const retryStepMutation = useMutation({
        mutationFn: (step: FilmAgentStep) =>
            retryFilmAgentStep(projectId, detail!.run.id, step.id, {
                expectedRunRevision: detail!.run.revision,
                expectedStepRevision: step.revision,
                reason: "用户从短剧制作面板重试失败步骤",
            }),
        onSuccess: (next) => {
            queryClient.setQueryData(["film-agent-run", projectId, next.run.id], next);
            invalidateProduction();
        },
    });
    const quoteMutation = useMutation({
        mutationFn: async () => {
            const version = selectionVersionRef.current;
            const quote = await createFilmProductionImageQuote(
                projectId,
                {
                    rootRunId,
                    shotId,
                    storyboardArtifactRevisionId: artifactChoices.find((item) => item.key === "storyboard")!.revision!.id,
                    promptArtifactRevisionId: artifactChoices.find((item) => item.key === "prompt")!.revision!.id,
                    feasibilityArtifactRevisionId: artifactChoices.find((item) => item.key === "feasibility")!.revision!.id,
                    logicalModelId: imageModelId,
                    referenceResourceIds,
                    options: imageOptions,
                },
                stableOperationKey(quoteKeyRef, "film-quote", JSON.stringify([rootRunId, shotId, imageModelId, imageOptions, referenceResourceIds, artifactChoices.map((item) => item.revision?.id)])),
            );
            return { quote, version };
        },
        onSuccess: ({ quote: next, version }) => {
            if (version !== selectionVersionRef.current) return;
            setConfirmSubmit(false);
            setQuote(next);
        },
    });
    const submitMutation = useMutation({
        mutationFn: async () => {
            const version = selectionVersionRef.current;
            const result = await submitFilmProductionImageQuote(projectId, quote!.id, { quoteFingerprint: quote!.quoteFingerprint }, stableOperationKey(submitKeyRef, "film-submit", quote!.id));
            return { result, version };
        },
        onSuccess: ({ result: next, version }) => {
            if (version !== selectionVersionRef.current) return;
            clearGenerationSelection();
            setSelectedAttemptId(next.attempt.attempt.id);
            invalidateProduction();
        },
    });
    const quoteBatch = async (requestedShotIds = batchShotIds) => {
        const targetShotIds = normalizeFilmBatchShotIds(requestedShotIds);
        if (!rootRunId || !imageModelId || !imageOptions.size || !targetShotIds.length || artifactChoices.some((item) => item.revision?.status !== "locked")) return;
        const artifactRevisionIds = artifactChoices.map((item) => item.revision!.id);
        setBatchQuoting(true);
        setBatchItems((current) => {
            const byShot = new Map(current.map((item) => [item.shotId, item]));
            for (const shotId of targetShotIds) byShot.set(shotId, { shotId, status: "quoting" });
            return normalizeFilmBatchShotIds(batchShotIds).map((shotId) => byShot.get(shotId) || { shotId, status: "failed", error: "未生成报价" });
        });
        const results = await Promise.allSettled(
            targetShotIds.map(async (nextShotId) => {
                const signature = JSON.stringify([rootRunId, nextShotId, imageModelId, imageOptions, referenceResourceIds, artifactRevisionIds]);
                const previous = batchQuoteKeysRef.current.get(nextShotId);
                const idempotencyKey = previous?.signature === signature ? previous.key : `film-batch-quote:${createClientId()}`;
                batchQuoteKeysRef.current.set(nextShotId, { signature, key: idempotencyKey });
                const nextQuote = await createFilmProductionImageQuote(
                    projectId,
                    {
                        rootRunId,
                        shotId: nextShotId,
                        storyboardArtifactRevisionId: artifactChoices.find((item) => item.key === "storyboard")!.revision!.id,
                        promptArtifactRevisionId: artifactChoices.find((item) => item.key === "prompt")!.revision!.id,
                        feasibilityArtifactRevisionId: artifactChoices.find((item) => item.key === "feasibility")!.revision!.id,
                        logicalModelId: imageModelId,
                        referenceResourceIds,
                        options: imageOptions,
                    },
                    idempotencyKey,
                );
                return { shotId: nextShotId, quote: nextQuote };
            }),
        );
        const settled = new Map<string, FilmBatchQuoteItem>();
        results.forEach((result, index) => {
            const shotId = targetShotIds[index];
            settled.set(shotId, result.status === "fulfilled" ? { shotId, status: "quoted", quote: result.value.quote } : { shotId, status: "failed", error: readError(result.reason) });
        });
        setBatchItems((current) => current.map((item) => settled.get(item.shotId) || item));
        setBatchQuoting(false);
    };
    const submitBatch = async () => {
        const selectedShotSet = new Set(batchShotIds);
        const readyItems = batchItems.filter((item) => selectedShotSet.has(item.shotId) && item.quote && item.status === "quoted" && !isQuoteExpired(item.quote));
        if (!readyItems.length) return;
        setBatchSubmitting(true);
        setBatchItems((current) => current.map((item) => (readyItems.some((ready) => ready.shotId === item.shotId) ? { ...item, status: "submitting" } : item)));
        const results = await Promise.allSettled(
            readyItems.map(async (item) => {
                const signature = item.quote!.id;
                const previous = batchSubmitKeysRef.current.get(item.shotId);
                const idempotencyKey = previous?.signature === signature ? previous.key : `film-batch-submit:${createClientId()}`;
                batchSubmitKeysRef.current.set(item.shotId, { signature, key: idempotencyKey });
                const result = await submitFilmProductionImageQuote(projectId, item.quote!.id, { quoteFingerprint: item.quote!.quoteFingerprint }, idempotencyKey);
                return { shotId: item.shotId, attemptId: result.attempt.attempt.id };
            }),
        );
        let firstAttemptId = "";
        const settled = new Map<string, FilmBatchQuoteItem>();
        results.forEach((result, index) => {
            const shotId = readyItems[index].shotId;
            if (result.status === "fulfilled") {
                firstAttemptId ||= result.value.attemptId;
                settled.set(shotId, { ...readyItems[index], status: "submitted", attemptId: result.value.attemptId });
            } else {
                settled.set(shotId, { ...readyItems[index], status: "failed", error: readError(result.reason) });
            }
        });
        setBatchItems((current) => current.map((item) => settled.get(item.shotId) || item));
        setBatchConfirmSubmit(false);
        setBatchSubmitting(false);
        if (firstAttemptId) setSelectedAttemptId(firstAttemptId);
        invalidateProduction();
    };
    const qcMutation = useMutation({
        mutationFn: (input: { decision: "PASS" | "UNCERTAIN" | "FAIL"; action: "accept" | "hold" | "retry" }) =>
            createFilmProductionHumanQC(
                projectId,
                selectedAttempt!.attempt.id,
                { decision: input.decision, action: input.action, note: qcNote },
                stableOperationKey(qcKeyRef, "film-qc", JSON.stringify([selectedAttempt!.attempt.id, input.decision, input.action, qcNote])),
            ),
        onSuccess: () => {
            setQcNote("");
            invalidateProduction();
        },
    });

    if (hidden || project.project.type !== "short-drama") return null;

    return (
        <div className="pointer-events-none absolute bottom-4 right-3 flex flex-col items-end gap-2" style={{ zIndex: "var(--z-panel-floating)" }} data-canvas-no-zoom>
            <button
                type="button"
                className="pointer-events-auto inline-flex h-10 items-center gap-2 rounded-xl px-3 text-sm font-medium backdrop-blur-xl transition hover:-translate-y-px"
                style={{ background: theme.toolbar.panel, color: theme.node.text, boxShadow: "var(--workspace-overlay-shadow)" }}
                onClick={() => setOpen((value) => !value)}
                aria-expanded={open}
                aria-label="打开短剧制作面板"
            >
                <Clapperboard className="size-4" style={{ color: theme.accent.primary }} />
                <span>短剧制作</span>
                <ChevronDown className={`size-3.5 transition-transform ${open ? "rotate-180" : ""}`} />
            </button>
            {open ? (
                <section
                    className="pointer-events-auto flex flex-col overflow-hidden rounded-2xl border backdrop-blur-2xl"
                    style={{ background: theme.toolbar.panel, borderColor: theme.toolbar.border, color: theme.node.text, boxShadow: "var(--workspace-overlay-shadow)", maxHeight: "min(78vh, 720px)", width: "min(380px, calc(100vw - 24px))" }}
                    aria-label="短剧制作"
                >
                    <header className="flex shrink-0 items-center justify-between gap-3 border-b px-3 py-2.5" style={{ borderColor: theme.toolbar.border }}>
                        <div className="flex min-w-0 items-center gap-2">
                            <span className="grid size-7 shrink-0 place-items-center rounded-lg" style={{ background: theme.accent.primarySoft, color: theme.accent.primary }}>
                                <Sparkles className="size-3.5" />
                            </span>
                            <span className="min-w-0">
                                <strong className="block truncate text-sm">Agent 制作与生图</strong>
                                <span className="block truncate text-xs" style={{ color: theme.node.muted }}>
                                    规划 → 锁定产物 → 报价 → 验收
                                </span>
                            </span>
                        </div>
                        <button type="button" className="grid size-7 shrink-0 place-items-center rounded-md hover:bg-black/5 dark:hover:bg-white/10" style={{ color: theme.node.muted }} onClick={() => setOpen(false)} aria-label="关闭短剧制作面板">
                            <X className="size-4" />
                        </button>
                    </header>
                    <div className="thin-scrollbar min-h-0 flex-1 space-y-3 overflow-y-auto p-3">
                        {catalogQuery.isError || runsQuery.isError || modelsQuery.isError ? <Alert type="error" showIcon message="短剧运行时暂时不可用" description={readError(catalogQuery.error || runsQuery.error || modelsQuery.error)} /> : null}
                        <RunSection
                            activeRun={activeRun}
                            runs={runs}
                            runId={runId}
                            onRunChange={handleRunChange}
                            objective={objective}
                            onObjectiveChange={setObjective}
                            routeId={routeId}
                            routes={catalogQuery.data?.intentRoutes || []}
                            onRouteChange={setRouteId}
                            agentId={agentId}
                            agents={catalogQuery.data?.agents || []}
                            onAgentChange={setAgentId}
                            textModelId={textModelId}
                            textModels={textModels}
                            onTextModelChange={setTextModelId}
                            reviewBeforeExecution={reviewBeforeExecution}
                            onReviewBeforeExecutionChange={setReviewBeforeExecution}
                            pendingDecision={pendingDecision}
                            activeRunHasTextModel={Boolean(filmRunLogicalModelId(activeRun || detail?.run))}
                            creating={createRunMutation.isPending}
                            onCreate={() => createRunMutation.mutate()}
                            onDecision={(action) => decisionMutation.mutate(action)}
                            decisionPending={decisionMutation.isPending}
                        />
                        {detailQuery.isLoading || lineageLoading ? (
                            <div className="flex items-center justify-center py-6">
                                <Spin size="small" />
                            </div>
                        ) : detail ? (
                            <>
                                <AgentProgressSection detail={detail} retryingStepId={retryStepMutation.isPending ? retryStepMutation.variables?.id || "" : ""} onRetry={(step) => retryStepMutation.mutate(step)} />
                                <ArtifactSection
                                    choices={artifactChoices}
                                    locking={lockMutation.isPending}
                                    rollingBack={rollbackMutation.isPending}
                                    onLock={(choice) => lockMutation.mutate(choice)}
                                    onRollback={(choice, targetRevisionId) => rollbackMutation.mutate({ choice, targetRevisionId })}
                                />
                            </>
                        ) : (
                            <InlineNotice text="先选择或创建一个 Agent Run" />
                        )}
                        <Segmented
                            block
                            value={generationMode}
                            onChange={(value) => setGenerationMode(value as "single" | "batch")}
                            options={[
                                {
                                    value: "batch",
                                    label: (
                                        <span className="inline-flex items-center gap-1.5">
                                            <ListChecks className="size-3.5" />
                                            批量镜头
                                        </span>
                                    ),
                                },
                                {
                                    value: "single",
                                    label: (
                                        <span className="inline-flex items-center gap-1.5">
                                            <ImagePlus className="size-3.5" />
                                            单个镜头
                                        </span>
                                    ),
                                },
                            ]}
                        />
                        {generationMode === "batch" ? (
                            <BatchGenerationSection
                                shots={shots}
                                selectedShotIds={batchShotIds}
                                onToggleShot={(nextShotId, checked) => {
                                    setBatchShotIds((current) => normalizeFilmBatchShotIds(checked ? [...current, nextShotId] : current.filter((item) => item !== nextShotId)));
                                    if (!checked) setBatchItems((current) => current.filter((item) => item.shotId !== nextShotId));
                                }}
                                onSelectAll={() => setBatchShotIds(normalizeFilmBatchShotIds(shots.map((shot) => shot.id)))}
                                imageModels={imageModels}
                                imageModelId={imageModelId}
                                onImageModelChange={handleImageModelChange}
                                imageOptions={imageOptions}
                                imageOptionsLabel={imageOptionsLabel}
                                canQuote={canQuote && batchShotIds.length > 0}
                                items={batchItems}
                                latestAttemptsByShot={latestAttemptsByShot}
                                summary={batchSummary}
                                quoting={batchQuoting}
                                onQuote={() => quoteBatch()}
                                onRetryFailed={() => quoteBatch(batchItems.filter((item) => batchShotIds.includes(item.shotId) && item.status === "failed").map((item) => item.shotId))}
                                confirmSubmit={batchConfirmSubmit}
                                onConfirmSubmit={setBatchConfirmSubmit}
                                submitting={batchSubmitting}
                                onSubmit={() => void submitBatch()}
                            />
                        ) : (
                            <GenerationSection
                                shots={shots}
                                shotId={shotId}
                                onShotChange={handleShotChange}
                                imageModels={imageModels}
                                imageModelId={imageModelId}
                                onImageModelChange={handleImageModelChange}
                                imageOptions={imageOptions}
                                imageOptionsLabel={imageOptionsLabel}
                                canQuote={canQuote}
                                quote={quote}
                                quoting={quoteMutation.isPending}
                                onQuote={() => {
                                    if (quote && isQuoteExpired(quote)) clearGenerationSelection();
                                    quoteMutation.mutate();
                                }}
                                confirmSubmit={confirmSubmit}
                                onConfirmSubmit={setConfirmSubmit}
                                submitting={submitMutation.isPending}
                                onSubmit={() => submitMutation.mutate()}
                            />
                        )}
                        <AttemptSection
                            attempts={attempts}
                            selectedAttempt={selectedAttempt}
                            selectedAttemptId={selectedAttemptId}
                            onSelect={setSelectedAttemptId}
                            qcNote={qcNote}
                            onQcNoteChange={setQcNote}
                            qcPending={qcMutation.isPending}
                            onQC={(input) => qcMutation.mutate(input)}
                        />
                        <FilmVisualQCSection projectId={projectId} selectedAttempt={selectedAttempt} models={visualQCModels} referenceResourceIds={referenceResourceIds} onProductionChanged={invalidateProduction} />
                        <FilmVideoProductionSection
                            projectId={projectId}
                            rootRunId={rootRunId}
                            shots={shots}
                            imageAttempts={attempts}
                            videoModels={videoModels}
                            visualQCModels={visualQCModels}
                            promptRevision={videoPromptRevision}
                            onProductionChanged={invalidateProduction}
                            onImportSequence={onImportVideoSequence}
                        />
                        {createRunMutation.isError || retryStepMutation.isError || quoteMutation.isError || submitMutation.isError || qcMutation.isError ? (
                            <Alert type="error" showIcon message={readError(createRunMutation.error || retryStepMutation.error || quoteMutation.error || submitMutation.error || qcMutation.error)} />
                        ) : null}
                    </div>
                </section>
            ) : null}
        </div>
    );
}

function AgentProgressSection({ detail, retryingStepId, onRetry }: { detail: FilmAgentRunDetail; retryingStepId: string; onRetry: (step: FilmAgentStep) => void }) {
    const steps = detail.steps.slice().sort((left, right) => left.position - right.position);
    const completed = steps.filter((step) => step.status === "completed" || step.status === "skipped").length;
    return (
        <section className="space-y-2">
            <div className="flex items-center justify-between gap-2">
                <SectionTitle icon={<ListChecks className="size-3.5" />} title="Agent 进度" />
                <span className="text-[var(--fs-tiny)] tabular-nums" style={{ color: "var(--foreground-muted)" }}>
                    {completed}/{steps.length}
                </span>
            </div>
            <div className="space-y-1">
                {steps.map((step, index) => {
                    const failed = step.status === "failed";
                    const active = detail.run.currentStepId === step.id;
                    return (
                        <div key={step.id} className="flex min-w-0 items-start gap-2 rounded-md px-2 py-1.5" style={{ background: active || failed ? "var(--library-surface)" : "transparent" }}>
                            <span className="mt-0.5 grid size-4 shrink-0 place-items-center rounded-full text-[var(--fs-micro)] font-medium" style={agentStepMarkerStyle(step.status)}>
                                {step.status === "completed" ? <Check className="size-2.5" /> : index + 1}
                            </span>
                            <span className="min-w-0 flex-1">
                                <span className="flex min-w-0 items-center gap-1.5">
                                    <span className="truncate text-xs font-medium">{step.agentId}</span>
                                    <span className="shrink-0 text-[var(--fs-tiny)]" style={{ color: "var(--foreground-muted)" }}>
                                        {agentStepStatusLabel(step.status)}
                                    </span>
                                </span>
                                <span className="block truncate text-[var(--fs-tiny)]" style={{ color: failed ? "var(--status-error)" : "var(--foreground-muted)" }} title={failed ? step.failure : filmStepSkillLabel(step)}>
                                    {failed ? step.failure || step.failureCode || "步骤执行失败" : filmStepSkillLabel(step)}
                                </span>
                            </span>
                            {failed ? (
                                <Button size="small" type="text" icon={<RotateCcw className="size-3" />} loading={retryingStepId === step.id} disabled={Boolean(retryingStepId && retryingStepId !== step.id)} onClick={() => onRetry(step)}>
                                    重试
                                </Button>
                            ) : null}
                        </div>
                    );
                })}
            </div>
        </section>
    );
}

function filmStepSkillLabel(step: FilmAgentStep) {
    try {
        const skills = JSON.parse(step.skillIdsJson);
        if (Array.isArray(skills)) return skills.filter((item): item is string => typeof item === "string").join(" · ") || step.stepKey;
    } catch {
        // The backend owns this JSON; a malformed legacy row still needs a readable fallback.
    }
    return step.stepKey;
}

function agentStepStatusLabel(status: string) {
    return { planned: "待解锁", ready: "待执行", running: "执行中", awaiting_human: "待确认", completed: "已完成", failed: "失败", cancelled: "已取消", skipped: "已跳过" }[status] || status;
}

function agentStepMarkerStyle(status: string) {
    if (status === "completed") return { background: "color-mix(in srgb, var(--status-success) 14%, transparent)", color: "var(--status-success)" };
    if (status === "failed") return { background: "color-mix(in srgb, var(--status-error) 12%, transparent)", color: "var(--status-error)" };
    if (status === "running" || status === "ready") return { background: "color-mix(in srgb, var(--status-loading) 12%, transparent)", color: "var(--status-loading)" };
    return { background: "var(--surface-active)", color: "var(--foreground-muted)" };
}

function RunSection({
    activeRun,
    runs,
    runId,
    onRunChange,
    objective,
    onObjectiveChange,
    routeId,
    routes,
    onRouteChange,
    agentId,
    agents,
    onAgentChange,
    textModelId,
    textModels,
    onTextModelChange,
    reviewBeforeExecution,
    onReviewBeforeExecutionChange,
    pendingDecision,
    activeRunHasTextModel,
    creating,
    onCreate,
    onDecision,
    decisionPending,
}: {
    activeRun?: FilmAgentRun;
    runs: FilmAgentRun[];
    runId: string;
    onRunChange: (value: string) => void;
    objective: string;
    onObjectiveChange: (value: string) => void;
    routeId: string;
    routes: FilmAgentIntentRoute[];
    onRouteChange: (value: string) => void;
    agentId: string;
    agents: Array<{ id: string; description: string }>;
    onAgentChange: (value: string) => void;
    textModelId: string;
    textModels: PublicLogicalModel[];
    onTextModelChange: (value: string) => void;
    reviewBeforeExecution: boolean;
    onReviewBeforeExecutionChange: (value: boolean) => void;
    pendingDecision?: { id: string; question: string };
    activeRunHasTextModel: boolean;
    creating: boolean;
    onCreate: () => void;
    onDecision: (action: "approve" | "cancel") => void;
    decisionPending: boolean;
}) {
    const selectedRoute = routes.find((route) => route.id === routeId);
    const requiresAgent = Boolean(selectedRoute?.requiresDisambiguation);
    const agentById = new Map(agents.map((agent) => [agent.id, agent]));
    return (
        <section className="space-y-2">
            <SectionTitle icon={<Sparkles className="size-3.5" />} title="Agent 规划" />
            {runs.length ? (
                <Select
                    className="w-full"
                    value={runId || undefined}
                    placeholder="选择 Agent Run"
                    onChange={onRunChange}
                    options={runs.map((run) => ({ value: run.id, label: `${run.intentRouteId || "Run"} · ${run.status} · ${run.objective.slice(0, 24)}` }))}
                />
            ) : null}
            {activeRun && pendingDecision && activeRunHasTextModel ? (
                <Alert
                    type="warning"
                    showIcon
                    message="等待人工确认"
                    description={pendingDecision.question}
                    action={
                        <span className="flex gap-1">
                            <Button size="small" type="primary" loading={decisionPending} onClick={() => onDecision("approve")}>
                                确认
                            </Button>
                            <Button size="small" disabled={decisionPending} onClick={() => onDecision("cancel")}>
                                取消
                            </Button>
                        </span>
                    }
                />
            ) : null}
            {activeRun && pendingDecision && !activeRunHasTextModel ? (
                <Alert
                    type="error"
                    showIcon
                    message="此 Run 未绑定逻辑文本模型"
                    description="确认后无法执行。请取消这条 Run，待管理员发布逻辑文本模型后重新创建。"
                    action={
                        <Button size="small" danger disabled={decisionPending} onClick={() => onDecision("cancel")}>
                            取消 Run
                        </Button>
                    }
                />
            ) : null}
            <Input.TextArea value={objective} onChange={(event) => onObjectiveChange(event.target.value)} autoSize={{ minRows: 2, maxRows: 4 }} placeholder="描述这次短剧制作目标" />
            <div className="grid grid-cols-2 gap-2">
                <Select
                    className="w-full"
                    value={routeId}
                    placeholder="Intent 路由"
                    onChange={onRouteChange}
                    options={[{ value: FILM_AUTO_INTENT_ROUTE, label: "自动识别 Intent" }, ...routes.map((route) => ({ value: route.id, label: route.name ? `${route.id} · ${route.name}` : route.id }))]}
                />
                <Select className="w-full" value={textModelId || undefined} placeholder="文本模型" onChange={onTextModelChange} options={textModels.map((model) => ({ value: model.id, label: model.name }))} />
            </div>
            {!textModels.length ? (
                <Alert
                    type="error"
                    showIcon
                    message="尚未发布逻辑文本模型"
                    description="Film Agent 必须绑定后台逻辑文本模型，普通设置里的本机 CLI 或自定义渠道不会绕过持久化 Worker。"
                    action={
                        <Button size="small" href="/admin/models">
                            前往发布
                        </Button>
                    }
                />
            ) : null}
            {requiresAgent ? (
                <Select
                    className="w-full"
                    value={agentId || undefined}
                    placeholder="选择执行 Agent"
                    onChange={onAgentChange}
                    options={(selectedRoute?.candidateAgentIds || []).map((id) => ({ value: id, label: agentById.has(id) ? id : `${id}（未注册）` }))}
                />
            ) : null}
            <label className="flex cursor-pointer items-center gap-2 text-xs" style={{ color: "inherit" }}>
                <input type="checkbox" checked={reviewBeforeExecution} onChange={(event) => onReviewBeforeExecutionChange(event.target.checked)} />
                生成前保留人工确认
            </label>
            <Button block type="primary" icon={<Sparkles className="size-3.5" />} loading={creating} disabled={!objective.trim() || !textModelId || (requiresAgent && !agentId)} onClick={onCreate}>
                {activeRun ? "新建规划 Run" : "开始 Agent 规划"}
            </Button>
        </section>
    );
}

type FilmArtifactChoice = {
    key: string;
    label: string;
    types: readonly string[];
    artifact?: FilmProductionArtifact;
    revision?: FilmProductionArtifactRevision;
    revisions: FilmProductionArtifactRevision[];
    runId?: string;
    runRevision?: number;
};

function ArtifactSection({
    choices,
    locking,
    rollingBack,
    onLock,
    onRollback,
}: {
    choices: FilmArtifactChoice[];
    locking: boolean;
    rollingBack: boolean;
    onLock: (choice: FilmArtifactChoice) => void;
    onRollback: (choice: FilmArtifactChoice, targetRevisionId: string) => void;
}) {
    const [historyChoice, setHistoryChoice] = useState<FilmArtifactChoice | null>(null);
    const [targetRevisionId, setTargetRevisionId] = useState("");
    const history = historyChoice?.revisions || [];
    const targetRevision = history.find((revision) => revision.id === targetRevisionId);

    const openHistory = (choice: FilmArtifactChoice) => {
        setHistoryChoice(choice);
        setTargetRevisionId(choice.revision?.id || "");
    };

    return (
        <section className="space-y-2">
            <SectionTitle icon={<FileCheck2 className="size-3.5" />} title="制作产物" />
            {choices.map((choice) => (
                <div key={choice.key} className="flex items-center justify-between gap-2 rounded-lg p-2" style={{ background: "var(--library-surface)" }}>
                    <span className="min-w-0">
                        <span className="block truncate text-xs font-medium">{choice.label}</span>
                        <span className="block truncate text-[var(--fs-tiny)]" style={{ color: "var(--foreground-muted)" }}>
                            {choice.revision ? `${choice.revision.status} · v${choice.revision.version}` : "未生成"}
                        </span>
                    </span>
                    <span className="flex shrink-0 items-center gap-1">
                        {choice.revisions.length > 1 ? (
                            <Button size="small" icon={<RotateCcw className="size-3" />} onClick={() => openHistory(choice)}>
                                版本
                            </Button>
                        ) : null}
                        {choice.revision && choice.revision.status !== "locked" ? (
                            <Button size="small" icon={<LockKeyhole className="size-3" />} loading={locking} onClick={() => onLock(choice)}>
                                锁定
                            </Button>
                        ) : choice.revision ? (
                            <Tag color="success">已锁定</Tag>
                        ) : (
                            <Tag>缺失</Tag>
                        )}
                    </span>
                </div>
            ))}
            <Modal title={historyChoice ? `${historyChoice.label} 版本历史` : "版本历史"} open={Boolean(historyChoice)} onCancel={() => setHistoryChoice(null)} footer={null} destroyOnHidden centered>
                {historyChoice ? (
                    <div className="space-y-3">
                        <Select
                            className="w-full"
                            value={targetRevisionId || undefined}
                            onChange={setTargetRevisionId}
                            options={history
                                .slice()
                                .sort((left, right) => right.version - left.version)
                                .map((revision) => ({
                                    value: revision.id,
                                    label: `v${revision.version} · ${revision.status}${revision.id === historyChoice.revision?.id ? " · 当前" : ""}`,
                                }))}
                        />
                        {targetRevision ? (
                            <div className="rounded-lg p-2 text-xs" style={{ background: "var(--library-surface)", color: "var(--foreground-muted)" }}>
                                <div>创建于 {new Date(targetRevision.createdAt).toLocaleString("zh-CN")}</div>
                                <div className="mt-1 truncate">Digest: {targetRevision.contentDigest}</div>
                            </div>
                        ) : null}
                        <div className="flex justify-end gap-2">
                            <Button onClick={() => setHistoryChoice(null)}>取消</Button>
                            <Button
                                type="primary"
                                icon={<RotateCcw className="size-3.5" />}
                                loading={rollingBack}
                                disabled={!targetRevision || targetRevision.id === historyChoice.revision?.id}
                                onClick={() => {
                                    if (!targetRevision) return;
                                    onRollback(historyChoice, targetRevision.id);
                                    setHistoryChoice(null);
                                }}
                            >
                                回退到此版本
                            </Button>
                        </div>
                    </div>
                ) : null}
            </Modal>
        </section>
    );
}

function GenerationSection({
    shots,
    shotId,
    onShotChange,
    imageModels,
    imageModelId,
    onImageModelChange,
    imageOptions,
    imageOptionsLabel,
    canQuote,
    quote,
    quoting,
    onQuote,
    confirmSubmit,
    onConfirmSubmit,
    submitting,
    onSubmit,
}: {
    shots: ProjectDetail["shots"];
    shotId: string;
    onShotChange: (value: string) => void;
    imageModels: PublicLogicalModel[];
    imageModelId: string;
    onImageModelChange: (value: string) => void;
    imageOptions: FilmProductionImageOptions;
    imageOptionsLabel: string;
    canQuote: boolean;
    quote: FilmProductionImageQuote | null;
    quoting: boolean;
    onQuote: () => void;
    confirmSubmit: boolean;
    onConfirmSubmit: (value: boolean) => void;
    submitting: boolean;
    onSubmit: () => void;
}) {
    return (
        <section className="space-y-2">
            <SectionTitle icon={<ImagePlus className="size-3.5" />} title="镜头生图" />
            <div className="grid grid-cols-2 gap-2">
                <Select className="w-full" value={shotId || undefined} placeholder="选择镜头" onChange={onShotChange} options={shots.map((shot) => ({ value: shot.id, label: `${shot.position + 1}. ${shot.title}` }))} />
                <Select className="w-full" value={imageModelId || undefined} placeholder="图片模型" onChange={onImageModelChange} options={imageModels.map((model) => ({ value: model.id, label: model.name }))} />
            </div>
            <Button block icon={<Coins className="size-3.5" />} disabled={!canQuote} loading={quoting} onClick={onQuote}>
                生成报价（{imageOptionsLabel}）
            </Button>
            {!imageOptions.size ? <InlineNotice text="当前图片模型没有声明 9:16 能力，暂不能提交短剧竖屏报价" /> : null}
            {quote ? (
                <div className="space-y-2 rounded-lg p-2.5" style={{ background: "var(--library-surface)" }}>
                    <div className="flex items-center justify-between gap-2 text-xs">
                        <span>模型：{quote.model}</span>
                        <strong>{quote.cost.required ? `${formatCredits(quote.cost.amountMicrocredits)} 积分` : "免费"}</strong>
                    </div>
                    <div className="text-[var(--fs-tiny)]" style={{ color: "var(--foreground-muted)" }}>
                        报价有效至 {new Date(quote.expiresAt).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" })}
                    </div>
                    <Button block type="primary" icon={<Check className="size-3.5" />} onClick={() => onConfirmSubmit(true)}>
                        确认扣费并生成
                    </Button>
                </div>
            ) : null}
            <Modal title="确认生成" open={confirmSubmit} onCancel={() => onConfirmSubmit(false)} onOk={onSubmit} confirmLoading={submitting} okText="确认扣费" cancelText="返回" centered>
                {quote ? (
                    <p className="text-sm">
                        将按 {quote.model} 生成 {quote.count} 张图片，预计消耗 <strong>{quote.cost.required ? `${formatCredits(quote.cost.amountMicrocredits)} 积分` : "免费额度"}</strong>。
                    </p>
                ) : null}
            </Modal>
        </section>
    );
}

function BatchGenerationSection({
    shots,
    selectedShotIds,
    onToggleShot,
    onSelectAll,
    imageModels,
    imageModelId,
    onImageModelChange,
    imageOptions,
    imageOptionsLabel,
    canQuote,
    items,
    latestAttemptsByShot,
    summary,
    quoting,
    onQuote,
    onRetryFailed,
    confirmSubmit,
    onConfirmSubmit,
    submitting,
    onSubmit,
}: {
    shots: ProjectDetail["shots"];
    selectedShotIds: string[];
    onToggleShot: (shotId: string, checked: boolean) => void;
    onSelectAll: () => void;
    imageModels: PublicLogicalModel[];
    imageModelId: string;
    onImageModelChange: (value: string) => void;
    imageOptions: FilmProductionImageOptions;
    imageOptionsLabel: string;
    canQuote: boolean;
    items: FilmBatchQuoteItem[];
    latestAttemptsByShot: Map<string, FilmProductionAttemptView>;
    summary: ReturnType<typeof summarizeFilmBatch>;
    quoting: boolean;
    onQuote: () => void;
    onRetryFailed: () => void;
    confirmSubmit: boolean;
    onConfirmSubmit: (value: boolean) => void;
    submitting: boolean;
    onSubmit: () => void;
}) {
    const selected = new Set(selectedShotIds);
    const quoteableItems = items.filter((item) => selected.has(item.shotId) && item.quote && item.status === "quoted" && !isQuoteExpired(item.quote));
    const itemByShot = new Map(items.map((item) => [item.shotId, item]));

    return (
        <section className="space-y-2">
            <SectionTitle icon={<ListChecks className="size-3.5" />} title="批量镜头生图" />
            <div className="grid grid-cols-2 gap-2">
                <Select className="w-full" value={imageModelId || undefined} placeholder="图片模型" onChange={onImageModelChange} options={imageModels.map((model) => ({ value: model.id, label: model.name }))} />
                <div className="flex items-center justify-end rounded-md px-2 text-xs" style={{ background: "var(--library-surface)", color: "var(--foreground-muted)" }}>
                    {selectedShotIds.length}/12 个镜头
                </div>
            </div>
            <div className="flex items-center justify-between gap-2 text-xs">
                <span style={{ color: "var(--foreground-muted)" }}>一次报价，按镜头独立执行</span>
                <Button size="small" type="link" onClick={onSelectAll} disabled={selectedShotIds.length >= Math.min(12, shots.length)}>
                    全选
                </Button>
            </div>
            <div className="max-h-48 space-y-1 overflow-y-auto rounded-lg p-1" style={{ background: "var(--library-surface)" }}>
                {shots.slice(0, 12).map((shot) => {
                    const item = itemByShot.get(shot.id);
                    const latest = latestAttemptsByShot.get(shot.id);
                    const status = item ? batchItemStatusLabel(item.status) : latest ? latestAttemptStatusLabel(latest) : "待生成";
                    return (
                        <label key={shot.id} className="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-xs hover:bg-black/5 dark:hover:bg-white/8">
                            <input type="checkbox" checked={selected.has(shot.id)} onChange={(event) => onToggleShot(shot.id, event.target.checked)} />
                            <span className="min-w-0 flex-1 truncate">
                                {shot.position + 1}. {shot.title}
                            </span>
                            <span className="shrink-0" style={{ color: item?.status === "failed" ? "var(--danger)" : "var(--foreground-muted)" }}>
                                {status}
                            </span>
                        </label>
                    );
                })}
            </div>
            <Button block icon={<Coins className="size-3.5" />} disabled={!canQuote || !selectedShotIds.length} loading={quoting} onClick={onQuote}>
                {summary.quoted ? "重新生成报价" : "生成批量报价"}（{imageOptionsLabel}）
            </Button>
            {!imageOptions.size ? <InlineNotice text="当前图片模型没有声明 9:16 能力，暂不能提交短剧竖屏报价" /> : null}
            {items.length ? (
                <div className="space-y-2 rounded-lg p-2.5" style={{ background: "var(--library-surface)" }}>
                    <div className="flex items-center justify-between gap-2 text-xs">
                        <span>
                            {summary.submitted}/{summary.total} 已提交 · {summary.failed} 个失败
                        </span>
                        <strong>{summary.amountMicrocredits ? `${formatCredits(summary.amountMicrocredits)} 积分` : "免费"}</strong>
                    </div>
                    {summary.active ? <InlineNotice text={`${summary.active} 个镜头处理中，页面刷新后会从服务端恢复`} /> : null}
                    {summary.failed ? (
                        <div className="flex items-center justify-between gap-2 text-xs" style={{ color: "var(--danger)" }}>
                            <span>失败镜头不会影响已成功的镜头</span>
                            <Button size="small" danger type="link" onClick={onRetryFailed} disabled={quoting}>
                                重新报价失败项
                            </Button>
                        </div>
                    ) : null}
                    {quoteableItems.length ? (
                        <Button block type="primary" icon={<Check className="size-3.5" />} onClick={() => onConfirmSubmit(true)}>
                            确认扣费并提交 {quoteableItems.length} 个镜头
                        </Button>
                    ) : null}
                </div>
            ) : null}
            <Modal title="确认批量生成" open={confirmSubmit} onCancel={() => onConfirmSubmit(false)} onOk={onSubmit} confirmLoading={submitting} okText="确认扣费并提交" cancelText="返回" centered>
                <p className="text-sm">
                    将提交 {quoteableItems.length} 个独立镜头任务，预计消耗 <strong>{summary.amountMicrocredits ? `${formatCredits(summary.amountMicrocredits)} 积分` : "免费额度"}</strong>。每个镜头会单独记录 Task、Attempt 和费用。
                </p>
            </Modal>
        </section>
    );
}

function batchItemStatusLabel(status: FilmBatchQuoteItem["status"]) {
    switch (status) {
        case "quoting":
            return "报价中";
        case "quoted":
            return "已报价";
        case "submitting":
            return "提交中";
        case "submitted":
            return "已提交";
        case "failed":
            return "失败";
    }
}

function latestAttemptStatusLabel(item: FilmProductionAttemptView) {
    if (item.accepted) return "已接受";
    if (item.attempt.status === "succeeded") return "待验收";
    if (item.attempt.status === "failed" || item.attempt.status === "cancelled") return "失败";
    return "处理中";
}

function AttemptSection({
    attempts,
    selectedAttempt,
    selectedAttemptId,
    onSelect,
    qcNote,
    onQcNoteChange,
    qcPending,
    onQC,
}: {
    attempts: FilmProductionAttemptView[];
    selectedAttempt: FilmProductionAttemptView | null;
    selectedAttemptId: string;
    onSelect: (value: string) => void;
    qcNote: string;
    onQcNoteChange: (value: string) => void;
    qcPending: boolean;
    onQC: (input: { decision: "PASS" | "UNCERTAIN" | "FAIL"; action: "accept" | "hold" | "retry" }) => void;
}) {
    return (
        <section className="space-y-2">
            <SectionTitle icon={<FileCheck2 className="size-3.5" />} title="结果与 QC" />
            {attempts.length ? (
                <Select
                    className="w-full"
                    value={selectedAttemptId || undefined}
                    placeholder="选择 Attempt"
                    onChange={onSelect}
                    options={attempts.map((item) => ({ value: item.attempt.id, label: `镜头 ${item.attempt.shotId.slice(0, 8)} · #${item.attempt.number} · ${item.attempt.status}` }))}
                />
            ) : (
                <InlineNotice text="提交报价后，结果会在这里恢复" />
            )}
            {selectedAttempt ? (
                <div className="space-y-2 rounded-lg p-2.5" style={{ background: "var(--library-surface)" }}>
                    <div className="flex items-center justify-between gap-2 text-xs">
                        <Tag color={attemptTone(selectedAttempt.attempt.status)}>{selectedAttempt.attempt.status}</Tag>
                        <span style={{ color: "var(--foreground-muted)" }}>{selectedAttempt.task.stage}</span>
                    </div>
                    {selectedAttempt.result?.url || selectedAttempt.task.previewUrl ? (
                        <Image src={selectedAttempt.result?.url || selectedAttempt.task.previewUrl} alt="短剧镜头结果" preview={{ mask: "查看" }} className="max-h-48 w-full object-contain" />
                    ) : null}
                    {selectedAttempt.currentQc ? <ImageQCState qc={selectedAttempt.currentQc} /> : null}
                    {selectedAttempt.accepted ? (
                        <div className="flex items-center gap-1 text-xs text-green-600">
                            <Check className="size-3.5" />
                            已接受，可进入后续视频阶段
                        </div>
                    ) : selectedAttempt.attempt.status === "succeeded" ? (
                        <>
                            <Input.TextArea value={qcNote} onChange={(event) => onQcNoteChange(event.target.value)} autoSize={{ minRows: 1, maxRows: 3 }} placeholder="验收备注（可选）" />
                            <div className="grid grid-cols-3 gap-2">
                                <Button size="small" type="primary" icon={<Check className="size-3" />} loading={qcPending} onClick={() => onQC({ decision: "PASS", action: "accept" })}>
                                    通过
                                </Button>
                                <Button size="small" icon={<AlertCircle className="size-3" />} loading={qcPending} onClick={() => onQC({ decision: "UNCERTAIN", action: "hold" })}>
                                    待复核
                                </Button>
                                <Button size="small" danger icon={<RotateCcw className="size-3" />} loading={qcPending} onClick={() => onQC({ decision: "FAIL", action: "retry" })}>
                                    不通过
                                </Button>
                            </div>
                        </>
                    ) : selectedAttempt.attempt.status === "failed" || selectedAttempt.attempt.status === "cancelled" ? (
                        <InlineNotice text="任务未成功，请重新报价后再提交" />
                    ) : (
                        <InlineNotice text="任务处理中，刷新后自动恢复" />
                    )}
                </div>
            ) : null}
        </section>
    );
}

function ImageQCState({ qc }: { qc: NonNullable<FilmProductionAttemptView["currentQc"]> }) {
    const failed = qc.decision === "FAIL";
    const pendingReview = qc.decision === "UNCERTAIN" || qc.decision === "NOT_ASSESSABLE";
    const label = failed ? "未通过" : pendingReview ? "待人工复核" : "已通过";
    return (
        <div className={`space-y-1 rounded-md px-2 py-1.5 text-xs ${failed ? "text-red-600" : pendingReview ? "text-amber-600" : "text-green-600"}`}>
            <div className="flex items-center gap-1.5">
                {failed || pendingReview ? <AlertCircle className="size-3.5 shrink-0" /> : <Check className="size-3.5 shrink-0" />}
                <span>
                    {label} · {qc.source === "system" ? "系统检查" : "人工检查"}
                </span>
            </div>
            {qc.issueCodes.length ? <div className="pl-5 text-[11px] opacity-80">{qc.issueCodes.join(" · ")}</div> : null}
            {qc.note ? <div className="pl-5 text-[11px] opacity-80">{qc.note}</div> : null}
        </div>
    );
}

function SectionTitle({ icon, title }: { icon: React.ReactNode; title: string }) {
    return (
        <div className="flex items-center gap-1.5 text-xs font-semibold">
            <span style={{ color: "var(--primary)" }}>{icon}</span>
            {title}
        </div>
    );
}

function InlineNotice({ text }: { text: string }) {
    return (
        <div className="rounded-lg px-2.5 py-2 text-xs" style={{ background: "var(--library-surface)", color: "var(--foreground-muted)" }}>
            {text}
        </div>
    );
}

function stableOperationKey(ref: { current: { signature: string; key: string } }, prefix: string, signature: string) {
    if (ref.current.signature !== signature || !ref.current.key) {
        ref.current = { signature, key: `${prefix}:${createClientId()}` };
    }
    return ref.current.key;
}

function resolveRequiredArtifacts(details: FilmAgentRunDetail[]): FilmArtifactChoice[] {
    return REQUIRED_ARTIFACTS.map((definition) => {
        const candidates = details.flatMap((detail) =>
            detail.artifacts
                .filter((item) => (definition.types as readonly string[]).includes(item.artifactType))
                .map((artifact) => ({
                    artifact,
                    revision: artifact.currentRevisionId ? detail.artifactRevisions.find((item) => item.id === artifact.currentRevisionId) : undefined,
                    revisions: detail.artifactRevisions.filter((item) => item.artifactId === artifact.id).sort((left, right) => left.version - right.version),
                    runId: detail.run.id,
                    runRevision: detail.run.revision,
                })),
        );
        const selected = candidates.sort(compareArtifactCandidates).at(-1);
        return { ...definition, revisions: [], ...selected };
    });
}

function compareArtifactCandidates(left: { artifact: FilmProductionArtifact; revision?: FilmProductionArtifactRevision }, right: { artifact: FilmProductionArtifact; revision?: FilmProductionArtifactRevision }) {
    const leftTime = Date.parse(left.revision?.createdAt || left.artifact.updatedAt);
    const rightTime = Date.parse(right.revision?.createdAt || right.artifact.updatedAt);
    if (leftTime !== rightTime) return leftTime - rightTime;
    const statusRank = (status?: string) => (status === "locked" ? 2 : status === "review" ? 1 : 0);
    return statusRank(left.revision?.status) - statusRank(right.revision?.status);
}

function resolveVideoPromptRevision(details: FilmAgentRunDetail[]) {
    const candidates = details.flatMap((detail) =>
        detail.artifacts
            .filter((artifact) => (VIDEO_PROMPT_ARTIFACT_TYPES as readonly string[]).includes(artifact.artifactType))
            .map((artifact) => ({
                artifact,
                revision: artifact.currentRevisionId ? detail.artifactRevisions.find((item) => item.id === artifact.currentRevisionId) : undefined,
            }))
            .filter((item): item is { artifact: FilmProductionArtifact; revision: FilmProductionArtifactRevision } => Boolean(item.revision?.status === "locked")),
    );
    const explicit = candidates.filter((item) => item.artifact.artifactType === "ai-video-prompts");
    return (explicit.length ? explicit : candidates).sort(compareArtifactCandidates).at(-1)?.revision;
}

function supportsImageToVideo(model: PublicLogicalModel) {
    const operations = model.capabilitySpec.operations || [];
    const imageInput = model.capabilitySpec.inputs?.image;
    return (operations.length === 0 || operations.includes("image_to_video")) && (!imageInput || (imageInput.min <= 1 && imageInput.max >= 1));
}

function isQuoteExpired(quote: FilmProductionImageQuote) {
    const expiresAt = Date.parse(quote.expiresAt);
    return !Number.isFinite(expiresAt) || expiresAt <= Date.now() || quote.status !== "pending";
}

function preferredRun(runs: FilmAgentRun[]) {
    return runs.slice().sort((left, right) => Date.parse(right.updatedAt) - Date.parse(left.updatedAt))[0];
}

function isActiveRun(detail?: FilmAgentRunDetail) {
    return detail ? ["planning", "ready", "running", "awaiting_human"].includes(detail.run.status) : false;
}

function hasActiveAttempt(attempts?: FilmProductionAttemptView[]) {
    return Boolean(attempts?.some((item) => ["queued", "running", "uncertain"].includes(item.attempt.status) || item.visualQcAttempts.some((attempt) => ["queued", "running", "uncertain"].includes(attempt.attempt.status))));
}

function attemptTone(status: string) {
    if (status === "succeeded") return "success";
    if (status === "failed" || status === "cancelled") return "error";
    if (status === "uncertain") return "warning";
    return "processing";
}

function readError(error: unknown) {
    return error instanceof Error ? error.message : "请求失败，请稍后重试";
}
