import { AlertTriangle, Boxes, CheckCircle2, Film, Image, Layers3, LoaderCircle, PackageSearch, ScanSearch, Sparkles, UserRound, WandSparkles } from "lucide-react";

import { CanvasNodeType, type CanvasNodeData } from "@/types/canvas";

const labels: Record<string, string> = {
    production_frame: "ECOMMERCE PRODUCTION",
    product_input: "PRODUCT INPUT",
    product_dna: "PRODUCT DNA",
    model_profile: "MODEL PROFILE",
    scene_pack: "SCENE PACK",
    preset_snapshot: "PRESET SNAPSHOT",
    presentation_mode: "PRESENTATION MODE",
    creative_direction: "CREATIVE DIRECTION",
    scene_plan: "SCENE PLAN",
    creative_shot_plan: "SHOT PLAN",
    shot_plan: "SHOT PLAN",
    generation: "GENERATION",
    generation_request: "GENERATION REQUEST",
    result: "RESULT GRID",
    generated_asset: "GENERATED ASSET",
    qc: "QUALITY CONTROL",
    qa_report: "QUALITY CONTROL",
    motion_plan: "MOTION PLAN",
    video_sequence: "VIDEO SEQUENCE",
    needs_you: "NEEDS YOU",
};

export function EcommerceNodeCard({ node }: { node: CanvasNodeData }) {
    if (node.ecommerceKind === "generated_asset") return <GeneratedAssetCard node={node} />;
    if (node.ecommerceKind === "video_sequence") return <VideoSequenceCard node={node} />;
    const state = node.ecommerceState;
    const Icon = node.ecommerceKind === "product_input" || node.ecommerceKind === "product_dna"
        ? PackageSearch
        : node.ecommerceKind === "model_profile"
          ? UserRound
          : node.ecommerceKind === "scene_pack"
            ? Layers3
            : node.ecommerceKind === "creative_direction" || node.ecommerceKind === "shot_plan"
              ? WandSparkles
        : node.ecommerceKind === "generation"
          ? Sparkles
          : node.ecommerceKind === "generation_request"
            ? Sparkles
          : node.ecommerceKind === "result"
            ? Image
            : node.ecommerceKind === "qc" || node.ecommerceKind === "qa_report"
              ? ScanSearch
              : Boxes;
    const AttentionIcon = state?.attention === "none" ? CheckCircle2 : AlertTriangle;

    return (
        <div className="flex h-full w-full flex-col overflow-hidden p-4">
            <div className="flex items-center justify-between gap-2 text-[var(--fs-tiny)] font-semibold text-foreground/42">
                <span className="inline-flex items-center gap-1.5"><Icon className="size-3.5" />{labels[node.ecommerceKind || ""] || node.ecommerceKind}</span>
                <span className="inline-flex items-center gap-1"><AttentionIcon className={`size-3 ${state?.attention === "none" ? "text-emerald-400" : "text-amber-400"}`} />{state?.evidence || "unknown"}</span>
            </div>
            <div className="mt-2 truncate text-sm font-semibold text-foreground/88" title={node.title}>{node.title}</div>
            <p className="mt-3 line-clamp-3 whitespace-pre-line text-[var(--fs-caption)] leading-5 text-foreground/58">{node.metadata?.content || "等待电商领域事实"}</p>
            <div className="mt-auto flex items-center justify-between gap-2 border-t border-border/60 pt-2 text-[var(--fs-micro)] text-foreground/38">
                <span>{node.ecommerceRef?.artifactRevision ? `v${node.ecommerceRef.artifactRevision}` : state?.lifecycle || "draft"}</span>
                <span>{state?.production || "not_started"}</span>
            </div>
        </div>
    );
}

function GeneratedAssetCard({ node }: { node: CanvasNodeData }) {
    const state = node.ecommerceState;
    const imageURL = node.type === CanvasNodeType.Image ? node.metadata?.content : "";
    const loading = state?.production === "scheduled" || state?.production === "queued" || state?.production === "running";
    const statusLabel = productionLabel(state?.production);
    return (
        <div className="relative h-full w-full overflow-hidden bg-foreground/[.035]">
            {imageURL ? <img src={imageURL} alt={node.title} className="h-full w-full object-cover" loading="lazy" decoding="async" draggable={false} /> : (
                <div className="grid h-full place-items-center px-4 text-center text-foreground/35">
                    <div>
                        {loading ? <LoaderCircle className="mx-auto size-7 animate-spin" /> : <Image className="mx-auto size-7" />}
                        <p className="mt-2 text-[var(--fs-micro)]">{statusLabel}</p>
                    </div>
                </div>
            )}
            <div className="absolute inset-x-2 top-2 flex items-center justify-between gap-2">
                <span className="max-w-[70%] truncate rounded bg-black/65 px-2 py-1 text-[var(--fs-micro)] font-semibold text-white">{node.title}</span>
                <span className={`shrink-0 rounded px-2 py-1 text-[var(--fs-micro)] font-semibold ${state?.attention === "error" ? "bg-red-500 text-white" : state?.production === "approved" ? "bg-emerald-500 text-white" : "bg-background/90 text-foreground"}`}>{statusLabel}</span>
            </div>
            <div className="absolute inset-x-2 bottom-2 flex items-center justify-between gap-2 rounded bg-black/65 px-2 py-1 text-[var(--fs-micro)] text-white">
                <span className="truncate">{node.metadata?.workflowDescription || "等待生成结果"}</span>
                <span className="shrink-0">{node.ecommerceRef?.attemptId ? "有版本记录" : "待执行"}</span>
            </div>
        </div>
    );
}

function VideoSequenceCard({ node }: { node: CanvasNodeData }) {
    const ready = Boolean(node.ecommerceRef?.artifactId);
    return (
        <div className="flex h-full w-full overflow-hidden">
            <div className="grid w-36 shrink-0 place-items-center border-r border-border/60 bg-foreground/[.035]">
                <Film className={`size-9 ${ready ? "text-emerald-500" : "text-foreground/25"}`} />
            </div>
            <div className="flex min-w-0 flex-1 flex-col p-4">
                <div className="text-[var(--fs-tiny)] font-semibold text-foreground/42">VIDEO SEQUENCE</div>
                <div className="mt-2 truncate text-sm font-semibold" title={node.title}>{node.title}</div>
                <p className="mt-2 line-clamp-3 whitespace-pre-line text-[var(--fs-caption)] leading-5 text-foreground/58">{node.metadata?.workflowDescription}</p>
                <div className="mt-auto flex items-center justify-between border-t border-border/60 pt-2 text-[var(--fs-micro)] text-foreground/42">
                    <span>{node.metadata?.seconds || "15"} 秒</span>
                    <span>{ready ? "计划已保存" : "等待已接受图片"}</span>
                </div>
            </div>
        </div>
    );
}

function productionLabel(status?: string) {
    return ({
        not_started: "待执行",
        ready: "已就绪",
        scheduled: "等待调度",
        queued: "排队中",
        running: "生成中",
        generated: "已生成",
        qa: "待质检",
        qc_failed: "QA 未通过",
        approved: "已接受",
        failed: "生成失败",
        cancelled: "已取消",
    } as Record<string, string>)[status || ""] || status || "待执行";
}
