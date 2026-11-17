// @vitest-environment node
import { describe, expect, it, vi } from "vitest";
import { createRendererRecovery } from "./renderer-recovery";

describe("renderer recovery", () => {
	it("reloads a crashed renderer and bounds repeated crashes", () => {
		const reload = vi.fn();
		const log = vi.fn();
		const recover = createRendererRecovery(reload, log);
		recover({ reason: "crashed", exitCode: -1 }, 0);
		recover({ reason: "crashed", exitCode: -1 }, 1);
		recover({ reason: "crashed", exitCode: -1 }, 2);
		recover({ reason: "crashed", exitCode: -1 }, 3);
		expect(reload).toHaveBeenCalledTimes(3);
		expect(log).toHaveBeenCalledWith(expect.stringContaining("exitCode=-1"));
		recover({ reason: "crashed", exitCode: -1 }, 300_001);
		expect(reload).toHaveBeenCalledTimes(4);
	});

	it("does not reload a deliberately terminated renderer", () => {
		const reload = vi.fn();
		const recover = createRendererRecovery(reload, vi.fn());
		recover({ reason: "clean-exit", exitCode: 0 }, 0);
		expect(reload).not.toHaveBeenCalled();
	});
});
