import { useState } from "react";
import { ArrowRightIcon, CheckCircleIcon, PlusIcon, SparkleIcon, WarningIcon } from "@phosphor-icons/react";
import { Meta, Project, cash, go, ago, useApi } from "../api";
import { Empty, Status, projectStatus } from "../ui";
import { ProjectTable, presetIcon } from "./Projects";
import { NewProjectModal } from "./NewProject";
import { approve } from "./Autopilot";

interface Costs { total: number; this_month: number }

const pipeline = [
  { value: "running", label: "Agent working" },
  { value: "review", label: "In review" },
  { value: "draft", label: "Draft" },
  { value: "delivered", label: "Delivered" },
];

const greeting = () => { const h = new Date().getHours(); return h < 12 ? "Good morning" : h < 18 ? "Good afternoon" : "Good evening"; };

export default function Overview() {
  const { data: projects, reload } = useApi<Project[]>("/projects");
  const { data: costs } = useApi<Costs>("/costs");
  const { data: meta } = useApi<Meta>("/meta");
  const [creating, setCreating] = useState(false);

  const all = projects ?? [];
  const count = (s: string) => all.filter((p) => projectStatus(p) === s).length;
  const active = all.filter((p) => ["running", "review", "draft"].includes(projectStatus(p)));
  const review = all.filter((p) => projectStatus(p) === "review" && !p.autopilot_stage);
  const approvals = all.filter((p) => p.autopilot_stage === "approve");
  const checks = meta ? [
    { ok: meta.env.anthropic, label: "Claude key or subscription token for the editor agent", fix: "Settings → AI services" },
    { ok: meta.env.claude, label: "Claude Code CLI installed", fix: "bundled in the Docker image; else npm i -g @anthropic-ai/claude-code" },
    { ok: meta.env.ffmpeg, label: "ffmpeg installed", fix: "bundled in the Docker image" },
    { ok: meta.env.gemini, label: "Gemini key for image and video generation (optional)", fix: "Settings → AI services" },
    { ok: meta.env.eleven, label: "ElevenLabs key for voiceover, sound effects and music (optional)", fix: "Settings → AI services" },
  ] : [];

  return (
    <div className="page">
      <div className="page-head hello">
        <div className="spacer">
          <div className="eyebrow">{new Date().toLocaleDateString(undefined, { weekday: "long", day: "numeric", month: "long" })}</div>
          <h1>{greeting()}</h1>
          <p>{active.length} project{active.length === 1 ? "" : "s"} in motion, {review.length} waiting for your review.</p>
        </div>
        <a className="btn" href="#/clients">Companies</a>
        <button className="btn primary" onClick={() => setCreating(true)}><PlusIcon size={16} /> New project</button>
      </div>

      <div className="card kpis">
        <Kpi label="Active projects" value={String(active.length)} sub={`${count("running")} with the agent now`} />
        <Kpi label="Delivered" value={String(count("delivered"))} sub={`of ${all.length} project${all.length === 1 ? "" : "s"}`} />
        <Kpi label="Spent this month" value={cash(costs?.this_month ?? 0)} sub="Claude, Gemini and ElevenLabs" />
        <Kpi label="Spent in total" value={cash(costs?.total ?? 0)} sub="see Costs for the breakdown" />
      </div>

      {checks.some((c) => !c.ok) && (
        <div className="card">
          <div className="card-h"><WarningIcon size={16} color="var(--warn)" /><h2>Finish setup</h2></div>
          <div className="filelist">
            {checks.map((c) => (
              <div className="item" key={c.label}>
                {c.ok ? <CheckCircleIcon size={16} color="var(--success)" /> : <WarningIcon size={16} color="var(--warn)" />}
                <span className="spacer">{c.label}</span>
                {!c.ok && <span className="small muted mono">{c.fix}</span>}
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="home-grid">
        <div className="stack">
          {approvals.length > 0 && (
            <div className="card">
              <div className="card-h"><h2 className="spacer">Autopilot: waiting for your approval</h2><span className="faint small">{approvals.length}</span></div>
              <div className="filelist">
                {approvals.map((p) => (
                  <div className="item" key={p.id}>
                    <a className="spacer ellipsis" href={`#/projects/${p.id}`}><strong>{p.title}</strong> <span className="faint">· {p.tenant_name}</span></a>
                    <a className="btn sm" href={`#/projects/${p.id}`}>Watch</a>
                    <button className="btn sm primary" onClick={() => approve(p.id, reload)}>Approve</button>
                  </div>
                ))}
              </div>
            </div>
          )}
          {review.length > 0 && (
            <div className="card">
              <div className="card-h"><h2 className="spacer">Needs your review</h2><span className="faint small">{review.length}</span></div>
              <div className="filelist">
                {review.slice(0, 5).map((p) => {
                  const Icon = presetIcon[p.kind] ?? SparkleIcon;
                  return (
                    <a className="item link-row" key={p.id} href={`#/projects/${p.id}`}>
                      <Icon size={16} />
                      <span className="spacer ellipsis"><strong>{p.title}</strong> <span className="faint">· {p.tenant_name}</span></span>
                      <span className="faint small nowrap">{ago(p.created_at)}</span>
                      <ArrowRightIcon size={14} />
                    </a>
                  );
                })}
              </div>
            </div>
          )}
          <div className="card">
            <div className="card-h"><h2 className="spacer">Recent projects</h2><a className="btn ghost sm" href="#/projects">View all</a></div>
            {projects && projects.length === 0 ? (
              <Empty icon={<SparkleIcon size={28} />} title="Your first edit is one upload away">
                Add your company from its website, drop in a long video and pick "Hook reel".
                <button className="btn primary" onClick={() => setCreating(true)}><PlusIcon size={16} /> New project</button>
              </Empty>
            ) : <ProjectTable projects={all.slice(0, 8)} />}
          </div>
        </div>

        <div className="stack">
          <div className="card">
            <div className="card-h"><h2>Pipeline</h2></div>
            <div className="card-b bars">
              {pipeline.map((s) => (
                <button className="pipe-row" key={s.value} onClick={() => go("/projects")}>
                  <Status value={s.value} />
                  <span className="bar-track"><span className="bar-fill" style={{ width: `${all.length ? (count(s.value) / all.length) * 100 : 0}%` }} /></span>
                  <span className="num">{count(s.value)}</span>
                </button>
              ))}
            </div>
          </div>
          {meta && (
            <div className="card">
              <div className="card-h"><h2>Start something</h2></div>
              <div className="filelist">
                {meta.presets.map((p) => {
                  const Icon = presetIcon[p.id] ?? SparkleIcon;
                  return (
                    <button className="item link-row" key={p.id} onClick={() => setCreating(true)}>
                      <Icon size={16} />
                      <span className="spacer ellipsis">{p.label}</span>
                      <span className="faint small">{p.aspect}</span>
                    </button>
                  );
                })}
              </div>
            </div>
          )}
        </div>
      </div>
      {creating && <NewProjectModal onClose={() => setCreating(false)} />}
    </div>
  );
}

function Kpi({ label, value, sub }: { label: string; value: string; sub: string }) {
  return (
    <div className="kpi">
      <div className="eyebrow">{label}</div>
      <div className="value">{value}</div>
      <div className="sub">{sub}</div>
    </div>
  );
}
