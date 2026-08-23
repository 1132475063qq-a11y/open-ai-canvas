import type { FilmProductionResult, FilmVideoSequenceView } from "@/services/api/film-production";
import type { TimelineClip, TimelineDirectMedia, TimelineProject } from "@/types/timeline";
import { resourceFileUrl } from "@/services/api/resources";
import { DEFAULT_AUDIO_TRACK_ID, DEFAULT_VIDEO_TRACK_ID, normalizeTimelineProject } from "./timeline-tracks";

export type FilmVideoTimelineImport = {
    timeline: TimelineProject;
    importedCount: number;
    replaced: boolean;
};

export function canImportFilmVideoSequence(sequence: FilmVideoSequenceView) {
    return hasAcceptedFilmVideoSequenceReview(sequence) && sequence.slots.length > 0 && sequence.slots.every((slot) => Boolean(acceptedFilmVideoAttempt(slot.attempts)?.result?.url));
}

export function importFilmVideoSequenceToTimeline(current: TimelineProject | null | undefined, sequence: FilmVideoSequenceView): FilmVideoTimelineImport {
    const accepted = sequence.slots.map((slot) => ({ slot: slot.slot, attempt: acceptedFilmVideoAttempt(slot.attempts) })).sort((left, right) => left.slot.position - right.slot.position);
    if (!hasAcceptedFilmVideoSequenceReview(sequence)) {
        throw new Error("整组视频必须通过当前版本的连续性验收后才能加入时间线");
    }
    if (!accepted.length || accepted.some((item) => !item.attempt?.result?.url)) {
        throw new Error("整组视频必须全部通过逐镜人工验收后才能加入时间线");
    }

    const base = normalizeTimelineProject(current);
    const prefix = filmVideoTimelinePrefix(sequence.sequence.id);
    const existing = base.clips.filter((clip) => clip.nodeId.startsWith(prefix));
    const retained = base.clips.filter((clip) => !clip.nodeId.startsWith(prefix));
    const startMs = existing.length ? Math.min(...existing.map((clip) => clip.startMs)) : visualEndMs(retained);
    let cursorMs = startMs;
    const clips: TimelineClip[] = [];

    for (const item of accepted) {
        const attempt = item.attempt!;
        const media = filmVideoDirectMedia(sequence, item.slot.id, item.slot.position, item.slot.shotId, item.slot.durationMs, attempt.result!);
        clips.push({
            id: `clip-${media.id}`,
            kind: "video",
            nodeId: media.id,
            trackId: DEFAULT_VIDEO_TRACK_ID,
            startMs: cursorMs,
            durationMs: item.slot.durationMs,
            sourceStartMs: 0,
            sourceDurationMs: media.durationMs || item.slot.durationMs,
            title: media.title,
            directMedia: media,
        });
        cursorMs += item.slot.durationMs;
    }
    if (sequence.sequence.musicResourceId) {
        const sourceDurationMs = sequence.sequence.musicDurationMs || sequence.sequence.targetDurationMs;
        clips.push({
            id: `clip-${filmVideoTimelinePrefix(sequence.sequence.id)}music`,
            kind: "audio",
            nodeId: `${filmVideoTimelinePrefix(sequence.sequence.id)}music`,
            trackId: DEFAULT_AUDIO_TRACK_ID,
            startMs: startMs,
            durationMs: Math.min(sequence.sequence.targetDurationMs || cursorMs - startMs, sourceDurationMs),
            sourceStartMs: 0,
            sourceDurationMs,
            title: `${sequence.sequence.title} · 基础音乐`,
            volume: 0.8,
            fadeInMs: 250,
            fadeOutMs: 500,
            directMedia: {
                id: `${filmVideoTimelinePrefix(sequence.sequence.id)}music`,
                kind: "audio",
                title: `${sequence.sequence.title} · 基础音乐`,
                storageKey: `resource:${sequence.sequence.musicResourceId}`,
                url: resourceFileUrl(sequence.sequence.musicResourceId),
                durationMs: sourceDurationMs,
                mimeType: "audio/*",
            },
        });
    }

    return {
        timeline: normalizeTimelineProject({ ...base, clips: [...retained, ...clips], updatedAt: new Date().toISOString() }),
        importedCount: clips.length,
        replaced: existing.length > 0,
    };
}

function hasAcceptedFilmVideoSequenceReview(sequence: FilmVideoSequenceView) {
    return (
        sequence.sequence.status === "completed" &&
        sequence.continuity?.ledger.status === "ready" &&
        sequence.continuity.ledger.mediaState === "available" &&
        sequence.sequenceReview?.valid === true &&
        sequence.sequenceReview.decision === "PASS" &&
        sequence.sequenceReview.action === "accept"
    );
}

function acceptedFilmVideoAttempt(attempts: FilmVideoSequenceView["slots"][number]["attempts"]) {
    return attempts.find((attempt) => attempt.accepted && attempt.result?.url);
}

function filmVideoDirectMedia(sequence: FilmVideoSequenceView, slotId: string, position: number, shotId: string, durationMs: number, result: FilmProductionResult): TimelineDirectMedia {
    const payload = parseFilmVideoResultPayload(result.payload);
    return {
        id: `${filmVideoTimelinePrefix(sequence.sequence.id)}${slotId}`,
        kind: "video",
        title: `${sequence.sequence.title} · 镜头 ${position + 1}`,
        storageKey: payload.storageKey,
        url: result.url || payload.url,
        durationMs: payload.durationMs || durationMs,
        width: payload.width,
        height: payload.height,
        mimeType: payload.mimeType || "video/mp4",
        content: shotId,
    };
}

function parseFilmVideoResultPayload(raw?: string): Partial<TimelineDirectMedia> {
    if (!raw) return {};
    try {
        const payload = JSON.parse(raw) as { video?: Partial<TimelineDirectMedia> };
        return payload.video || {};
    } catch {
        return {};
    }
}

function filmVideoTimelinePrefix(sequenceId: string) {
    return `film-video:${sequenceId}:`;
}

function visualEndMs(clips: TimelineClip[]) {
    return clips.reduce((end, clip) => (clip.kind === "video" || clip.kind === "image" || clip.kind === "text" ? Math.max(end, clip.startMs + clip.durationMs) : end), 0);
}
