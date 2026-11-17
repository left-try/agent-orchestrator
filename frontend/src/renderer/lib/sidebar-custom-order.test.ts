import { beforeEach, expect, it } from "vitest";
import { readSidebarCustomOrder, writeSidebarCustomOrder } from "./sidebar-custom-order";

beforeEach(() => window.localStorage.clear());

it("persists independent custom orders and restores them after a renderer restart", () => {
	writeSidebarCustomOrder("workspace:folders:.", ["services", "apps"]);
	writeSidebarCustomOrder("workspace:chats:services/api", ["chat-2", "chat-1"]);

	expect(readSidebarCustomOrder("workspace:folders:.")).toEqual(["services", "apps"]);
	expect(readSidebarCustomOrder("workspace:chats:services/api")).toEqual(["chat-2", "chat-1"]);
});

it("ignores malformed stored custom orders", () => {
	window.localStorage.setItem("ao.sidebar.custom-orders", "not json");
	expect(readSidebarCustomOrder("workspace:folders:.")).toEqual([]);
});
