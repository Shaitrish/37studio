import { useState } from "react";
import { ArrowSquareOutIcon, BookmarkSimpleIcon, DownloadSimpleIcon, FilmStripIcon, LayoutIcon, TrashIcon } from "@phosphor-icons/react";
import { api, bytes, clock, compact, date, fail, LibraryItem, pct, PublishedVideo, Template, toast, useApi, useTenants, views } from "../api";
import { Empty, HelpButton, Field, FileTile, Modal, Seg, TenantPicker } from "../ui";
import { platforms } from "./Social";

/** Every finished video and image from every project: the ten videos cut from one class sit together here. */
export default function Library() {
  const { data, reload } = useApi<LibraryItem[]>("/library");
  const tenants = useTenants();
  const [kind, setKind] = useState("all");
  const [tenant, setTenant] = useState("");
  const [q, setQ] = useState("");
  const [open, setOpen] = useState<LibraryItem>();
  const [saving, setSaving] = useState<LibraryItem>();
  const [picked, setPicked] = useState<Set<string>>(); // file urls; set while choosing several to delete
  const { data: templates, reload: reloadTemplates } = useApi<Template[]>("/templates");
  const { data: published } = useApi<PublishedVideo[]>("/stats/videos");
  // A library file is "out/<path>" in its project; posts name it the same way.
  const pubOf = (x: LibraryItem) => published?.find((v) => v.project_id === x.project_id && v.file === `out/${x.file.path}`);
  const remove = (x: LibraryItem) => confirm(`Delete "${x.file.name}" from "${x.title}"? This can't be undone. The project stays.`) &&
    api(`/projects/${x.project_id}/files?path=out/${encodeURIComponent(x.file.path)}`, { method: "DELETE" })
      .then(() => { setOpen(undefined); reload(); toast("Deleted"); }, fail);
  const removeMany = async () => {
    const items = (data ?? []).filter((x) => picked?.has(x.file.url));
    if (!items.length || !confirm(`Delete ${items.length} file${items.length > 1 ? "s" : ""}? This can't be undone. Their projects stay.`)) return;
    let failed = 0;
    for (const x of items) {
      await api(`/projects/${x.project_id}/files?path=out/${encodeURIComponent(x.file.path)}`, { method: "DELETE" }).catch((e) => { failed++; fail(e); });
    }
    if (failed < items.length) toast(`Deleted ${items.length - failed} file${items.length - failed > 1 ? "s" : ""}`);
    setPicked(undefined);
    reload();
  };
  const shown = (data ?? []).filter((x) =>
    (kind === "all" || x.file.kind === kind || (kind === "published" && pubOf(x))) && (!tenant || String(x.tenant_id) === tenant) &&
    (!q || `${x.title} ${x.file.name}`.toLowerCase().includes(q.toLowerCase())));

  return (
    <div className="page">
      <div className="page-head">
        <div className="spacer">
          <h1>Library</h1>
          <p>Every finished video and image, from every project. Drafts stay in their projects.</p>
        </div>
        <HelpButton to="library-and-templates" />
      </div>
      <div className="row wrap">
        <Seg value={kind} onChange={setKind} options={[{ value: "all", label: "All" }, { value: "video", label: "Videos" }, { value: "image", label: "Images" },
          { value: "published", label: `Published${published?.length ? ` (${published.length})` : ""}` },
          { value: "templates", label: `Templates${templates?.length ? ` (${templates.length})` : ""}` }]} />
        <span className="spacer" />
        <TenantPicker tenants={tenants} value={Number(tenant) || undefined} all="All brands" onChange={(id) => setTenant(id ? String(id) : "")} />
        <input className="input" style={{ width: 220 }} value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search titles" aria-label="Search titles" />
        {kind !== "templates" && !!data?.length && (picked ? <>
          <button className="btn danger" disabled={!picked.size} onClick={removeMany}><TrashIcon size={15} /> Delete {picked.size || ""}</button>
          <button className="btn" onClick={() => setPicked(undefined)}>Cancel</button>
        </> : <button className="btn" onClick={() => setPicked(new Set())}>Select</button>)}
      </div>
      {kind === "templates" ? (
        <Templates templates={(templates ?? []).filter((t) => !tenant || !t.tenant_id || String(t.tenant_id) === tenant)
          .filter((t) => !q || t.name.toLowerCase().includes(q.toLowerCase()))} reload={reloadTemplates} />
      ) : data && !shown.length ? (
        <div className="card">
          <Empty icon={<FilmStripIcon size={28} />} title={kind === "published" ? "Nothing published yet" : data.length ? "Nothing matches" : "No finished videos yet"}>
            {kind === "published" ? "Videos you post from a project's Publish button show up here with their numbers from every platform." :
              data.length ? "Try another brand, type or search." : "Every video a project exports shows up here."}
          </Empty>
        </div>
      ) : (
        <div className="tiles library">
          {shown.map((x) => (
            <div key={x.file.url} className="stack" style={{ gap: 6, minWidth: 0 }}>
              <FileTile f={x.file} selected={picked?.has(x.file.url)} onOpen={() => picked
                ? setPicked((s) => { const n = new Set(s); n.has(x.file.url) ? n.delete(x.file.url) : n.add(x.file.url); return n; })
                : setOpen(x)} />
              <div className="small ellipsis" title={x.title}>
                <a href={`#/projects/${x.project_id}`}>{x.title}</a>
                <div className="faint ellipsis">{x.tenant_name} · {date(x.file.mod)}</div>
                {pubOf(x) && <PlatformChips v={pubOf(x)!} />}
              </div>
            </div>
          ))}
        </div>
      )}
      {open && (
        <Modal title={open.title} size="xl" onClose={() => setOpen(undefined)}
          sub={[open.tenant_name, open.file.seconds && clock(open.file.seconds), bytes(open.file.size), date(open.file.mod)].filter(Boolean).join(" · ")}
          footer={<>
            <a className="btn" href={open.file.url} download={open.file.name}><DownloadSimpleIcon size={15} /> Download</a>
            {open.file.kind === "video" && <button className="btn" onClick={() => { setSaving(open); setOpen(undefined); }}><BookmarkSimpleIcon size={15} /> Save as template</button>}
            <button className="btn danger" onClick={() => remove(open)}><TrashIcon size={15} /> Delete</button>
            <span className="spacer" />
            <a className="btn primary" href={`#/projects/${open.project_id}`} onClick={() => setOpen(undefined)}><ArrowSquareOutIcon size={15} /> Open project to edit or publish</a>
          </>}>
          <div className="asset-ed" style={{ gridTemplateColumns: "1fr" }}>
            <div className="pv">
              {open.file.kind === "video" ? <video src={open.file.url} controls autoPlay playsInline /> : <img src={open.file.url} alt={open.title} />}
            </div>
          </div>
          {pubOf(open) && <PlatformTable v={pubOf(open)!} />}
        </Modal>
      )}
      {saving && <SaveTemplate projectId={saving.project_id} title={saving.title} onClose={() => setSaving(undefined)} onSaved={() => { reloadTemplates(); setKind("templates"); }} />}
    </div>
  );
}

