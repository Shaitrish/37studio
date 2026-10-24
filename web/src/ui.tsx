import { ReactNode, useEffect, useRef, useState } from "react";
import { CaretDownIcon, CaretLeftIcon, CaretRightIcon, DownloadSimpleIcon, FileAudioIcon, FileTextIcon, FilmStripIcon, ImageIcon, PencilSimpleIcon, TrashIcon, TextTIcon, CloudArrowUpIcon, XIcon } from "@phosphor-icons/react";
import { AssetMeta, bytes, clock, fail, FileInfo, Project, Quality, Tenant } from "./api";

export function Modal({ title, sub, onClose, children, footer, size }: {
  title: string; sub?: string; onClose: () => void; children: ReactNode; footer?: ReactNode; size?: "lg" | "xl";
}) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    addEventListener("keydown", onKey);
    return () => removeEventListener("keydown", onKey);
  }, [onClose]);
  return (
    <div className="backdrop" onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
      <div className={`modal ${size ?? ""}`} role="dialog" aria-modal="true" aria-label={title}>
        <div className="modal-h">
          <div className="spacer">
            <h2>{title}</h2>
            {sub && <p className="small">{sub}</p>}
          </div>
          <button className="btn ghost sm icon" onClick={onClose} aria-label="Close"><XIcon size={16} /></button>
        </div>
        <div className="modal-b">{children}</div>
        {footer && <div className="modal-f">{footer}</div>}
      </div>
    </div>
  );
}

const statusStyle: Record<string, [string, string]> = {
  draft: ["", "Draft"],
  queued: ["blue", "Queued"],
  running: ["violet running", "Agent working"],
  review: ["amber", "In review"],
  delivered: ["green", "Delivered"],
  failed: ["red", "Failed"],
  scheduled: ["blue", "Scheduled"],
  building: ["violet running", "Making video"],
  rendering: ["violet running", "Rendering"],
  captions: ["violet running", "Writing captions"],
  approve: ["amber", "Needs approval"],
  approval: ["amber", "Needs approval"],
  skipped: ["", "Skipped"],
  posting: ["violet running", "Posting"],
  posted: ["green", "Posted"],
};

export function Status({ value }: { value: string }) {
  const [cls, label] = statusStyle[value] ?? ["", value];
  return <span className={`pill ${cls}`}><span className="dot" />{label}</span>;
}

// A video queued from a source shows its place in the queue until it's exported; "failed" too, so it stands out.
export const projectStatus = (p: Pick<Project, "running" | "status" | "clip_stage">) =>
  p.running ? "running" : p.clip_stage && p.clip_stage !== "done" ? p.clip_stage : p.status;

export function Empty({ icon, title, children }: { icon: ReactNode; title: string; children?: ReactNode }) {
  return <div className="empty">{icon}<strong>{title}</strong>{children}</div>;
}

export function Field({ label, hint, full, children }: { label: string; hint?: string; full?: boolean; children: ReactNode }) {
  return (
    <label className={`field ${full ? "full" : ""}`}>
      <span>{label} {hint && <span className="hint">· {hint}</span>}</span>
      {children}
    </label>
  );
}

export function Seg<T extends string>({ value, options, onChange }: { value: T; options: { value: T; label: string }[]; onChange: (v: T) => void }) {
  return (
    <div className="seg" role="radiogroup">
      {options.map((o) => (
        <button type="button" key={o.value} role="radio" aria-checked={o.value === value} className={o.value === value ? "on" : ""} onClick={() => onChange(o.value)}>
          {o.label}
        </button>
      ))}
    </div>
  );
}

export function DropZone({ onFiles, accept, label = "Drop files here or click to browse", compact }: {
  onFiles: (files: File[]) => void; accept?: string; label?: string; compact?: boolean;
}) {
  const input = useRef<HTMLInputElement>(null);
  const [over, setOver] = useState(false);
  return (
    <div
      className={`drop ${over ? "over" : ""} ${compact ? "compact" : ""}`}
      role="button"
      tabIndex={0}
      onClick={() => input.current?.click()}
      onKeyDown={(e) => (e.key === "Enter" || e.key === " ") && input.current?.click()}
      onDragOver={(e) => { e.preventDefault(); setOver(true); }}
      onDragLeave={() => setOver(false)}
      onDrop={(e) => { e.preventDefault(); setOver(false); onFiles([...e.dataTransfer.files]); }}
    >
      <CloudArrowUpIcon size={compact ? 16 : 22} />
      <span>{label}</span>
      <input ref={input} type="file" multiple hidden accept={accept} onChange={(e) => { onFiles([...(e.target.files ?? [])]); e.target.value = ""; }} />
    </div>
  );
}

