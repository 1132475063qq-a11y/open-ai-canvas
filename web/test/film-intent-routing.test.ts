import { describe, expect, test } from "bun:test";

import { collectLockedFilmInputRevisionIds, FILM_AUTO_INTENT_ROUTE, filmIntentSelection } from "../src/lib/canvas/film-intent-routing";

describe("Film intent routing UI contract", () => {
    test("keeps automatic routing provider-neutral and sends an Agent only for disambiguation", () => {
        expect(filmIntentSelection(FILM_AUTO_INTENT_ROUTE, undefined, "quality_control_editor")).toEqual({ intentRouteId: undefined, agentId: undefined });
        expect(filmIntentSelection("IR-11", { id: "IR-11", requiresDisambiguation: true }, "visual_development_designer")).toEqual({
            intentRouteId: "IR-11",
            agentId: "visual_development_designer",
        });
        expect(filmIntentSelection("IR-07", { id: "IR-07" }, "narrative_screenwriter")).toEqual({ intentRouteId: "IR-07", agentId: undefined });
    });

    test("passes only current locked Artifact revisions and removes duplicates", () => {
        const details = [
            {
                artifacts: [
                    { id: "script", currentRevisionId: "script-v2" },
                    { id: "storyboard", currentRevisionId: "storyboard-v1" },
                ],
                artifactRevisions: [
                    { id: "script-v1", artifactId: "script", status: "locked" },
                    { id: "script-v2", artifactId: "script", status: "locked" },
                    { id: "storyboard-v1", artifactId: "storyboard", status: "review" },
                ],
            },
            {
                artifacts: [{ id: "script", currentRevisionId: "script-v2" }],
                artifactRevisions: [{ id: "script-v2", artifactId: "script", status: "locked" }],
            },
        ];
        expect(collectLockedFilmInputRevisionIds(details)).toEqual(["script-v2"]);
    });
});
