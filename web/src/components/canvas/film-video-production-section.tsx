import { useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, Button, Checkbox, Input, Modal, Select, Tag } from "antd";
import { AlertTriangle, Check, Coins, Film, ListChecks, ListVideo, RotateCcw, Square } from "lucide-react";

import { formatCredits } from "@/constant/credits";
import { FilmVideoVisualQCSection } from "@/components/canvas/film-video-visual-qc-section";
import { FilmVideoSequenceVisualQCSection } from "@/components/canvas/film-video-sequence-visual-qc-section";
import { createClientId } from "@/lib/client-id";
import type { FilmProductionArtifactRevision } from "@/services/api/film-agent-runtime";
import { createFilmVideoHumanQC, createFilmVideoQuote, createFilmVideoSequence, createFilmVideoSequenceReview, listFilmVideoSequences, submitFilmVideoQuote, type FilmProductionAttemptView, type FilmVideoQuote, type FilmVideoSequenceView } from "@/services/api/film-production";
import { canImportFilmVideoSequence } from "@/lib/timeline/film-video-sequence";
import { cancelGenerationTask } from "@/services/api/task-center";
import type { PublicLogicalModel } from "@/services/api/logical-models";
import type { ProjectDetail } from "@/services/api/projects";

type FilmVideoProductionSectionProps = {
    projectId: string;
    rootRunId: string;
    shots: ProjectDetail["shots"];
    imageAttempts: FilmProductionAttemptView[];
    videoModels: PublicLogicalModel[];
    visualQCModels: PublicLogicalModel[];
    promptRevision?: FilmProductionArtifactRevision;
    onProductionChanged: () => void;
    onImportSequence?: (sequence: FilmVideoSequenceView) => void;
};

const CONTINUITY_DIMENSIONS = [
    ["character_state", "人物"],
    ["costume_state", "服装"],
    ["prop_state", "道具"],
    ["scene_state", "场景"],
    ["knowledge_state", "信息状态"],
    ["audio_state", "声音"],
    ["screen_direction", "屏幕方向"],
    ["action_axis", "动作轴"],
    ["ui_state", "画面文字/UI"],
] as const;

function defaultContinuityChecks() {
    return Object.fromEntries(CONTINUITY_DIMENSIONS.map(([key]) => [key, true])) as Record<(typeof CONTINUITY_DIMENSIONS)[number][0], boolean>;
}

