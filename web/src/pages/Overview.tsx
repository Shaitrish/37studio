import { useState } from "react";
import { ArrowRightIcon, BookOpenIcon, CheckCircleIcon, PlusIcon, SparkleIcon, WarningIcon } from "@phosphor-icons/react";
import { Meta, Project, cash, go, ago, help, repoURL, toast, useApi } from "../api";
import { Empty, HelpButton, Status, projectStatus } from "../ui";
import { ProjectTable, presetIcon } from "./Projects";
import { NewProjectModal } from "./NewProject";
import { approve } from "./Autopilot";

interface Costs { total: number; this_month: number }

// The Docker image has every tool the agent runs; the desktop app uses this computer's, so it says how to install
// the missing ones with the usual package manager: apt (Ubuntu, Debian), Homebrew (macOS) or winget (Windows).
const manager: Record<string, string> = { linux: "sudo apt install", darwin: "brew install", windows: "winget install" };
const packages: Record<string, Record<string, string>> = {
  ffmpeg: { linux: "ffmpeg", darwin: "ffmpeg", windows: "Gyan.FFmpeg" },
  node: { linux: "nodejs npm", darwin: "node", windows: "OpenJS.NodeJS.LTS" },
  python: { linux: "python3 python3-venv", darwin: "python", windows: "Python.Python.3.13" },
  git: { linux: "git", darwin: "git", windows: "Git.Git" },
};
const claudeInstall = (os: string) => `${os === "linux" ? "sudo " : ""}npm install -g @anthropic-ai/claude-code`;
const installCmd = (tool: string, os: string) =>
  tool === "claude" ? claudeInstall(os) : `${manager[os] ?? manager.linux} ${packages[tool][os] ?? packages[tool].linux}`;
// Everything missing in one go; winget takes one package at a time, and Claude Code installs with npm, so it goes last.
function installAll(missing: string[], os: string) {
  const pkgs = missing.filter((t) => packages[t]).map((t) => packages[t][os] ?? packages[t].linux);
  const lines = !pkgs.length ? [] : os === "windows" ? pkgs.map((p) => `winget install ${p}`) : [`${manager[os] ?? manager.linux} ${pkgs.join(" ")}`];
  return [...lines, ...(missing.includes("claude") ? [claudeInstall(os)] : [])].join("\n");
}

const pipeline = [
  { value: "running", label: "Agent working" },
  { value: "review", label: "In review" },
  { value: "draft", label: "Draft" },
  { value: "delivered", label: "Delivered" },
];

const learn = [
  ["start-here", "How 37Studio works"],
  ["connect-ai-services", "Connect your AI services"],
  ["make-a-video", "Make your first video"],
  ["publish", "Publish to social accounts"],
  ["troubleshooting", "Fix a problem"],
];

const greeting = () => { const h = new Date().getHours(); return h < 12 ? "Good morning" : h < 18 ? "Good afternoon" : "Good evening"; };

