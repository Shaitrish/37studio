import { useEffect, useState } from "react";
import { WarningIcon, CaretRightIcon, FilmSlateIcon, GlobeIcon, PencilSimpleIcon, PlusIcon, TrashIcon, UsersThreeIcon } from "@phosphor-icons/react";
import { api, Campaign, fail, go, Project, Tenant, toast, upload, useApi } from "../api";
import { AssetEditor, Avatar, DropZone, Empty, FileTile, Progress, Seg } from "../ui";
import { ProjectTable } from "./Projects";
import { NewProjectModal } from "./NewProject";
import { ScanModal, TenantModal } from "./Clients";
import { CampaignGrid, CampaignModal } from "./Campaigns";
import { SocialTab } from "./Social";
import { AutopilotTab } from "./Autopilot";

const folders = [
  { id: "logo", label: "Logos & marks", hint: "SVG or transparent PNG, light and dark variants", accept: ".svg,image/*" },
  { id: "people", label: "People", hint: "founders, team, presenters, customers", accept: "image/*" },
  { id: "images", label: "Images", hint: "photos, product shots, heroes, backgrounds", accept: "image/*" },
  { id: "graphics", label: "Icons & illustrations", hint: "SVG icons, illustrations, patterns", accept: ".svg,image/*" },
  { id: "videos", label: "Videos", hint: "brand videos, b-roll, intros", accept: "video/*" },
  { id: "fonts", label: "Fonts", hint: "TTF, OTF, WOFF", accept: ".ttf,.otf,.woff,.woff2" },
  { id: "audio", label: "Audio & music", hint: "stings, licensed music, voiceovers", accept: "audio/*" },
  { id: "design", label: "Design references", hint: "site screenshots, style guides, raw tokens", accept: undefined },
  { id: "other", label: "Other", hint: "anything else", accept: undefined },
];
const contained = new Set(["logo", "graphics"]);
const extOf = (name: string) => (name.includes(".") ? name.split(".").pop()!.toLowerCase() : "other");