export function KindIcon({ kind, size = 20 }: { kind: string; size?: number }) {
  const I = { video: FilmStripIcon, image: ImageIcon, audio: FileAudioIcon, font: TextTIcon }[kind] ?? FileTextIcon;
  return <I size={size} />;
}

const metaFields: [keyof AssetMeta, string, string, number][] = [
  ["caption", "Caption", "what is on screen: who, which product or feature", 4],
  ["use", "Use in video", "hero shot, testimonial, feature demo, background…", 2],
  ["context", "Context", "nearby heading or text", 2],
  ["section", "Section", "where it appeared", 0],
  ["source", "Source", "page URL", 0],
];

/** Big preview beside every metadata field project agents read when picking assets. Ctrl+Enter saves and moves on. */
export function AssetEditor({ f, meta, pos, onSave, onStep, onClose }: {
  f: FileInfo; meta?: AssetMeta; pos?: [number, number]; onSave: (m: AssetMeta) => Promise<unknown>; onStep?: (d: number) => void; onClose: () => void;
}) {
  const [m, setM] = useState<AssetMeta>(meta ?? {});
  const [busy, setBusy] = useState(false);
  const [dims, setDims] = useState("");
  const dirty = metaFields.some(([k]) => (m[k] ?? "") !== (meta?.[k] ?? ""));
  const [i, n] = pos ?? [0, 1];
  // Unsaved edits are saved before moving or closing; a failed save keeps the editor open.
  async function save(then: () => void) {
    if (dirty) {
      setBusy(true);
      try { await onSave(m); } catch (e) { setBusy(false); return fail(e); }
    }
    then();
  }
  const step = (d: number) => save(() => onStep?.(d));
  const close = () => (!dirty || confirm("Discard unsaved changes?")) && onClose();
  const preview =
    f.kind === "image" ? <img src={f.url} alt="" onLoad={(e) => setDims(`${e.currentTarget.naturalWidth}×${e.currentTarget.naturalHeight}`)} /> :
    f.kind === "video" ? <video src={f.url} controls playsInline onLoadedMetadata={(e) => setDims(`${e.currentTarget.videoWidth}×${e.currentTarget.videoHeight}`)} /> :
    f.kind === "audio" ? <audio src={f.url} controls /> :
    <KindIcon kind={f.kind} size={40} />;
  return (
    <Modal title={f.name} sub={[f.seconds && clock(f.seconds), dims, bytes(f.size), f.path].filter(Boolean).join(" · ")} size="xl" onClose={close}
      footer={<>
        {n > 1 && <>
          <button className="btn sm icon" disabled={busy || i === 0} onClick={() => step(-1)} aria-label="Previous asset" title="Previous"><CaretLeftIcon size={14} /></button>
          <span className="small muted mono">{i + 1} / {n}</span>
          <button className="btn sm icon" disabled={busy || i === n - 1} onClick={() => step(1)} aria-label="Next asset" title="Next (Ctrl+Enter saves and moves on)"><CaretRightIcon size={14} /></button>
        </>}
        <span className="spacer" />
        <button className="btn" onClick={close}>Cancel</button>
        <button className="btn primary" disabled={busy || !dirty} onClick={() => save(onClose)}>{busy ? "Saving…" : "Save"}</button>
      </>}>
      <div className="asset-ed" onKeyDown={(e) => {
        if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) { e.preventDefault(); i < n - 1 ? step(1) : save(onClose); }
      }}>
        <div className={`pv ${f.kind === "image" ? "checker" : ""}`}>{preview}</div>
        <div className="stack" style={{ gap: 12 }}>
          {metaFields.map(([k, label, hint, rows], j) => (
            <Field key={k} label={label} hint={hint}>
              {rows ? <textarea className="textarea" rows={rows} autoFocus={j === 0} value={m[k] ?? ""} onChange={(e) => setM({ ...m, [k]: e.target.value })} />
                : <input className="input" value={m[k] ?? ""} onChange={(e) => setM({ ...m, [k]: e.target.value })} />}
            </Field>
          ))}
          {m.source?.startsWith("http") && <a className="small muted ellipsis" href={m.source} target="_blank" rel="noreferrer">Open source page ↗</a>}
        </div>
      </div>
    </Modal>
  );
}

