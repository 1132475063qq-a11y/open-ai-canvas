export const ECOMMERCE_PROJECT_TYPE = "ecommerce" as const;
export const SHORT_DRAMA_PROJECT_TYPE = "short-drama" as const;

export type ProjectDomain = "ecommerce" | "short-drama" | "free";
export type ProjectDetailViewKey = "overview" | "chapters" | "canvases" | "assets" | "settings" | "ecommerce" | "ecommerce-assets";

export const SHORT_DRAMA_DETAIL_VIEW_KEYS: readonly ProjectDetailViewKey[] = ["overview", "chapters", "canvases", "assets", "settings"];
export const ECOMMERCE_DETAIL_VIEW_KEYS: readonly ProjectDetailViewKey[] = ["ecommerce", "ecommerce-assets", "canvases", "settings"];
export const ECOMMERCE_PROJECT_SETTING_KEYS = ["name", "description", "aspectRatio", "status"] as const;

export function projectDomain(projectType?: string | null): ProjectDomain {
    if (projectType === ECOMMERCE_PROJECT_TYPE) return "ecommerce";
    if (projectType === SHORT_DRAMA_PROJECT_TYPE) return "short-drama";
    return "free";
}

export function isEcommerceProjectType(projectType?: string | null) {
    return projectDomain(projectType) === "ecommerce";
}

export function projectDetailViewKeys(projectType?: string | null) {
    return isEcommerceProjectType(projectType) ? ECOMMERCE_DETAIL_VIEW_KEYS : SHORT_DRAMA_DETAIL_VIEW_KEYS;
}

export function projectDetailDefaultView(projectType?: string | null): ProjectDetailViewKey {
    return isEcommerceProjectType(projectType) ? "ecommerce" : "overview";
}

export function projectHubPath(projectType?: string | null) {
    return isEcommerceProjectType(projectType) ? "/ecommerce" : "/projects";
}

export function projectDetailPath(projectId: string, projectType: string | null | undefined, view: ProjectDetailViewKey) {
    const allowedViews = projectDetailViewKeys(projectType);
    const targetView = allowedViews.includes(view) ? view : projectDetailDefaultView(projectType);
    return `/projects/${projectId}/${targetView}`;
}
