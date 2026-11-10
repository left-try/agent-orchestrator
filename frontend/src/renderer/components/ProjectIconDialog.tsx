import { Blocks, BookOpen, Bot, Box, BriefcaseBusiness, Cloud, Code2, Coffee, Database, Flame, Folder, Gamepad2, Globe, Heart, Image as ImageIcon, Laptop, Layers, Leaf, Lightbulb, Music, Palette, Rocket, Server, Shield, Sparkles, Star, Terminal, Wrench, Zap, type LucideIcon } from "lucide-react";
import { useEffect, useState } from "react";
import { Button } from "./ui/button";
import { Input } from "./ui/input";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "./ui/dialog";

const icons = { folder: Folder, code: Code2, box: Box, rocket: Rocket, star: Star, heart: Heart, blocks: Blocks, book: BookOpen, bot: Bot, briefcase: BriefcaseBusiness, cloud: Cloud, coffee: Coffee, database: Database, flame: Flame, game: Gamepad2, globe: Globe, image: ImageIcon, laptop: Laptop, layers: Layers, leaf: Leaf, idea: Lightbulb, music: Music, palette: Palette, server: Server, shield: Shield, sparkles: Sparkles, terminal: Terminal, tools: Wrench, lightning: Zap } satisfies Record<string, LucideIcon>;
const colors = ["#ef4444", "#f97316", "#eab308", "#22c55e", "#06b6d4", "#3b82f6", "#8b5cf6", "#ec4899"];
const key = "ao.projectIconOverrides";
export type ProjectIconChoice = { type: "image"; src: string } | { type: "symbol"; icon: keyof typeof icons; color: string };

function readChoices(): Record<string, ProjectIconChoice> {
 try { return JSON.parse(localStorage.getItem(key) || "{}") || {}; } catch { return {}; }
}
export function readProjectIcon(id: string): ProjectIconChoice | undefined { return readChoices()[id]; }
function saveChoice(id: string, choice?: ProjectIconChoice) {
 const choices = readChoices();
 if (choice) choices[id] = choice; else delete choices[id];
 localStorage.setItem(key, JSON.stringify(choices));
}
export function CustomProjectIcon({ choice, className }: { choice: ProjectIconChoice; className?: string }) {
 const [failedSrc, setFailedSrc] = useState<string>();
 if (choice.type === "image") {
  if (failedSrc === choice.src) return <Folder className={className ?? "size-icon-md"} aria-hidden="true" />;
  return <img alt="" src={choice.src} onError={() => setFailedSrc(choice.src)} className={`${className ?? "size-icon-md"} rounded-[4px] object-cover`} />;
 }
 const Icon = icons[choice.icon] ?? Folder;
 return <span className={className ?? "size-icon-md"} style={{ color: choice.color }}><Icon className="size-full" strokeWidth={1.75} /></span>;
}

