import { useState } from "react";
import { ArrowSquareOutIcon, BookmarkSimpleIcon, DownloadSimpleIcon, FilmStripIcon, LayoutIcon, TrashIcon } from "@phosphor-icons/react";
import { api, bytes, clock, date, fail, LibraryItem, Template, toast, useApi, useTenants } from "../api";
import { Empty, Field, FileTile, Modal, Seg, TenantPicker } from "../ui";

/** Every finished video and image from every project: the ten videos cut from one class sit together here. */
export default function Library() {
  const { data } = useApi<LibraryItem[]>("/library");
  const tenants = useTenants();
  const [kind, setKind] = useState("all");
  const [tenant, setTenant] = useState("");
  const [q, setQ] = useState("");
  const [open, setOpen] = useState<LibraryItem>();
  const [saving, setSaving] = useState<LibraryItem>();
  const { data: templates, reload: reloadTemplates } = useApi<Template[]>("/templates");
  const shown = (data ?? []).filter((x) =>
    (kind === "all" || x.file.kind === kind) && (!tenant || String(x.tenant_id) === tenant) &&
    (!q || `${x.title} ${x.file.name}`.toLowerCase().includes(q.toLowerCase())));

  return (
    <div className="page">
      <div className="page-head">
        <div className="spacer">
          <h1>Library</h1>
          <p>Every finished video and image, from every project. Drafts stay in their projects.</p>
        </div>
      </div>
      <div className="row wrap">
        <Seg value={kind} onChange={setKind} options={[{ value: "all", label: "All" }, { value: "video", label: "Videos" }, { value: "image", label: "Images" },
          { value: "templates", label: `Templates${templates?.length ? ` (${templates.length})` : ""}` }]} />
        <span className="spacer" />
        <TenantPicker tenants={tenants} value={Number(tenant) || undefined} all="All brands" onChange={(id) => setTenant(id ? String(id) : "")} />
        <input className="input" style={{ width: 220 }} value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search titles" aria-label="Search titles" />
      </div>
      {kind === "templates" ? (
        <Templates templates={(templates ?? []).filter((t) => !tenant || !t.tenant_id || String(t.tenant_id) === tenant)
          .filter((t) => !q || t.name.toLowerCase().includes(q.toLowerCase()))} reload={reloadTemplates} />
      ) : data && !shown.length ? (
        <div className="card">
          <Empty icon={<FilmStripIcon size={28} />} title={data.length ? "Nothing matches" : "No finished videos yet"}>
            {data.length ? "Try another brand, type or search." : "Every video a project exports shows up here."}
          </Empty>
        </div>
      ) : (
        <div className="tiles library">
          {shown.map((x) => (
            <div key={x.file.url} className="stack" style={{ gap: 6, minWidth: 0 }}>
              <FileTile f={x.file} onOpen={() => setOpen(x)} />
              <div className="small ellipsis" title={x.title}>
                <a href={`#/projects/${x.project_id}`}>{x.title}</a>
                <div className="faint ellipsis">{x.tenant_name} · {date(x.file.mod)}</div>
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
            <span className="spacer" />
            <a className="btn primary" href={`#/projects/${open.project_id}`} onClick={() => setOpen(undefined)}><ArrowSquareOutIcon size={15} /> Open project to edit or publish</a>
          </>}>
          <div className="asset-ed" style={{ gridTemplateColumns: "1fr" }}>
            <div className="pv">
              {open.file.kind === "video" ? <video src={open.file.url} controls autoPlay playsInline /> : <img src={open.file.url} alt={open.title} />}
            </div>
          </div>
        </Modal>
      )}
      {saving && <SaveTemplate projectId={saving.project_id} title={saving.title} onClose={() => setSaving(undefined)} onSaved={() => { reloadTemplates(); setKind("templates"); }} />}
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
    <Modal title="Save as template" sub="New projects can start from this video's look: layout, fonts, colors, motion, caption style, intro and outro. The agent swaps in the new content."
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
