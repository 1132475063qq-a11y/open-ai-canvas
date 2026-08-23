import { useEffect, useMemo, useRef, useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { Alert, Button, Modal, Select, Tag } from "antd";
import { AlertCircle, Check, Coins, ScanSearch } from "lucide-react";

import { formatCredits } from "@/constant/credits";
import { supportsFilmVisualQCModel } from "@/lib/canvas/film-visual-qc";
import { createClientId } from "@/lib/client-id";
import { createFilmVisualQCQuote, submitFilmVisualQCQuote, type FilmProductionAttemptView, type FilmProductionQCReport, type FilmVisualQCAttemptView, type FilmVisualQCQuote } from "@/services/api/film-production";
import type { PublicLogicalModel } from "@/services/api/logical-models";

type FilmVisualQCSectionProps = {
    projectId: string;
    selectedAttempt: FilmProductionAttemptView | null;
    models: PublicLogicalModel[];
    referenceResourceIds: string[];
    onProductionChanged: () => void;
};

type VisualQCDimension = {
    dimension: string;
    decision: string;
    issueCodes: string[];
    observations: string[];
    rationale: string;
};

const dimensionLabels: Record<string, string> = {
    identity: "人物身份",
    anatomy: "人体结构",
    contact: "接触关系",
    acting: "表演状态",
    props: "道具",
    scene: "场景",
    composition: "构图",
    visual_continuity: "视觉连续性",
};

export function FilmVisualQCSection({ projectId, selectedAttempt, models, referenceResourceIds, onProductionChanged }: FilmVisualQCSectionProps) {
    const [modelId, setModelId] = useState("");
    const [quote, setQuote] = useState<FilmVisualQCQuote | null>(null);
    const [confirmSubmit, setConfirmSubmit] = useState(false);
    const quoteKeyRef = useRef({ signature: "", key: "" });
    const submitKeyRef = useRef({ signature: "", key: "" });
    const sourceAttemptId = selectedAttempt?.attempt.id || "";
    const attempts = selectedAttempt?.visualQcAttempts || [];
    const latestAttempt = attempts.at(-1);
    const activeAttempt = attempts.find((item) => ["queued", "running", "uncertain"].includes(item.attempt.status));

    useEffect(() => {
        if (!modelId || !models.some((model) => model.id === modelId)) setModelId(models[0]?.id || "");
    }, [modelId, models]);

    useEffect(() => {
        setQuote(null);
        setConfirmSubmit(false);
        quoteKeyRef.current = { signature: "", key: "" };
        submitKeyRef.current = { signature: "", key: "" };
    }, [modelId, sourceAttemptId]);

    useEffect(() => {
        if (!quote) return;
        const expiresAt = Date.parse(quote.expiresAt);
        if (!Number.isFinite(expiresAt) || expiresAt <= Date.now()) {
            setQuote(null);
            setConfirmSubmit(false);
            return;
        }
        const timer = window.setTimeout(
            () => {
                setQuote(null);
                setConfirmSubmit(false);
            },
            expiresAt - Date.now() + 50,
        );
        return () => window.clearTimeout(timer);
    }, [quote]);

    const quoteMutation = useMutation({
        mutationFn: () => createFilmVisualQCQuote(projectId, { sourceAttemptId, logicalModelId: modelId, referenceResourceIds }, operationKey(quoteKeyRef, "film-visual-qc-quote", JSON.stringify([sourceAttemptId, modelId, referenceResourceIds]))),
        onSuccess: (nextQuote) => {
            setQuote(nextQuote);
            setConfirmSubmit(false);
        },
    });

    const submitMutation = useMutation({
        mutationFn: () => submitFilmVisualQCQuote(projectId, quote!.id, { quoteFingerprint: quote!.quoteFingerprint }, operationKey(submitKeyRef, "film-visual-qc-submit", quote!.id)),
        onSuccess: () => {
            setQuote(null);
            setConfirmSubmit(false);
            submitKeyRef.current = { signature: "", key: "" };
            onProductionChanged();
        },
    });

    const canQuote = Boolean(selectedAttempt?.attempt.status === "succeeded" && selectedAttempt.result && modelId && !activeAttempt);
    const error = quoteMutation.error || submitMutation.error;

    return (
        <section className="space-y-2">
            <div className="flex items-center gap-1.5 text-xs font-semibold">
                <ScanSearch className="size-3.5" style={{ color: "var(--primary)" }} />
                模型视觉 QC
            </div>
            {models.length ? (
                <div className="grid grid-cols-[minmax(0,1fr)_auto] gap-2">
                    <Select className="min-w-0" value={modelId || undefined} placeholder="多模态文本模型" onChange={setModelId} options={models.map((model) => ({ value: model.id, label: model.name }))} />
                    <Button icon={<Coins className="size-3.5" />} disabled={!canQuote} loading={quoteMutation.isPending} onClick={() => quoteMutation.mutate()}>
                        报价
                    </Button>
                </div>
            ) : (
                <InlineState text="没有可用的图片输入文本模型" />
            )}
            {activeAttempt ? <InlineState text={`${visualQCStatusLabel(activeAttempt.attempt.status)} · ${activeAttempt.task.stage}`} /> : null}
            {quote ? (
                <div className="space-y-2 rounded-lg p-2.5" style={{ background: "var(--library-surface)" }}>
                    <div className="flex items-center justify-between gap-2 text-xs">
                        <span className="truncate">{quote.model} · 八维检查</span>
                        <strong className="shrink-0">{quote.cost.required ? `${formatCredits(quote.cost.amountMicrocredits)} 积分` : "免费"}</strong>
                    </div>
                    <Button block type="primary" icon={<Check className="size-3.5" />} onClick={() => setConfirmSubmit(true)}>
                        确认运行视觉 QC
                    </Button>
                </div>
            ) : null}
            {latestAttempt ? <VisualQCAttemptState attempt={latestAttempt} /> : selectedAttempt?.attempt.status === "succeeded" ? <InlineState text="尚未运行模型视觉 QC" /> : null}
            {error ? <Alert type="error" showIcon message={readError(error)} /> : null}
            <Modal title="确认视觉 QC 费用" open={confirmSubmit} onCancel={() => setConfirmSubmit(false)} onOk={() => submitMutation.mutate()} confirmLoading={submitMutation.isPending} okText="确认扣费并运行" cancelText="返回" centered>
                {quote ? (
                    <p className="text-sm">
                        将使用 {quote.model} 检查当前图片，预计消耗 <strong>{quote.cost.required ? `${formatCredits(quote.cost.amountMicrocredits)} 积分` : "免费额度"}</strong>。
                    </p>
                ) : null}
            </Modal>
        </section>
    );
}

function VisualQCAttemptState({ attempt }: { attempt: FilmVisualQCAttemptView }) {
    const dimensions = useMemo(() => visualQCDimensions(attempt.report), [attempt.report]);
    return (
        <div className="space-y-2 rounded-lg p-2.5" style={{ background: "var(--library-surface)" }}>
            <div className="flex items-center justify-between gap-2 text-xs">
                <Tag color={attemptTone(attempt.attempt.status)}>{visualQCStatusLabel(attempt.attempt.status)}</Tag>
                <span className="min-w-0 truncate" style={{ color: "var(--foreground-muted)" }}>
                    {attempt.task.stage}
                </span>
            </div>
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
                        <span>{modelVerdictLabel(attempt.report.decision)}，等待人工裁决</span>
                    </div>
                    <details>
                        <summary className="cursor-pointer text-xs font-medium">查看八维证据</summary>
                        <div className="mt-2 divide-y" style={{ borderColor: "var(--border-semantic)" }}>
                            {dimensions.map((dimension) => (
                                <div key={dimension.dimension} className="space-y-1 py-2 text-xs">
                                    <div className="flex items-center justify-between gap-2">
                                        <strong>{dimensionLabels[dimension.dimension] || dimension.dimension}</strong>
                                        <Tag color={decisionTone(dimension.decision)}>{dimension.decision}</Tag>
                                    </div>
                                    {dimension.observations.map((observation, index) => (
                                        <div key={`${dimension.dimension}-${index}`} style={{ color: "var(--foreground-muted)" }}>
                                            {observation}
                                        </div>
                                    ))}
                                    {dimension.issueCodes.length ? <div style={{ color: "var(--status-error)" }}>{dimension.issueCodes.join(" · ")}</div> : null}
                                </div>
                            ))}
                        </div>
                    </details>
                </>
            ) : null}
        </div>
    );
}

function visualQCDimensions(report?: FilmProductionQCReport): VisualQCDimension[] {
    const raw = report?.evidence?.dimensions;
    if (!Array.isArray(raw)) return [];
    return raw.flatMap((value) => {
        if (!value || typeof value !== "object") return [];
        const item = value as Record<string, unknown>;
        if (typeof item.dimension !== "string" || typeof item.decision !== "string") return [];
        return [
            {
                dimension: item.dimension,
                decision: item.decision,
                issueCodes: stringArray(item.issueCodes),
                observations: stringArray(item.observations),
                rationale: typeof item.rationale === "string" ? item.rationale : "",
            },
        ];
    });
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

function visualQCStatusLabel(status: string) {
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
    return (
        <div className="rounded-lg px-2.5 py-2 text-xs" style={{ background: "var(--library-surface)", color: "var(--foreground-muted)" }}>
            {text}
        </div>
    );
}

function readError(error: unknown) {
    return error instanceof Error ? error.message : "视觉 QC 请求失败";
}
