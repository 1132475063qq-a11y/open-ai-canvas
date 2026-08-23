import { useEffect, useMemo, useRef, useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { Alert, Button, Modal, Select, Tag } from "antd";
import { AlertCircle, Check, Coins, ScanSearch, Square } from "lucide-react";

import { formatCredits } from "@/constant/credits";
import { captureVideoQCFrames } from "@/lib/canvas/canvas-video-frame";
import { createClientId } from "@/lib/client-id";
import {
    createFilmVideoSequenceVisualQCQuote,
    submitFilmVideoSequenceVisualQCQuote,
    type FilmVideoSequenceReview,
    type FilmVideoSequenceView,
    type FilmVideoSequenceVisualQCAttemptView,
    type FilmVideoSequenceVisualQCQuote,
} from "@/services/api/film-production";
import type { PublicLogicalModel } from "@/services/api/logical-models";
import { uploadResourceFile } from "@/services/api/resources";
import { cancelGenerationTask } from "@/services/api/task-center";

type FilmVideoSequenceVisualQCSectionProps = {
    projectId: string;
    sequence: FilmVideoSequenceView;
    models: PublicLogicalModel[];
    onChanged: () => void;
};

type UploadedSlotSamples = {
    slotId: string;
    sampleFrames: Array<{ timeMs: number; resourceId: string }>;
};

type ModelVerdict = {
    key: string;
    label: string;
    decision: string;
    issueCodes: string[];
    observations: string[];
};

const SEQUENCE_SAMPLE_RATIOS = [0, 1] as const;

const dimensionLabels: Record<string, string> = {
    identity_continuity: "人物身份",
    costume_continuity: "服装",
    prop_continuity: "道具",
    scene_continuity: "场景",
    screen_direction_axis: "轴线与屏幕方向",
    action_continuity: "动作承接",
    lighting_color_continuity: "光线与色彩",
    motion_transition: "运动过渡",
    lip_sync_audio_continuity: "口型与声音",
};

export function FilmVideoSequenceVisualQCSection({ projectId, sequence, models, onChanged }: FilmVideoSequenceVisualQCSectionProps) {
    const scopeSignature = useMemo(
        () =>
            JSON.stringify([
                sequence.sequence.id,
                sequence.sequence.revision,
                sequence.slots.map((item) => [item.slot.id, item.slot.revision, item.slot.currentAttemptId, item.slot.resultId]),
            ]),
        [sequence],
    );
    const sources = useMemo(
        () =>
            sequence.slots.flatMap((item) => {
                const attempt = item.attempts.find((candidate) => candidate.attempt.id === item.slot.currentAttemptId);
                return attempt?.accepted && attempt.result?.url ? [{ slotId: item.slot.id, attemptId: attempt.attempt.id, url: attempt.result.url }] : [];
            }),
        [sequence.slots],
    );
    const requiredImages = sources.length * SEQUENCE_SAMPLE_RATIOS.length;
    const eligibleModels = useMemo(() => models.filter((model) => maxModelImageInputs(model) >= requiredImages), [models, requiredImages]);
    const [modelId, setModelId] = useState("");
    const [quote, setQuote] = useState<FilmVideoSequenceVisualQCQuote | null>(null);
    const [confirmSubmit, setConfirmSubmit] = useState(false);
    const [uploadedSlots, setUploadedSlots] = useState<UploadedSlotSamples[]>([]);
    const [preparationStage, setPreparationStage] = useState("");
    const quoteKeyRef = useRef({ signature: "", key: "" });
    const submitKeyRef = useRef({ signature: "", key: "" });
    const attempts = sequence.sequenceVisualQcAttempts || [];
    const latestAttempt = attempts.at(-1);
    const activeAttempt = attempts.find((item) => ["queued", "running", "uncertain"].includes(item.attempt.status));

    useEffect(() => {
        if (!modelId || !eligibleModels.some((model) => model.id === modelId)) setModelId(eligibleModels[0]?.id || "");
    }, [eligibleModels, modelId]);

    useEffect(() => {
        setQuote(null);
        setConfirmSubmit(false);
        setUploadedSlots([]);
        setPreparationStage("");
        quoteKeyRef.current = { signature: "", key: "" };
        submitKeyRef.current = { signature: "", key: "" };
    }, [scopeSignature]);

    useEffect(() => {
        setQuote(null);
        setConfirmSubmit(false);
        quoteKeyRef.current = { signature: "", key: "" };
        submitKeyRef.current = { signature: "", key: "" };
    }, [modelId]);

    useEffect(() => {
        if (!quote) return;
        const expiresAt = Date.parse(quote.expiresAt);
        if (!Number.isFinite(expiresAt) || expiresAt <= Date.now()) {
            setQuote(null);
            setConfirmSubmit(false);
            return;
        }
        const timer = window.setTimeout(() => {
            setQuote(null);
            setConfirmSubmit(false);
        }, expiresAt - Date.now() + 50);
        return () => window.clearTimeout(timer);
    }, [quote]);

    const quoteMutation = useMutation({
        mutationFn: async () => {
            let slots = uploadedSlots;
            if (!slots.length) {
                const next: UploadedSlotSamples[] = [];
                for (let sourceIndex = 0; sourceIndex < sources.length; sourceIndex += 1) {
                    const source = sources[sourceIndex];
                    setPreparationStage(`正在采样第 ${sourceIndex + 1}/${sources.length} 个视频`);
                    const frames = await captureVideoQCFrames(source.url, SEQUENCE_SAMPLE_RATIOS);
                    setPreparationStage(`正在上传第 ${sourceIndex + 1}/${sources.length} 组证据`);
                    const resources = await Promise.all(
                        frames.map((frame) =>
                            uploadResourceFile(frame.blob, "image", {
                                width: frame.width,
                                height: frame.height,
                                fileName: `film-sequence-qc-${source.attemptId}-${frame.timeMs}.jpg`,
                            }),
                        ),
                    );
                    next.push({ slotId: source.slotId, sampleFrames: frames.map((frame, index) => ({ timeMs: frame.timeMs, resourceId: resources[index].id })) });
                }
                slots = next;
                setUploadedSlots(next);
            }
            setPreparationStage("正在生成整组报价");
            const input = { sequenceId: sequence.sequence.id, logicalModelId: modelId, slots };
            return createFilmVideoSequenceVisualQCQuote(
                projectId,
                input,
                operationKey(quoteKeyRef, "film-video-sequence-visual-qc-quote", JSON.stringify([scopeSignature, modelId, slots])),
            );
        },
        onSuccess: (nextQuote) => {
            setQuote(nextQuote);
            setConfirmSubmit(false);
            setPreparationStage("");
        },
        onError: () => setPreparationStage(""),
    });

    const submitMutation = useMutation({
        mutationFn: () =>
            submitFilmVideoSequenceVisualQCQuote(
                projectId,
                quote!.id,
                { quoteFingerprint: quote!.quoteFingerprint },
                operationKey(submitKeyRef, "film-video-sequence-visual-qc-submit", quote!.id),
            ),
        onSuccess: () => {
            setQuote(null);
            setConfirmSubmit(false);
            submitKeyRef.current = { signature: "", key: "" };
            onChanged();
        },
    });

    const cancelMutation = useMutation({
        mutationFn: () => cancelGenerationTask(activeAttempt!.task.id),
        onSuccess: onChanged,
    });

    const canQuote = Boolean(sources.length === sequence.slots.length && modelId && !activeAttempt);
    const error = quoteMutation.error || submitMutation.error || cancelMutation.error;

    return (
        <section className="space-y-2 border-t pt-2" style={{ borderColor: "var(--border)" }}>
            <div className="flex items-center justify-between gap-2 text-xs font-semibold">
                <span className="flex min-w-0 items-center gap-1.5">
                    <ScanSearch className="size-3.5 shrink-0" style={{ color: "var(--primary)" }} />
                    <span className="truncate">整组模型连续性 QC</span>
                </span>
                <span className="shrink-0" style={{ color: "var(--foreground-muted)" }}>{sequence.slots.length} 镜头 · {requiredImages} 帧</span>
            </div>
            {eligibleModels.length ? (
                <div className="grid grid-cols-[minmax(0,1fr)_auto] gap-2">
                    <Select
                        className="min-w-0"
                        value={modelId || undefined}
                        placeholder="多模态文本模型"
                        onChange={setModelId}
                        options={eligibleModels.map((model) => ({ value: model.id, label: `${model.name} · 最多 ${maxModelImageInputs(model)} 图` }))}
                    />
                    <Button icon={<Coins className="size-3.5" />} disabled={!canQuote} loading={quoteMutation.isPending} onClick={() => quoteMutation.mutate()}>
                        报价
                    </Button>
                </div>
            ) : (
                <InlineState text={`当前序列需要模型支持至少 ${requiredImages} 张图片输入`} />
            )}
            {preparationStage ? <InlineState text={preparationStage} /> : null}
            {activeAttempt ? (
                <div className="flex items-center justify-between gap-2 text-xs">
                    <span className="min-w-0 truncate" style={{ color: "var(--foreground-muted)" }}>{sequenceQCStatusLabel(activeAttempt.attempt.status)} · {activeAttempt.task.stage}</span>
                    {["queued", "running"].includes(activeAttempt.attempt.status) ? (
                        <Button size="small" danger icon={<Square className="size-3" />} loading={cancelMutation.isPending} onClick={() => cancelMutation.mutate()}>
                            停止
                        </Button>
                    ) : null}
                </div>
            ) : null}
            {quote ? (
                <div className="space-y-2 rounded-lg p-2.5" style={{ background: "var(--library-surface)" }}>
                    <div className="flex items-center justify-between gap-2 text-xs">
                        <span className="truncate">{quote.model} · {quote.slotEvidence.length} 镜头 · 九维检查</span>
                        <strong className="shrink-0">{quote.cost.required ? `${formatCredits(quote.cost.amountMicrocredits)} 积分` : "免费"}</strong>
                    </div>
                    <div className="text-xs" style={{ color: "var(--foreground-muted)" }}>{quote.evidenceLimitation}</div>
                    <Button block type="primary" icon={<Check className="size-3.5" />} onClick={() => setConfirmSubmit(true)}>
                        确认运行整组连续性 QC
                    </Button>
                </div>
            ) : null}
            {latestAttempt ? <SequenceVisualQCAttemptState attempt={latestAttempt} /> : <InlineState text="尚未运行整组模型连续性 QC" />}
            {error ? <Alert type="error" showIcon message={readError(error)} /> : null}
            <Modal
                title="确认整组连续性 QC 费用"
                open={confirmSubmit}
                onCancel={() => setConfirmSubmit(false)}
                onOk={() => submitMutation.mutate()}
                confirmLoading={submitMutation.isPending}
                okText="确认扣费并运行"
                cancelText="返回"
                centered
            >
                {quote ? (
                    <p className="text-sm">
                        将使用 {quote.model} 检查 {quote.slotEvidence.length} 个已接受视频的 {quote.slotEvidence.reduce((total, slot) => total + slot.samples.length, 0)} 张首尾帧，预计消耗 <strong>{quote.cost.required ? `${formatCredits(quote.cost.amountMicrocredits)} 积分` : "免费额度"}</strong>。模型结论不会自动通过整组序列。
                    </p>
                ) : null}
            </Modal>
        </section>
    );
}

function SequenceVisualQCAttemptState({ attempt }: { attempt: FilmVideoSequenceVisualQCAttemptView }) {
    const dimensions = useMemo(() => dimensionVerdicts(attempt.report), [attempt.report]);
    const transitions = useMemo(() => transitionVerdicts(attempt.report), [attempt.report]);
    return (
        <div className="space-y-2 rounded-lg p-2.5" style={{ background: "var(--library-surface)" }}>
            <div className="flex items-center justify-between gap-2 text-xs">
                <Tag color={attemptTone(attempt.attempt.status)}>{sequenceQCStatusLabel(attempt.attempt.status)}</Tag>
                <span className="min-w-0 truncate" style={{ color: "var(--foreground-muted)" }}>{attempt.slotEvidence.length} 镜头 · {attempt.task.stage}</span>
            </div>
            {!attempt.valid ? <div className="text-xs" style={{ color: "var(--status-warning)" }}>序列、槽位或视频版本已变化，此结论已失效。</div> : null}
            {attempt.attempt.error ? (
                <div className="flex items-start gap-1.5 text-xs" style={{ color: "var(--status-error)" }}>
                    <AlertCircle className="mt-0.5 size-3.5 shrink-0" />
                    <span>{attempt.attempt.error}</span>
                </div>
            ) : null}
            {attempt.report ? (
                <>
                    <div className="flex items-center gap-1.5 text-xs" style={{ color: "var(--status-warning)" }}>
                        <AlertCircle className="size-3.5 shrink-0" />
                        <span>{modelVerdictLabel(attempt.report.decision)}，仍需人工整组裁决</span>
                    </div>
                    <details>
                        <summary className="cursor-pointer text-xs font-medium">查看九维与相邻镜头证据</summary>
                        <div className="mt-2 divide-y" style={{ borderColor: "var(--border-semantic)" }}>
                            {[...dimensions, ...transitions].map((verdict) => (
                                <div key={verdict.key} className="space-y-1 py-2 text-xs">
                                    <div className="flex items-center justify-between gap-2">
                                        <strong>{verdict.label}</strong>
                                        <Tag color={decisionTone(verdict.decision)}>{verdict.decision}</Tag>
                                    </div>
                                    {verdict.observations.map((observation, index) => <div key={`${verdict.key}-${index}`} style={{ color: "var(--foreground-muted)" }}>{observation}</div>)}
                                    {verdict.issueCodes.length ? <div style={{ color: "var(--status-error)" }}>{verdict.issueCodes.join(" · ")}</div> : null}
                                </div>
                            ))}
                        </div>
                    </details>
                </>
            ) : null}
        </div>
    );
}

function dimensionVerdicts(report?: FilmVideoSequenceReview): ModelVerdict[] {
    const raw = report?.evidence?.dimensions;
    if (!Array.isArray(raw)) return [];
    return raw.flatMap((value) => {
        const item = objectValue(value);
        const dimension = stringValue(item.dimension);
        const decision = stringValue(item.decision);
        if (!dimension || !decision) return [];
        return [{ key: `dimension:${dimension}`, label: dimensionLabels[dimension] || dimension, decision, issueCodes: stringArray(item.issueCodes), observations: stringArray(item.observations) }];
    });
}

function transitionVerdicts(report?: FilmVideoSequenceReview): ModelVerdict[] {
    const raw = report?.evidence?.transitions;
    if (!Array.isArray(raw)) return [];
    return raw.flatMap((value, index) => {
        const item = objectValue(value);
        const fromShotId = stringValue(item.fromShotId);
        const toShotId = stringValue(item.toShotId);
        const decision = stringValue(item.decision);
        if (!fromShotId || !toShotId || !decision) return [];
        return [{ key: `transition:${fromShotId}:${toShotId}`, label: `过渡 ${index + 1}`, decision, issueCodes: stringArray(item.issueCodes), observations: stringArray(item.observations) }];
    });
}

function maxModelImageInputs(model: PublicLogicalModel) {
    return Math.max(0, ...[model.capabilitySpec, ...model.capabilityProfiles].map((profile) => profile.inputs?.image?.max || 0));
}

function objectValue(value: unknown) {
    return value && typeof value === "object" ? value as Record<string, unknown> : {};
}

function stringValue(value: unknown) {
    return typeof value === "string" ? value : "";
}

function stringArray(value: unknown) {
    return Array.isArray(value) ? value.filter((item): item is string => typeof item === "string") : [];
}

function operationKey(ref: { current: { signature: string; key: string } }, prefix: string, signature: string) {
    if (ref.current.signature !== signature || !ref.current.key) ref.current = { signature, key: `${prefix}:${createClientId()}` };
    return ref.current.key;
}

function modelVerdictLabel(decision: string) {
    if (decision === "PASS") return "模型判断通过";
    if (decision === "FAIL") return "模型判断不通过";
    return "模型判断待复核";
}

function sequenceQCStatusLabel(status: string) {
    return { queued: "等待运行", running: "检查中", succeeded: "检查完成", failed: "检查失败", cancelled: "已取消", uncertain: "费用待核对" }[status] || status;
}

function attemptTone(status: string) {
    if (status === "succeeded") return "success";
    if (status === "failed" || status === "cancelled") return "error";
    if (status === "uncertain") return "warning";
    return "processing";
}

function decisionTone(decision: string) {
    if (decision === "PASS") return "success";
    if (decision === "FAIL") return "error";
    return "warning";
}

function InlineState({ text }: { text: string }) {
    return <div className="rounded-lg px-2.5 py-2 text-xs" style={{ background: "var(--library-surface)", color: "var(--foreground-muted)" }}>{text}</div>;
}

function readError(error: unknown) {
    return error instanceof Error ? error.message : "整组视频连续性 QC 请求失败";
}
