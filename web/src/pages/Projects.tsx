import { useState } from "react";
import { BookOpenIcon, ClosedCaptioningIcon, FilmSlateIcon, ImageIcon, MegaphoneIcon, PlusIcon, MagicWandIcon, TrashIcon } from "@phosphor-icons/react";
import { api, go, Project, cash, fail, toast, useApi, useTenants, ago } from "../api";
import { Empty, HelpButton, Seg, Status, TenantTag, projectStatus } from "../ui";
import { NewProjectModal } from "./NewProject";

export const presetIcon: Record<string, typeof FilmSlateIcon> = {
  "hook-reel": FilmSlateIcon, captions: ClosedCaptioningIcon, explainer: BookOpenIcon, promo: MegaphoneIcon, thumbnail: ImageIcon, custom: MagicWandIcon,
};

const filters = [
  { value: "all", label: "All" },
  { value: "running", label: "Working" },
  { value: "review", label: "In review" },
  { value: "delivered", label: "Delivered" },
];

export default function Projects() {
  const { data: projects, reload } = useApi<Project[]>("/projects");
  const [filter, setFilter] = useState("all");
  const [creating, setCreating] = useState(false);
  const [picked, setPicked] = useState<Set<number>>(new Set());
  const shown = (projects ?? []).filter((p) => filter === "all" || projectStatus(p) === filter);
  const removeMany = async () => {
    const n = picked.size;
    if (!confirm(`Delete ${n} project${n > 1 ? "s" : ""} with all ${n > 1 ? "their" : "its"} videos and files? Posts already published stay on the platforms. This can't be undone.`)) return;
    let failed = 0;
    for (const id of picked) await api(`/projects/${id}`, { method: "DELETE" }).catch((e) => { failed++; fail(e); });
    if (failed < n) toast(`Deleted ${n - failed} project${n - failed > 1 ? "s" : ""}`);
    setPicked(new Set());
    reload();
  };

  return (
    <div className="page">
      <div className="page-head">
        <div className="spacer">
          <h1>Projects</h1>
          <p>Every edit job: reels, captioned clips, explainers, promos and thumbnails.</p>
        </div>
        <HelpButton to="make-a-video" />
        <button className="btn primary" onClick={() => setCreating(true)}><PlusIcon size={16} /> New video</button>
      </div>
      <div className="row wrap">
        <Seg value={filter} options={filters} onChange={setFilter} />
        <span className="spacer" />
        {!!picked.size && <>
          <button className="btn danger" onClick={removeMany}><TrashIcon size={15} /> Delete {picked.size}</button>
          <button className="btn" onClick={() => setPicked(new Set())}>Clear</button>
        </>}
      </div>
      <div className="card">
        {projects && shown.length === 0 ? (
          <Empty icon={<FilmSlateIcon size={28} />} title={filter === "all" ? "No projects yet" : "Nothing here"}>
            {filter === "all" && <button className="btn primary" onClick={() => setCreating(true)}><PlusIcon size={16} /> Start the first one</button>}
          </Empty>
        ) : <ProjectTable projects={shown} picked={picked} onPick={setPicked} />}
      </div>
      {creating && <NewProjectModal onClose={() => setCreating(false)} />}
    </div>
  );
}

export function ProjectTable({ projects, hideClient, picked, onPick }: {
  projects: Project[]; hideClient?: boolean; picked?: Set<number>; onPick?: (s: Set<number>) => void;
}) {
  const tenants = useTenants();
  const flip = (ids: number[], on: boolean) => { const n = new Set(picked); ids.forEach((id) => (on ? n.add(id) : n.delete(id))); onPick!(n); };
  const all = projects.length > 0 && projects.every((p) => picked?.has(p.id));
  return (
    <div className="table-wrap">
      <table className="table">
        <thead>
          <tr>
            {onPick && <th style={{ width: 1 }}><input type="checkbox" checked={all} aria-label="Select all"
              onChange={(e) => flip(projects.map((p) => p.id), e.target.checked)} /></th>}
            <th>Project</th>
            {!hideClient && <th>Brand</th>}
            <th>Status</th>
            <th className="num">Cost</th>
            <th className="num">Created</th>
          </tr>
        </thead>
        <tbody>
          {projects.map((p) => {
            const Icon = presetIcon[p.kind] ?? MagicWandIcon;
            return (
              <tr key={p.id} className="link" onClick={() => go(`/projects/${p.id}`)}>
                {onPick && <td onClick={(e) => e.stopPropagation()}><input type="checkbox" checked={!!picked?.has(p.id)} aria-label={`Select ${p.title}`}
                  onChange={(e) => flip([p.id], e.target.checked)} /></td>}
                <td>
                  <a href={`#/projects/${p.id}`} className="row" onClick={(e) => e.stopPropagation()}>
                    <Icon size={16} className="faint" /> <strong style={{ fontWeight: 500 }}>{p.title}</strong>
                    <span className="faint small">{p.aspect}</span>
                    {p.campaign_name && <span className="pill violet">{p.campaign_name}</span>}
                  </a>
                </td>
                {!hideClient && <td className="muted"><TenantTag tenants={tenants} id={p.tenant_id} name={p.tenant_name} /></td>}
                <td><Status value={projectStatus(p)} /></td>
                <td className="num">{cash(p.quote?.cost)}</td>
                <td className="num muted">{ago(p.created_at)}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
