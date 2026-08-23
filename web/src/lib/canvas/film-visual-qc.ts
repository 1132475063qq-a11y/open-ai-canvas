import type { PublicLogicalModel } from "@/services/api/logical-models";

export function supportsFilmVisualQCModel(model: PublicLogicalModel) {
    if (!model.available || model.capability !== "text") return false;
    return [model.capabilitySpec, ...model.capabilityProfiles].some((profile) => profile.capability === "text" && (profile.inputs?.image?.max || 0) >= 1);
}