export default function ClientPage({ id, tab = "library" }: { id: number; tab?: string }) {
  const { data: t, error, reload } = useApi<Tenant>(`/tenants/${id}`);
  const { data: projects } = useApi<Project[]>(`/projects?tenant=${id}`);
  const setTab = (t: string) => { location.hash = `/clients/${id}/${t}`; };
  const [editing, setEditing] = useState(false);
  const [creating, setCreating] = useState(false);
  const [rescan, setRescan] = useState(false);
  const [newCampaign, setNewCampaign] = useState(false);
  const { data: campaigns } = useApi<Campaign[]>(`/campaigns?tenant=${id}`);
  const scanning = !!t && (t.scanning || t.scan_status === "running");
  useEffect(() => {
    if (!scanning) return;
    const timer = setInterval(reload, 4000);
    return () => clearInterval(timer);
  }, [scanning, reload]);

  if (error) return <div className="page"><Empty icon={<UsersThreeIcon size={28} />} title="Brand not found"><a className="btn" href="#/clients">Back to brands</a></Empty></div>;
  if (!t) return null;

  async function remove() {
    if (!confirm(`Delete ${t!.name} and their whole asset library? This cannot be undone.`)) return;
    try { await api(`/tenants/${id}`, { method: "DELETE" }); go("/clients"); } catch (e) { fail(e); }
  }

  return (
    <div className="page">
      <div>
        <div className="crumbs"><a href="#/clients">Brands</a><CaretRightIcon size={14} /></div>
        <div className="page-head">
          <Avatar tenant={t} size="lg" />
          <div className="spacer" style={{ minWidth: 0 }}>
            <h1 className="ellipsis">{t.name}</h1>
            <p>{[t.teacher, t.email, t.phone].filter(Boolean).join(" · ") || "No contact details yet"}</p>
          </div>
          <button className="btn" disabled={scanning} onClick={() => setRescan(true)}><GlobeIcon size={14} /> {t.website || t.source ? "Re-scan" : "Scan website or folder"}</button>
          <button className="btn" onClick={() => setEditing(true)}><PencilSimpleIcon size={14} /> Edit</button>
          <button className="btn primary" onClick={() => setCreating(true)}><PlusIcon size={16} /> New video</button>
        </div>
      </div>

      {scanning && (
        <div className="card card-b row" style={{ background: "var(--primary-soft)", borderColor: "var(--border)" }}>
          <div className="typing"><i /><i /><i /></div>
          <span><strong>Scanning {t.website || t.source?.name}</strong> <span className="muted">· {t.scan_note || "Starting."} Takes 5–10 minutes; this page updates by itself.</span></span>
        </div>
      )}
      {!scanning && t.scan_status === "failed" && (
        <div className="card card-b row" style={{ background: "var(--danger-soft)", borderColor: "var(--border)", alignItems: "flex-start" }}>
          <WarningIcon size={16} color="var(--danger)" style={{ flex: "none", marginTop: 2 }} />
          <div className="spacer small" style={{ whiteSpace: "pre-wrap" }}><strong>Scan failed.</strong> {t.scan_note}</div>
          <button className="btn sm" onClick={() => setRescan(true)}>Try again</button>
        </div>
      )}

      <Seg value={tab} onChange={setTab} options={[
        { value: "library", label: `Brand library (${t.assets?.length ?? 0})` },
        { value: "campaigns", label: `Campaigns (${campaigns?.length ?? 0})` },
        { value: "projects", label: `Projects (${projects?.length ?? 0})` },
        { value: "social", label: "Social" },
        { value: "autopilot", label: "Autopilot" },
        { value: "design", label: "Design system" },
        { value: "knowledge", label: "Knowledge" },
        { value: "profile", label: "Profile" },
      ]} />

      {tab === "library" && <Library tenant={t} reload={reload} />}

      {tab === "social" && <SocialTab tenantId={id} />}

      {tab === "autopilot" && <AutopilotTab tenant={t} />}

      {tab === "design" && <DesignSystem tenant={t} reload={reload} onScan={() => setRescan(true)} />}

      {tab === "campaigns" && (
        <div className="stack">
          {!!campaigns?.length && <div className="row"><span className="spacer" /><button className="btn" onClick={() => setNewCampaign(true)}><PlusIcon size={15} /> New campaign</button></div>}
          <CampaignGrid campaigns={campaigns} onNew={() => setNewCampaign(true)} hideClient />
        </div>
      )}

      {tab === "projects" && (
        <div className="card">
          {projects?.length ? <ProjectTable projects={projects} hideClient /> : (
            <Empty icon={<FilmSlateIcon size={28} />} title="No projects for this brand yet">
              <button className="btn primary" onClick={() => setCreating(true)}><PlusIcon size={16} /> New video</button>
            </Empty>
          )}
        </div>
      )}

      {tab === "knowledge" && <Knowledge tenant={t} reload={reload} onScan={() => setRescan(true)} />}

      {tab === "profile" && (
        <div className="card">
          <div className="card-b stack">
            <Detail label="Profile" value={t.bio} />
            <Detail label="Editor notes" value={t.notes} />
            <div className="grid cols-2">
              <Detail label="Website" value={t.website} />
              <Detail label="Project folder" value={t.source ? `${t.source.name} · ${t.source.files} files, copied ${t.source.uploaded.slice(0, 10)}` : ""} />
              <Detail label="Spoken language" value={t.language} />
              <Detail label="Brand font" value={t.font} />
            </div>
            <div className="row"><span className="spacer" /><button className="btn ghost danger sm" onClick={remove}><TrashIcon size={14} /> Delete brand</button></div>
          </div>
        </div>
      )}

      {newCampaign && <CampaignModal tenantId={id} onClose={() => setNewCampaign(false)} />}
      {rescan && <ScanModal tenant={t} onClose={() => setRescan(false)} onStarted={reload} />}
      {editing && <TenantModal tenant={t} onClose={() => setEditing(false)} onSaved={() => reload()} />}
      {creating && <NewProjectModal tenantId={id} onClose={() => setCreating(false)} />}
    </div>
  );
}

