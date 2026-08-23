import type { CanvasNodeData } from "@/types/canvas";

type CanvasDocumentIdentity = {
    projectId?: string;
    nodes: CanvasNodeData[];
};

export function isEcommerceCanvasDocument(canvas: CanvasDocumentIdentity, ecommerceProjectIds: ReadonlySet<string>) {
    if (canvas.projectId && ecommerceProjectIds.has(canvas.projectId)) return true;
    return canvas.nodes.some((node) => Boolean(node.ecommerceKind || node.ecommerceRef || node.metadata?.skillDomain === "ecommerce"));
}
