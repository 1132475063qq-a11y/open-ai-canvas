import { useMemo, useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { App, Button, Select, Tag } from "antd";
import { Image as ImageIcon, Link2, Package, Plus, Trash2 } from "lucide-react";

import { AssetMediaPreview } from "@/components/asset-media-preview";
import { WorkspaceState } from "@/components/layout/workspace-state";
import { linkProjectAsset, unlinkProjectAsset, updateProjectAssetRole, type ProjectAsset } from "@/services/api/projects";
import { useAssetStore, type Asset } from "@/stores/use-asset-store";

import { mediaLabel, StatusPill, type ProjectDetailViewProps } from "./shared";

const ecommerceAssetRoles = [
    { value: "product_primary", label: "主商品" },
    { value: "product_front", label: "商品正面" },
    { value: "product_back", label: "商品背面" },
    { value: "product_detail", label: "商品细节" },
    { value: "product_supporting", label: "搭配商品" },
    { value: "packaging", label: "包装" },
    { value: "logo", label: "Logo" },
    { value: "model_reference", label: "授权模特" },
    { value: "scene_reference", label: "场景参考" },
    { value: "brand_reference", label: "品牌参考" },
];

const ecommerceAssetRoleLabels = Object.fromEntries(ecommerceAssetRoles.map((item) => [item.value, item.label]));

export default function EcommerceAssetsView({ detail, refreshProject }: ProjectDetailViewProps) {
    const { message } = App.useApp();
    const personalAssets = useAssetStore((state) => state.assets);
    const [assetId, setAssetId] = useState("");
    const [role, setRole] = useState("product_primary");
    const projectAssetIds = useMemo(() => new Set(detail.assets.map((asset) => asset.id)), [detail.assets]);
    const availableAssets = personalAssets.filter((asset) => !projectAssetIds.has(asset.id));
    const selectedAsset = personalAssets.find((asset) => asset.id === assetId);
    const addMutation = useMutation({
        mutationFn: () => {
            if (!assetId) throw new Error("请选择一个商品素材");
            return linkProjectAsset(detail.project.id, { assetId, category: selectedAsset?.category || "other", role });
        },
        onSuccess: () => { setAssetId(""); refreshProject(); message.success("商品素材已加入项目"); },
        onError: (error) => message.error(error instanceof Error ? error.message : "商品素材加入失败"),
    });
    const removeMutation = useMutation({
        mutationFn: (id: string) => unlinkProjectAsset(detail.project.id, id),
        onSuccess: () => { refreshProject(); message.success("商品素材已移出项目"); },
        onError: (error) => message.error(error instanceof Error ? error.message : "商品素材移除失败"),
    });
    const roleMutation = useMutation({
        mutationFn: ({ id, nextRole }: { id: string; nextRole: string }) => updateProjectAssetRole(detail.project.id, id, nextRole),
        onSuccess: () => { refreshProject(); message.success("项目内资产角色已更新"); },
        onError: (error) => message.error(error instanceof Error ? error.message : "资产角色更新失败"),
    });

    return (
        <div className="space-y-5">
            <header className="flex flex-col gap-3 border-b border-border/70 pb-4 sm:flex-row sm:items-end sm:justify-between">
                <div><div className="flex items-center gap-2 text-xs font-semibold text-[var(--workspace-accent)]"><Package className="size-3.5" />ECOMMERCE ASSETS</div><h2 className="mt-1 text-xl font-semibold">商品、模特、场景与品牌资产</h2></div>
                <div className="grid w-full gap-2 sm:w-[560px] sm:grid-cols-[minmax(0,1fr)_140px_auto]"><Select className="min-w-0" showSearch allowClear value={assetId || undefined} options={availableAssets.map((asset) => ({ label: `${asset.title} · ${mediaLabel(asset.kind)}`, value: asset.id }))} placeholder="从个人素材库选择图片" optionFilterProp="label" onChange={(value) => setAssetId(value || "")} /><Select value={role} options={ecommerceAssetRoles} onChange={setRole} /><Button type="primary" icon={<Plus className="size-3.5" />} disabled={!selectedAsset || selectedAsset.kind !== "image"} loading={addMutation.isPending} onClick={() => addMutation.mutate()}>加入</Button></div>
            </header>
            {!availableAssets.length && !detail.assets.length ? <p className="rounded-lg border border-dashed border-border/80 px-4 py-5 text-sm leading-6 text-foreground/48">个人素材库里暂时没有可引用的商品图。可以先回到“素材”上传，再回到这里加入项目。</p> : null}
            {detail.assets.length ? <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">{detail.assets.map((asset) => <EcommerceAssetCard key={asset.id} asset={asset} personalAsset={personalAssets.find((item) => item.id === asset.id)} removing={removeMutation.isPending && removeMutation.variables === asset.id} roleUpdating={roleMutation.isPending && roleMutation.variables?.id === asset.id} onRoleChange={(nextRole) => roleMutation.mutate({ id: asset.id, nextRole })} onRemove={() => removeMutation.mutate(asset.id)} />)}</div> : <WorkspaceState icon="assets" title="还没有电商资产" description="从个人素材库加入第一张主商品图片。" />}
        </div>
    );
}

function EcommerceAssetCard({ asset, personalAsset, removing, roleUpdating, onRoleChange, onRemove }: { asset: ProjectAsset; personalAsset?: Asset; removing: boolean; roleUpdating: boolean; onRoleChange: (role: string) => void; onRemove: () => void }) {
    return <article className="overflow-hidden rounded-lg border border-border/70 bg-background/70 shadow-sm"><div className="relative aspect-[4/3] bg-foreground/[.045]"><AssetMediaPreview asset={personalAsset} alt={asset.title} className="h-full w-full object-cover" fallback={<div className="grid h-full place-items-center text-foreground/25"><ImageIcon className="size-9" /></div>} /><div className="absolute inset-x-2 top-2 flex items-center justify-between"><StatusPill status={asset.status} /><Tag className="m-0 !rounded-md !border-0 !bg-black/60 !text-white">{ecommerceAssetRoleLabels[asset.projectRole || ""] || "未分组"}</Tag></div></div><div className="p-3"><div className="flex items-start justify-between gap-2"><div className="min-w-0"><h3 className="truncate text-sm font-semibold" title={asset.title}>{asset.title}</h3><p className="mt-1 truncate text-xs text-foreground/45" title={asset.id}>{asset.id}</p></div><Link2 className="size-4 shrink-0 text-[var(--workspace-accent)]" /></div><Select className="mt-3 w-full" size="small" value={asset.projectRole || undefined} placeholder="选择资产角色" options={ecommerceAssetRoles} loading={roleUpdating} onChange={onRoleChange} /><div className="mt-3 flex items-center justify-between border-t border-border/60 pt-2 text-xs text-foreground/45"><span>{mediaLabel(asset.mediaType)} · v{Math.max(1, asset.versionCount)}</span><Button type="text" danger size="small" icon={<Trash2 className="size-3.5" />} loading={removing} onClick={onRemove}>移出</Button></div></div></article>;
}
