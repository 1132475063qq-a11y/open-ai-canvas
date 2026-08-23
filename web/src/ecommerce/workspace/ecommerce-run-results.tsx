import { useEffect, useMemo, useState } from "react";
import { App, Button, Checkbox, Empty, Input, InputNumber, Modal, Progress, Segmented, Select, Tag, Tooltip } from "antd";
import { ArrowDown, ArrowUp, Check, CircleAlert, Clock3, Coins, Download, Film, GitCompareArrows, Image as ImageIcon, LoaderCircle, RefreshCw, RotateCcw, ScanSearch, Sparkles, Wrench } from "lucide-react";

import {
    createProjectEcommerceVideoSequence,
    retryProjectEcommerceSlot,
    reviewProjectEcommerceSlot,
    type EcommerceQADimensionAssessment,
    type EcommerceProductionAttempt,
    type EcommerceQAReview,
    type EcommerceRunView,
    type EcommerceSlotView,
} from "@/services/api/projects";

import {
    ecommerceAspectOptions,
    ecommerceIssueOptions,
    ecommerceResultURL,
    ecommerceSlotStatusLabel,
    formatCredits,
    routeKey,
    splitRouteKey,
} from "./ecommerce-workspace-utils";

type Props = {
    projectId: string;
    runView: EcommerceRunView;
    archived: boolean;
    audioOptions: Array<{ value: string; label: string }>;
    onRunChange: (run: EcommerceRunView) => void;
    onOpenCanvas: () => void;
};

type RetryDraft = {
    slot: EcommerceSlotView;
    kind: "retry" | "repair" | "variation";
    promptPatch: string;
    route: string;
    quotedAttempt?: EcommerceProductionAttempt;
    quoteFingerprint?: string;
    quoteExpiresAt?: string;
    quoteAmount?: number;
};

type QADraft = {
    slot: EcommerceSlotView;
    reviewId: string;
    attemptId: string;
    resultId: string;
    decision: "PASS" | "UNCERTAIN" | "FAIL";
    issueCodes: string[];
    note: string;
    dimensions: EcommerceQADimensionAssessment[];
};

const ecommerceQADimensionDefinitions = [
    { key: "product_fidelity", label: "商品结构与保真", required: true },
    { key: "color_material", label: "颜色与材质", required: true },
    { key: "logo_text", label: "Logo 与文字", required: true },
    { key: "model_identity", label: "模特身份", required: false },
    { key: "anatomy_contact", label: "人体与接触关系", required: false },
    { key: "scene_consistency", label: "场景一致性", required: true },
    { key: "series_consistency", label: "系列一致性与差异度", required: false },
    { key: "commercial_quality", label: "商业成片质量", required: true },
] as const;

const ecommerceQAScoreOptions = [1, 2, 3, 4, 5].map((value) => ({ value, label: `${value} / 5` }));

