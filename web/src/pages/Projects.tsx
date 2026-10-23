import { useState } from "react";
import { BookOpenIcon, ClosedCaptioningIcon, FilmSlateIcon, ImageIcon, MegaphoneIcon, PlusIcon, MagicWandIcon } from "@phosphor-icons/react";
import { go, Project, cash, useApi, ago } from "../api";
import { Empty, Seg, Status, projectStatus } from "../ui";
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
  const { data: projects } = useApi<Project[]>("/projects");
  const [filter, setFilter] = useState("all");
  const [creating, setCreating] = useState(false);
  const shown = (projects ?? []).filter((p) => filter === "all" || projectStatus(p) === filter);

  return (
    <div className="page">
      <div className="page-head">
        <div className="spacer">
          <h1>Projects</h1>
          <p>Every edit job: reels, captioned clips, explainers, promos and thumbnails.</p>
        </div>
        <button className="btn primary" onClick={() => setCreating(true)}><PlusIcon size={16} /> New project</button>
      </div>
      <Seg value={filter} options={filters} onChange={setFilter} />
      <div className="card">
        {projects && shown.length === 0 ? (
          <Empty icon={<FilmSlateIcon size={28} />} title={filter === "all" ? "No projects yet" : "Nothing here"}>
            {filter === "all" && <button className="btn primary" onClick={() => setCreating(true)}><PlusIcon size={16} /> Start the first one</button>}
          </Empty>
        ) : <ProjectTable projects={shown} />}
      </div>
      {creating && <NewProjectModal onClose={() => setCreating(false)} />}
    </div>
  );
}

export function ProjectTable({ projects, hideClient }: { projects: Project[]; hideClient?: boolean }) {
  return (
    <div className="table-wrap">
      <table className="table">
        <thead>
          <tr>
            <th>Project</th>
            {!hideClient && <th>Company</th>}
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
                <td>
                  <a href={`#/projects/${p.id}`} className="row" onClick={(e) => e.stopPropagation()}>
                    <Icon size={16} className="faint" /> <strong style={{ fontWeight: 500 }}>{p.title}</strong>
                    <span className="faint small">{p.aspect}</span>
                    {p.campaign_name && <span className="pill violet">{p.campaign_name}</span>}
                  </a>
                </td>
                {!hideClient && <td className="muted">{p.tenant_name}</td>}
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
