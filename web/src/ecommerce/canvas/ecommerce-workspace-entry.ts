import type { ProjectAsset } from "@/services/api/projects";
import type { CanvasNodeData } from "@/types/canvas";

export type EcommerceWorkspaceAssetSelection = {
    productAssetIds: string[];
    supportingAssetIds: string[];
    modelAssetIds: string[];
    sceneAssetIds: string[];
    brandAssetIds: string[];
};

export type EcommerceWorkspaceAssetSelectionPatch = Partial<EcommerceWorkspaceAssetSelection>;

export const ECOMMERCE_WORKSPACE_ASSET_QUERY_KEYS = ["productAssetIds", "supportingAssetIds", "modelAssetIds", "sceneAssetIds", "brandAssetIds"] as const;

const ecommerceSelectionKeyByRole: Record<string, keyof EcommerceWorkspaceAssetSelection> = {
    product_primary: "productAssetIds",
    product_front: "productAssetIds",
    product_back: "productAssetIds",
    product_detail: "productAssetIds",
    product_supporting: "supportingAssetIds",
    packaging: "supportingAssetIds",
    model_reference: "modelAssetIds",
    scene_reference: "sceneAssetIds",
    logo: "brandAssetIds",
    brand_reference: "brandAssetIds",
};

export function emptyEcommerceWorkspaceAssetSelection(): EcommerceWorkspaceAssetSelection {
    return {
        productAssetIds: [],
        supportingAssetIds: [],
        modelAssetIds: [],
        sceneAssetIds: [],
        brandAssetIds: [],
    };
}

export function ecommerceSelectionKeyForRole(role?: string): keyof EcommerceWorkspaceAssetSelection | undefined {
    return ecommerceSelectionKeyByRole[role || ""];
}

export function ecommerceWorkspaceSelectionFromNodes(nodes: Pick<CanvasNodeData, "ecommerceRef" | "metadata">[], assets: Pick<ProjectAsset, "id" | "mediaType" | "projectRole">[]): EcommerceWorkspaceAssetSelection {
    const selection = emptyEcommerceWorkspaceAssetSelection();
    const assetsById = new Map(assets.filter((asset) => asset.mediaType === "image").map((asset) => [asset.id, asset]));

    for (const node of nodes) {
        const assetId = node.ecommerceRef?.assetId || node.metadata?.assetId;
        const asset = assetId ? assetsById.get(assetId) : undefined;
        const key = ecommerceSelectionKeyForRole(asset?.projectRole);
        if (asset && key && !selection[key].includes(asset.id)) selection[key].push(asset.id);
    }

    return selection;
}

export function ecommerceWorkspaceSelectionSearchParams(selection: EcommerceWorkspaceAssetSelection): URLSearchParams {
    const params = new URLSearchParams();
    for (const key of ECOMMERCE_WORKSPACE_ASSET_QUERY_KEYS) {
        for (const assetId of uniqueNonEmpty(selection[key])) params.append(key, assetId);
    }
    return params;
}

export function ecommerceWorkspaceSelectionFromSearchParams(params: Pick<URLSearchParams, "getAll">, allowedAssetIds?: ReadonlySet<string>): EcommerceWorkspaceAssetSelectionPatch {
    const selection: EcommerceWorkspaceAssetSelectionPatch = {};
    for (const key of ECOMMERCE_WORKSPACE_ASSET_QUERY_KEYS) {
        const values = uniqueNonEmpty(params.getAll(key)).filter((assetId) => !allowedAssetIds || allowedAssetIds.has(assetId));
        if (values.length) selection[key] = values;
    }
    return selection;
}

export function mergeEcommerceWorkspaceAssetSelection(defaults: EcommerceWorkspaceAssetSelection, patch: EcommerceWorkspaceAssetSelectionPatch): EcommerceWorkspaceAssetSelection {
    return {
        productAssetIds: patch.productAssetIds || defaults.productAssetIds,
        supportingAssetIds: patch.supportingAssetIds || defaults.supportingAssetIds,
        modelAssetIds: patch.modelAssetIds || defaults.modelAssetIds,
        sceneAssetIds: patch.sceneAssetIds || defaults.sceneAssetIds,
        brandAssetIds: patch.brandAssetIds || defaults.brandAssetIds,
    };
}

function uniqueNonEmpty(values: string[]) {
    return [...new Set(values.map((value) => value.trim()).filter(Boolean))];
}
