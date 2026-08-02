export async function verifySignature(
	path: string,
	expiry: string,
	signature: string,
	secret: string
): Promise<boolean> {
	// The message to sign is `${path}:${expiry}`
	const message = `${path}:${expiry}`;
	
	const encoder = new TextEncoder();
	const keyData = encoder.encode(secret);
	
	try {
		const cryptoKey = await crypto.subtle.importKey(
			'raw',
			keyData,
			{ name: 'HMAC', hash: 'SHA-256' },
			false,
			['sign', 'verify']
		);
		
		// Convert hex signature back to Uint8Array
		const sigBytes = new Uint8Array(
			signature.match(/[\da-f]{2}/gi)?.map(h => parseInt(h, 16)) || []
		);
		
		const isValid = await crypto.subtle.verify(
			'HMAC',
			cryptoKey,
			sigBytes,
			encoder.encode(message)
		);
		
		return isValid;
	} catch (e) {
		return false;
	}
}
