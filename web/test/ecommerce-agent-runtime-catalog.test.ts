import { expect, test } from "bun:test";

import type { EcommerceAgentRuntimeCatalog, EcommerceWorkspace } from "../src/services/api/projects";

const catalogFixture = {
    registry: {
        id: "ecommerce-agent-team",
        version: "0.1.0",
        domain: "ecommerce",
        sourceDigest: "digest",
        agentCount: 1,
        skillCount: 1,
        intentRouteCount: 1,
        handoffRouteCount: 1,
    },
    agents: [{ id: "ecommerce_orchestrator", description: "编排", skillIds: ["ecommerce-orchestration"] }],
    skills: [{ id: "ecommerce-orchestration", version: "1.0.0", description: "编排 Skill", ownerAgentIds: ["ecommerce_orchestrator"] }],
    intentRoutes: [
        {
            id: "IR-01",
            name: "商品事实分析",
            triggerPhrases: ["分析商品资产"],
            primaryAgentId: "ecommerce_orchestrator",
            skillIds: ["ecommerce-orchestration"],
            requiredInputArtifactTypes: [],
            optionalInputArtifactTypes: [],
            outputArtifactTypes: ["generation_request"],
        },
    ],
    handoffRoutes: [
        {
            id: "HR-01",
            name: "交接",
            fromAgentIds: ["ecommerce_orchestrator"],
            toAgentIds: ["ecommerce_orchestrator"],
            skillIds: ["ecommerce-orchestration"],
            inputArtifactTypes: [],
            inputResolutionMode: "static",
            outputArtifactTypes: ["generation_request"],
            messageType: "request",
            executionMode: "agent",
            requiresLockedInput: true,
            fanout: "single",
        },
    ],
    artifactTypes: [{ id: "generation_request", domain: "ecommerce", responsible: "ecommerce_orchestrator" }],
} satisfies EcommerceAgentRuntimeCatalog;

test("Ecommerce workspace accepts a legacy response without agentRuntime", () => {
    const legacyWorkspace = {
        schemaVersion: 1,
        artifacts: [],
        presets: { schemaVersion: 1, system: [], custom: [] },
        providerRoutes: [],
        runs: [],
    } satisfies EcommerceWorkspace;

    expect(legacyWorkspace.agentRuntime).toBeUndefined();
});

test("Ecommerce Agent Runtime catalog uses the persisted camelCase contract", () => {
    const catalog: EcommerceAgentRuntimeCatalog = catalogFixture;
    expect(catalog.registry.sourceDigest).toBe("digest");
    expect(catalog.agents[0]?.skillIds).toEqual(["ecommerce-orchestration"]);
    expect(catalog.skills[0]?.ownerAgentIds).toEqual(["ecommerce_orchestrator"]);
    expect(catalog.intentRoutes[0]?.primaryAgentId).toBe("ecommerce_orchestrator");
    expect(catalog.handoffRoutes[0]?.fromAgentIds).toEqual(["ecommerce_orchestrator"]);
    expect(Object.prototype.hasOwnProperty.call(catalog.artifactTypes[0], "artifactType")).toBe(false);
    expect(catalog.artifactTypes[0]?.responsible).toBe("ecommerce_orchestrator");
});
