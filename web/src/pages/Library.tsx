import { useState } from "react";
import { ArrowSquareOutIcon, DownloadSimpleIcon, FilmStripIcon } from "@phosphor-icons/react";
import { bytes, clock, date, LibraryItem, Tenant, useApi } from "../api";
import { Empty, FileTile, Modal, Seg } from "../ui";

/** Every finished video and image from every project: the ten videos cut from one class sit together here. */
export default function Library() {
  const { data } = useApi<LibraryItem[]>("/library");
  const { data: tenants } = useApi<Tenant[]>("/tenants");
  const [kind, setKind] = useState("all");
  const [tenant, setTenant] = useState("");
  const [q, setQ] = useState("");
  const [open, setOpen] = useState<LibraryItem>();
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
        <Seg value={kind} onChange={setKind} options={[{ value: "all", label: "All" }, { value: "video", label: "Videos" }, { value: "image", label: "Images" }]} />
        <span className="spacer" />
        <select className="select" style={{ width: "auto" }} value={tenant} onChange={(e) => setTenant(e.target.value)} aria-label="Company">
          <option value="">All companies</option>
          {tenants?.map((t) => <option key={t.id} value={t.id}>{t.name}</option>)}
        </select>
        <input className="input" style={{ width: 220 }} value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search titles" aria-label="Search titles" />
      </div>
      {data && !shown.length ? (
        <div className="card">
          <Empty icon={<FilmStripIcon size={28} />} title={data.length ? "Nothing matches" : "No finished videos yet"}>
            {data.length ? "Try another company, type or search." : "Every video a project exports shows up here."}
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
    </div>
  );
}
