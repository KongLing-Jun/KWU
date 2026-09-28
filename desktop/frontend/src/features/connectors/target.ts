export function isConnectorTargetUnavailable(error: unknown): boolean {
	const message = error instanceof Error
		? error.message
		: typeof error === "object" && error !== null && "message" in error
			? String((error as { message?: unknown }).message ?? "")
			: String(error ?? "");
	return /connector target tab ".*" is unavailable/i.test(message);
}
