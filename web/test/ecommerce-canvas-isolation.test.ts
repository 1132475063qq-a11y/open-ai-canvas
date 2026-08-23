import { describe, expect, test } from "bun:test";

import { isEcommerceCanvasDocument } from "../src/ecommerce/canvas/ecommerce-canvas-isolation";
import { createEcommerceCanvasProjection, reconcileEcommerceCanvasProjection } from "../src/ecommerce/canvas/ecommerce-canvas-projection";
import { ecommerceWorkspaceSelectionFromNodes, ecommerceWorkspaceSelectionFromSearchParams, ecommerceWorkspaceSelectionSearchParams, mergeEcommerceWorkspaceAssetSelection } from "../src/ecommerce/canvas/ecommerce-workspace-entry";
import { isFilmWorkspace } from "../src/lib/canvas/film-workspace";
import { ECOMMERCE_PROJECT_SETTING_KEYS, projectDetailDefaultView, projectDetailPath, projectDetailViewKeys, projectHubPath } from "../src/lib/project-domain";
import type { ProjectDetail } from "../src/services/api/projects";
import { CanvasNodeType } from "../src/types/canvas";

function detail(projectType = "ecommerce"): ProjectDetail {
    return {
        project: {
            id: "project-ecommerce",
            userId: "user-1",
            name: "Commerce Test",
            type: projectType,
            aspectRatio: "9:16",
            sourceType: "blank",
            description: "",
            stylePresetId: "",
            status: "active",
            revision: 1,
            createdAt: "2026-08-22T00:00:00Z",
            updatedAt: "2026-08-22T00:00:00Z",
        },
        units: [],
        canvases: [],
        canvasUnitLinks: [],
        assets: [
            {
                id: "asset-product",
                title: "Product front",
                mediaType: "image",
                category: "product",
                projectRole: "product_primary",
                status: "ready",
                versionCount: 1,
                usages: [],
                position: 0,
                updatedAt: "2026-08-22T00:00:00Z",
            },
        ],
        assetFolders: [],
        workflows: [],
        shots: [],
        ecommerceArtifacts: [],
        shotReferences: [],
        assetCandidates: [],
    };
}

test("ecommerce projection contains only ecommerce nodes and stable connections", () => {
    const projection = createEcommerceCanvasProjection(detail());
    expect(projection.nodes.length).toBe(9);
    expect(new Set(projection.nodes.map((node) => node.id)).size).toBe(projection.nodes.length);
    expect(projection.nodes.every((node) => node.metadata?.skillDomain === "ecommerce")).toBe(true);
    expect(projection.connections.every((connection) => connection.id.includes(":ecommerce:edge:"))).toBe(true);
});

test("selected ecommerce asset nodes round-trip into a role-scoped workspace entry", () => {
    const assets = [
        ...detail().assets,
        { ...detail().assets[0], id: "asset-model", projectRole: "model_reference" },
        { ...detail().assets[0], id: "asset-scene", projectRole: "scene_reference" },
        { ...detail().assets[0], id: "asset-logo", projectRole: "logo" },
    ];
    const selection = ecommerceWorkspaceSelectionFromNodes(
        [
            { ecommerceRef: { projectId: "project-ecommerce", assetId: "asset-product" } },
            { metadata: { assetId: "asset-model" } },
            { ecommerceRef: { projectId: "project-ecommerce", assetId: "asset-scene" } },
            { metadata: { assetId: "asset-logo" } },
            { metadata: { assetId: "asset-outside-project" } },
        ],
        assets,
    );

    expect(selection.productAssetIds).toEqual(["asset-product"]);
    expect(selection.modelAssetIds).toEqual(["asset-model"]);
    expect(selection.sceneAssetIds).toEqual(["asset-scene"]);
    expect(selection.brandAssetIds).toEqual(["asset-logo"]);

    const params = ecommerceWorkspaceSelectionSearchParams(selection);
    params.append("productAssetIds", "asset-outside-project");
    const parsed = ecommerceWorkspaceSelectionFromSearchParams(params, new Set(assets.map((asset) => asset.id)));
    const merged = mergeEcommerceWorkspaceAssetSelection({ productAssetIds: ["default-product"], supportingAssetIds: ["default-package"], modelAssetIds: [], sceneAssetIds: [], brandAssetIds: [] }, parsed);

    expect(merged.productAssetIds).toEqual(["asset-product"]);
    expect(merged.supportingAssetIds).toEqual(["default-package"]);
    expect(merged.modelAssetIds).toEqual(["asset-model"]);
    expect(merged.sceneAssetIds).toEqual(["asset-scene"]);
    expect(merged.brandAssetIds).toEqual(["asset-logo"]);
});