export default function Overview() {
  const { data: projects, reload } = useApi<Project[]>("/projects");
  const { data: costs } = useApi<Costs>("/costs");
  const { data: meta, reload: recheck } = useApi<Meta>("/meta");
  const [creating, setCreating] = useState(false);

  const all = projects ?? [];
  const count = (s: string) => all.filter((p) => projectStatus(p) === s).length;
  const active = all.filter((p) => ["running", "review", "draft"].includes(projectStatus(p)));
  const review = all.filter((p) => projectStatus(p) === "review" && !p.autopilot_stage);
  const approvals = all.filter((p) => p.autopilot_stage === "approve");
  const desktop = !!meta?.env.desktop, os = meta?.os ?? "linux";
  const tool = (id: string, ok: boolean, label: string) =>
    ({ id, ok, label, fix: desktop ? installCmd(id, os) : id === "claude" ? "bundled in the Docker image; else npm i -g @anthropic-ai/claude-code" : "bundled in the Docker image" });
  const checks = meta ? [
    { ok: meta.env.anthropic || meta.env.claude_login, label: "Claude for the editor agent: a key, a plan token, or Claude Code signed in on this computer", fix: "Settings → AI services" },
    tool("claude", meta.env.claude, "Claude Code CLI installed"),
    // Only Windows: Claude Code runs the agent's commands through Git for Windows' bash, so without it every
    // step the agent takes fails. Every Unix already has a shell, so the row would be noise there.
    ...(os === "windows" ? [tool("git", meta.env.shell, "Git for Windows, which runs the commands the agent uses")] : []),
    tool("ffmpeg", meta.env.ffmpeg, "ffmpeg installed, for cutting and encoding video"),
    tool("node", meta.env.node, "Node.js 22 or newer, for rendering and the timeline editor"),
    tool("python", meta.env.python, "Python 3, for transcription and YouTube or Drive imports"),
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
        <HelpButton to="start-here" />
        <a className="btn" href="#/clients">Brands</a>
        <button className="btn primary" onClick={() => setCreating(true)}><PlusIcon size={16} /> New video</button>
      </div>

      <div className="card kpis">
        <Kpi label="Active projects" value={String(active.length)} sub={`${count("running")} with the agent now`} />
        <Kpi label="Delivered" value={String(count("delivered"))} sub={`of ${all.length} project${all.length === 1 ? "" : "s"}`} />
        <Kpi label="Spent this month" value={cash(costs?.this_month ?? 0)} sub="all your AI services" />
        <Kpi label="Spent in total" value={cash(costs?.total ?? 0)} sub="see Costs for the breakdown" />
      </div>

      {checks.some((c) => !c.ok) && (
        <div className="card">
          <div className="card-h"><WarningIcon size={16} color="var(--warn)" /><h2 className="spacer">Finish setup</h2><HelpButton to="install-37studio" label="Setup guide" /></div>
          <div className="filelist">
            {checks.map((c) => (
              <div className="item" key={c.label}>
                {c.ok ? <CheckCircleIcon size={16} color="var(--success)" /> : <WarningIcon size={16} color="var(--warn)" />}
                <span className="spacer">{c.label}</span>
                {!c.ok && (c.fix.startsWith("Settings") ? <a className="btn sm" href="#/settings/ai">Open AI services</a> : <span className="small muted mono">{c.fix}</span>)}
              </div>
            ))}
          </div>
          {desktop && (() => {
            const missing = checks.flatMap((c) => ("id" in c && !c.ok ? [c.id] : []));
            if (!missing.length) return null;
            const cmd = installAll(missing, os);
            return (
              <div className="stack" style={{ padding: "12px 16px 16px", gap: 8 }}>
                <span className="small muted">
                  The desktop app uses the tools installed on this computer. Run this in a terminal to install what's missing, then click Check again.{" "}
                  <a href={`${repoURL}/blob/main/docs/desktop.md`} target="_blank" rel="noreferrer" style={{ color: "var(--primary)" }}>Setup guide</a>
                </span>
                <pre className="mono" style={{ margin: 0, padding: "10px 12px", background: "var(--muted)", borderRadius: "var(--radius-sm)", whiteSpace: "pre-wrap", wordBreak: "break-word" }}>{cmd}</pre>
                <span className="row" style={{ gap: 8 }}>
                  <button className="btn sm" onClick={() => navigator.clipboard?.writeText(cmd).then(() => toast("Copied. Paste it into a terminal."))}>Copy</button>
                  <button className="btn ghost sm" onClick={recheck}>Check again</button>
                </span>
              </div>
            );
          })()}
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
                Add your brand from its website, drop in a long video and pick "Hook reel".
                <button className="btn primary" onClick={() => setCreating(true)}><PlusIcon size={16} /> New video</button>
              </Empty>
            ) : <ProjectTable projects={all.slice(0, 8)} />}
          </div>
        </div>

        <div className="stack">
          <div className="card">
            <div className="card-h"><h2 className="spacer">Learn 37Studio</h2><a className="btn ghost sm" href="#/guide">Full guide</a></div>
            <div className="filelist">
              {learn.map(([id, label]) => (
                <button className="item link-row" key={id} onClick={() => help(id)}>
                  <BookOpenIcon size={16} />
                  <span className="spacer ellipsis">{label}</span>
                  <ArrowRightIcon size={14} />
                </button>
              ))}
            </div>
          </div>
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