export function FilmVideoProductionSection({ projectId, rootRunId, shots, imageAttempts, videoModels, visualQCModels, promptRevision, onProductionChanged, onImportSequence }: FilmVideoProductionSectionProps) {
    const queryClient = useQueryClient();
    const [selectedImageAttemptIds, setSelectedImageAttemptIds] = useState<string[]>([]);
    const [sequenceId, setSequenceId] = useState("");
    const [slotId, setSlotId] = useState("");
    const [videoModelId, setVideoModelId] = useState("");
    const [quote, setQuote] = useState<FilmVideoQuote | null>(null);
    const [confirmSubmit, setConfirmSubmit] = useState(false);
    const [qcNote, setQcNote] = useState("");
    const [sequenceReviewNote, setSequenceReviewNote] = useState("");
    const [continuityChecks, setContinuityChecks] = useState(defaultContinuityChecks);
    const sequenceKeyRef = useRef({ signature: "", key: "" });
    const quoteKeyRef = useRef({ signature: "", key: "" });
    const submitKeyRef = useRef({ signature: "", key: "" });
    const qcKeyRef = useRef({ signature: "", key: "" });
    const sequenceReviewKeyRef = useRef({ signature: "", key: "" });

    const acceptedImages = useMemo(
        () =>
            imageAttempts
                .filter((item) => item.accepted && item.result?.url)
                .slice()
                .sort((left, right) => right.attempt.number - left.attempt.number),
        [imageAttempts],
    );
    const acceptedById = useMemo(() => new Map(acceptedImages.map((item) => [item.attempt.id, item])), [acceptedImages]);
    const shotById = useMemo(() => new Map(shots.map((shot) => [shot.id, shot])), [shots]);
    const sequencesQuery = useQuery({
        queryKey: ["film-video-sequences", projectId, rootRunId],
        queryFn: () => listFilmVideoSequences(projectId, { rootRunId, limit: 50 }),
        enabled: Boolean(rootRunId),
        refetchInterval: (query) => (hasActiveVideoSequence(query.state.data?.sequences) ? 5_000 : false),
    });
    const sequences = sequencesQuery.data?.sequences || [];
    const selectedSequence = sequences.find((item) => item.sequence.id === sequenceId) || null;
    const selectedSlot = selectedSequence?.slots.find((item) => item.slot.id === slotId) || null;
    const latestAttempt = selectedSlot?.attempts[0] || null;
    const selectedModel = videoModels.find((model) => model.id === videoModelId);
    const resolution = resolveFilmVideoResolution(selectedModel);
    const canCreateSequence = Boolean(rootRunId && promptRevision?.status === "locked" && selectedImageAttemptIds.length);
    const sequenceRepairAllowed = Boolean(selectedSequence?.sequenceReview?.valid && selectedSequence.sequenceReview.decision === "FAIL" && selectedSequence.sequenceReview.action === "retry" && latestAttempt?.accepted);
    const canQuote = Boolean(selectedSlot && videoModelId && (!latestAttempt || latestAttempt.retryAllowed || sequenceRepairAllowed));
    const canReview = latestAttempt?.attempt.status === "succeeded" && !latestAttempt.accepted;
    const systemTechnicalFailure = latestAttempt?.qcReports.some((report) => report.source === "system" && report.assessmentKind === "technical_media_qc" && report.decision === "FAIL");
    const allVideoSlotsAccepted = Boolean(selectedSequence?.slots.length && selectedSequence.slots.every((item) => item.slot.status === "accepted" && item.attempts.some((attempt) => attempt.accepted && attempt.result?.url)));
    const canImportSequence = Boolean(selectedSequence && canImportFilmVideoSequence(selectedSequence));
    const selectedRework = selectedSequence?.reworkEvents.find((event) => event.slotId === selectedSlot?.slot.id && event.attemptId === latestAttempt?.attempt.id && event.status === "open");

    const invalidateVideo = () => {
        void queryClient.invalidateQueries({ queryKey: ["film-video-sequences", projectId] });
        onProductionChanged();
    };
    const clearQuote = () => {
        setQuote(null);
        setConfirmSubmit(false);
        quoteKeyRef.current = { signature: "", key: "" };
        submitKeyRef.current = { signature: "", key: "" };
    };

    useEffect(() => {
        setSequenceId("");
        setSlotId("");
        setSelectedImageAttemptIds([]);
        setSequenceReviewNote("");
        setContinuityChecks(defaultContinuityChecks());
        clearQuote();
        sequenceKeyRef.current = { signature: "", key: "" };
        sequenceReviewKeyRef.current = { signature: "", key: "" };
    }, [rootRunId]);
    useEffect(() => {
        setSelectedImageAttemptIds((current) => {
            const valid = current.filter((id) => acceptedById.has(id));
            if (valid.length) return valid.length === current.length ? current : valid;
            if (!acceptedImages.length) return current.length ? [] : current;
            const latestByShot = new Map<string, string>();
            for (const item of acceptedImages) {
                if (!latestByShot.has(item.attempt.shotId)) latestByShot.set(item.attempt.shotId, item.attempt.id);
            }
            return [...latestByShot.values()];
        });
    }, [acceptedById, acceptedImages]);
    useEffect(() => {
        if (!sequenceId && sequences.length) setSequenceId(sequences[0].sequence.id);
    }, [sequenceId, sequences]);
    useEffect(() => {
        if (selectedSequence && !selectedSequence.slots.some((item) => item.slot.id === slotId)) {
            setSlotId(selectedSequence.slots[0]?.slot.id || "");
            clearQuote();
        }
    }, [selectedSequence, slotId]);
    useEffect(() => {
        setSequenceReviewNote("");
        setContinuityChecks(defaultContinuityChecks());
        sequenceReviewKeyRef.current = { signature: "", key: "" };
    }, [sequenceId]);
    useEffect(() => {
        if (!videoModelId && videoModels.length) setVideoModelId(videoModels[0].id);
    }, [videoModelId, videoModels]);
    useEffect(() => {
        if (!quote) return;
        const expiresAt = Date.parse(quote.expiresAt);
        const timeout = expiresAt - Date.now() + 50;
        if (!Number.isFinite(expiresAt) || timeout <= 0) {
            clearQuote();
            return;
        }
        const timer = window.setTimeout(clearQuote, timeout);
        return () => window.clearTimeout(timer);
    }, [quote]);

    const createSequenceMutation = useMutation({
        mutationFn: () => {
            const selected = selectedImageAttemptIds
                .map((id) => acceptedById.get(id))
                .filter((item): item is FilmProductionAttemptView => Boolean(item))
                .sort((left, right) => (shotById.get(left.attempt.shotId)?.position || 0) - (shotById.get(right.attempt.shotId)?.position || 0));
            const slots = selected.map((item) => ({
                shotId: item.attempt.shotId,
                sourceImageAttemptId: item.attempt.id,
                durationMs: normalizeShotDuration(shotById.get(item.attempt.shotId)?.durationMs),
            }));
            const targetDurationMs = slots.reduce((total, slot) => total + slot.durationMs, 0);
            const input = {
                rootRunId,
                promptArtifactRevisionId: promptRevision!.id,
                title: `9:16 短剧视频 · ${slots.length} 镜头`,
                aspectRatio: "9:16",
                targetDurationMs,
                slots,
            };
            return createFilmVideoSequence(projectId, input, stableOperationKey(sequenceKeyRef, "film-video-sequence", JSON.stringify(input)));
        },
        onSuccess: (result) => {
            setSequenceId(result.sequence.id);
            setSlotId(result.slots[0]?.slot.id || "");
            sequenceKeyRef.current = { signature: "", key: "" };
            invalidateVideo();
        },
    });
    const quoteMutation = useMutation({
        mutationFn: () => {
            const input = {
                sequenceId: selectedSequence!.sequence.id,
                slotId: selectedSlot!.slot.id,
                logicalModelId: videoModelId,
                ...(latestAttempt?.retryAllowed || sequenceRepairAllowed ? { retryOfAttemptId: latestAttempt!.attempt.id } : {}),
                options: { ...(resolution ? { resolution } : {}) },
            };
            return createFilmVideoQuote(projectId, input, stableOperationKey(quoteKeyRef, "film-video-quote", JSON.stringify(input)));
        },
        onSuccess: (result) => setQuote(result),
    });
    const submitMutation = useMutation({
        mutationFn: () => submitFilmVideoQuote(projectId, quote!.id, { quoteFingerprint: quote!.quoteFingerprint }, stableOperationKey(submitKeyRef, "film-video-submit", quote!.id)),
        onSuccess: () => {
            clearQuote();
            invalidateVideo();
        },
    });
    const qcMutation = useMutation({
        mutationFn: (input: { decision: "PASS" | "UNCERTAIN" | "FAIL"; action: "accept" | "hold" | "retry" }) =>
            createFilmVideoHumanQC(projectId, latestAttempt!.attempt.id, { decision: input.decision, action: input.action, note: qcNote }, stableOperationKey(qcKeyRef, "film-video-qc", JSON.stringify([latestAttempt!.attempt.id, input, qcNote]))),
        onSuccess: () => {
            setQcNote("");
            qcKeyRef.current = { signature: "", key: "" };
            invalidateVideo();
        },
    });
    const sequenceReviewMutation = useMutation({
        mutationFn: (decision: "PASS" | "UNCERTAIN" | "FAIL") => {
            const action = decision === "PASS" ? "accept" : decision === "UNCERTAIN" ? "hold" : "retry";
            const dimensions = Object.fromEntries(Object.entries(continuityChecks).map(([key, passed]) => [key, passed ? "PASS" : decision === "UNCERTAIN" ? "UNCERTAIN" : "FAIL"]));
            const shots = Object.fromEntries((selectedSequence?.slots || []).map((item) => [item.slot.shotId, "PASS"]));
            const failedDimensions = Object.entries(continuityChecks).filter(([, passed]) => !passed).map(([key]) => `CONTINUITY_${key.toUpperCase()}`);
            const issueCodes = decision === "PASS" ? [] : failedDimensions.length ? failedDimensions : [decision === "UNCERTAIN" ? "CONTINUITY_REVIEW_PENDING" : "SEQUENCE_REVIEW_FAILED"];
            const input = { decision, action, issueCodes, evidence: { dimensions, shots }, note: sequenceReviewNote } as const;
            return createFilmVideoSequenceReview(projectId, selectedSequence!.sequence.id, input, stableOperationKey(sequenceReviewKeyRef, "film-video-sequence-review", JSON.stringify([selectedSequence!.sequence.id, input])));
        },
        onSuccess: () => {
            setSequenceReviewNote("");
            sequenceReviewKeyRef.current = { signature: "", key: "" };
            invalidateVideo();
        },
    });
    const cancelMutation = useMutation({
        mutationFn: () => cancelGenerationTask(latestAttempt!.task.id),
        onSuccess: invalidateVideo,
    });

    const mutationError = createSequenceMutation.error || quoteMutation.error || submitMutation.error || qcMutation.error || sequenceReviewMutation.error || cancelMutation.error;

    return (
        <section className="space-y-2">
            <div className="flex items-center gap-1.5 text-xs font-semibold">
                <Film className="size-3.5" style={{ color: "var(--primary)" }} />
                图片转视频
            </div>
            <Select
                mode="multiple"
                className="w-full"
                value={selectedImageAttemptIds}
                maxTagCount="responsive"
                placeholder="选择已接受图片"
                onChange={(ids) => setSelectedImageAttemptIds(uniqueAttemptsByShot(ids, acceptedById))}
                options={acceptedImages.map((item) => ({
                    value: item.attempt.id,
                    label: `${shotLabel(shots, item.attempt.shotId)} · 图片 #${item.attempt.number}`,
                }))}
            />
            <div className="grid grid-cols-2 gap-2">
                <div className="rounded-md border px-2 py-1.5 text-xs" style={{ borderColor: "var(--border)", color: "var(--foreground-muted)" }}>
                    9:16 · {selectedImageAttemptIds.reduce((total, id) => total + normalizeShotDuration(shotById.get(acceptedById.get(id)?.attempt.shotId || "")?.durationMs), 0) / 1_000}s
                </div>
                <Button type="primary" icon={<Film className="size-3.5" />} disabled={!canCreateSequence} loading={createSequenceMutation.isPending} onClick={() => createSequenceMutation.mutate()}>
                    创建序列
                </Button>
            </div>
            {!acceptedImages.length ? <InlineNotice text="通过图片人工验收后，可创建视频序列" /> : promptRevision?.status !== "locked" ? <InlineNotice text="缺少已锁定的逐镜视频 Prompt" /> : null}
            {sequences.length ? (
                <div className="space-y-2">
                    <div className="grid grid-cols-2 gap-2">
                        <Select
                            className="w-full"
                            value={sequenceId || undefined}
                            placeholder="视频序列"
                            onChange={(value) => {
                                setSequenceId(value);
                                setSlotId("");
                                clearQuote();
                            }}
                            options={sequences.map((item) => ({ value: item.sequence.id, label: `${item.sequence.title} · ${item.sequence.status}` }))}
                        />
                        <Select
                            className="w-full"
                            value={slotId || undefined}
                            placeholder="镜头槽位"
                            onChange={(value) => {
                                setSlotId(value);
                                clearQuote();
                            }}
                            options={(selectedSequence?.slots || []).map((item) => ({ value: item.slot.id, label: `${item.slot.position + 1}. ${shotLabel(shots, item.slot.shotId)} · ${item.slot.status}` }))}
                        />
                    </div>
                    {selectedSequence?.continuity ? <ContinuityState sequence={selectedSequence} shots={shots} /> : null}
                    {selectedSequence?.continuity && allVideoSlotsAccepted ? (
                        <FilmVideoSequenceVisualQCSection projectId={projectId} sequence={selectedSequence} models={visualQCModels} onChanged={invalidateVideo} />
                    ) : null}
                    {selectedSequence?.continuity ? (
                        <SequenceReviewPanel
                            sequence={selectedSequence}
                            allSlotsAccepted={allVideoSlotsAccepted}
                            checks={continuityChecks}
                            note={sequenceReviewNote}
                            loading={sequenceReviewMutation.isPending}
                            onCheck={(key, checked) => setContinuityChecks((current) => ({ ...current, [key]: checked }))}
                            onNote={setSequenceReviewNote}
                            onReview={(decision) => sequenceReviewMutation.mutate(decision)}
                        />
                    ) : null}
                    {canImportSequence && onImportSequence ? (
                        <Button block icon={<ListVideo className="size-3.5" />} onClick={() => onImportSequence(selectedSequence!)}>
                            加入时间线并打开
                        </Button>
                    ) : null}
                </div>
            ) : null}
            {selectedSlot ? (
                <div className="space-y-2 rounded-lg p-2.5" style={{ background: "var(--library-surface)" }}>
                    <div className="flex items-center justify-between gap-2 text-xs">
                        <Tag color={videoStatusTone(selectedSlot.slot.status)}>{selectedSlot.slot.status}</Tag>
                        <span style={{ color: "var(--foreground-muted)" }}>{selectedSlot.slot.durationMs / 1_000}s · 9:16</span>
                    </div>
                    <Select
                        className="w-full"
                        value={videoModelId || undefined}
                        placeholder="视频模型"
                        onChange={(value) => {
                            setVideoModelId(value);
                            clearQuote();
                        }}
                        options={videoModels.map((model) => ({ value: model.id, label: model.name }))}
                    />
                    {latestAttempt?.result?.url ? <video className="mx-auto max-h-72 w-full bg-black object-contain" style={{ aspectRatio: "9 / 16" }} controls preload="metadata" src={latestAttempt.result.url} /> : null}
                    {latestAttempt ? <AttemptState attempt={latestAttempt} onCancel={() => cancelMutation.mutate()} cancelling={cancelMutation.isPending} /> : null}
                    {latestAttempt?.currentQc ? <VideoQCState qc={latestAttempt.currentQc} /> : null}
                    {latestAttempt?.attempt.status === "succeeded" && latestAttempt.result?.url ? (
                        <FilmVideoVisualQCSection projectId={projectId} attempt={latestAttempt} models={visualQCModels} onChanged={invalidateVideo} />
                    ) : null}
                    {selectedRework ? (
                        <div className="flex items-start gap-1.5 text-xs text-red-600">
                            <AlertTriangle className="mt-0.5 size-3.5 shrink-0" />
                            <span>最小返工范围：当前镜头槽位（{selectedRework.reasonCode}）</span>
                        </div>
                    ) : null}
                    {canReview ? (
                        <>
                            <Input.TextArea value={qcNote} onChange={(event) => setQcNote(event.target.value)} autoSize={{ minRows: 1, maxRows: 3 }} placeholder="视频验收备注（可选）" />
                            <div className="grid grid-cols-3 gap-2">
                                <Button size="small" type="primary" icon={<Check className="size-3" />} disabled={systemTechnicalFailure} loading={qcMutation.isPending} onClick={() => qcMutation.mutate({ decision: "PASS", action: "accept" })}>
                                    接受视频
                                </Button>
                                <Button size="small" icon={<AlertTriangle className="size-3" />} loading={qcMutation.isPending} onClick={() => qcMutation.mutate({ decision: "UNCERTAIN", action: "hold" })}>
                                    待复核
                                </Button>
                                <Button size="small" danger icon={<RotateCcw className="size-3" />} loading={qcMutation.isPending} onClick={() => qcMutation.mutate({ decision: "FAIL", action: "retry" })}>
                                    请求重试
                                </Button>
                            </div>
                        </>
                    ) : null}
                    {canQuote ? (
                        <Button block icon={<Coins className="size-3.5" />} loading={quoteMutation.isPending} onClick={() => quoteMutation.mutate()}>
                            {latestAttempt?.retryAllowed || sequenceRepairAllowed ? "重新报价" : "生成视频报价"}（{resolution || "自动清晰度"}）
                        </Button>
                    ) : null}
                </div>
            ) : null}
            {quote ? (
                <div className="space-y-2 rounded-lg p-2.5" style={{ background: "var(--library-surface)" }}>
                    <div className="flex items-center justify-between gap-2 text-xs">
                        <span>
                            {quote.model} · {quote.durationMs / 1_000}s
                        </span>
                        <strong>{quote.cost.required ? `${formatCredits(quote.cost.amountMicrocredits)} 积分` : "免费"}</strong>
                    </div>
                    <Button block type="primary" icon={<Check className="size-3.5" />} onClick={() => setConfirmSubmit(true)}>
                        确认费用并生成
                    </Button>
                </div>
            ) : null}
            {sequencesQuery.isError || mutationError ? <Alert type="error" showIcon message={readError(sequencesQuery.error || mutationError)} /> : null}
            <Modal title="确认视频生成" open={confirmSubmit} onCancel={() => setConfirmSubmit(false)} onOk={() => submitMutation.mutate()} confirmLoading={submitMutation.isPending} okText="确认扣费" cancelText="返回" centered>
                {quote ? (
                    <p className="text-sm">
                        将生成一个 {quote.aspectRatio}、{quote.durationMs / 1_000} 秒视频，预计消耗 <strong>{quote.cost.required ? `${formatCredits(quote.cost.amountMicrocredits)} 积分` : "免费额度"}</strong>。
                    </p>
                ) : null}
            </Modal>
        </section>
    );
}

