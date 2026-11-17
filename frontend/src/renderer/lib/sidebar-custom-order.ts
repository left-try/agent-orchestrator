const STORAGE_KEY = "ao.sidebar.custom-orders";

function readAllOrders(): Record<string, string[]> {
	try {
		const saved: unknown = JSON.parse(window.localStorage.getItem(STORAGE_KEY) ?? "null");
		if (saved && typeof saved === "object" && !Array.isArray(saved)) {
			return Object.fromEntries(
				Object.entries(saved).flatMap(([key, value]) =>
					Array.isArray(value)
						? [[key, [...new Set(value.filter((id): id is string => typeof id === "string"))]]]
						: [],
				),
			);
		}
	} catch {}
	return {};
}

export function readSidebarCustomOrder(key: string): string[] {
	return readAllOrders()[key] ?? [];
}

export function writeSidebarCustomOrder(key: string, order: readonly string[]): void {
	const orders = readAllOrders();
	if (order.length > 0) orders[key] = [...order];
	else delete orders[key];
	try {
		window.localStorage.setItem(STORAGE_KEY, JSON.stringify(orders));
	} catch {}
}