/** Where a published video went, with its views on each: the platform it did best on comes first. */
function PlatformChips({ v }: { v: PublishedVideo }) {
  const posts = [...v.posts].sort((a, b) => (b.views ?? -1) - (a.views ?? -1));
  return (
    <div className="row wrap faint" style={{ gap: 8, marginTop: 3 }} title={`${views(v.views)} in all, ${pct(v.engagement_rate)} engagement`}>
      {posts.map((p, i) => {
        const P = platforms[p.platform]?.Icon;
        return <span key={i} className="row nowrap" style={{ gap: 3, color: p.platform === v.best && posts.length > 1 ? "var(--fg)" : undefined }}>{P && <P size={12} />}{p.views == null ? "…" : compact(p.views)}</span>;
      })}
    </div>
  );
}

/** One published video's numbers on every platform, side by side. */
function PlatformTable({ v }: { v: PublishedVideo }) {
  return (
    <div className="table-wrap" style={{ marginTop: 12 }}>
      <table className="table">
        <thead><tr><th>Posted to</th><th className="num">Views</th><th className="num">Watched</th><th className="num">Avg watch</th><th className="num">Likes</th>
          <th className="num">Comments</th><th className="num">Shares</th><th className="num">Saves</th><th className="num">Engagement</th></tr></thead>
        <tbody>
          {v.posts.map((p, i) => {
            const P = platforms[p.platform]?.Icon;
            return (
              <tr key={i}>
                <td><span className="row" style={{ gap: 6 }}>{P && <P size={14} />}<a className="link" href={p.url} target="_blank" rel="noreferrer">{p.account}</a>
                  {p.platform === v.best && v.posts.length > 1 && <span className="pill green">best</span>}</span></td>
                {p.views == null ? <td colSpan={8} className="small faint">No numbers yet: 37Studio reads them every 6 hours</td> : <>
                  <td className="num"><strong>{compact(p.views)}</strong></td>
                  <td className="num">{p.retention != null ? pct(p.retention) : p.skip_rate != null ? `${(100 - p.skip_rate).toFixed(0)}% past 3s` : "—"}</td>
                  <td className="num">{p.watch_avg == null ? "—" : `${p.watch_avg.toFixed(1)}s`}</td>
                  <td className="num">{compact(p.likes)}</td><td className="num">{compact(p.comments)}</td><td className="num">{compact(p.shares)}</td>
                  <td className="num">{compact(p.saves)}</td><td className="num">{pct(p.engagement_rate)}</td>
                </>}
              </tr>
            );
          })}
        </tbody>
      </table>
      <p className="small muted" style={{ margin: "8px 0 0" }}>{views(v.views)} in all · {pct(v.engagement_rate)} engagement</p>
    </div>
  );
}

