import { useEffect, useMemo, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, App, Button, Checkbox, Input, InputNumber, Progress, Select, Skeleton, Switch, Tag, Tooltip } from "antd";
import { Bot, Boxes, Check, ChevronDown, CircleAlert, Coins, Copy, Database, Image as ImageIcon, Layers3, LoaderCircle, Package, RefreshCw, ScanSearch, Settings2, Sparkles, StopCircle, UserRound, WandSparkles } from "lucide-react";

import { AssetMediaPreview } from "@/components/asset-media-preview";
import {
    approveProjectEcommerceRun,
    cancelProjectEcommerceRun,
    createProjectEcommerceRun,
    getProjectEcommerceRun,
    getProjectEcommerceWorkspace,
    refreshProjectEcommerceRunQuote,
    submitProjectEcommerceRun,
    type EcommerceArtifact,
    type EcommercePreset,
    type EcommerceRunView,
    type ProjectDetail,
} from "@/services/api/projects";
import { useAssetStore } from "@/stores/use-asset-store";

import { EcommercePresetEditor } from "./ecommerce-preset-editor";
import { EcommerceRunResults } from "./ecommerce-run-results";
import {
    ecommerceAspectOptions,
    ecommerceAssetSelectionDefaults,
    ecommerceCategoryOptions,
    ecommerceChannelOptions,
    ecommercePixelSize,
    ecommerceResolutionOptions,
    ecommerceRunStatusLabel,
    formatCredits,
    latestRunArtifact,
    parseJSONArray,
    parseJSONObject,
    routeKey,
    shouldPollEcommerceRun,
    splitRouteKey,
} from "./ecommerce-workspace-utils";

type Props = {
    detail: ProjectDetail;
    refreshProject: () => void;
    onCreateCanvas: () => void;
};

const activeAgentArtifacts = [
    { type: "product_dna", label: "商品理解", agent: "ProductIntelligenceAgent", icon: Package },
    { type: "creative_direction", label: "视觉方向", agent: "CreativeDirectorAgent", icon: WandSparkles },
    { type: "scene_pack", label: "场景导演", agent: "SceneDirectorAgent", icon: Layers3 },
    { type: "generation_request", label: "执行编译", agent: "EcommerceOrchestrator", icon: Bot },
] as const;

const defaultUserGoal = "呈现真实生活状态中的商品价值，并形成可直接筛选投放的系列套图";
const defaultModelBrief = "自然、有生活感，符合商品目标人群；避免僵硬棚拍姿势";
const defaultLighting = "自然方向光，保留可信接触阴影";
const defaultPalette = "自然综合色，品牌色只作为局部强调";

function ecommerceFormString(value: unknown, fallback = "") {
    return typeof value === "string" ? value : fallback;
}

function ecommerceFactString(value: unknown) {
    if (typeof value === "string") return value;
    if (Array.isArray(value)) return value.filter((item): item is string => typeof item === "string").join("、");
    return "";
}

