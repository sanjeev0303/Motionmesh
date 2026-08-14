import { UploadVideoFields, UploadVideoResult } from "../types/index.js";
import { handleApiError } from "../utils/handleApiError.js";
import { uploadFile } from "../services/uploadFile.js";

type onProgressType = {
    onProgress?: (progress: {
        loaded: number;
        total: number;
        percent: number;
    }) => void;
}

export interface FileMeta {
    filename: string;
    contentType: string;
    size: number;
}

function extractFileMeta(file: File | Buffer | Uint8Array): FileMeta {
    if (typeof File !== "undefined" && file instanceof File) {
        return {
            filename: file.name,
            contentType: file.type || "application/octet-stream",
            size: file.size,
        };
    }
    return {
        filename: "upload",
        contentType: "application/octet-stream",
        size: (file as Buffer | Uint8Array).byteLength,
    };
}

export class MotionMeshClient {
    private apiKey: string;
    private baseURL: string;

    constructor(options: { apiKey: string, baseURL?: string }) {
        if (!options.apiKey) {
            throw new Error("apiKey is required");
        }
        this.apiKey = options.apiKey;
        this.baseURL = options.baseURL || process.env.MOTIONMESH_BASE_URL || process.env.BASE_URL || "https://api.motionmesh.co.in/v1";
    }

    private async request(path: string, options: RequestInit = {}) {
        const url = `${this.baseURL}${path}`;
        const headers = new Headers(options.headers);
        headers.set("Authorization", `Bearer ${this.apiKey}`);
        
        if (!(options.body instanceof FormData)) {
            headers.set("Content-Type", "application/json");
        }

        const response = await fetch(url, { ...options, headers });
        if (!response.ok) {
            await handleApiError(response, "api_request");
        }
        
        // Some endpoints return 204 No Content
        if (response.status === 204) return null;
        
        return response.json();
    }

    videos = {
        list: async (options?: { limit?: number; cursor?: string; external_user_id?: string }) => {
            const queryParams = new URLSearchParams();
            if (options?.limit) queryParams.append("limit", String(options.limit));
            if (options?.cursor) queryParams.append("cursor", options.cursor);
            if (options?.external_user_id) queryParams.append("external_user_id", options.external_user_id);
            const queryStr = queryParams.toString();
            const path = queryStr ? `/videos?${queryStr}` : `/videos`;
            const data = await this.request(path);
            return data.videos;
        },
        get: async (videoId: string) => {
            return this.request(`/videos/${videoId}`);
        },
        playback: async (videoId: string) => {
            return this.request(`/videos/${videoId}/playback`);
        }
    };

    mediaConverter = {
        createJob: async (videoId: string) => {
            const data = await this.request(`/videos/${videoId}/transcode`, {
                method: "POST"
            });
            return data;
        },
        listJobs: async (options?: { limit?: number }) => {
            const queryParams = new URLSearchParams();
            if (options?.limit) queryParams.append("limit", String(options.limit));
            const queryStr = queryParams.toString();
            const path = queryStr ? `/jobs?${queryStr}` : `/jobs`;
            const data = await this.request(path);
            return data;
        }
    };

    buckets = {
        list: async () => {
            return this.request(`/buckets`);
        }
    };

    async uploadVideo(options: UploadVideoFields, onProgress?: onProgressType): Promise<UploadVideoResult> {
        if (!options.video) {
            throw new Error("Video file is required");
        }

        const {
            filename,
            size
        } = extractFileMeta(options.video);

        // 1. Initiate multipart upload
        const initialResponse = await this.request('/videos/multipart', {
            method: "POST",
            body: JSON.stringify({
                filename,
                size_bytes: size,
                bucket_id: (options as any).bucketId,
                transcode_bucket_id: (options as any).transcodeBucketId,
                external_user_id: (options as any).externalUserId
            })
        });

        // 2. Upload file parts directly to S3
        // In the new API structure, the parts array is returned by /multipart/{id}/parts
        // We'll determine the number of parts needed based on a default 5MB size
        const partSize = 5 * 1024 * 1024;
        const totalParts = Math.ceil(size / partSize);
        
        // Fetch part URLs
        const partsResponse = await this.request(`/videos/multipart/${initialResponse.video.id}/parts?upload_id=${initialResponse.upload_id}&count=${totalParts}`);
        
        const uploadData = {
            objectId: initialResponse.video.id,
            key: initialResponse.object_key,
            uploadId: initialResponse.upload_id,
            parts: partsResponse.parts,
            partSize
        };

        const { objectId, key, uploadId, completedParts } = await uploadFile(options.video as any, uploadData, onProgress);

        // 3. Complete multipart upload
        await this.request(`/videos/multipart/${objectId}/complete`, {
            method: "POST",
            body: JSON.stringify({
                upload_id: uploadId,
                parts: completedParts
            })
        });

        return { key };
    }
}

export const motionmesh = MotionMeshClient;
