import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, expect, test, vi } from "vitest";
import { ProjectIconDialog, readProjectIcon } from "./ProjectIconDialog";

beforeEach(() => localStorage.clear());

test("saves a selected symbol and color for one project and restores the default", () => {
 const onSave = vi.fn();
 const props = { open: true, onOpenChange: vi.fn(), projectId: "one", projectName: "One", onSave };
 const { rerender } = render(<ProjectIconDialog {...props} />);
 fireEvent.click(screen.getByRole("button", { name: "rocket" }));
 fireEvent.click(screen.getByRole("button", { name: "Color #ef4444" }));
 fireEvent.click(screen.getByRole("button", { name: "Save" }));
 expect(readProjectIcon("one")).toEqual({ type: "symbol", icon: "rocket", color: "#ef4444" });
 expect(readProjectIcon("two")).toBeUndefined();
 rerender(<ProjectIconDialog {...props} open={false} value={readProjectIcon("one")} />);
 rerender(<ProjectIconDialog {...props} value={readProjectIcon("one")} />);
 fireEvent.click(screen.getByRole("button", { name: /Use repository icon/ }));
 fireEvent.click(screen.getByRole("button", { name: "Save" }));
 expect(readProjectIcon("one")).toBeUndefined();
});

test("validates and saves an HTTPS image URL", () => {
 render(<ProjectIconDialog open onOpenChange={vi.fn()} projectId="one" projectName="One" onSave={vi.fn()} />);
 const input = screen.getByLabelText("Image URL (HTTPS)");
 fireEvent.change(input, { target: { value: "http://example.com/icon.png" } });
 fireEvent.click(screen.getByRole("button", { name: "Use URL" }));
 expect(screen.getByRole("alert")).toHaveTextContent("valid HTTPS");
 fireEvent.change(input, { target: { value: "https://example.com/icon.png" } });
 fireEvent.click(screen.getByRole("button", { name: "Use URL" }));
 fireEvent.click(screen.getByRole("button", { name: "Save" }));
 expect(readProjectIcon("one")).toEqual({ type: "image", src: "https://example.com/icon.png" });
});

test("rejects non-PNG uploads without changing the saved icon", () => {
 render(<ProjectIconDialog open onOpenChange={vi.fn()} projectId="one" projectName="One" onSave={vi.fn()} />);
 const file = new File(["not a png"], "icon.jpg", { type: "image/jpeg" });
 fireEvent.change(screen.getByLabelText("Upload PNG"), { target: { files: [file] } });
 expect(screen.getByRole("alert")).toHaveTextContent("Choose a PNG");
 expect(readProjectIcon("one")).toBeUndefined();
});