export function EcommerceProductionWorkspace({ detail, refreshProject, onCreateCanvas }: Props) {
    const { message, modal } = App.useApp();
    const queryClient = useQueryClient();
    const personalAssets = useAssetStore((state) => state.assets);
    const projectId = detail.project.id;
    const workspaceQuery = useQuery({
        queryKey: ["ecommerce-workspace", projectId],
        queryFn: () => getProjectEcommerceWorkspace(projectId),
        refetchOnMount: "always",
        refetchInterval: (query) => shouldPollEcommerceRun(query.state.data?.workspace.activeRun?.run.status) ? 2500 : false,
    });
    const workspace = workspaceQuery.data?.workspace;
    const [runView, setRunView] = useState<EcommerceRunView>();
    const [selectedRunId, setSelectedRunId] = useState("");
    const [busyKey, setBusyKey] = useState("");
    const hydratedRunKeyRef = useRef("");

    const imageAssets = detail.assets.filter((asset) => asset.mediaType === "image");
    const audioAssets = detail.assets.filter((asset) => asset.mediaType === "audio");
    const assetOptions = imageAssets.map((asset) => ({ value: asset.id, label: asset.title || asset.id }));
    const audioOptions = audioAssets.map((asset) => ({ value: asset.id, label: asset.title || asset.id }));
    const initialAssetSelection = ecommerceAssetSelectionDefaults(detail.assets);
    const [productAssetIds, setProductAssetIds] = useState<string[]>(() => initialAssetSelection.productAssetIds);
    const [supportingAssetIds, setSupportingAssetIds] = useState<string[]>(() => initialAssetSelection.supportingAssetIds);
    const [modelAssetIds, setModelAssetIds] = useState<string[]>(() => initialAssetSelection.modelAssetIds);
    const [sceneAssetIds, setSceneAssetIds] = useState<string[]>(() => initialAssetSelection.sceneAssetIds);
    const [brandAssetIds, setBrandAssetIds] = useState<string[]>(() => initialAssetSelection.brandAssetIds);
    const [presetId, setPresetId] = useState("");
    const [targetChannel, setTargetChannel] = useState("taobao_jd");
    const [aspectRatio, setAspectRatio] = useState(detail.project.aspectRatio || "3:4");
    const [resolution, setResolution] = useState<"1k" | "2k" | "4k">("4k");
    const [outputCount, setOutputCount] = useState(6);
    const [category, setCategory] = useState("general");
    const [reviewBeforeGeneration, setReviewBeforeGeneration] = useState(false);
    const [userGoal, setUserGoal] = useState(defaultUserGoal);
    const [modelBrief, setModelBrief] = useState(defaultModelBrief);
    const [sceneBrief, setSceneBrief] = useState("");
    const [brandBrief, setBrandBrief] = useState("");
    const [productColor, setProductColor] = useState("");
    const [productMaterial, setProductMaterial] = useState("");
    const [mustPreserve, setMustPreserve] = useState("");
    const [lighting, setLighting] = useState(defaultLighting);
    const [palette, setPalette] = useState(defaultPalette);
    const [negativePrompt, setNegativePrompt] = useState("");
    const [selectedRoute, setSelectedRoute] = useState("");
    const [presetEditorOpen, setPresetEditorOpen] = useState(false);

    const presets = useMemo(() => [...(workspace?.presets.custom || []), ...(workspace?.presets.system || [])], [workspace?.presets]);
    const selectedPreset = presets.find((preset) => preset.id === presetId || preset.presetKey === presetId);
    const readyRoutes = workspace?.providerRoutes.filter((route) => route.routeReady) || [];

    useEffect(() => {
        if (presetId || !presets.length) return;
        const preferred = presets.find((preset) => preset.id === "model.top-wear") || presets[0];
        setPresetId(preferred.id);
        setCategory(preferred.category || "general");
    }, [presetId, presets]);

    useEffect(() => {
        if (selectedRoute || !readyRoutes.length) return;
        setSelectedRoute(routeKey(readyRoutes[0].channelId, readyRoutes[0].model));
    }, [selectedRoute, readyRoutes]);

    useEffect(() => {
        const active = workspace?.activeRun;
        if (!active) return;
        if (!selectedRunId || selectedRunId === active.run.id) {
            setSelectedRunId(active.run.id);
            setRunView(active);
        }
    }, [workspace?.activeRun?.run.id, workspace?.activeRun?.run.updatedAt, selectedRunId]);

    // A workspace refresh restores the server-owned Run, but the editor state is
    // local React state. Hydrate once per selected Run so polling never erases
    // changes the user is making for a new production.
    useEffect(() => {
        const run = runView?.run;
        if (!run) return;
        const runKey = `${projectId}:${run.id}`;
        if (hydratedRunKeyRef.current === runKey) return;
        hydratedRunKeyRef.current = runKey;

        setProductAssetIds(parseJSONArray(run.productAssetIdsJson));
        setSupportingAssetIds(parseJSONArray(run.supportingAssetIdsJson));
        setModelAssetIds(parseJSONArray(run.modelAssetIdsJson));
        setSceneAssetIds(parseJSONArray(run.sceneAssetIdsJson));
        setBrandAssetIds(parseJSONArray(run.brandAssetIdsJson));
        setPresetId(run.presetId || "");
        setTargetChannel(run.targetChannel || "taobao_jd");
        setAspectRatio(run.aspectRatio || "3:4");
        if (run.resolution === "1k" || run.resolution === "2k" || run.resolution === "4k") setResolution(run.resolution);
        setOutputCount(Math.min(12, Math.max(1, run.outputCount || 6)));
        setCategory(run.category || "general");
        setReviewBeforeGeneration(Boolean(run.reviewBeforeGeneration));
        setUserGoal(run.userGoal || defaultUserGoal);
        setModelBrief(run.modelBrief || defaultModelBrief);
        setSceneBrief(run.sceneBrief || "");
        setBrandBrief(run.brandBrief || "");

        const advanced = parseJSONObject(run.advancedJson);
        setLighting(ecommerceFormString(advanced.lighting, defaultLighting));
        setPalette(ecommerceFormString(advanced.palette, defaultPalette));
        setNegativePrompt(ecommerceFormString(advanced.negativePrompt));

        const productDNA = parseJSONObject(latestRunArtifact(workspace?.artifacts || [], run.id, "product_dna")?.payloadJson);
        const recordedFacts = productDNA.recordedFacts && typeof productDNA.recordedFacts === "object" && !Array.isArray(productDNA.recordedFacts)
            ? productDNA.recordedFacts as Record<string, unknown>
            : {};
        setProductColor(ecommerceFactString(recordedFacts.color));
        setProductMaterial(ecommerceFactString(recordedFacts.material));
        setMustPreserve(ecommerceFactString(recordedFacts.mustPreserve || productDNA.mustPreserve));

        const quoteChannelId = run.quoteChannelId?.trim();
        const quoteModel = run.quoteModel?.trim();
        if (quoteChannelId && quoteModel) {
            setSelectedRoute(routeKey(quoteChannelId, quoteModel));
        }
    }, [projectId, runView?.run.id, workspace?.artifacts, workspace?.providerRoutes]);

    const applyRun = (next: EcommerceRunView) => {
        setRunView(next);
        setSelectedRunId(next.run.id);
        void queryClient.invalidateQueries({ queryKey: ["ecommerce-workspace", projectId] });
        refreshProject();
    };

    const loadRun = async (runId: string) => {
        setSelectedRunId(runId);
        setBusyKey("load-run");
        try {
            const result = await getProjectEcommerceRun(projectId, runId);
            setRunView(result.run);
        } catch (error) {
            message.error(error instanceof Error ? error.message : "生产记录读取失败");
        } finally {
            setBusyKey("");
        }
    };

    const createRun = async () => {
        if (!productAssetIds.length) {
            message.warning("至少选择一张主商品图片");
            return;
        }
        if (!selectedPreset) {
            message.warning("请选择商拍预设");
            return;
        }
        const route = splitRouteKey(selectedRoute);
        const facts = Object.fromEntries(Object.entries({ color: productColor.trim(), material: productMaterial.trim(), mustPreserve: mustPreserve.trim() }).filter(([, value]) => value));
        setBusyKey("create-run");
        try {
            const result = await createProjectEcommerceRun(projectId, {
                idempotencyKey: createIdempotencyKey(projectId),
                productAssetIds,
                supportingAssetIds,
                modelAssetIds: selectedPreset.kernel === "MODEL_INTERACTION" ? modelAssetIds : [],
                sceneAssetIds,
                brandAssetIds,
                presetId: selectedPreset.id,
                category,
                targetChannel,
                aspectRatio,
                resolution,
                outputCount,
                reviewBeforeGeneration,
                userGoal: userGoal.trim(),
                modelMode: selectedPreset.kernel === "MODEL_INTERACTION" ? (modelAssetIds.length ? "uploaded" : "ai") : "none",
                modelBrief: modelBrief.trim(),
                sceneBrief: sceneBrief.trim(),
                brandBrief: brandBrief.trim(),
                productFacts: facts,
                advanced: { lighting: lighting.trim(), palette: palette.trim(), negativePrompt: negativePrompt.trim() },
                channelId: route.channelId,
                model: route.model,
            });
            applyRun(result.run);
            message.success(reviewBeforeGeneration ? "Agent 方案已生成，等待确认" : result.run.quote ? "Agent 方案与费用报价已生成" : "Agent 方案已生成，请配置可用图片模型");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "系列方案创建失败");
        } finally {
            setBusyKey("");
        }
    };

    const approvePlan = async () => {
        if (!runView) return;
        setBusyKey("approve-plan");
        try {
            const result = await approveProjectEcommerceRun(projectId, runView.run.id);
            applyRun(result.run);
            message.success("Agent 方案已确认");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "方案确认失败");
        } finally {
            setBusyKey("");
        }
    };

    const refreshQuote = async () => {
        if (!runView || !selectedRoute) return;
        const route = splitRouteKey(selectedRoute);
        setBusyKey("refresh-quote");
        try {
            const result = await refreshProjectEcommerceRunQuote(projectId, runView.run.id, route);
            applyRun(result.run);
            message.success("费用报价已刷新");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "报价刷新失败");
        } finally {
            setBusyKey("");
        }
    };

    const submitRun = async () => {
        if (!runView?.quote?.fingerprint) return;
        setBusyKey("submit-run");
        try {
            const result = await submitProjectEcommerceRun(projectId, runView.run.id, runView.quote.fingerprint);
            applyRun(result.run);
            window.dispatchEvent(new CustomEvent("wallet:updated"));
            message.success("系列任务已原子提交，页面关闭后仍会继续执行");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "系列任务提交失败");
        } finally {
            setBusyKey("");
        }
    };

    const cancelRun = () => {
        if (!runView) return;
        modal.confirm({
            title: "取消这组电商生产？",
            content: "尚未调用 Provider 的任务会退款；运行中的上游任务会进入取消与费用核对。已成功图片和版本记录会保留。",
            okText: "确认取消",
            cancelText: "继续生产",
            okButtonProps: { danger: true },
            onOk: async () => {
                setBusyKey("cancel-run");
                try {
                    const result = await cancelProjectEcommerceRun(projectId, runView.run.id);
                    applyRun(result.run);
                    window.dispatchEvent(new CustomEvent("wallet:updated"));
                    message.success("系列生产已取消，已有结果和审计记录已保留");
                } catch (error) {
                    message.error(error instanceof Error ? error.message : "系列取消失败");
                    throw error;
                } finally {
                    setBusyKey("");
                }
            },
        });
    };

    if (workspaceQuery.isLoading) return <div className="space-y-5"><Skeleton active paragraph={{ rows: 3 }} /><Skeleton active paragraph={{ rows: 7 }} /></div>;
    if (workspaceQuery.isError || !workspace) return <Alert type="error" showIcon message="电商工作台读取失败" description={workspaceQuery.error instanceof Error ? workspaceQuery.error.message : "请检查后端连接后重试"} action={<Button icon={<RefreshCw className="size-4" />} onClick={() => void workspaceQuery.refetch()}>重试</Button>} />;

    const archived = detail.project.status === "archived";
    const quoteExpired = runView?.quote ? new Date(runView.quote.expiresAt).getTime() <= Date.now() : false;
    const progress = runView ? ecommerceRunProgress(runView) : 0;
    const resolutionOptions = ecommerceResolutionOptions.map((option) => ({
        value: option.value,
        label: `${option.label} · ${ecommercePixelSize(aspectRatio, option.value).replace("x", "×")}`,
    }));

    return (
        <div className="space-y-6">
            <header className="flex flex-col gap-3 border-b border-border/70 pb-4 lg:flex-row lg:items-end lg:justify-between">
                <div className="min-w-0"><div className="flex items-center gap-2 text-xs font-semibold text-[var(--workspace-accent)]"><Boxes className="size-3.5" />ECOMMERCE PRODUCTION</div><h2 className="mt-1 text-xl font-semibold">电商创意工作台</h2><div className="mt-2 flex flex-wrap items-center gap-2"><Tag className="m-0 !rounded-md">双内核</Tag><Tag className="m-0 !rounded-md">6 张默认套图</Tag><Tag className="m-0 !rounded-md">4K 默认</Tag><Tag className="m-0 !rounded-md">人工 QA</Tag></div></div>
                <div className="flex w-full min-w-0 items-center gap-2 lg:w-auto"><Select className="min-w-0 flex-1 lg:w-72" allowClear value={selectedRunId || undefined} options={workspace.runs.map((run) => ({ value: run.id, label: `${new Date(run.createdAt).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" })} · ${ecommerceRunStatusLabel(run.status)}` }))} placeholder="历史生产记录" onChange={(value) => value ? void loadRun(value) : (setSelectedRunId(""), setRunView(undefined))} /><Tooltip title="刷新运行状态"><Button icon={<RefreshCw className={`size-4 ${workspaceQuery.isFetching ? "animate-spin" : ""}`} />} aria-label="刷新运行状态" onClick={() => void workspaceQuery.refetch()} /></Tooltip></div>
            </header>

            <section aria-labelledby="ecommerce-config-title">
                <div className="grid gap-3 xl:grid-cols-[minmax(210px,1.25fr)_minmax(180px,1fr)_120px_104px_170px_auto] xl:items-end">
                    <label className="grid gap-1.5 text-xs"><span id="ecommerce-config-title" className="font-medium text-foreground/65">商拍预设</span><div className="flex gap-1.5"><Select className="min-w-0 flex-1" showSearch value={presetId || undefined} optionFilterProp="label" options={presetOptions(presets)} onChange={(value) => { const next = presets.find((preset) => preset.id === value); setPresetId(value); if (next) setCategory(next.category || "general"); }} /><Tooltip title={selectedPreset?.system ? "复制并编辑预设" : "编辑为新版本"}><Button icon={<Copy className="size-4" />} aria-label="编辑预设" disabled={!selectedPreset} onClick={() => setPresetEditorOpen(true)} /></Tooltip></div></label>
                    <label className="grid gap-1.5 text-xs"><span className="font-medium text-foreground/65">图片模型</span><Select value={selectedRoute || undefined} placeholder={readyRoutes.length ? "选择图片模型" : "没有可用模型"} options={readyRoutes.map((route) => ({ value: routeKey(route.channelId, route.model), label: `${route.channelName} · ${route.modelDisplayName}` }))} onChange={setSelectedRoute} /></label>
                    <label className="grid gap-1.5 text-xs"><span className="font-medium text-foreground/65">渠道</span><Select value={targetChannel} options={ecommerceChannelOptions} onChange={setTargetChannel} /></label>
                    <label className="grid gap-1.5 text-xs"><span className="font-medium text-foreground/65">比例</span><Select value={aspectRatio} options={ecommerceAspectOptions} onChange={setAspectRatio} /></label>
                    <label className="grid gap-1.5 text-xs"><span className="font-medium text-foreground/65">分辨率</span><Select value={resolution} options={resolutionOptions} onChange={setResolution} /></label>
                    <Button type="primary" className="!h-8 xl:!h-[32px]" icon={busyKey === "create-run" ? <LoaderCircle className="size-4 animate-spin" /> : <Sparkles className="size-4" />} loading={busyKey === "create-run"} disabled={archived || !productAssetIds.length || !selectedPreset} onClick={() => void createRun()}>生成系列套图</Button>
                </div>

                <div className="mt-5 grid gap-4 border-t border-border/65 pt-4 md:grid-cols-2 xl:grid-cols-5">
                    <AssetSelector label="主商品" required value={productAssetIds} options={assetOptions} maxCount={8} placeholder="正面、背面、细节图" onChange={setProductAssetIds} />
                    <AssetSelector label="搭配 / 包装" value={supportingAssetIds} options={assetOptions} maxCount={6} placeholder="搭配商品与包装" onChange={setSupportingAssetIds} />
                    <AssetSelector label="模特参考" value={modelAssetIds} options={assetOptions} maxCount={6} placeholder={selectedPreset?.kernel === "MODEL_INTERACTION" ? "留空则创建 AI 模特" : "当前为静物内核"} disabled={selectedPreset?.kernel !== "MODEL_INTERACTION"} onChange={setModelAssetIds} />
                    <AssetSelector label="场景参考" value={sceneAssetIds} options={assetOptions} maxCount={6} placeholder="宽景、局部、光线" onChange={setSceneAssetIds} />
                    <AssetSelector label="品牌包" value={brandAssetIds} options={assetOptions} maxCount={4} placeholder="Logo 与品牌参考" onChange={setBrandAssetIds} />
                </div>

                {productAssetIds.length ? <div className="mt-3 flex gap-2 overflow-x-auto pb-1">{productAssetIds.map((assetId, index) => { const asset = detail.assets.find((item) => item.id === assetId); const personal = personalAssets.find((item) => item.id === assetId); return <div key={assetId} className="relative h-16 w-20 shrink-0 overflow-hidden rounded-md border border-border/70 bg-foreground/[.04]"><AssetMediaPreview asset={personal} alt={asset?.title || "商品参考"} className="h-full w-full object-cover" fallback={<div className="grid h-full place-items-center"><ImageIcon className="size-5 text-foreground/25" /></div>} /><span className="absolute left-1 top-1 rounded bg-black/65 px-1 text-[10px] text-white">{index === 0 ? "主" : index + 1}</span></div>; })}</div> : null}

                <div className="mt-4 grid gap-3 lg:grid-cols-[minmax(0,1.5fr)_160px_180px] lg:items-end">
                    <label className="grid gap-1.5 text-xs"><span className="font-medium text-foreground/65">本次目标</span><Input value={userGoal} maxLength={500} onChange={(event) => setUserGoal(event.target.value)} /></label>
                    <label className="grid gap-1.5 text-xs"><span className="font-medium text-foreground/65">商品品类</span><Select value={category} options={ecommerceCategoryOptions} onChange={setCategory} /></label>
                    <label className="grid gap-1.5 text-xs"><span className="font-medium text-foreground/65">输出数量</span><InputNumber className="w-full" min={1} max={12} value={outputCount} addonAfter="张" onChange={(value) => setOutputCount(Math.min(12, Math.max(1, value || 6)))} /></label>
                </div>

                <details className="group mt-4 border-t border-border/65 pt-3">
                    <summary className="flex cursor-pointer list-none items-center justify-between gap-3 text-sm font-medium"><span className="flex items-center gap-2"><Settings2 className="size-4 text-foreground/48" />高级设置</span><ChevronDown className="size-4 text-foreground/40 transition-transform group-open:rotate-180" /></summary>
                    <div className="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                        <label className="grid gap-1.5 text-xs"><span className="font-medium text-foreground/60">商品颜色事实</span><Input value={productColor} onChange={(event) => setProductColor(event.target.value)} placeholder="只填写已确认颜色" /></label>
                        <label className="grid gap-1.5 text-xs"><span className="font-medium text-foreground/60">商品材质事实</span><Input value={productMaterial} onChange={(event) => setProductMaterial(event.target.value)} placeholder="只填写已确认材质" /></label>
                        <label className="grid gap-1.5 text-xs"><span className="font-medium text-foreground/60">必须保留信息</span><Input value={mustPreserve} onChange={(event) => setMustPreserve(event.target.value)} placeholder="结构、Logo 位置、包装文字" /></label>
                        {selectedPreset?.kernel === "MODEL_INTERACTION" ? <label className="grid gap-1.5 text-xs"><span className="font-medium text-foreground/60">AI 模特要求</span><Input.TextArea value={modelBrief} autoSize={{ minRows: 2, maxRows: 4 }} onChange={(event) => setModelBrief(event.target.value)} /></label> : null}
                        <label className="grid gap-1.5 text-xs"><span className="font-medium text-foreground/60">场景要求</span><Input.TextArea value={sceneBrief} autoSize={{ minRows: 2, maxRows: 4 }} placeholder={selectedPreset?.definition.sceneTemplate} onChange={(event) => setSceneBrief(event.target.value)} /></label>
                        <label className="grid gap-1.5 text-xs"><span className="font-medium text-foreground/60">品牌要求</span><Input.TextArea value={brandBrief} autoSize={{ minRows: 2, maxRows: 4 }} placeholder="品牌色、禁用项、卖点、必须保留信息" onChange={(event) => setBrandBrief(event.target.value)} /></label>
                        <label className="grid gap-1.5 text-xs"><span className="font-medium text-foreground/60">光线</span><Input value={lighting} onChange={(event) => setLighting(event.target.value)} /></label>
                        <label className="grid gap-1.5 text-xs"><span className="font-medium text-foreground/60">色彩</span><Input value={palette} onChange={(event) => setPalette(event.target.value)} /></label>
                        <label className="grid gap-1.5 text-xs"><span className="font-medium text-foreground/60">补充排除项</span><Input value={negativePrompt} onChange={(event) => setNegativePrompt(event.target.value)} /></label>
                    </div>
                    <div className="mt-4 flex items-center justify-between gap-3 border-t border-border/60 pt-3"><div><div className="text-sm font-medium">生成前审方案</div><div className="mt-0.5 text-xs text-foreground/45">开启后，Agent 规划完成会先暂停。</div></div><Switch checked={reviewBeforeGeneration} onChange={setReviewBeforeGeneration} /></div>
                </details>
            </section>

            {!readyRoutes.length ? <ProviderReadiness routes={workspace.providerRoutes} /> : null}

            {runView ? <section className="border-t border-border/70 pt-5" aria-labelledby="ecommerce-run-title">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between"><div><div className="flex items-center gap-2 text-xs font-semibold text-[var(--workspace-accent)]"><Bot className="size-3.5" />AGENT RUNTIME</div><h3 id="ecommerce-run-title" className="mt-1 text-lg font-semibold">{ecommerceRunStatusLabel(runView.run.status)}</h3><p className="mt-1 text-xs text-foreground/45">Run {runView.run.id} · {runView.run.kernel} · {runView.run.outputCount} 槽位 · {(runView.run.resolution || "auto").toUpperCase()} {runView.run.pixelSize ? `· ${runView.run.pixelSize.replace("x", "×")}` : ""}</p></div><div className="flex items-center gap-2"><Tag color={runView.run.status === "ready" ? "success" : runView.run.status === "failed" ? "error" : runView.run.status === "needs_you" ? "warning" : runView.run.status === "cancelled" ? "default" : "processing"} className="m-0 !rounded-md">{progress}%</Tag>{!["ready", "failed", "cancelled"].includes(runView.run.status) ? <Button size="small" danger type="text" icon={<StopCircle className="size-3.5" />} loading={busyKey === "cancel-run"} onClick={cancelRun}>取消生产</Button> : null}</div></div>
                <Progress className="mt-3" percent={progress} showInfo={false} />

                <AgentPlan artifacts={workspace.artifacts} runView={runView} />

                {runView.run.error ? <Alert className="mt-4" type="warning" showIcon message="当前生产需要处理" description={runView.run.error} /> : null}
                {runView.run.status === "awaiting_review" ? <div className="mt-4 flex flex-col gap-3 rounded-lg border border-[var(--workspace-accent)]/25 bg-[var(--workspace-accent-soft)] p-4 sm:flex-row sm:items-center sm:justify-between"><div><div className="text-sm font-semibold">方案等待确认</div><div className="mt-1 text-xs text-foreground/50">确认后进入费用报价，不会立即调用 Provider。</div></div><Button type="primary" icon={<Check className="size-4" />} loading={busyKey === "approve-plan"} onClick={() => void approvePlan()}>确认 Agent 方案</Button></div> : null}

                {(runView.run.status === "awaiting_cost" || (runView.run.status === "needs_you" && readyRoutes.length > 0)) ? <div className="mt-4 grid gap-4 rounded-lg border border-amber-500/30 bg-amber-500/[.055] p-4 lg:grid-cols-[minmax(0,1fr)_minmax(220px,0.7fr)_auto] lg:items-end"><div><div className="flex items-center gap-2 text-sm font-semibold"><Coins className="size-4 text-amber-600" />费用确认</div>{runView.quote ? <div className="mt-2 flex flex-wrap gap-x-5 gap-y-1 text-sm"><span>模型 <strong>{runView.quote.model}</strong></span><span>输出 <strong>{(runView.quote.resolution || "auto").toUpperCase()} · {(runView.quote.pixelSize || runView.run.pixelSize || "按模型默认").replace("x", "×")}</strong></span><span>{runView.quote.count} 张 × {formatCredits(runView.quote.unitMicrocredits)} 积分</span><span>预计总价 <strong className="tabular-nums">{formatCredits(runView.quote.totalMicrocredits)} 积分</strong></span></div> : <p className="mt-2 text-sm text-foreground/52">选择可用模型并获取报价。</p>}{runView.quote ? <p className={`mt-1 text-xs ${quoteExpired ? "text-red-500" : "text-foreground/45"}`}>{quoteExpired ? "报价已过期，请刷新" : `有效至 ${new Date(runView.quote.expiresAt).toLocaleTimeString("zh-CN")}`}</p> : null}</div><label className="grid gap-1.5 text-xs"><span className="font-medium text-foreground/60">报价模型</span><Select value={selectedRoute || undefined} options={readyRoutes.map((route) => ({ value: routeKey(route.channelId, route.model), label: `${route.channelName} · ${route.modelDisplayName} · ${formatCredits(route.unitPriceMicrocredits)} 积分/张` }))} onChange={setSelectedRoute} /></label><div className="flex gap-2"><Button icon={<RefreshCw className="size-4" />} loading={busyKey === "refresh-quote"} disabled={!selectedRoute} onClick={() => void refreshQuote()}>{runView.quote ? "刷新报价" : "获取报价"}</Button>{runView.quote && !quoteExpired && runView.run.status === "awaiting_cost" ? <Button type="primary" danger icon={<Coins className="size-4" />} loading={busyKey === "submit-run"} onClick={() => void submitRun()}>确认费用并生成</Button> : null}</div></div> : null}
            </section> : null}

            {runView && ["generating", "qa", "needs_you", "ready", "failed"].includes(runView.run.status) ? <EcommerceRunResults projectId={projectId} runView={runView} archived={archived} audioOptions={audioOptions} onRunChange={applyRun} onOpenCanvas={onCreateCanvas} /> : null}

            <EcommercePresetEditor open={presetEditorOpen} projectId={projectId} preset={selectedPreset} onClose={() => setPresetEditorOpen(false)} onSaved={(preset) => { setPresetEditorOpen(false); setPresetId(preset.id); void queryClient.invalidateQueries({ queryKey: ["ecommerce-workspace", projectId] }); }} />
        </div>
    );
}