function Detail({ label, value }: { label: string; value?: string }) {
  return (
    <div>
      <div className="small muted">{label}</div>
      <div style={{ whiteSpace: "pre-wrap" }}>{value || <span className="faint">-</span>}</div>
    </div>
  );
}

function Library({ tenant, reload }: { tenant: Tenant; reload: () => void }) {
  const [ext, setExt] = useState("all");
  const counts = new Map<string, number>();
  tenant.assets?.forEach((a) => counts.set(extOf(a.name), (counts.get(extOf(a.name)) ?? 0) + 1));
  const types = [...counts.entries()].sort((a, b) => b[1] - a[1]);
  return (
    <div className="stack">
      {types.length > 1 && (
        <Seg value={ext} onChange={setExt} options={[
          { value: "all", label: `All ${tenant.assets?.length ?? 0}` },
          ...types.map(([e, n]) => ({ value: e, label: `${e.toUpperCase()} ${n}` })),
        ]} />
      )}
      {folders.map((f) => <Folder key={f.id} tenant={tenant} folder={f} reload={reload} ext={ext} />)}
    </div>
  );
}

function Folder({ tenant, folder, reload, ext }: { tenant: Tenant; folder: (typeof folders)[number]; reload: () => void; ext: string }) {
  const files = tenant.assets?.filter((a) => a.path.startsWith(folder.id + "/") && (ext === "all" || extOf(a.name) === ext)) ?? [];
  const [pct, setPct] = useState<number>();
  const [at, setAt] = useState<number>();
  const cur = at === undefined ? undefined : files[at];
  const short = (f: (typeof files)[number]) => ({ ...f, name: f.path.slice(folder.id.length + 1) });
  async function add(list: File[]) {
    if (!list.length) return;
    setPct(0);
    try { await upload(`/tenants/${tenant.id}/assets?folder=${folder.id}`, list, setPct); toast(`Added to ${folder.label}`); reload(); } catch (e) { fail(e); }
    setPct(undefined);
  }
  const edit = (body: object) => api(`/tenants/${tenant.id}/assets`, { method: "PUT", body }).then(reload, fail);
  async function del(path: string) {
    if (!confirm(`Delete ${path}?`)) return;
    api(`/tenants/${tenant.id}/assets?path=${encodeURIComponent(path)}`, { method: "DELETE" }).then(reload, fail);
  }
  if (ext !== "all" && !files.length) return null;
  return (
    <div className="card">
      <div className="card-h">
        <h2>{folder.label}</h2><span className="small faint spacer">{folder.hint}</span>
        <span className="small muted">{files.length}</span>
      </div>
      <div className="card-b stack" style={{ gap: 12 }}>
        {files.length > 0 && (
          <div className="tiles">
            {files.map((f, i) => (
              <FileTile key={f.path} f={short(f)} contain={contained.has(folder.id)} onDelete={() => del(f.path)}
                meta={tenant.asset_meta?.[f.path]} onOpen={() => setAt(i)} onEdit={() => setAt(i)}
                moveTo={{ options: folders.filter((x) => x.id !== folder.id), onMove: (to) => edit({ path: f.path, folder: to }) }} />
            ))}
          </div>
        )}
        {ext === "all" && (pct === undefined ? <DropZone compact onFiles={add} accept={folder.accept} label={`Add ${folder.label.toLowerCase()}`} /> : <Progress pct={pct} />)}
      </div>
      {cur && <AssetEditor key={cur.path} f={short(cur)} meta={tenant.asset_meta?.[cur.path]} pos={[at!, files.length]} onStep={(d) => setAt(at! + d)}
        onSave={(m) => api(`/tenants/${tenant.id}/assets`, { method: "PUT", body: { path: cur.path, ...m } }).then(reload)} onClose={() => setAt(undefined)} />}
    </div>
  );
}

