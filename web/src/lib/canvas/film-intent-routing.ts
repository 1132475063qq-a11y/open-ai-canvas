export const FILM_AUTO_INTENT_ROUTE = "__auto__";

type FilmIntentSelection = {
    intentRouteId?: string;
    agentId?: string;
};

type FilmIntentRouteLike = {
    id: string;
    requiresDisambiguation?: boolean;
};

type FilmArtifactDetailLike = {
    artifacts: Array<{ id: string; currentRevisionId?: string }>;
    artifactRevisions: Array<{ id: string; artifactId: string; status: string }>;
};

export function filmIntentSelection(routeValue: string, route: FilmIntentRouteLike | undefined, agentId: string): FilmIntentSelection {
    const intentRouteId = routeValue && routeValue !== FILM_AUTO_INTENT_ROUTE ? routeValue : undefined;
    const selectedAgentId = route?.requiresDisambiguation && agentId.trim() ? agentId.trim() : undefined;
    return { intentRouteId, agentId: selectedAgentId };
}

export function collectLockedFilmInputRevisionIds(details: FilmArtifactDetailLike[], limit = 50): string[] {
    const ids: string[] = [];
    const seen = new Set<string>();
    for (const detail of details) {
        const currentByArtifact = new Map(detail.artifacts.map((artifact) => [artifact.id, artifact.currentRevisionId || ""]));
        for (const revision of detail.artifactRevisions) {
            if (revision.status !== "locked" || currentByArtifact.get(revision.artifactId) !== revision.id || seen.has(revision.id)) continue;
            seen.add(revision.id);
            ids.push(revision.id);
            if (ids.length >= limit) return ids;
        }
    }
    return ids;
}