function AssetSelector({ label, required = false, value, options, maxCount, placeholder, disabled, onChange }: { label: string; required?: boolean; value: string[]; options: Array<{ value: string; label: string }>; maxCount: number; placeholder: string; disabled?: boolean; onChange: (value: string[]) => void }) {
    return <label className="grid min-w-0 gap-1.5 text-xs"><span className="font-medium text-foreground/65">{label}{required ? <span className="ml-1 text-red-500">*</span> : null}</span><Select mode="multiple" maxCount={maxCount} maxTagCount="responsive" allowClear showSearch value={value} options={options} placeholder={placeholder} optionFilterProp="label" disabled={disabled} onChange={onChange} /></label>;
}

function ProviderReadiness({ routes }: { routes: NonNullable<Awaited<ReturnType<typeof getProjectEcommerceWorkspace>>["workspace"]["providerRoutes"]> }) {
    return <Alert type="warning" showIcon icon={<CircleAlert className="size-4" />} message="尚无可用于电商套图的图片模型" description={<div className="mt-2 space-y-2">{routes.length ? routes.slice(0, 6).map((route) => <div key={`${route.channelId}:${route.model}`} className="flex flex-col gap-1 text-xs sm:flex-row sm:items-start sm:justify-between"><span className="font-medium">{route.channelName} · {route.modelDisplayName}</span><span className="text-foreground/50">{route.blockers.join("；") || "模型未就绪"}</span></div>) : <span className="text-xs">请在渠道设置中添加图片模型、价格和至少 3 张参考图能力。</span>}<a href="/settings" className="inline-flex text-xs font-medium text-[var(--workspace-accent)]">打开渠道设置</a></div>} />;
}

