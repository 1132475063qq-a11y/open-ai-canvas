import { expect, test } from "bun:test";

import { freeformCanvasDocuments, isFreeformCanvasDocument } from "../src/lib/canvas/canvas-library-domain";
import { isFilmWorkspace } from "../src/lib/canvas/film-workspace";
import { CanvasNodeType, type CanvasNodeData } from "../src/types/canvas";

function textNode(id: string, patch: Partial<CanvasNodeData> = {}): CanvasNodeData {
    return {
        id,
        type: CanvasNodeType.Text,
        title: id,
        position: { x: 0, y: 0 },
        width: 320,
        height: 180,
        ...patch,
    };
}

test("freeform canvas does not enter the Film workspace", () => {
    expect(isFilmWorkspace({ featureEnabled: true })).toBe(false);
    expect(isFilmWorkspace({ featureEnabled: true, projectId: "project-1", projectType: "" })).toBe(false);
});

test("Film controls require a short-drama project context", () => {
    expect(isFilmWorkspace({ featureEnabled: false, projectId: "project-1", projectType: "short-drama" })).toBe(false);
    expect(isFilmWorkspace({ featureEnabled: true, projectId: "project-1", projectType: "ecommerce" })).toBe(false);
    expect(isFilmWorkspace({ featureEnabled: true, projectId: "project-1", projectType: "short-drama" })).toBe(true);
});

test("generic canvas library contains only independent freeform documents", () => {
    const freeform = { id: "freeform", nodes: [textNode("shared", { metadata: { skillDomain: "shared" } })] };
    const filmProject = { id: "film-project", projectId: "project-film", nodes: [] };
    const ecommerceProject = { id: "ecommerce-project", projectId: "project-ecommerce", nodes: [] };

    expect(isFreeformCanvasDocument(freeform)).toBe(true);
    expect(isFreeformCanvasDocument(filmProject)).toBe(false);
    expect(isFreeformCanvasDocument(ecommerceProject)).toBe(false);
    expect(freeformCanvasDocuments([filmProject, freeform, ecommerceProject]).map((canvas) => canvas.id)).toEqual(["freeform"]);
});

test("generic canvas library rejects orphaned legacy Film and ecommerce semantics", () => {
    const legacyFilm = { id: "legacy-film", nodes: [textNode("film", { metadata: { skillDomain: "film" } })] };
    const legacyEcommerceSkill = { id: "legacy-ecommerce-skill", nodes: [textNode("ecommerce-skill", { metadata: { skillDomain: "ecommerce" } })] };
    const legacyEcommerceNode = { id: "legacy-ecommerce-node", nodes: [textNode("ecommerce-node", { ecommerceKind: "product_dna" })] };

    expect(isFreeformCanvasDocument(legacyFilm)).toBe(false);
    expect(isFreeformCanvasDocument(legacyEcommerceSkill)).toBe(false);
    expect(isFreeformCanvasDocument(legacyEcommerceNode)).toBe(false);
    expect(freeformCanvasDocuments([legacyFilm, legacyEcommerceSkill, legacyEcommerceNode])).toEqual([]);
});