function ContinuityState({ sequence, shots }: { sequence: FilmVideoSequenceView; shots: ProjectDetail["shots"] }) {
    const continuity = sequence.continuity!;
    return (
        <details className="text-xs">
            <summary className="cursor-pointer select-none" style={{ color: "var(--foreground-muted)" }}>
                Continuity Ledger · {continuity.shots.length} 镜头 · {continuity.ledger.issueCount} 项待真实媒体复核
            </summary>
            <div className="mt-2 space-y-1.5 pl-2">
                {continuity.issues.slice(0, 4).map((issue) => (
                    <div key={issue.id} className="flex items-start gap-1.5">
                        <AlertTriangle className="mt-0.5 size-3 shrink-0 text-amber-500" />
                        <span>{shotLabel(shots, issue.shotId || "")}：{issue.message}</span>
                    </div>
                ))}
                {continuity.issues.length > 4 ? <span style={{ color: "var(--foreground-muted)" }}>另有 {continuity.issues.length - 4} 项逐镜记录</span> : null}
            </div>
        </details>
    );
}

function SequenceReviewPanel({
    sequence,
    allSlotsAccepted,
    checks,
    note,
    loading,
    onCheck,
    onNote,
    onReview,
}: {
    sequence: FilmVideoSequenceView;
    allSlotsAccepted: boolean;
    checks: Record<string, boolean>;
    note: string;
    loading: boolean;
    onCheck: (key: string, checked: boolean) => void;
    onNote: (value: string) => void;
    onReview: (decision: "PASS" | "UNCERTAIN" | "FAIL") => void;
}) {
    const review = sequence.sequenceReview;
    const allDimensionsPassed = CONTINUITY_DIMENSIONS.every(([key]) => checks[key]);
    const reviewLabel = review?.valid ? review.decision === "PASS" ? "整组已通过" : review.decision === "FAIL" ? "整组未通过" : "整组待复核" : review ? "整组裁决已过期" : "尚未整组裁决";
    const reviewTone = review?.valid ? review.decision === "PASS" ? "success" : review.decision === "FAIL" ? "error" : "warning" : "default";
    return (
        <div className="space-y-2 border-t pt-2" style={{ borderColor: "var(--border)" }}>
            <div className="flex items-center justify-between gap-2 text-xs font-semibold">
                <span className="flex items-center gap-1.5"><ListChecks className="size-3.5" style={{ color: "var(--primary)" }} />跨镜头连续性</span>
                <Tag color={reviewTone}>{reviewLabel}</Tag>
            </div>
            <div className="grid grid-cols-2 gap-x-2 gap-y-1">
                {CONTINUITY_DIMENSIONS.map(([key, label]) => (
                    <Checkbox key={key} checked={Boolean(checks[key])} disabled={!allSlotsAccepted || loading} onChange={(event) => onCheck(key, event.target.checked)}>
                        <span className="text-xs">{label}</span>
                    </Checkbox>
                ))}
            </div>
            {review?.valid && review.note ? <div className="text-xs" style={{ color: "var(--foreground-muted)" }}>{review.note}</div> : null}
            {review && !review.valid ? <div className="text-xs text-amber-600">槽位或版本已变化，需要重新进行整组裁决。</div> : null}
            {!allSlotsAccepted ? <div className="text-xs" style={{ color: "var(--foreground-muted)" }}>全部视频槽位接受后，整组裁决才可提交。</div> : null}
            {allSlotsAccepted ? (
                <>
                    <Input.TextArea value={note} onChange={(event) => onNote(event.target.value)} autoSize={{ minRows: 1, maxRows: 3 }} placeholder="整组验收备注" disabled={loading} />
                    <div className="grid grid-cols-3 gap-2">
                        <Button size="small" type="primary" icon={<Check className="size-3" />} disabled={!allDimensionsPassed} loading={loading} onClick={() => onReview("PASS")}>整组通过</Button>
                        <Button size="small" icon={<AlertTriangle className="size-3" />} loading={loading} onClick={() => onReview("UNCERTAIN")}>待复核</Button>
                        <Button size="small" danger icon={<RotateCcw className="size-3" />} loading={loading} onClick={() => onReview("FAIL")}>整组重做</Button>
                    </div>
                </>
            ) : null}
        </div>
    );
}

