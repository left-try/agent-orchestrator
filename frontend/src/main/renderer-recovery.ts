// Limit automatic reloads so a persistent crash does not trap the user in a loop.
export function createRendererRecovery(reload: () => void, log: (message: string) => void) {
	let crashes: number[] = [];
	return (details: { reason: string; exitCode: number }, now = Date.now()) => {
		log(`AO: renderer process gone: reason=${details.reason} exitCode=${details.exitCode}`);
		if (details.reason === "clean-exit") return;
		crashes = crashes.filter((at) => now - at <= 300_000);
		if (crashes.length >= 3) {
			log("AO: renderer recovery stopped after repeated crashes");
			return;
		}
		crashes.push(now);
		reload();
	};
}