export function FileTile({ f, onOpen, onDelete, selected, contain, meta, onEdit, moveTo }: {
  f: FileInfo; onOpen?: () => void; onDelete?: () => void; selected?: boolean; contain?: boolean;
  meta?: AssetMeta; onEdit?: () => void; moveTo?: { options: { id: string; label: string }[]; onMove: (folder: string) => void };
}) {
  const thumb =
    f.kind === "image" ? <img src={f.url} alt="" loading="lazy" /> :
    f.kind === "video" ? <video src={f.url + "#t=0.5"} preload="metadata" muted playsInline /> :
    <KindIcon kind={f.kind} size={26} />;
  return (
    <div className={`tile ${selected ? "selected" : ""}`}>
      {onOpen ? (
        <button className="cover" onClick={onOpen} aria-label={`Preview ${f.name}`}>
          <div className={`thumb ${contain ? "contain" : ""}`}>{thumb}</div>
        </button>
      ) : <div className={`thumb ${contain ? "contain" : ""}`}>{thumb}</div>}
      <span className="ext">{f.name.split(".").pop()?.toUpperCase()}</span>
      <div className="meta">
        <div className="ellipsis" title={f.path}>{f.name}</div>
        <div className="faint">{f.seconds ? clock(f.seconds) + " · " : ""}{bytes(f.size)}</div>
        {meta?.caption && <div className="caption" title={[meta.caption, meta.section && `Section: ${meta.section}`, meta.use && `Use: ${meta.use}`, meta.source].filter(Boolean).join("\n")}>{meta.caption}</div>}
      </div>
      <div className="acts">
        {moveTo && (
          <select className="btn sm move" value="" aria-label={`Move ${f.name}`} title="Move to folder" onChange={(e) => e.target.value && moveTo.onMove(e.target.value)}>
            <option value="">Move…</option>
            {moveTo.options.map((o) => <option key={o.id} value={o.id}>{o.label}</option>)}
          </select>
        )}
        {onEdit && <button className="btn sm icon" aria-label={`Edit ${f.name}`} title="Edit caption" onClick={onEdit}><PencilSimpleIcon size={14} /></button>}
        <a className="btn sm icon" href={f.url} download={f.name} aria-label={`Download ${f.name}`} title="Download"><DownloadSimpleIcon size={14} /></a>
        {onDelete && <button className="btn sm icon danger" onClick={onDelete} aria-label={`Delete ${f.name}`} title="Delete"><TrashIcon size={14} /></button>}
      </div>
    </div>
  );
}

export function Avatar({ tenant, size }: { tenant: Pick<Tenant, "name" | "brand_color" | "assets" | "logo">; size?: "lg" | "sm" | "xs" }) {
  const logo = tenant.assets?.find((a) => a.path.startsWith("logo/") && a.kind === "image") ?? (tenant.logo ? { url: tenant.logo } : undefined);
  const initials = tenant.name.split(/\s+/).map((w) => w[0]).slice(0, 2).join("").toUpperCase();
  return (
    <div className={`avatar ${size ?? ""}`} style={{ background: tenant.brand_color || undefined }}>
      {logo ? <img src={logo.url} alt="" /> : initials}
    </div>
  );
}

/** A brand's logo beside its name; falls back to the plain name until the list loads. */
export function TenantTag({ tenants, id, name }: { tenants?: Tenant[]; id: number; name: string }) {
  const t = tenants?.find((x) => x.id === id);
  return <span className="tenant-tag">{t && <Avatar tenant={t} size="xs" />}<span className="ellipsis">{name}</span></span>;
}

