import type { FilmProductionAttemptView, FilmProductionImageQuote } from "@/services/api/film-production";

export const FILM_BATCH_MAX_SHOTS = 12;

export type FilmBatchQuoteItem = {
    shotId: string;
    status: "quoting" | "quoted" | "submitting" | "submitted" | "failed";
    quote?: FilmProductionImageQuote;
    attemptId?: string;
    error?: string;
};

export type FilmBatchSummary = {
    total: number;
    quoted: number;
    submitted: number;
    failed: number;
    active: number;
    amountMicrocredits: number;
};

export function normalizeFilmBatchShotIds(shotIds: string[], max = FILM_BATCH_MAX_SHOTS) {
    const unique = [...new Set(shotIds.map((shotId) => shotId.trim()).filter(Boolean))];
    return unique.slice(0, Math.max(1, max));
}

export function summarizeFilmBatch(items: FilmBatchQuoteItem[], attempts: FilmProductionAttemptView[] = []): FilmBatchSummary {
    const attemptIds = new Set(attempts.map((item) => item.attempt.id));
    const submitted = items.filter((item) => item.status === "submitted" || (item.attemptId && attemptIds.has(item.attemptId))).length;
    const active = items.filter((item) => item.status === "quoting" || item.status === "submitting").length;
    const quoted = items.filter((item) => item.quote && item.status !== "failed").length;
    const failed = items.filter((item) => item.status === "failed").length;
    const amountMicrocredits = items.reduce((sum, item) => sum + (item.quote?.cost.amountMicrocredits || 0), 0);
    return { total: items.length, quoted, submitted, failed, active, amountMicrocredits };
}

export function latestFilmAttemptsByShot(attempts: FilmProductionAttemptView[]) {
    const latest = new Map<string, FilmProductionAttemptView>();
    for (const item of attempts) {
        const current = latest.get(item.attempt.shotId);
        if (!current || Date.parse(item.attempt.createdAt) > Date.parse(current.attempt.createdAt)) {
            latest.set(item.attempt.shotId, item);
        }
    }
    return latest;
}
