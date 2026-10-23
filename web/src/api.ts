import { useCallback, useEffect, useState } from "react";

export interface FileInfo { path: string; name: string; url: string; kind: "video" | "image" | "audio" | "font" | "other"; size: number; seconds?: number; mod: string }
export interface QuoteLine { label: string; detail: string; cost: number }
export interface Quote { lines: QuoteLine[]; cost: number }
export interface Tenant {
  id: number; name: string; contact: string; email: string; phone: string; website: string; teacher: string; bio: string;
  language: string; brand_color: string; accent_color: string; font: string; notes: string;
  project_count?: number; cost_usd?: number; assets?: FileInfo[]; projects?: Project[];
  scan_status: "" | "running" | "done" | "failed"; scan_note: string; knowledge?: string; design?: string; scanning?: boolean;
  asset_meta?: Record<string, AssetMeta>; source?: SourceInfo | null;
}
/** The company's project folder, copied into the app (source.go). */
export interface SourceInfo { name: string; files: number; bytes: number; uploaded: string }
export interface Project {
  id: number; tenant_id: number; tenant_name: string; campaign_id: number | null; campaign_name: string | null; title: string; kind: string; aspect: string; seconds: number;
  quality: string; brief: string; focus: string; language: string; status: string; session_id: string; created_at: string;
  quote: Quote; running: boolean; studio_port?: number | null; inputs?: FileInfo[];
  render?: { pct: number; stage: string; out: string; started: string } | null;
  generated?: (FileInfo & { provider: string; item: string; cost: number })[]; comp_assets?: FileInfo[]; outputs?: FileInfo[]; drafts?: FileInfo[]; composition?: string;
  posts?: Post[]; social?: Captions | null;
  autopilot_id?: number | null; autopilot_slot?: string; autopilot_stage?: string;
  // Several videos from one source (clip.go): the source lists its videos; each video names its source.
  parent_id?: number | null; parent_title?: string; clip_stage?: "" | "queued" | "building" | "rendering" | "done" | "failed";
  children?: Pick<Project, "id" | "title" | "status" | "clip_stage">[];
  template_id?: number | null;
  transcript?: { state: "running" | "done" | "skipped" | "failed"; pct?: number; line?: string; from?: string } | null;
  import?: { url: string; pct: number; line: string; error?: string; done: boolean } | null;
}
/** A saved composition new projects can start from (template.go). tenant_id null: every company. */
export interface Template {
  id: number; name: string; notes: string; tenant_id: number | null; tenant_name: string | null; project_id: number;
  kind: string; aspect: string; seconds: number; thumb?: string; created_at: string;
}
export interface LibraryItem { file: FileInfo; project_id: number; title: string; tenant_id: number; tenant_name: string }
export type Platform = "facebook" | "instagram" | "youtube" | "tiktok" | "whatsapp";
export interface SocialAccount { id: number; tenant_id: number; platform: Platform; account_id: string; name: string; picture: string; created_at: string }
export interface Post {
  id: number; project_id: number; account_id: number; platform: Platform; account_name: string; file: string; title: string; caption: string;
  privacy: string; status: "approval" | "scheduled" | "posting" | "posted" | "failed"; publish_at: string; url: string; error: string; created_at: string;
}
export type Captions = Partial<Record<Platform | "youtube_title", string>>;
export interface AssetMeta { caption?: string; section?: string; context?: string; source?: string; use?: string }
export interface Campaign {
  id: number; tenant_id: number; tenant_name: string; name: string; brief: string; video_count: number; created_at: string;
  projects?: Project[]; cost?: number;
}
export interface Voice { id: string; name: string; language?: string; gender?: string; use?: string; preview?: string }
export const languages: [string, string][] = [["bn", "Bangla"], ["en", "English"], ["hi", "Hindi"], ["ur", "Urdu"], ["ar", "Arabic"], ["es", "Spanish"]];
export const languageName = (code: string) => languages.find(([c]) => c === code)?.[1] ?? code;
export interface Preset { id: string; label: string; aspect: string; seconds: number; blurb: string }
export interface Quality { id: string; label: string; model: string; effort: string; note: string; harness: string }
export interface Meta { presets: Preset[]; qualities: Quality[]; hf_version: string; data_dir: string; env: Record<string, boolean> }
export interface ChatEvent { id: number; kind: string; body: string; created_at: string }

export async function api<T = any>(path: string, opts: { method?: string; body?: unknown } = {}): Promise<T> {
  const res = await fetch("/api" + path, {
    method: opts.method ?? (opts.body !== undefined ? "POST" : "GET"),
    headers: opts.body !== undefined ? { "Content-Type": "application/json" } : undefined,
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
  });
  const data = await res.json().catch(() => null);
  if (!res.ok) throw new Error(data?.error ?? res.statusText);
  return data as T;
}

export const repoURL = "https://github.com/Shaitrish/37studio";