function VideoQCState({ qc }: { qc: NonNullable<FilmVideoSequenceView["slots"][number]["attempts"][number]["currentQc"]> }) {
    const failed = qc.decision === "FAIL";
    const pendingReview = qc.decision === "UNCERTAIN" || qc.decision === "NOT_ASSESSABLE";
    const label = failed ? "未通过" : pendingReview ? "待人工复核" : "已通过";
    return (
        <div className={`space-y-1 rounded-md px-2 py-1.5 text-xs ${failed ? "text-red-600" : pendingReview ? "text-amber-600" : "text-green-600"}`}>
            <div className="flex items-center gap-1.5">
                {failed || pendingReview ? <AlertTriangle className="size-3.5 shrink-0" /> : <Check className="size-3.5 shrink-0" />}
                <span>{label} · {qc.source === "system" ? "系统检查" : "人工检查"}</span>
            </div>
            {qc.issueCodes.length ? <div className="pl-5 text-[11px] opacity-80">{qc.issueCodes.join(" · ")}</div> : null}
            {qc.note ? <div className="pl-5 text-[11px] opacity-80">{qc.note}</div> : null}
        </div>
    );
}

function AttemptState({ attempt, onCancel, cancelling }: { attempt: FilmVideoSequenceView["slots"][number]["attempts"][number]; onCancel: () => void; cancelling: boolean }) {
    if (attempt.accepted) {
        return (
            <div className="flex items-center gap-1 text-xs text-green-600">
                <Check className="size-3.5" />
                已接受
            </div>
        );
    }
    if (["queued", "running"].includes(attempt.attempt.status)) {
        return (
            <div className="flex items-center justify-between gap-2 text-xs">
                <span style={{ color: "var(--foreground-muted)" }}>{attempt.task.stage}</span>
                <Button size="small" danger icon={<Square className="size-3" />} loading={cancelling} onClick={onCancel}>
                    停止
                </Button>
            </div>
        );
    }
    if (attempt.retryAllowed) return <InlineNotice text="该镜头可重新报价；不会静默扣费重试" />;
    return null;
}

