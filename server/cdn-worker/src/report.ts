export async function reportUsage(accountId: string, bytes: number, apiUrl: string): Promise<void> {
	try {
		// In a real implementation, you'd want to batch these or use Cloudflare Analytics Engine
		// For this implementation, we fire a POST to the internal usage endpoint
		await fetch(`${apiUrl}/v1/internal/usage/cdn`, {
			method: 'POST',
			headers: {
				'Content-Type': 'application/json',
				// In reality, you'd add a shared secret header here to authenticate this worker to the API
			},
			body: JSON.stringify({
				account_id: accountId,
				bytes: bytes,
				timestamp: new Date().toISOString()
			})
		});
	} catch (e) {
		// Silently fail, it's a best-effort fire-and-forget report
		console.error("Failed to report CDN usage:", e);
	}
}