/** A brand dropdown that shows each logo, which a native select can't. `all` adds a "no filter" entry. */
export function TenantPicker({ tenants, value, onChange, all }: { tenants?: Tenant[]; value?: number; onChange: (id?: number) => void; all?: string }) {
  const ref = useRef<HTMLDetailsElement>(null);
  useEffect(() => {
    const away = (e: MouseEvent) => ref.current && !ref.current.contains(e.target as Node) && (ref.current.open = false);
    addEventListener("mousedown", away);
    return () => removeEventListener("mousedown", away);
  }, []);
  const cur = tenants?.find((t) => t.id === value);
  const pick = (id?: number) => { ref.current!.open = false; onChange(id); };
  return (
    <details ref={ref} className="picker">
      <summary className="select" aria-label="Brand">
        {cur ? <><Avatar tenant={cur} size="xs" /><span className="ellipsis">{cur.name}</span></> : <span className="ellipsis">{all ?? "Choose a brand"}</span>}
        <CaretDownIcon size={13} className="faint" />
      </summary>
      <div className="picker-menu">
        {all && <button type="button" className={!cur ? "on" : ""} onClick={() => pick()}>{all}</button>}
        {tenants?.map((t) => (
          <button type="button" key={t.id} className={t.id === value ? "on" : ""} onClick={() => pick(t.id)}>
            <Avatar tenant={t} size="xs" /><span className="ellipsis">{t.name}</span>
          </button>
        ))}
      </div>
    </details>
  );
}

export function Progress({ pct }: { pct: number }) {
  return <div className="progress" role="progressbar" aria-valuenow={pct}><div style={{ width: `${pct}%` }} /></div>;
}

// 37Studio mark: squared "37" with an ember record dot. Tile and numerals invert with the theme.
export function Mark({ size = 28 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 32 32" fill="none" aria-hidden="true">
      <rect width="32" height="32" rx="3" fill="var(--ink)" />
      <path d="M8 9.5H14.5V22.5H8M10.2 16H14.5M17.6 9.5H24.5L19.4 22.5" stroke="var(--ink-fg)" strokeWidth="2.6" />
      <rect x="22" y="19.8" width="2.8" height="2.8" fill="var(--primary)" />
    </svg>
  );
}

/** Model menu entries: Claude Code's tiers first, then each provider OpenCode can run with this copy's keys. */
export function ModelOptions({ qualities, claudeLabel = (q: Quality) => q.label }: { qualities: Quality[]; claudeLabel?: (q: Quality) => string }) {
  const groups = [...new Set(qualities.filter((q) => q.harness).map((q) => q.group ?? "OpenCode"))];
  return (
    <>
      <optgroup label="Claude">{qualities.filter((q) => !q.harness).map((q) => <option key={q.id} value={q.id}>{claudeLabel(q)}</option>)}</optgroup>
      {groups.map((g) => (
        <optgroup key={g} label={g}>
          {qualities.filter((q) => q.harness && (q.group ?? "OpenCode") === g).map((q) => <option key={q.id} value={q.id}>{q.label}</option>)}
        </optgroup>
      ))}
    </>
  );
}

/** Claude's tiers as buttons, every other model in a menu beside them, so a long list doesn't crowd the choice most people make. */
export function ModelPicker({ value, qualities, onChange }: { value: string; qualities: Quality[]; onChange: (id: string) => void }) {
  const tiers = qualities.filter((q) => !q.harness);
  const others = qualities.filter((q) => q.harness);
  const other = others.find((q) => q.id === value);
  return (
    <div className="row wrap">
      <Seg value={other ? "" : value} options={tiers.map((q) => ({ value: q.id, label: q.label }))} onChange={onChange} />
      {others.length > 0 && (
        <select className="select" style={{ width: "auto", fontWeight: other ? 600 : undefined }} value={other ? value : ""} onChange={(e) => e.target.value && onChange(e.target.value)} aria-label="Other models">
          <option value="">Other models…</option>
          {[...new Set(others.map((q) => q.group ?? "OpenCode"))].map((g) => (
            <optgroup key={g} label={g}>{others.filter((q) => (q.group ?? "OpenCode") === g).map((q) => <option key={q.id} value={q.id}>{q.label}</option>)}</optgroup>
          ))}
        </select>
      )}
    </div>
  );
}