test("run-id changes preserve a user's ecommerce frame and node layout", () => {
    const initial = createEcommerceCanvasProjection(detail());
    const moved = initial.nodes.map((node) => (node.ecommerceKind === "production_frame" ? { ...node, position: { x: 720, y: 460 } } : node.ecommerceKind === "product_input" ? { ...node, position: { x: 260, y: 230 } } : node));
    const withRun = createEcommerceCanvasProjection(detail(), {
        schemaVersion: 1,
        artifacts: [],
        presets: { schemaVersion: 1, system: [], custom: [] },
        providerRoutes: [],
        runs: [],
        activeRun: {
            run: {
                id: "run-1",
                projectId: "project-ecommerce",
                userId: "user-1",
                idempotencyKey: "run-1-key",
                status: "awaiting_cost",
                kernel: "MODEL_INTERACTION",
                category: "apparel",
                presetId: "model.top-wear",
                presetVersion: 1,
                targetChannel: "taobao_jd",
                aspectRatio: "9:16",
                resolution: "4k",
                pixelSize: "2160x3840",
                outputCount: 6,
                reviewBeforeGeneration: false,
                productAssetIdsJson: '["asset-product"]',
                supportingAssetIdsJson: "[]",
                modelAssetIdsJson: "[]",
                sceneAssetIdsJson: "[]",
                brandAssetIdsJson: "[]",
                userGoal: "",
                modelMode: "ai",
                modelBrief: "",
                sceneBrief: "",
                brandBrief: "",
                advancedJson: "{}",
                productDnaArtifactId: "artifact-product-dna",
                scenePackArtifactId: "artifact-scene-pack",
                presetSnapshotArtifactId: "artifact-preset",
                generationRequestArtifactId: "artifact-generation",
                createdAt: "2026-08-22T00:00:00Z",
                updatedAt: "2026-08-22T00:00:00Z",
            },
            slots: [],
            routes: [],
        },
    });
    const reconciled = reconcileEcommerceCanvasProjection(moved, initial.connections, withRun, "project-ecommerce");
    const frame = reconciled.nodes.find((node) => node.ecommerceKind === "production_frame");
    const product = reconciled.nodes.find((node) => node.ecommerceKind === "product_input");
    expect(frame?.position).toEqual({ x: 720, y: 460 });
    expect(product?.position).toEqual({ x: 260, y: 230 });
    expect(reconciled.nodes.filter((node) => node.ecommerceKind === "production_frame")).toHaveLength(1);
});

describe("canvas domain isolation", () => {
    test("film workspace rejects ecommerce projects", () => {
        expect(isFilmWorkspace({ featureEnabled: true, projectId: "project-ecommerce", projectType: "ecommerce" })).toBe(false);
    });

    test("ecommerce documents are detected by project identity or ecommerce semantics", () => {
        expect(isEcommerceCanvasDocument({ projectId: "project-ecommerce", nodes: [] }, new Set(["project-ecommerce"]))).toBe(true);
        expect(
            isEcommerceCanvasDocument(
                {
                    projectId: "project-film",
                    nodes: [
                        {
                            id: "commerce-node",
                            type: CanvasNodeType.Text,
                            title: "ProductDNA",
                            position: { x: 0, y: 0 },
                            width: 300,
                            height: 180,
                            ecommerceKind: "product_dna",
                        },
                    ],
                },
                new Set(),
            ),
        ).toBe(true);
        expect(
            isEcommerceCanvasDocument(
                {
                    projectId: "project-film",
                    nodes: [
                        {
                            id: "film-node",
                            type: CanvasNodeType.Text,
                            title: "Shot",
                            position: { x: 0, y: 0 },
                            width: 300,
                            height: 180,
                            metadata: { skillDomain: "film" },
                        },
                    ],
                },
                new Set(),
            ),
        ).toBe(false);
    });
});

test("ecommerce project details expose only the ecommerce workbench surface", () => {
    expect(projectDetailViewKeys("ecommerce")).toEqual(["ecommerce", "ecommerce-assets", "canvases", "settings"]);
    expect(projectDetailViewKeys("ecommerce")).not.toContain("overview");
    expect(projectDetailViewKeys("ecommerce")).not.toContain("chapters");
    expect(projectDetailViewKeys("ecommerce")).not.toContain("assets");
    expect(projectDetailDefaultView("ecommerce")).toBe("ecommerce");
    expect(projectHubPath("ecommerce")).toBe("/ecommerce");
    expect(projectDetailPath("p-commerce", "ecommerce", "chapters")).toBe("/projects/p-commerce/ecommerce");
    expect(projectDetailPath("p-commerce", "ecommerce", "overview")).toBe("/projects/p-commerce/ecommerce");
    expect(ECOMMERCE_PROJECT_SETTING_KEYS).toEqual(["name", "description", "aspectRatio", "status"]);
    expect(ECOMMERCE_PROJECT_SETTING_KEYS).not.toContain("stylePresetId" as never);
});

test("short drama retains its own project details and return path", () => {
    expect(projectDetailViewKeys("short-drama")).toEqual(["overview", "chapters", "canvases", "assets", "settings"]);
    expect(projectDetailDefaultView("short-drama")).toBe("overview");
    expect(projectHubPath("short-drama")).toBe("/projects");
    expect(projectDetailPath("p-film", "short-drama", "chapters")).toBe("/projects/p-film/chapters");
});
