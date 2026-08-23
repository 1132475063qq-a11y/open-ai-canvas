export type RelationTemplate = {
    from: string;
    relation: string;
    to: string;
    required?: boolean;
};

export type ExpectedRelation = RelationTemplate & {
    source: "skill_template";
};

export type ExpectedRelationGraph = {
    schemaVersion: 1;
    sourceSkillId: string;
    sourceSkillVersion: number;
    relations: ExpectedRelation[];
};

export function bindExpectedRelationGraph(skillId: string, skillVersion: number, templates: readonly RelationTemplate[], bindings: Readonly<Record<string, string>>): ExpectedRelationGraph {
    if (!skillId.trim()) throw new Error("ExpectedRelationGraph requires a skill id");
    if (!Number.isInteger(skillVersion) || skillVersion < 1) throw new Error("ExpectedRelationGraph requires a positive skill version");

    const relations = templates.map((template) => ({
        from: resolveRelationValue(template.from, bindings),
        relation: template.relation,
        to: resolveRelationValue(template.to, bindings),
        required: template.required ?? true,
        source: "skill_template" as const,
    }));

    return { schemaVersion: 1, sourceSkillId: skillId, sourceSkillVersion: skillVersion, relations };
}

function resolveRelationValue(value: string, bindings: Readonly<Record<string, string>>): string {
    const match = /^\{\{([a-zA-Z0-9_.-]+)\}\}$/.exec(value);
    if (!match) return value;
    const resolved = bindings[match[1]];
    if (!resolved?.trim()) throw new Error(`ExpectedRelationGraph binding is missing: ${match[1]}`);
    return resolved;
}