function InlineNotice({ text }: { text: string }) {
    return (
        <div className="rounded-lg px-2.5 py-2 text-xs" style={{ background: "var(--library-surface)", color: "var(--foreground-muted)" }}>
            {text}
        </div>
    );
}

function uniqueAttemptsByShot(ids: string[], attempts: Map<string, FilmProductionAttemptView>) {
    const byShot = new Map<string, string>();
    for (const id of ids) {
        const shotId = attempts.get(id)?.attempt.shotId;
        if (shotId) byShot.set(shotId, id);
    }
    return [...byShot.values()];
}

function normalizeShotDuration(durationMs?: number) {
    const duration = Number.isFinite(durationMs) ? Number(durationMs) : 3_000;
    return Math.min(60_000, Math.max(1_000, Math.round(duration)));
}

function resolveFilmVideoResolution(model?: PublicLogicalModel) {
    const values = (model?.capabilitySpec.options?.vquality?.values || model?.capabilitySpec.options?.resolution?.values || [])
        .filter((value): value is string => typeof value === "string")
        .map((value) => value.trim())
        .filter(Boolean);
    const fallback = typeof model?.defaultOptions?.vquality === "string" ? model.defaultOptions.vquality.trim() : "";
    if (values.includes("*")) return fallback || undefined;
    for (const preferred of ["1080p", "1080", "720p", "720", "4k"]) {
        const match = values.find((value) => value.toLowerCase() === preferred);
        if (match) return match;
    }
    return values[0] || fallback || undefined;
}