function Knowledge({ tenant, reload, onScan }: { tenant: Tenant; reload: () => void; onScan: () => void }) {
  const [text, setText] = useState(tenant.knowledge ?? "");
  useEffect(() => setText(tenant.knowledge ?? ""), [tenant.knowledge]);
  const dirty = text !== (tenant.knowledge ?? "");
  const save = () => api(`/tenants/${tenant.id}`, { method: "PUT", body: { knowledge: text } }).then(() => { toast("Knowledge base saved"); reload(); }, fail);
  return (
    <div className="card">
      <div className="card-h">
        <h2>Knowledge base</h2>
        <span className="small faint spacer">every project agent for this brand reads this first</span>
        <button className="btn sm" disabled={!dirty} onClick={save}>Save</button>
      </div>
      <div className="card-b stack" style={{ gap: 12 }}>
        {tenant.scan_status === "done" && tenant.scan_note && <p className="small muted" style={{ whiteSpace: "pre-wrap" }}>{tenant.scan_note}</p>}
        {!text && !tenant.scanning ? (
          <Empty icon={<GlobeIcon size={28} />} title="No knowledge base yet">
            Scan their website or project folder, or write what they do, who they teach, their offers and tone.
            <button className="btn primary" onClick={onScan}><GlobeIcon size={15} /> Scan</button>
          </Empty>
        ) : null}
        <textarea className="textarea mono" rows={22} value={text} onChange={(e) => setText(e.target.value)}
          placeholder="# What they do&#10;# Audience&#10;# Offers & courses&#10;# Tone of voice&#10;# Proof & testimonials&#10;# Video ideas" />
      </div>
    </div>
  );
}

