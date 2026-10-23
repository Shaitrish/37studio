import { useRef, useState } from "react";
import { FolderOpenIcon, GlobeIcon, PlusIcon, UsersThreeIcon } from "@phosphor-icons/react";
import { api, bytes, fail, folderFiles, go, Tenant, toast, cash, upload, useApi } from "../api";
import { Avatar, Empty, Field, Modal, Progress } from "../ui";

export default function Clients() {
  const { data: tenants } = useApi<Tenant[]>("/tenants");
  const [creating, setCreating] = useState(false);
  const [scanning, setScanning] = useState(false);
  return (
    <div className="page">
      <div className="page-head">
        <div className="spacer">
          <h1>Companies</h1>
          <p>Your businesses and brands. Each has a brand library, design system, knowledge base and social accounts the agents work from.</p>
        </div>
        <button className="btn" onClick={() => setCreating(true)}><PlusIcon size={16} /> Add manually</button>
        <button className="btn primary" onClick={() => setScanning(true)}><GlobeIcon size={16} /> Add from website or folder</button>
      </div>
      {tenants && tenants.length === 0 ? (
        <div className="card">
          <Empty icon={<UsersThreeIcon size={28} />} title="No companies yet">
            Paste its website or choose its project folder: the agent builds the brand library, design system and knowledge base.
            <button className="btn primary" onClick={() => setScanning(true)}><GlobeIcon size={16} /> Add from website or folder</button>
          </Empty>
        </div>
      ) : (
        <div className="grid cards">
          {tenants?.map((t) => (
            <a key={t.id} href={`#/clients/${t.id}`} className="card client-card">
              <div className="row">
                <Avatar tenant={t} />
                <div style={{ minWidth: 0 }}>
                  <h2 className="ellipsis">{t.name}</h2>
                  <div className="small muted ellipsis">{t.teacher || t.contact || t.email || "No contact yet"}</div>
                </div>
              </div>
              <div className="row small muted">
                <span className="swatch" style={{ background: t.brand_color }} />
                <span className="swatch" style={{ background: t.accent_color }} />
                <span className="spacer" />
                <span>{t.project_count} project{t.project_count === 1 ? "" : "s"}</span>·<span>{cash(t.cost_usd ?? 0)} spent</span>
              </div>
            </a>
          ))}
        </div>
      )}
      {creating && <TenantModal onClose={() => setCreating(false)} onSaved={(t) => go(`/clients/${t.id}`)} />}
      {scanning && <ScanModal onClose={() => setScanning(false)} />}
    </div>
  );
}

const languages = [["en", "English"], ["bn", "Bangla"], ["hi", "Hindi"], ["ur", "Urdu"], ["ar", "Arabic"], ["es", "Spanish"]];

export function TenantModal({ tenant, onClose, onSaved }: { tenant?: Tenant; onClose: () => void; onSaved: (t: Tenant) => void }) {
  const [f, setF] = useState<Partial<Tenant>>(tenant ?? { name: "", language: "en", brand_color: "#5f47d7", accent_color: "#f5b83d" });
  const set = (k: keyof Tenant) => (e: { target: { value: string } }) => setF((x) => ({ ...x, [k]: e.target.value }));
  async function save() {
    try {
      const t = await api<Tenant>(tenant ? `/tenants/${tenant.id}` : "/tenants", { method: tenant ? "PUT" : "POST", body: f });
      toast("Company saved");
      onSaved(t);
      onClose();
    } catch (e) { fail(e); }
  }
  return (
    <Modal
      title={tenant ? "Edit company" : "New company"}
      sub="Brand details go straight into the agent's instructions."
      size="lg"
      onClose={onClose}
      footer={<><button className="btn" onClick={onClose}>Cancel</button><button className="btn primary" onClick={save}>Save company</button></>}
    >
      <div className="form">
        <Field label="Company / brand name"><input className="input" value={f.name ?? ""} onChange={set("name")} autoFocus placeholder="Course37 Physics" /></Field>
        <Field label="Presenter / spokesperson"><input className="input" value={f.teacher ?? ""} onChange={set("teacher")} placeholder="Name shown on lower thirds" /></Field>
        <Field label="Contact person"><input className="input" value={f.contact ?? ""} onChange={set("contact")} /></Field>
        <Field label="Email"><input className="input" type="email" value={f.email ?? ""} onChange={set("email")} /></Field>
        <Field label="Phone"><input className="input" value={f.phone ?? ""} onChange={set("phone")} /></Field>
        <Field label="Website / course link"><input className="input" value={f.website ?? ""} onChange={set("website")} /></Field>
        <Field label="Spoken language" hint="drives transcription">
          <select className="select" value={f.language} onChange={set("language")}>
            {languages.map(([v, l]) => <option key={v} value={v}>{l}</option>)}
          </select>
        </Field>
        <Field label="Brand font" hint="optional">
          <input className="input" value={f.font ?? ""} onChange={set("font")} placeholder="e.g. Hind Siliguri, Inter" />
        </Field>
        <Field label="Primary color">
          <div className="row"><input className="input" type="color" value={f.brand_color} onChange={set("brand_color")} style={{ width: 56 }} /><input className="input mono" value={f.brand_color ?? ""} onChange={set("brand_color")} /></div>
        </Field>
        <Field label="Accent color">
          <div className="row"><input className="input" type="color" value={f.accent_color} onChange={set("accent_color")} style={{ width: 56 }} /><input className="input mono" value={f.accent_color ?? ""} onChange={set("accent_color")} /></div>
        </Field>
        <Field label="Profile" hint="who they are, what they teach, tone" full>
          <textarea className="textarea" value={f.bio ?? ""} onChange={set("bio")} placeholder="What they do, who they serve, how they sound. e.g. Bootstrapped SaaS for clinics; friendly, no jargon." />
        </Field>
        <Field label="Notes for the editor" hint="do's and don'ts" full>
          <textarea className="textarea" value={f.notes ?? ""} onChange={set("notes")} placeholder="Always end with the website CTA. No music with vocals." />
        </Field>
      </div>
    </Modal>
  );
}