function Templates({ templates, reload }: { templates: Template[]; reload: () => void }) {
  const remove = (t: Template) => confirm(`Delete the template "${t.name}"? Videos already made from it stay as they are.`) &&
    api(`/templates/${t.id}`, { method: "DELETE" }).then(reload, fail);
  if (!templates.length) {
    return (
      <div className="card">
        <Empty icon={<LayoutIcon size={28} />} title="No templates yet">
          Open a finished video's project and click <b>Save as template</b>. New projects can then start from its look: layout, fonts, motion, captions, intro and outro.
        </Empty>
      </div>
    );
  }
  return (
    <div className="tiles library">
      {templates.map((t) => (
        <div key={t.id} className="tile">
          <div className="thumb">{t.thumb ? <img src={t.thumb} alt="" loading="lazy" /> : <LayoutIcon size={26} />}</div>
          <div className="meta">
            <div className="ellipsis" title={t.name}>{t.name}</div>
            <div className="faint ellipsis">{t.tenant_name ?? "All brands"} · {t.aspect} · <a href={`#/projects/${t.project_id}`}>source</a></div>
            {t.notes && <div className="caption" title={t.notes}>{t.notes}</div>}
          </div>
          <div className="acts"><button className="btn sm icon danger" onClick={() => remove(t)} aria-label={`Delete ${t.name}`} title="Delete"><TrashIcon size={14} /></button></div>
        </div>
      ))}
    </div>
  );
}

/** Saves a project's composition as a template (template.go). */
export function SaveTemplate({ projectId, title, onClose, onSaved }: { projectId: number; title: string; onClose: () => void; onSaved?: () => void }) {
  const [name, setName] = useState(title);
  const [notes, setNotes] = useState("");
  const [shared, setShared] = useState(false);
  const [busy, setBusy] = useState(false);
  const save = () => {
    setBusy(true);
    api(`/projects/${projectId}/template`, { body: { name, notes, shared } })
      .then(() => { toast("Template saved. Pick it when you start a new project"); onSaved?.(); onClose(); }, (e) => { fail(e); setBusy(false); });
  };
  return (
    <Modal help="library-and-templates" title="Save as template" sub="New projects can start from this video's look: layout, fonts, colors, motion, caption style, intro and outro. The agent swaps in the new content."
      onClose={onClose} footer={<><button className="btn" onClick={onClose}>Cancel</button><button className="btn primary" disabled={busy || !name.trim()} onClick={save}>{busy ? "Saving…" : "Save template"}</button></>}>
      <div className="stack">
        <Field label="Name"><input className="input" value={name} onChange={(e) => setName(e.target.value)} autoFocus /></Field>
        <Field label="What should new videos keep?" hint="optional">
          <textarea className="textarea" rows={3} value={notes} onChange={(e) => setNotes(e.target.value)} placeholder="The animated intro, the lower thirds and the caption style. Replace the footage, the question and the voiceover." />
        </Field>
        <label className="row small"><input type="checkbox" checked={shared} onChange={(e) => setShared(e.target.checked)} /> Use it for every brand, not just this one</label>
      </div>
    </Modal>
  );
}