function AgentPlan({ artifacts, runView }: { artifacts: EcommerceArtifact[]; runView: EcommerceRunView }) {
    const runArtifacts = activeAgentArtifacts.map((definition) => ({ ...definition, artifact: latestRunArtifact(artifacts, runView.run.id, definition.type) }));
    const productDNA = parseJSONObject(latestRunArtifact(artifacts, runView.run.id, "product_dna")?.payloadJson);
    const scenePack = parseJSONObject(latestRunArtifact(artifacts, runView.run.id, "scene_pack")?.payloadJson);
    const modelProfile = parseJSONObject(latestRunArtifact(artifacts, runView.run.id, "model_profile")?.payloadJson);
    return <div className="mt-4">
        <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-4">{runArtifacts.map(({ type, label, agent, icon: Icon, artifact }) => <div key={type} className="flex min-h-16 items-center gap-3 rounded-lg border border-border/70 px-3 py-2"><span className={`grid size-8 shrink-0 place-items-center rounded-md ${artifact ? "bg-emerald-500/10 text-emerald-600" : "bg-foreground/[.05] text-foreground/35"}`}>{artifact ? <Check className="size-4" /> : <Icon className="size-4" />}</span><div className="min-w-0"><div className="truncate text-sm font-medium">{label}</div><div className="truncate text-[11px] text-foreground/43">{agent}{artifact ? ` · r${artifact.revision}` : ""}</div></div></div>)}</div>
        <details className="group mt-3 border-t border-border/60 pt-3"><summary className="flex cursor-pointer list-none items-center justify-between text-sm font-medium"><span className="flex items-center gap-2"><Database className="size-4 text-foreground/45" />展开生产方案与逐镜 Prompt</span><ChevronDown className="size-4 text-foreground/40 transition-transform group-open:rotate-180" /></summary><div className="mt-4 grid gap-4 xl:grid-cols-3"><PlanFact title="ProductDNA" icon={<Package className="size-4" />} rows={[factRow("品类", productDNA.category), factRow("事实", productDNA.recordedFacts), factRow("保真", productDNA.mustPreserve)]} /><PlanFact title="Model Profile" icon={<UserRound className="size-4" />} rows={[factRow("模式", modelProfile.mode || runView.run.modelMode), factRow("状态", modelProfile.status), factRow("要求", modelProfile.brief || runView.run.modelBrief)]} /><PlanFact title="Scene Pack" icon={<Layers3 className="size-4" />} rows={[factRow("场景", scenePack.brief || runView.run.sceneBrief), factRow("光线", scenePack.lighting), factRow("色彩", scenePack.palette)]} /></div><div className="mt-4 divide-y divide-border/60 border-y border-border/60">{runView.slots.map(({ slot }) => <details key={slot.id} className="py-2"><summary className="grid cursor-pointer grid-cols-[32px_minmax(0,1fr)_auto] items-center gap-2 text-sm"><span className="grid size-6 place-items-center rounded bg-foreground/[.055] text-xs tabular-nums">{slot.position}</span><span className="truncate font-medium">{slot.title}</span><span className="text-xs text-foreground/42">{slot.role}</span></summary><pre className="mt-2 max-h-64 overflow-auto whitespace-pre-wrap rounded-lg bg-foreground/[.035] p-3 font-sans text-xs leading-5 text-foreground/58">{slot.prompt}</pre></details>)}</div></details>
    </div>;
}

