import type { CanvasNodeData } from "@/types/canvas";

type CanvasLibraryDocument = {
    projectId?: string;
    nodes: CanvasNodeData[];
};

/**
 * The generic canvas library owns only independent documents. Project-linked
 * and legacy domain documents stay in their Film or ecommerce workspaces.
 */
export function isFreeformCanvasDocument(canvas: CanvasLibraryDocument) {
    if (canvas.projectId) return false;
    return !canvas.nodes.some((node) => {
        const skillDomain = node.metadata?.skillDomain;
        return skillDomain === "film" || skillDomain === "ecommerce" || Boolean(node.ecommerceKind || node.ecommerceRef || node.ecommerceState);
    });
}

export function freeformCanvasDocuments<T extends CanvasLibraryDocument>(canvases: readonly T[]) {
    return canvases.filter(isFreeformCanvasDocument);
}
