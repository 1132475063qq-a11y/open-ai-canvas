const VIDEO_FRAME_TIMEOUT_MS = 20_000;
const LAST_FRAME_EPSILON_SECONDS = 0.001;
const QC_FRAME_MAX_EDGE = 1280;

export type CapturedVideoFrame = {
    timeMs: number;
    durationMs: number;
    width: number;
    height: number;
    blob: Blob;
};

const DEFAULT_QC_SAMPLE_RATIOS = [0, 0.25, 0.5, 0.75, 1] as const;

export async function captureVideoQCFrames(source: Blob | string, ratios: readonly number[] = DEFAULT_QC_SAMPLE_RATIOS): Promise<CapturedVideoFrame[]> {
    if (ratios.length < 2 || ratios.length > 8 || ratios.some((ratio, index) => !Number.isFinite(ratio) || ratio < 0 || ratio > 1 || (index > 0 && ratio <= ratios[index - 1]))) {
        throw new Error("视频 QC 采样比例必须包含 2-8 个递增时间点");
    }
    return withLoadedVideo(source, async (video) => {
        const durationMs = Math.max(1, Math.round(video.duration * 1000));
        const scale = Math.min(1, QC_FRAME_MAX_EDGE / Math.max(video.videoWidth, video.videoHeight));
        const width = Math.max(1, Math.round(video.videoWidth * scale));
        const height = Math.max(1, Math.round(video.videoHeight * scale));
        const frames: CapturedVideoFrame[] = [];
        for (const ratio of ratios) {
            const timeMs = ratio === 1 ? Math.max(0, durationMs - 1) : Math.round(durationMs * ratio);
            await seekVideo(video, timeMs / 1000, "无法定位视频采样时间点");
            const canvas = document.createElement("canvas");
            canvas.width = width;
            canvas.height = height;
            const context = canvas.getContext("2d");
            if (!context) throw new Error("浏览器无法创建视频采样画布");
            context.drawImage(video, 0, 0, width, height);
            frames.push({ timeMs, durationMs, width, height, blob: await canvasToBlob(canvas, "image/jpeg", 0.9, "视频采样帧编码失败") });
        }
        return frames;
    });
}

export async function captureVideoLastFrame(source: Blob | string) {
    return withLoadedVideo(source, async (video) => {
        const targetTime = Math.max(0, video.duration - LAST_FRAME_EPSILON_SECONDS);
        await seekVideo(video, targetTime, "无法定位到视频最后一帧");
        const canvas = document.createElement("canvas");
        canvas.width = video.videoWidth;
        canvas.height = video.videoHeight;
        const context = canvas.getContext("2d");
        if (!context) throw new Error("浏览器无法创建图片画布");
        context.drawImage(video, 0, 0, canvas.width, canvas.height);
        return canvasToBlob(canvas, "image/png", undefined, "尾帧图片编码失败");
    });
}

async function withLoadedVideo<T>(source: Blob | string, run: (video: HTMLVideoElement) => Promise<T>) {
    const blob = await readVideoBlob(source);
    const objectUrl = URL.createObjectURL(blob);
    const video = document.createElement("video");
    video.muted = true;
    video.playsInline = true;
    video.preload = "auto";
    try {
        const loaded = waitForVideoEvent(video, "loadeddata", "视频读取超时或编码不受浏览器支持");
        video.src = objectUrl;
        video.load();
        await loaded;
        if (!Number.isFinite(video.duration) || video.duration <= 0) throw new Error("无法确定视频时长");
        if (!video.videoWidth || !video.videoHeight) throw new Error("无法读取视频画面尺寸");
        return await run(video);
    } finally {
        video.pause();
        video.removeAttribute("src");
        video.load();
        URL.revokeObjectURL(objectUrl);
    }
}

async function seekVideo(video: HTMLVideoElement, targetTime: number, errorMessage: string) {
    const clamped = Math.min(Math.max(0, targetTime), Math.max(0, video.duration - LAST_FRAME_EPSILON_SECONDS));
    if (Math.abs(video.currentTime - clamped) <= LAST_FRAME_EPSILON_SECONDS) return;
    const seeked = waitForVideoEvent(video, "seeked", errorMessage);
    video.currentTime = clamped;
    await seeked;
}

async function readVideoBlob(source: Blob | string) {
    if (source instanceof Blob) return source;
    try {
        const response = await fetch(source);
        if (!response.ok) throw new Error(String(response.status));
        return await response.blob();
    } catch {
        throw new Error("无法读取视频文件，请重新上传视频后再截取尾帧");
    }
}

function waitForVideoEvent(video: HTMLVideoElement, eventName: "loadeddata" | "seeked", errorMessage: string) {
    return new Promise<void>((resolve, reject) => {
        let timer = 0;
        const cleanup = () => {
            window.clearTimeout(timer);
            video.removeEventListener(eventName, onSuccess);
            video.removeEventListener("error", onError);
        };
        const onSuccess = () => {
            cleanup();
            resolve();
        };
        const onError = () => {
            cleanup();
            reject(new Error(errorMessage));
        };
        video.addEventListener(eventName, onSuccess, { once: true });
        video.addEventListener("error", onError, { once: true });
        timer = window.setTimeout(onError, VIDEO_FRAME_TIMEOUT_MS);
    });
}

function canvasToBlob(canvas: HTMLCanvasElement, mimeType: string, quality: number | undefined, errorMessage: string) {
    return new Promise<Blob>((resolve, reject) => canvas.toBlob((blob) => (blob ? resolve(blob) : reject(new Error(errorMessage))), mimeType, quality));
}