function PlanFact({ title, icon, rows }: { title: string; icon: React.ReactNode; rows: string[] }) {
    return <div className="min-w-0"><div className="flex items-center gap-2 text-sm font-semibold">{icon}{title}</div><div className="mt-2 space-y-1.5">{rows.filter(Boolean).map((row) => <p key={row} className="line-clamp-3 text-xs leading-5 text-foreground/52">{row}</p>)}</div></div>;
}

function factRow(label: string, value: unknown) {
    if (value === undefined || value === null || value === "") return "";
    const text = typeof value === "string" ? value : JSON.stringify(value);
    return `${label}：${text}`;
}

function presetOptions(presets: EcommercePreset[]) {
    return presets.map((preset) => ({ value: preset.id, label: `${preset.kernel === "MODEL_INTERACTION" ? "人物互动" : "静物商拍"} · ${preset.name}${preset.system ? "" : ` · v${preset.version}`}` }));
}

function ecommerceRunProgress(runView: EcommerceRunView) {
    if (runView.run.status === "awaiting_review") return 20;
    if (runView.run.status === "awaiting_cost") return 30;
    if (!runView.slots.length) return runView.run.status === "ready" ? 100 : 5;
    const weights: Record<string, number> = { planned: 0, scheduled: 8, queued: 15, running: 45, qa: 82, failed: 100, cancelled: 100, accepted: 100 };
    return Math.round(runView.slots.reduce((sum, item) => sum + (weights[item.slot.status] ?? 0), 0) / runView.slots.length);
}

function createIdempotencyKey(projectId: string) {
    const random = globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(36).slice(2)}`;
    return `ecommerce:${projectId}:${random}`;
}
