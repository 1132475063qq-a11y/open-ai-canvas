import { describe, expect, test } from "bun:test";

import { latestFilmAttemptsByShot, normalizeFilmBatchShotIds, summarizeFilmBatch, type FilmBatchQuoteItem } from "../src/lib/canvas/film-batch-production";
import type { FilmProductionAttemptView, FilmProductionImageQuote } from "../src/services/api/film-production";

describe("Film multi-shot production", () => {
    test("deduplicates selected shots and caps a batch at twelve", () => {
        const ids = Array.from({ length: 14 }, (_, index) => `shot-${index}`);
        expect(normalizeFilmBatchShotIds([ids[0], ids[0], ...ids])).toEqual(ids.slice(0, 12));
    });

    test("keeps successful quotes available when another shot fails", () => {
        const items: FilmBatchQuoteItem[] = [
            { shotId: "shot-1", status: "quoted", quote: { cost: { amountMicrocredits: 120, required: true } } as FilmProductionImageQuote },
            { shotId: "shot-2", status: "failed", error: "provider unavailable" },
            { shotId: "shot-3", status: "submitted", attemptId: "attempt-3", quote: { cost: { amountMicrocredits: 80, required: true } } as FilmProductionImageQuote },
        ];
        expect(summarizeFilmBatch(items)).toEqual({ total: 3, quoted: 2, submitted: 1, failed: 1, active: 0, amountMicrocredits: 200 });
    });

    test("restores the latest attempt for each shot after a refresh", () => {
        const older = { attempt: { id: "attempt-old", shotId: "shot-1", createdAt: "2026-01-01T00:00:00Z" } } as FilmProductionAttemptView;
        const attempts = [
            older,
            { attempt: { id: "attempt-new", shotId: "shot-1", createdAt: "2026-01-02T00:00:00Z" } } as FilmProductionAttemptView,
            { attempt: { id: "attempt-2", shotId: "shot-2", createdAt: "2026-01-01T00:00:00Z" } } as FilmProductionAttemptView,
        ];
        const latest = latestFilmAttemptsByShot(attempts);
        expect(latest.get("shot-1")?.attempt.id).toBe("attempt-new");
        expect(latest.get("shot-2")?.attempt.id).toBe("attempt-2");
    });
});