/** Creates a company from its website and/or project folder (or re-scans one): the agent pulls logo, photos, colors and copy. */
export function ScanModal({ tenant, onClose, onStarted }: { tenant?: Tenant; onClose: () => void; onStarted?: () => void }) {
  const [url, setUrl] = useState(tenant?.website ?? "");
  const [notes, setNotes] = useState("");
  const [folder, setFolder] = useState<ReturnType<typeof folderFiles>>();
  const [busy, setBusy] = useState(false);
  const [pct, setPct] = useState<number>();
  const pick = useRef<HTMLInputElement>(null);
  const created = useRef<number | undefined>(undefined); // a new company whose folder upload failed: retry into it, don't make another
  const ready = !!url.trim() || !!folder?.keep.length || !!tenant?.source;
  async function start() {
    setBusy(true);
    try {
      let id = tenant?.id ?? created.current;
      // With a folder, the server waits for the upload; without one, creating the company starts the scan.
      if (!id) id = created.current = (await api<Tenant>("/tenants/scan", { body: { url, notes, folder: folder?.keep.length ? folder.name : "" } })).id;
      if (folder?.keep.length) await upload(`/tenants/${id}/source`, folder.keep, setPct, (f) => encodeURIComponent(f.webkitRelativePath));
      if (tenant || folder?.keep.length) await api(`/tenants/${id}/scan`, { body: { url, notes } });
      if (tenant) { onStarted?.(); onClose(); } else go(`/clients/${id}`);
    } catch (e) { fail(e); setBusy(false); setPct(undefined); }
  }
  const src = tenant?.source;
  return (
    <Modal
      title={tenant ? "Re-scan company" : "Add company"}
      sub="Reads the website and/or the company's own project folder, sorts every asset (logos, people, images, SVGs, videos, fonts) into the brand library, extracts the design system (colors, type scale, spacing, radii, shadows, components) and writes a knowledge base. Every project uses all three."
      onClose={onClose}
      footer={<><button className="btn" onClick={onClose}>Cancel</button><button className="btn primary" disabled={!ready || busy} onClick={start}><GlobeIcon size={15} /> {busy ? (pct === undefined ? "Starting…" : `Uploading ${pct}%`) : "Scan"}</button></>}
    >
      <div className="stack" style={{ gap: 14 }}>
        <Field label="Website" hint="optional with a folder"><input className="input" value={url} onChange={(e) => setUrl(e.target.value)} placeholder="course37.com" autoFocus onKeyDown={(e) => e.key === "Enter" && ready && start()} /></Field>
        <Field label="Project folder" hint="optional: their website, app or docs folder">
          <div className="row">
            <button className="btn" type="button" disabled={busy} onClick={() => pick.current?.click()}><FolderOpenIcon size={15} /> {folder || src ? "Choose another" : "Choose folder"}</button>
            <span className="small muted spacer ellipsis">
              {folder ? `${folder.name} · ${folder.keep.length} files · ${bytes(folder.bytes)}${folder.skipped ? ` · ${folder.skipped} left out` : ""}`
                : src ? `${src.name} · ${src.files} files, copied ${src.uploaded.slice(0, 10)}` : "None"}
            </span>
          </div>
          <input ref={pick} type="file" multiple hidden {...{ webkitdirectory: "" }} onChange={(e) => e.target.files?.length && setFolder(folderFiles(e.target.files))} />
        </Field>
        {pct !== undefined && <Progress pct={pct} />}
        <Field label="Anything we already know" hint="optional">
          <textarea className="textarea" rows={3} value={notes} onChange={(e) => setNotes(e.target.value)} placeholder="The founder fronts their videos. Focus on the paid product, not the free blog." />
        </Field>
        <p className="small muted">The folder is copied into 37Studio, without dependencies, build output, hidden files or anything that looks like a secret (keys, .env). Every agent can read it, so a video about a feature uses its real screens and text. Choose it again after big changes.</p>
        <p className="small muted">Takes 5–10 minutes. Manual edits are kept: a re-scan only fills fields that are still empty. Uses Sonnet 5.5; the cost shows under Costs.</p>
      </div>
    </Modal>
  );
}