/** Reads design.md's YAML frontmatter just enough to preview it: top-level blocks of `key: value` lines. */
function frontmatter(md: string) {
  const m = md.match(/^---\r?\n([\s\S]*?)\r?\n---/);
  const blocks: Record<string, [string, string][]> = {};
  let cur = "";
  for (const line of m?.[1].split(/\r?\n/) ?? []) {
    if (/^\S/.test(line)) { cur = line.split(":")[0].trim(); blocks[cur] = []; continue; }
    const kv = line.match(/^\s+([\w.-]+)\s*:\s*(.+)$/);
    if (kv && cur) blocks[cur].push([kv[1], kv[2].trim().replace(/^["']|["']$/g, "")]);
  }
  return blocks;
}

const isColor = (v: string) => /^(#[0-9a-f]{3,8}\b|rgba?\(|hsla?\(|oklch\(|(linear|radial|conic)-gradient\()/i.test(v);
const field = (v: string, k: string) => v.match(new RegExp(`${k}\\s*:\\s*"?([^",}]+)`))?.[1]?.trim();

function DesignSystem({ tenant, reload, onScan }: { tenant: Tenant; reload: () => void; onScan: () => void }) {
  const [text, setText] = useState(tenant.design ?? "");
  useEffect(() => setText(tenant.design ?? ""), [tenant.design]);
  const fm = frontmatter(text);
  const colors = (fm.colors ?? []).filter(([, v]) => isColor(v));
  const type = (fm.typography ?? []).filter(([, v]) => v.startsWith("{"));
  const radii = fm.radii ?? [];
  const families = [...new Set(type.map(([, v]) => field(v, "fontFamily")).filter(Boolean))] as string[];
  const shot = tenant.assets?.find((a) => a.path.startsWith("design/") && /full-page/.test(a.name));
  const dirty = text !== (tenant.design ?? "");

  // Load the site's fonts from Google Fonts so specimens render in the real face; unknown families fall back quietly.
  useEffect(() => {
    if (!families.length) return;
    const link = document.createElement("link");
    link.rel = "stylesheet";
    link.href = "https://fonts.googleapis.com/css2?" + families.map((f) => "family=" + encodeURIComponent(f) + ":wght@300;400;500;600;700;800").join("&") + "&display=swap";
    document.head.appendChild(link);
    return () => link.remove();
  }, [families.join("|")]);

  const save = () => api(`/tenants/${tenant.id}`, { method: "PUT", body: { design: text } }).then(() => { toast("Design system saved"); reload(); }, fail);

  if (!text && !tenant.scanning) {
    return (
      <div className="card">
        <Empty icon={<GlobeIcon size={28} />} title="No design system yet">
          Scan their website or project folder to extract colors, type scale, spacing, radii, shadows and component styles.
          <button className="btn primary" onClick={onScan}><GlobeIcon size={15} /> Scan</button>
        </Empty>
      </div>
    );
  }
  return (
    <div className="stack">
      {colors.length > 0 && (
        <div className="card">
          <div className="card-h"><h2>Colors</h2><span className="small faint">from their CSS · exact values</span></div>
          <div className="card-b swatches">
            {colors.map(([k, v]) => (
              <div key={k} className="swatch-card">
                <div className="chip-color" style={{ background: v }} />
                <strong className="small">{k}</strong>
                <span className="mono faint ellipsis" title={v}>{v}</span>
              </div>
            ))}
          </div>
        </div>
      )}
      <div className="grid cols-2">
        {type.length > 0 && (
          <div className="card">
            <div className="card-h"><h2>Typography</h2><span className="small faint">{families.join(" · ")}</span></div>
            <div className="filelist">
              {type.map(([k, v]) => {
                const fam = field(v, "fontFamily"), px = Number(field(v, "px")) || 16, weight = field(v, "weight");
                return (
                  <div className="item" key={k} style={{ alignItems: "baseline" }}>
                    <span className="small faint" style={{ width: 70, flex: "none" }}>{k}</span>
                    <span className="ellipsis spacer" style={{ fontFamily: fam ? `"${fam}", var(--font)` : undefined, fontSize: Math.min(px, 40), fontWeight: Number(weight) || undefined }}>
                      The quick brown fox
                    </span>
                    <span className="mono faint nowrap">{px}px {weight}</span>
                  </div>
                );
              })}
            </div>
          </div>
        )}
        {radii.length > 0 && (
          <div className="card">
            <div className="card-h"><h2>Corner radius</h2></div>
            <div className="card-b row wrap" style={{ gap: 14 }}>
              {radii.map(([k, v]) => (
                <div key={k} className="stack" style={{ gap: 4, alignItems: "center" }}>
                  <div style={{ width: 64, height: 48, border: "2px solid var(--primary)", background: "var(--primary-soft)", borderRadius: v }} />
                  <span className="small">{k}</span><span className="mono faint">{v}</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
      {shot && (
        <div className="card">
          <div className="card-h"><h2>Their site</h2><a className="btn ghost sm" href={shot.url} target="_blank" rel="noreferrer">Open full size</a></div>
          <div className="card-b"><div style={{ maxHeight: 420, overflowY: "auto", borderRadius: 8, border: "1px solid var(--border)" }}><img src={shot.url} alt="Full-page screenshot of their website" style={{ width: "100%", display: "block" }} /></div></div>
        </div>
      )}
      <div className="card">
        <div className="card-h">
          <h2>design.md</h2>
          <span className="small faint spacer">HyperFrames design spec: frontmatter is brand truth for every video</span>
          <button className="btn sm" disabled={!dirty} onClick={save}>Save</button>
        </div>
        <div className="card-b"><textarea className="textarea mono" rows={24} value={text} onChange={(e) => setText(e.target.value)} /></div>
      </div>
    </div>
  );
}