function shotLabel(shots: ProjectDetail["shots"], shotId: string) {
    const shot = shots.find((item) => item.id === shotId);
    return shot ? `${shot.position + 1}. ${shot.title}` : shotId.slice(0, 8);
}

function stableOperationKey(ref: { current: { signature: string; key: string } }, prefix: string, signature: string) {
    if (ref.current.signature !== signature || !ref.current.key) ref.current = { signature, key: `${prefix}:${createClientId()}` };
    return ref.current.key;
}

function hasActiveVideoSequence(sequences?: FilmVideoSequenceView[]) {
    return Boolean(sequences?.some((sequence) =>
        sequence.sequenceVisualQcAttempts?.some((attempt) => ["queued", "running", "uncertain"].includes(attempt.attempt.status))
        || sequence.slots.some((slot) => slot.attempts.some((attempt) => ["queued", "running", "uncertain"].includes(attempt.attempt.status))),
    ));
}

function videoStatusTone(status: string) {
    if (status === "accepted" || status === "completed") return "success";
    if (status === "failed" || status === "cancelled") return "error";
    if (status === "uncertain" || status === "needs_review") return "warning";
    return "processing";
}

function readError(error: unknown) {
    return error instanceof Error ? error.message : "视频制作请求失败，请稍后重试";
}
