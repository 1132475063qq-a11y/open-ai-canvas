export type FilmWorkspaceContext = {
    featureEnabled: boolean;
    projectId?: string | null;
    projectType?: string | null;
};

/** Film-only controls are visible only after the canvas is linked to a Film project. */
export function isFilmWorkspace({ featureEnabled, projectId, projectType }: FilmWorkspaceContext) {
    return featureEnabled && Boolean(projectId) && projectType === "short-drama";
}