function newReviewID() {
    return typeof crypto !== "undefined" && typeof crypto.randomUUID === "function" ? crypto.randomUUID() : `qa-${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

function dimensionApplicable(runView: EcommerceRunView, key: string) {
    if (key === "model_identity" || key === "anatomy_contact") return runView.run.kernel === "MODEL_INTERACTION";
    if (key === "series_consistency") return runView.run.outputCount > 1;
    return true;
}

function scoreVerdict(score: number) {
    if (score <= 2) return "FAIL";
    if (score === 3) return "UNCERTAIN";
    return "PASS";
}

function buildQADimensions(runView: EcommerceRunView, review?: EcommerceQAReview) {
    const previous = new Map((review?.dimensions || []).map((item) => [item.key, item]));
    return ecommerceQADimensionDefinitions.map((definition) => {
        const existing = previous.get(definition.key);
        if (!dimensionApplicable(runView, definition.key)) {
            return { key: definition.key, score: null, verdict: "NOT_APPLICABLE" as const, note: existing?.note || "" };
        }
        const score = typeof existing?.score === "number" && existing.score >= 1 && existing.score <= 5 ? existing.score : 4;
        return { key: definition.key, score, verdict: scoreVerdict(score), note: existing?.note || "" };
    });
}

function updateQADimension(dimensions: EcommerceQADimensionAssessment[], key: string, value: number | "na") {
    return dimensions.map((dimension) => dimension.key === key
        ? value === "na"
            ? { ...dimension, score: null, verdict: "NOT_APPLICABLE" }
            : { ...dimension, score: value, verdict: scoreVerdict(value) }
        : dimension);
}

function suggestedQADecision(dimensions: EcommerceQADimensionAssessment[], current: QADraft["decision"]): QADraft["decision"] {
    if (dimensions.some((dimension) => dimension.verdict === "FAIL")) return "FAIL";
    if (dimensions.some((dimension) => dimension.verdict === "UNCERTAIN") && current === "PASS") return "UNCERTAIN";
    return current;
}

export function EcommerceRunResults({ projectId, runView, archived, audioOptions, onRunChange, onOpenCanvas }: Props) {
    const { message } = App.useApp();
    const [busyKey, setBusyKey] = useState("");
    const [retryDraft, setRetryDraft] = useState<RetryDraft | null>(null);
    const [qaDraft, setQADraft] = useState<QADraft | null>(null);
    const [compareSlot, setCompareSlot] = useState<EcommerceSlotView | null>(null);
    const acceptedIds = useMemo(() => runView.slots.filter((item) => item.slot.accepted).map((item) => item.slot.id), [runView.slots]);
    const acceptedFingerprint = acceptedIds.join("|");
    const [videoSlotIds, setVideoSlotIds] = useState<string[]>(acceptedIds);
    const [videoDuration, setVideoDuration] = useState(15);
    const [videoAspect, setVideoAspect] = useState("9:16");
    const [musicResourceId, setMusicResourceId] = useState<string>();

    useEffect(() => {
        setVideoSlotIds((current) => {
            const retained = current.filter((id) => acceptedIds.includes(id));
            return [...retained, ...acceptedIds.filter((id) => !retained.includes(id))];
        });
    }, [acceptedFingerprint]);

    const readyRoutes = runView.routes.filter((route) => route.routeReady);
    const defaultRoute = routeKey(runView.run.quoteChannelId || readyRoutes[0]?.channelId || "", runView.run.quoteModel || readyRoutes[0]?.model || "");

    const openRetry = (slot: EcommerceSlotView, kind: RetryDraft["kind"]) => {
        setRetryDraft({ slot, kind, promptPatch: "", route: defaultRoute });
    };

    const getRetryQuote = async () => {
        if (!retryDraft) return;
        const route = splitRouteKey(retryDraft.route);
        setBusyKey(`retry-quote:${retryDraft.slot.slot.id}`);
        try {
            const result = await retryProjectEcommerceSlot(projectId, runView.run.id, retryDraft.slot.slot.id, {
                kind: retryDraft.kind,
                promptPatch: retryDraft.promptPatch.trim(),
                channelId: route.channelId,
                model: route.model,
            });
            setRetryDraft({
                ...retryDraft,
                quotedAttempt: result.retry.attempt,
                quoteFingerprint: result.retry.quote?.fingerprint,
                quoteExpiresAt: result.retry.quote?.expiresAt,
                quoteAmount: result.retry.quote?.totalMicrocredits,
            });
        } catch (error) {
            message.error(error instanceof Error ? error.message : "重试报价获取失败");
        } finally {
            setBusyKey("");
        }
    };

    const submitRetry = async () => {
        if (!retryDraft?.quotedAttempt || !retryDraft.quoteFingerprint) return;
        setBusyKey(`retry-submit:${retryDraft.slot.slot.id}`);
        try {
            const result = await retryProjectEcommerceSlot(projectId, runView.run.id, retryDraft.slot.slot.id, {
                attemptId: retryDraft.quotedAttempt.id,
                quoteFingerprint: retryDraft.quoteFingerprint,
            });
            if (result.retry.run) onRunChange(result.retry.run);
            setRetryDraft(null);
            message.success("单张任务已提交，费用只计入本次 Attempt");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "重试提交失败");
        } finally {
            setBusyKey("");
        }
    };

    const openQA = (slot: EcommerceSlotView, attemptId?: string) => {
        const attempts = slot.attempts.filter((item) => item.attempt.status === "succeeded");
        const selectedAttempt = attempts.find((item) => item.attempt.id === attemptId)
            || attempts.find((item) => item.attempt.id === slot.slot.activeAttemptId)
            || attempts.at(-1);
        if (!selectedAttempt?.attempt.resultId) {
            message.warning("该结果缺少后端 Result 记录，暂时不能进入质检");
            return;
        }
        const previous = slot.reviews.find((review) => review.attemptId === selectedAttempt.attempt.id && review.resultId === selectedAttempt.attempt.resultId);
        setQADraft({
            slot,
            reviewId: newReviewID(),
            attemptId: selectedAttempt.attempt.id,
            resultId: selectedAttempt.attempt.resultId,
            decision: previous?.decision === "PASS" || previous?.decision === "FAIL" ? previous.decision : "UNCERTAIN",
            issueCodes: previous?.issueCodes?.length ? previous.issueCodes : parseIssues(slot.slot.qaIssuesJson),
            note: previous?.note || slot.slot.qaNote || "",
            dimensions: buildQADimensions(runView, previous),
        });
    };

    const selectQAAttempt = (attemptId: string) => {
        if (!qaDraft) return;
        const selected = qaDraft.slot.attempts.find((item) => item.attempt.id === attemptId)?.attempt;
        if (!selected?.resultId) {
            message.warning("这个 Attempt 没有可绑定的 Result");
            return;
        }
        const previous = qaDraft.slot.reviews.find((review) => review.attemptId === selected.id && review.resultId === selected.resultId);
        setQADraft({
            ...qaDraft,
            reviewId: newReviewID(),
            attemptId: selected.id,
            resultId: selected.resultId,
            decision: previous?.decision === "PASS" || previous?.decision === "FAIL" ? previous.decision : "UNCERTAIN",
            issueCodes: previous?.issueCodes || [],
            note: previous?.note || "",
            dimensions: buildQADimensions(runView, previous),
        });
    };

    const submitQA = async () => {
        if (!qaDraft?.attemptId || !qaDraft.resultId || qaDraft.dimensions.length !== ecommerceQADimensionDefinitions.length) {
            message.error("请先完成全部质检维度并绑定有效 Result");
            return;
        }
        if (qaDraft.dimensions.some((dimension) => typeof dimension.score === "number" && dimension.score <= 3 && !dimension.note?.trim()) && !qaDraft.note.trim() && !qaDraft.issueCodes.length) {
            message.error("低于 4 分的维度需要填写维度备注、审核备注或问题类型");
            return;
        }
        setBusyKey(`qa:${qaDraft.slot.slot.id}`);
        try {
            const action = qaDraft.decision === "PASS" ? "accept" : qaDraft.decision === "FAIL" ? "reject" : "hold";
            const result = await reviewProjectEcommerceSlot(projectId, runView.run.id, qaDraft.slot.slot.id, {
                reviewId: qaDraft.reviewId,
                attemptId: qaDraft.attemptId,
                resultId: qaDraft.resultId,
                decision: qaDraft.decision,
                action,
                issueCodes: qaDraft.issueCodes,
                note: qaDraft.note.trim(),
                dimensions: qaDraft.dimensions,
            });
            onRunChange(result.run);
            setQADraft(null);
            setCompareSlot(null);
            message.success(qaDraft.decision === "PASS" ? "图片已接受，可进入视频阶段" : "质检结论已记录");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "质检保存失败");
        } finally {
            setBusyKey("");
        }
    };

    const createVideoPlan = async () => {
        if (!videoSlotIds.length) {
            message.warning("至少选择一张已接受图片");
            return;
        }
        setBusyKey("video-plan");
        try {
            const result = await createProjectEcommerceVideoSequence(projectId, runView.run.id, {
                slotIds: videoSlotIds,
                aspectRatio: videoAspect,
                durationSeconds: videoDuration,
                musicResourceId,
            });
            onRunChange(result.run);
            message.success("视频动作与顺序计划已保存");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "视频计划创建失败");
        } finally {
            setBusyKey("");
        }
    };

    return (
        <>
            <section className="border-t border-border/70 pt-5" aria-labelledby="ecommerce-results-title">
                <div className="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
                    <div><div className="flex items-center gap-2 text-xs font-semibold text-[var(--workspace-accent)]"><ImageIcon className="size-3.5" />SERIES OUTPUT</div><h3 id="ecommerce-results-title" className="mt-1 text-lg font-semibold">系列套图与逐张质检</h3><p className="mt-1 text-sm text-foreground/52">成功槽位独立保留，失败槽位可单张报价重做；每次结果都进入版本历史。</p></div>
                    <div className="flex items-center gap-2 text-xs text-foreground/48"><ScanSearch className="size-4" />已接受 {acceptedIds.length} / {runView.slots.length}</div>
                </div>

                <div className="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                    {runView.slots.map((slotView) => {
                        const { slot } = slotView;
                        const active = slotView.attempts.find((item) => item.attempt.id === slot.activeAttemptId) || slotView.attempts.at(-1);
                        const url = ecommerceResultURL(slot) || (active ? ecommerceResultURL(active.attempt) : "");
                        const latestReview = slotView.reviews[0];
                        const progress = active?.task?.progress || (slot.status === "accepted" || slot.status === "qa" ? 100 : 0);
                        const waiting = slot.status === "scheduled" || slot.status === "queued" || slot.status === "running";
                        return (
                            <article key={slot.id} className="overflow-hidden rounded-lg border border-border/75 bg-background shadow-sm">
                                <div className="relative aspect-[4/3] overflow-hidden bg-foreground/[.04]">
                                    {url ? <img src={url} alt={`${slot.position}. ${slot.title}`} className="h-full w-full object-cover" /> : <div className="grid h-full place-items-center px-5 text-center text-foreground/35">{waiting ? <LoaderCircle className="size-8 animate-spin" /> : <div><ImageIcon className="mx-auto size-8" /><p className="mt-2 text-xs">{slot.status === "failed" ? "本槽位生成失败" : "等待真实生成结果"}</p></div>}</div>}
                                    <div className="absolute inset-x-2 top-2 flex items-start justify-between gap-2"><span className="grid size-7 place-items-center rounded-md bg-black/60 text-xs font-semibold text-white">{slot.position}</span><Tag color={slot.accepted ? "success" : slot.status === "failed" ? "error" : slot.qaStatus === "UNCERTAIN" ? "warning" : "default"} className="m-0 !rounded-md !border-0 !bg-background/90 !text-foreground">{slot.accepted ? "已接受" : ecommerceSlotStatusLabel(slot.status)}</Tag></div>
                                    {url ? <div className="absolute bottom-2 right-2 flex gap-1"><Tooltip title="下载图片"><Button href={url} target="_blank" download type="text" className="!grid !size-8 !place-items-center !bg-black/60 !p-0 !text-white" icon={<Download className="size-3.5" />} aria-label="下载图片" /></Tooltip></div> : null}
                                </div>
                                <div className="p-3">
                                    <div className="flex items-start justify-between gap-2"><div className="min-w-0"><h4 className="truncate text-sm font-semibold">{slot.title}</h4><p className="mt-0.5 truncate text-xs text-foreground/44">{slot.role} · Attempt {active?.attempt.attemptNumber || 1}</p></div><QAStatus status={slot.qaStatus} /></div>
                                    {waiting ? <div className="mt-3"><Progress percent={Math.max(2, progress)} size="small" showInfo={false} /><p className="mt-1 text-xs text-foreground/45">{active?.task?.stage || ecommerceSlotStatusLabel(slot.status)}</p></div> : null}
                                    {slot.status === "failed" || active?.attempt.error || (slot.qaStatus === "FAIL" && slot.qaNote) ? <p className="mt-3 line-clamp-3 text-xs leading-5 text-red-500">{active?.attempt.error || slot.qaNote || "上游生成失败，可保留其他成功图片并单张重试。"}</p> : null}
                                    <details className="mt-3 border-t border-border/60 pt-2 text-xs"><summary className="cursor-pointer select-none text-foreground/52">查看逐镜 Prompt</summary><p className="mt-2 max-h-36 overflow-y-auto whitespace-pre-wrap leading-5 text-foreground/55">{active?.attempt.prompt || slot.prompt}</p></details>
                                    <div className="mt-3 flex flex-wrap gap-1.5">
                                        {url && active?.attempt.status === "succeeded" ? <Button size="small" type={slot.accepted ? "default" : "primary"} icon={<ScanSearch className="size-3.5" />} disabled={!active.attempt.resultId} onClick={() => openQA(slotView)}>{slot.accepted ? "复核" : "人工质检"}</Button> : null}
                                        <Button size="small" icon={<RefreshCw className="size-3.5" />} disabled={archived || waiting} onClick={() => openRetry(slotView, "retry")}>重做</Button>
                                        {url ? <Button size="small" icon={<Wrench className="size-3.5" />} disabled={archived || waiting} onClick={() => openRetry(slotView, "repair")}>局部修复</Button> : null}
                                        {url ? <Button size="small" icon={<Sparkles className="size-3.5" />} disabled={archived || waiting} onClick={() => openRetry(slotView, "variation")}>分支变化</Button> : null}
                                        {slotView.attempts.length > 1 ? <Button size="small" type="text" icon={<GitCompareArrows className="size-3.5" />} onClick={() => setCompareSlot(slotView)}>版本 {slotView.attempts.length}</Button> : null}
                                    </div>
                                    {latestReview ? <details className="mt-3 border-t border-border/60 pt-2 text-xs"><summary className="cursor-pointer select-none text-foreground/52">质检记录 · {slotView.reviews.length} 次 · 最新 {latestReview.decision}</summary><div className="mt-2 space-y-2 text-foreground/55">{slotView.reviews.slice(0, 3).map((review) => <div key={review.reviewId} className="rounded-md bg-foreground/[.035] p-2"><div className="flex items-center justify-between gap-2"><span>Attempt {review.attemptId === active?.attempt.id ? active.attempt.attemptNumber : "历史"} · {review.decision}</span><span>{new Date(review.reviewedAt).toLocaleString("zh-CN")}</span></div><p className="mt-1">{review.dimensions.filter((dimension) => dimension.verdict !== "NOT_APPLICABLE").map((dimension) => `${dimension.key} ${dimension.score}/5`).join(" · ") || "无适用评分"}</p>{review.runtimeEvidence ? <p className="mt-1">API 稳定性 {review.runtimeEvidence.apiStabilityScore}/5{review.runtimeEvidence.latencyMs != null ? ` · ${review.runtimeEvidence.latencyMs}ms` : ""}</p> : null}</div>)}</div></details> : null}
                                </div>
                            </article>
                        );
                    })}
                </div>
            </section>

            <section className="border-t border-border/70 pt-5" aria-labelledby="ecommerce-video-title">
                <div className="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between"><div><div className="flex items-center gap-2 text-xs font-semibold text-[var(--workspace-accent)]"><Film className="size-3.5" />MOTION DIRECTOR</div><h3 id="ecommerce-video-title" className="mt-1 text-lg font-semibold">图片转视频编排</h3><p className="mt-1 text-sm text-foreground/52">只使用已接受图片，默认生成 9:16、约 15 秒的电商短片计划。</p></div>{runView.run.videoSequenceArtifactId ? <Tag color="success" className="m-0 !rounded-md">视频计划已保存</Tag> : null}</div>
                {!acceptedIds.length ? <div className="mt-4 rounded-lg border border-dashed border-border/80 py-8"><Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="先通过并接受至少一张图片" /></div> : (
                    <div className="mt-4 grid gap-5 xl:grid-cols-[minmax(0,1fr)_320px]">
                        <div className="space-y-2">
                            {videoSlotIds.map((slotId, index) => {
                                const slot = runView.slots.find((item) => item.slot.id === slotId)?.slot;
                                if (!slot) return null;
                                return <div key={slotId} className="flex min-h-14 items-center gap-3 border-b border-border/60 px-1 py-2"><span className="grid size-7 shrink-0 place-items-center rounded-md bg-foreground/[.055] text-xs tabular-nums">{index + 1}</span>{ecommerceResultURL(slot) ? <img src={ecommerceResultURL(slot)} alt="" className="h-10 w-14 rounded object-cover" /> : null}<div className="min-w-0 flex-1"><div className="truncate text-sm font-medium">{slot.title}</div><div className="text-xs text-foreground/43">{slot.role}</div></div><Tooltip title="上移"><Button type="text" icon={<ArrowUp className="size-3.5" />} disabled={index === 0} aria-label="上移镜头" onClick={() => setVideoSlotIds(moveItem(videoSlotIds, index, index - 1))} /></Tooltip><Tooltip title="下移"><Button type="text" icon={<ArrowDown className="size-3.5" />} disabled={index === videoSlotIds.length - 1} aria-label="下移镜头" onClick={() => setVideoSlotIds(moveItem(videoSlotIds, index, index + 1))} /></Tooltip></div>;
                            })}
                        </div>
                        <div className="space-y-3 border-l-0 border-border/70 xl:border-l xl:pl-5">
                            <div className="grid grid-cols-2 gap-3"><label className="grid gap-1.5 text-xs"><span className="font-medium text-foreground/60">比例</span><Select value={videoAspect} options={ecommerceAspectOptions} onChange={setVideoAspect} /></label><label className="grid gap-1.5 text-xs"><span className="font-medium text-foreground/60">总时长</span><InputNumber className="w-full" min={Math.max(1, videoSlotIds.length)} max={60} value={videoDuration} addonAfter="秒" onChange={(value) => setVideoDuration(value || 15)} /></label></div>
                            <label className="grid gap-1.5 text-xs"><span className="font-medium text-foreground/60">基础音乐</span><Select allowClear showSearch value={musicResourceId} options={audioOptions} placeholder="可选项目音频资产" optionFilterProp="label" onChange={setMusicResourceId} /></label>
                            <Button block type="primary" icon={<Film className="size-4" />} disabled={archived} loading={busyKey === "video-plan"} onClick={() => void createVideoPlan()}>生成视频方案</Button>
                            {runView.run.videoSequenceArtifactId ? <Button block icon={<RotateCcw className="size-4" />} onClick={onOpenCanvas}>进入电商画布编排</Button> : null}
                        </div>
                    </div>
                )}
            </section>

            <Modal open={Boolean(retryDraft)} title={retryTitle(retryDraft?.kind)} onCancel={() => setRetryDraft(null)} footer={null} destroyOnHidden>
                {!retryDraft ? null : <div className="space-y-4">
                    <p className="text-sm leading-6 text-foreground/55">槽位 {retryDraft.slot.slot.position} · {retryDraft.slot.slot.title}。获取报价不会产生费用，确认提交后才会创建真实任务。</p>
                    <label className="grid gap-1.5 text-xs"><span className="font-medium text-foreground/60">模型路线</span><Select value={retryDraft.route || undefined} options={readyRoutes.map((route) => ({ value: routeKey(route.channelId, route.model), label: `${route.channelName} · ${route.modelDisplayName} · ${formatCredits(route.unitPriceMicrocredits)} 积分/张` }))} placeholder="选择已就绪模型" onChange={(value) => setRetryDraft({ ...retryDraft, route: value, quotedAttempt: undefined, quoteFingerprint: undefined })} /></label>
                    <label className="grid gap-1.5 text-xs"><span className="font-medium text-foreground/60">修改要求</span><Input.TextArea value={retryDraft.promptPatch} autoSize={{ minRows: 3, maxRows: 7 }} placeholder={retryDraft.kind === "repair" ? "例如：只修复右手与瓶身接触，其他内容保持不变" : "描述这次变化或重做要求"} onChange={(event) => setRetryDraft({ ...retryDraft, promptPatch: event.target.value, quotedAttempt: undefined, quoteFingerprint: undefined })} /></label>
                    {retryDraft.quoteFingerprint ? <div className="rounded-lg border border-amber-500/30 bg-amber-500/[.07] p-3"><div className="flex items-center justify-between gap-3"><span className="flex items-center gap-2 text-sm font-medium"><Coins className="size-4 text-amber-600" />本次付费重试</span><strong className="tabular-nums">{formatCredits(retryDraft.quoteAmount)} 积分</strong></div><p className="mt-1 flex items-center gap-1.5 text-xs text-foreground/48"><Clock3 className="size-3.5" />报价有效至 {retryDraft.quoteExpiresAt ? new Date(retryDraft.quoteExpiresAt).toLocaleTimeString("zh-CN") : "-"}</p></div> : null}
                    <div className="flex justify-end gap-2"><Button onClick={() => setRetryDraft(null)}>取消</Button>{retryDraft.quoteFingerprint ? <Button type="primary" danger icon={<Coins className="size-4" />} loading={busyKey.startsWith("retry-submit:")} onClick={() => void submitRetry()}>确认付费并提交</Button> : <Button type="primary" icon={<Coins className="size-4" />} disabled={!retryDraft.route} loading={busyKey.startsWith("retry-quote:")} onClick={() => void getRetryQuote()}>获取单张报价</Button>}</div>
                </div>}
            </Modal>

            <Modal open={Boolean(qaDraft)} title="人工视觉质检" onCancel={() => setQADraft(null)} footer={null} destroyOnHidden>
                {!qaDraft ? null : <div className="space-y-4">
                    <div className="grid gap-3 sm:grid-cols-[160px_minmax(0,1fr)]">{selectedQAAttempt(qaDraft) ? <img src={ecommerceResultURL(selectedQAAttempt(qaDraft)!)} alt="待质检结果" className="aspect-[4/3] w-full rounded-lg object-cover" /> : <div className="grid aspect-[4/3] place-items-center rounded-lg bg-foreground/[.04]"><ImageIcon className="size-7 text-foreground/25" /></div>}<div><p className="text-sm font-semibold">{qaDraft.slot.slot.title}</p><p className="mt-1 text-xs leading-5 text-foreground/48">当前审核绑定 Attempt {selectedQAAttempt(qaDraft)?.attemptNumber || "-"} · Result {qaDraft.resultId.slice(0, 12)}</p><Select className="mt-3 w-full" value={qaDraft.attemptId} options={qaDraft.slot.attempts.filter((item) => item.attempt.status === "succeeded" && item.attempt.resultId).map((item) => ({ value: item.attempt.id, label: `Attempt ${item.attempt.attemptNumber} · ${item.attempt.kind}` }))} onChange={selectQAAttempt} /></div></div>
                    <div><div className="mb-1.5 text-xs font-medium text-foreground/60">结论</div><Segmented block value={qaDraft.decision} options={[{ value: "PASS", label: "通过并接受" }, { value: "UNCERTAIN", label: "待确认" }, { value: "FAIL", label: "不通过" }]} onChange={(decision) => setQADraft({ ...qaDraft, decision: decision as QADraft["decision"] })} /></div>
                    <div className="grid gap-2 rounded-lg border border-border/70 p-3">
                        <div className="flex items-center justify-between gap-2"><div><div className="text-sm font-semibold">结构化质检维度</div><div className="mt-0.5 text-xs text-foreground/45">4-5 分通过，3 分待确认，1-2 分不通过</div></div><Tag className="m-0 !rounded-md">{qaDraft.dimensions.filter((dimension) => dimension.verdict === "PASS").length} / {qaDraft.dimensions.filter((dimension) => dimension.verdict !== "NOT_APPLICABLE").length} 通过</Tag></div>
                        <div className="grid gap-2 sm:grid-cols-2">
                            {ecommerceQADimensionDefinitions.map((definition) => {
                                const dimension = qaDraft.dimensions.find((item) => item.key === definition.key);
                                if (!dimension) return null;
                                const applicable = dimensionApplicable(runView, definition.key);
                                const value = dimension.verdict === "NOT_APPLICABLE" ? "na" : dimension.score ?? 4;
                                return <div key={definition.key} className="rounded-md bg-foreground/[.035] p-2.5"><div className="flex items-center justify-between gap-2"><span className="min-w-0 text-xs font-medium">{definition.label}{definition.required || applicable ? "" : " · 不适用"}</span><Tag color={dimensionColor(dimension.verdict)} className="m-0 shrink-0 !rounded-md">{dimension.verdict === "NOT_APPLICABLE" ? "不适用" : dimension.verdict}</Tag></div><div className="mt-2 flex items-center gap-2"><Select size="small" className="w-28" value={value} options={applicable ? ecommerceQAScoreOptions : [...ecommerceQAScoreOptions, { value: "na", label: "不适用" }]} onChange={(nextValue) => { const dimensions = updateQADimension(qaDraft.dimensions, definition.key, nextValue === "na" ? "na" : Number(nextValue) as 1 | 2 | 3 | 4 | 5); setQADraft({ ...qaDraft, dimensions, decision: suggestedQADecision(dimensions, qaDraft.decision) }); }} /><Input size="small" value={dimension.note || ""} placeholder="维度备注" onChange={(event) => setQADraft({ ...qaDraft, dimensions: qaDraft.dimensions.map((item) => item.key === definition.key ? { ...item, note: event.target.value } : item) })} /></div></div>;
                            })}
                        </div>
                    </div>
                    <div><div className="mb-2 text-xs font-medium text-foreground/60">问题类型</div><Checkbox.Group className="grid grid-cols-2 gap-2" value={qaDraft.issueCodes} options={ecommerceIssueOptions} onChange={(values) => setQADraft({ ...qaDraft, issueCodes: values as string[] })} /></div>
                    <label className="grid gap-1.5 text-xs"><span className="font-medium text-foreground/60">审核备注</span><Input.TextArea value={qaDraft.note} autoSize={{ minRows: 2, maxRows: 5 }} onChange={(event) => setQADraft({ ...qaDraft, note: event.target.value })} /></label>
                    {qaDraft.slot.reviews.length ? <details className="border-t border-border/60 pt-2 text-xs"><summary className="cursor-pointer select-none text-foreground/52">查看该槽位历史审核 · {qaDraft.slot.reviews.length} 次</summary><div className="mt-2 max-h-28 space-y-1.5 overflow-y-auto text-foreground/55">{qaDraft.slot.reviews.map((review) => <div key={review.reviewId} className="flex items-center justify-between gap-3"><span>{review.decision} · Attempt {review.attemptId === qaDraft.attemptId ? selectedQAAttempt(qaDraft)?.attemptNumber : "历史"}</span><span>{new Date(review.reviewedAt).toLocaleString("zh-CN")}</span></div>)}</div></details> : null}
                    <div className="flex justify-end gap-2"><Button onClick={() => setQADraft(null)}>取消</Button><Button type="primary" danger={qaDraft.decision === "FAIL"} icon={qaDraft.decision === "PASS" ? <Check className="size-4" /> : <ScanSearch className="size-4" />} loading={busyKey.startsWith("qa:")} onClick={() => void submitQA()}>保存质检结论</Button></div>
                </div>}
            </Modal>

            <Modal open={Boolean(compareSlot)} title="版本对比与回退" width={880} onCancel={() => setCompareSlot(null)} footer={null}>
                {!compareSlot ? null : <div className="grid max-h-[66vh] gap-3 overflow-y-auto sm:grid-cols-2 lg:grid-cols-3">{compareSlot.attempts.slice().reverse().map(({ attempt }) => {
                    const url = ecommerceResultURL(attempt);
                    return <article key={attempt.id} className={`overflow-hidden rounded-lg border ${attempt.id === compareSlot.slot.activeAttemptId ? "border-[var(--workspace-accent)]" : "border-border/75"}`}><div className="aspect-[4/3] bg-foreground/[.04]">{url ? <img src={url} alt={`Attempt ${attempt.attemptNumber}`} className="h-full w-full object-cover" /> : <div className="grid h-full place-items-center"><CircleAlert className="size-6 text-foreground/25" /></div>}</div><div className="p-3"><div className="flex items-center justify-between gap-2"><strong className="text-sm">Attempt {attempt.attemptNumber}</strong><Tag className="m-0 !rounded-md">{attempt.kind}</Tag></div><p className="mt-1 text-xs text-foreground/45">{ecommerceSlotStatusLabel(attempt.status)}</p>{attempt.status === "succeeded" ? <Button className="mt-3" block size="small" icon={<RotateCcw className="size-3.5" />} onClick={() => openQA(compareSlot, attempt.id)}>复核并恢复此版</Button> : null}</div></article>;
                })}</div>}
            </Modal>
        </>
    );
}

function QAStatus({ status }: { status: string }) {
    if (status === "PASS") return <span className="flex shrink-0 items-center gap-1 text-xs font-medium text-emerald-600"><Check className="size-3.5" />PASS</span>;
    if (status === "FAIL") return <span className="flex shrink-0 items-center gap-1 text-xs font-medium text-red-500"><CircleAlert className="size-3.5" />FAIL</span>;
    return <span className="flex shrink-0 items-center gap-1 text-xs font-medium text-amber-600"><Clock3 className="size-3.5" />{status === "UNCERTAIN" ? "待人工" : "待生成"}</span>;
}

function retryTitle(kind?: RetryDraft["kind"]) {
    if (kind === "repair") return "局部修复并重新报价";
    if (kind === "variation") return "创建分支变化并重新报价";
    return "单张重做并重新报价";
}

function parseIssues(raw: string) {
    try {
        const value = JSON.parse(raw) as unknown;
        return Array.isArray(value) ? value.filter((item): item is string => typeof item === "string" && item !== "visual_review_required") : [];
    } catch {
        return [];
    }
}

function selectedQAAttempt(draft: QADraft) {
    return draft.slot.attempts.find((item) => item.attempt.id === draft.attemptId)?.attempt;
}

function dimensionColor(verdict: string) {
    if (verdict === "PASS") return "success";
    if (verdict === "FAIL") return "error";
    if (verdict === "UNCERTAIN") return "warning";
    return "default";
}

function moveItem(values: string[], from: number, to: number) {
    const next = [...values];
    const [item] = next.splice(from, 1);
    next.splice(to, 0, item);
    return next;
}
