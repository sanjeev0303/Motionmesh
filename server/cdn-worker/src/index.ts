export interface Env {
	CDN_SIGNING_SECRET: string;
	CDN_FALLBACK_ORIGIN: string;
	MOTIONMESH_API_URL: string;
}

import { verifySignature } from './verify';
import { reportUsage } from './report';

export default {
	async fetch(request: Request, env: Env, ctx: ExecutionContext): Promise<Response> {
		const url = new URL(request.url);

		// Basic health check
		if (url.pathname === '/health') {
			return new Response('OK', { status: 200 });
		}

		// Ensure secrets are present
		if (!env.CDN_SIGNING_SECRET || !env.CDN_FALLBACK_ORIGIN) {
			return new Response('CDN Configuration Error', { status: 500 });
		}

		// Verify signature for playback paths
		// Expected format: /play/{id}?exp={expiry}&sig={signature}
		const expiry = url.searchParams.get('exp');
		const signature = url.searchParams.get('sig');

		if (!expiry || !signature) {
			return new Response('Missing signature parameters', { status: 403 });
		}

		if (parseInt(expiry, 10) < Math.floor(Date.now() / 1000)) {
			return new Response('Link expired', { status: 403 });
		}

		const isValid = await verifySignature(url.pathname, expiry, signature, env.CDN_SIGNING_SECRET);
		
		if (!isValid) {
			return new Response('Invalid signature', { status: 403 });
		}

		// Proxy request to the origin
		const originUrl = new URL(request.url);
		originUrl.hostname = env.CDN_FALLBACK_ORIGIN;
		
		// Create a new request to the origin
		const originRequest = new Request(originUrl.toString(), request);

		// Fetch from origin
		const response = await fetch(originRequest);

		// Extract bytes for reporting
		const contentLength = response.headers.get('content-length');
		if (contentLength && env.MOTIONMESH_API_URL) {
			// Extract account_id or video_id from URL if possible, or pass it via custom header from origin
			// Assuming format /play/{account_id}/{video_id}/...
			const pathParts = url.pathname.split('/');
			let accountId = 'unknown';
			if (pathParts.length > 2) {
				accountId = pathParts[2]; // Simplified assumption for the example
			}

			const bytes = parseInt(contentLength, 10);
			if (!isNaN(bytes) && bytes > 0) {
				// Fire and forget usage report
				ctx.waitUntil(reportUsage(accountId, bytes, env.MOTIONMESH_API_URL));
			}
		}

		return response;
	},
};