/** Multipart upload via XHR: fetch() can't report upload progress, and lectures are gigabytes. */
export function upload(path: string, files: File[], onProgress?: (pct: number) => void, field?: (f: File) => string): Promise<void> {
  return new Promise((resolve, reject) => {
    const form = new FormData();
    files.forEach((f) => form.append(field ? field(f) : "file", f, f.name));
    const xhr = new XMLHttpRequest();
    xhr.open("POST", "/api" + path);
    xhr.upload.onprogress = (e) => e.lengthComputable && onProgress?.(Math.round((e.loaded / e.total) * 100));
    xhr.onload = () => (xhr.status < 300 ? resolve() : reject(new Error(JSON.parse(xhr.responseText || "{}").error ?? "Upload failed")));
    xhr.onerror = () => reject(new Error("Upload failed"));
    xhr.send(form);
  });
}

// Left out of a picked project folder, like skipSource in source.go (which checks again): dependencies, build output,
// hidden files, anything that looks like a secret, and files over 100 MB.
const skipDirs = new Set(["node_modules", "vendor", "dist", "build", "out", "target", "coverage", "__pycache__", "venv", "Pods", "DerivedData"]);
const secretFile = /(\.(pem|key|p12|pfx|jks|keystore|kdbx|sqlite3?|db)$|^id_(rsa|dsa|ecdsa|ed25519)|credential|secret|service-account)/i;
export function folderFiles(list: FileList) {
  const keep: File[] = [];
  let skipped = 0;
  for (const f of Array.from(list)) {
    const dirs = f.webkitRelativePath.split("/").slice(1, -1);
    if (dirs.some((d) => skipDirs.has(d) || d.startsWith(".")) || f.name.startsWith(".") || secretFile.test(f.name) || f.size > 100e6) skipped++;
    else keep.push(f);
  }
  return { name: list[0]?.webkitRelativePath.split("/")[0] ?? "", keep, skipped, bytes: keep.reduce((n, f) => n + f.size, 0) };
}

/** Fetches `path` on mount and whenever it changes; `reload` refetches without flashing empty state. */
export function useApi<T>(path: string | null) {
  const [data, setData] = useState<T>();
  const [error, setError] = useState<string>();
  const reload = useCallback(() => {
    if (!path) return Promise.resolve();
    return api<T>(path).then((d) => { setData(d); setError(undefined); }, (e) => setError(e.message));
  }, [path]);
  useEffect(() => { reload(); }, [reload]);
  return { data, error, reload, setData };
}

/** Taka uses lakh/crore grouping and the ৳ sign: ৳12,34,567.50. */
export const money = (n: number, currency = "USD") =>
  new Intl.NumberFormat(currency === "BDT" ? "en-IN" : "en-US", { style: "currency", currency, currencyDisplay: "narrowSymbol", maximumFractionDigits: 2 }).format(n || 0);

// Costs are stored in USD (what providers bill); everything is shown in the display currency from Settings.
let display = { currency: "USD", fx: 1 };
export const setDisplay = (s: Record<string, string>) => { display = { currency: s.currency || "USD", fx: Number(s.fx_rate) || 1 }; };
export const cash = (usdAmount: number) => money(usdAmount * display.fx, display.currency);

export function bytes(n: number) {
  const u = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  while (n >= 1000 && i < u.length - 1) { n /= 1000; i++; }
  return `${n.toFixed(i > 1 ? 1 : 0)} ${u[i]}`;
}

export function clock(sec = 0) {
  const s = Math.round(sec);
  const hms = [Math.floor(s / 3600), Math.floor((s % 3600) / 60), s % 60];
  return hms[0] ? `${hms[0]}:${String(hms[1]).padStart(2, "0")}:${String(hms[2]).padStart(2, "0")}` : `${hms[1]}:${String(hms[2]).padStart(2, "0")}`;
}

/** SQLite current_timestamp is UTC without a zone marker. */
export const parseTime = (t: string) => new Date(t.includes("T") ? t : t.replace(" ", "T") + "Z");

export function ago(t: string) {
  const s = (Date.now() - parseTime(t).getTime()) / 1000;
  if (s < 60) return "just now";
  if (s < 3600) return `${Math.floor(s / 60)}m ago`;
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`;
  if (s < 86400 * 7) return `${Math.floor(s / 86400)}d ago`;
  return parseTime(t).toLocaleDateString();
}

export const date = (t: string) => (t ? parseTime(t).toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" }) : "");

export const go = (path: string) => { location.hash = path; };

type ToastFn = (msg: string, err?: boolean) => void;
let toastFn: ToastFn = () => {};
export const setToast = (fn: ToastFn) => { toastFn = fn; };
export const toast: ToastFn = (msg, err) => toastFn(msg, err);
export const fail = (e: unknown) => toast(e instanceof Error ? e.message : String(e), true);