export function ProjectIconDialog({ open, onOpenChange, projectName, value, onSave, projectId }: {
 open: boolean; onOpenChange: (open: boolean) => void; projectName: string; value?: ProjectIconChoice;
 onSave: (choice?: ProjectIconChoice) => void; projectId: string;
}) {
 const [choice, setChoice] = useState<ProjectIconChoice | undefined>(value);
 const [url, setUrl] = useState("");
 const [error, setError] = useState("");
 useEffect(() => { if (open) { setChoice(value); setUrl(value?.type === "image" && value.src.startsWith("https://") ? value.src : ""); setError(""); } }, [open, value]);
 const symbol = choice?.type === "symbol" ? choice : { type: "symbol" as const, icon: "folder" as const, color: colors[5] };
 const save = (next?: ProjectIconChoice) => {
  try { saveChoice(projectId, next); onSave(next); onOpenChange(false); }
  catch { setError("Could not save the icon. Free up storage and try again."); }
 };
 const upload = (file?: File) => {
  if (!file) return;
  if (file.type !== "image/png" || file.size > 5 * 1024 * 1024) { setError("Choose a PNG image smaller than 5 MB."); return; }
  const reader = new FileReader();
  reader.onerror = () => setError("Could not read this image.");
  reader.onload = () => {
   const image = new Image();
   image.onerror = () => setError("Could not open this PNG image.");
   image.onload = () => {
    const canvas = document.createElement("canvas"); canvas.width = 128; canvas.height = 128;
    const ctx = canvas.getContext("2d");
    if (!ctx) { setError("Could not process this image."); return; }
    const side = Math.min(image.width, image.height);
    ctx.drawImage(image, (image.width - side) / 2, (image.height - side) / 2, side, side, 0, 0, 128, 128);
    setChoice({ type: "image", src: canvas.toDataURL("image/png") }); setError("");
   };
   image.src = String(reader.result);
  };
  reader.readAsDataURL(file);
 };
 return <Dialog open={open} onOpenChange={onOpenChange}><DialogContent className="max-h-[min(90vh,760px)] overflow-y-auto rounded-2xl p-5 sm:rounded-2xl" onClick={(e) => e.stopPropagation()} onContextMenu={(e) => e.stopPropagation()}>
  <DialogTitle>Customize project icon</DialogTitle>
  <DialogDescription>Choose an icon for {projectName}. This setting is saved on this device.</DialogDescription>
  <div className="space-y-5 text-sm">
   <Button type="button" variant="secondary" className="rounded-xl" onClick={() => { setChoice(undefined); setError(""); }}>Use repository icon (default)</Button>
   <div><p className="mb-2 font-medium">Icon</p><div className="grid grid-cols-7 gap-1 sm:grid-cols-10">{Object.entries(icons).map(([name, Icon]) => <button key={name} type="button" aria-label={name} aria-pressed={choice?.type === "symbol" && choice.icon === name} title={name} className="flex size-9 items-center justify-center rounded-xl transition-colors hover:bg-interactive-hover focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary aria-pressed:bg-interactive-active" onClick={() => setChoice({ ...symbol, icon: name as keyof typeof icons })}><Icon className="size-5" style={{ color: symbol.color }} /></button>)}</div></div>
   <div><p className="mb-2 font-medium">Color</p><div className="flex flex-wrap gap-2">{colors.map((color) => <button key={color} type="button" aria-label={`Color ${color}`} aria-pressed={choice?.type === "symbol" && choice.color === color} className="size-7 rounded-full transition-transform hover:scale-110 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary aria-pressed:ring-2 aria-pressed:ring-foreground aria-pressed:ring-offset-2 aria-pressed:ring-offset-overlay" style={{ backgroundColor: color }} onClick={() => setChoice({ ...symbol, color })} />)}</div></div>
   <div className="space-y-2"><label className="block font-medium" htmlFor={`icon-url-${projectId}`}>Image URL (HTTPS)</label><div className="flex gap-2"><Input id={`icon-url-${projectId}`} className="min-w-0 flex-1 rounded-xl" value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://example.com/icon.png" /><Button type="button" variant="secondary" className="rounded-xl" onClick={() => { try { const parsed = new URL(url); if (parsed.protocol !== "https:") throw new Error(); setChoice({ type: "image", src: parsed.href }); setError(""); } catch { setError("Enter a valid HTTPS image URL."); } }}>Use URL</Button></div></div>
   <label className="block font-medium">Upload PNG <input type="file" accept="image/png" className="mt-2 block w-full text-xs text-muted-foreground file:mr-3 file:cursor-pointer file:rounded-xl file:border-0 file:bg-secondary file:px-3 file:py-2 file:text-sm file:text-secondary-foreground hover:file:bg-interactive-hover" onChange={(e) => upload(e.target.files?.[0])} /></label>
   <div className="flex items-center gap-3 text-muted-foreground">Preview: {choice ? <CustomProjectIcon choice={choice} className="size-8 rounded object-cover" /> : <span className="text-muted-foreground">Repository icon</span>}</div>
   {error && <p role="alert" className="text-destructive">{error}</p>}
   <div className="flex justify-end gap-2"><Button variant="secondary" onClick={() => onOpenChange(false)}>Cancel</Button><Button variant="primary" onClick={() => save(choice)}>Save</Button></div>
  </div>
 </DialogContent></Dialog>;
}
