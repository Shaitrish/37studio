// The agent chat's feed, shared by the project editor and the Assistant: messages, tool steps and subagents.
import { memo, useEffect, useMemo, useRef, useState } from "react";
import {
  CheckCircleIcon, CircleNotchIcon, FileTextIcon, GlobeIcon, ListChecksIcon, MagnifyingGlassIcon, PencilLineIcon, SparkleIcon, StopCircleIcon,
  TerminalWindowIcon, TreeStructureIcon, WrenchIcon, XCircleIcon,
} from "@phosphor-icons/react";
import { cash, ChatEvent, clock, fail, parseTime } from "./api";
import { Md } from "./ui";

/** A chat event with its body parsed once, when it arrives: re-renders don't parse the whole conversation again,
 *  and the same object each time lets unchanged messages skip rendering. */
export type Parsed = { id: number; kind: string; b: any; at: string };
export const parse = (e: ChatEvent): Parsed => ({ id: e.id, kind: e.kind, b: JSON.parse(e.body), at: e.created_at });

/** A subagent the agent started with its Agent tool (agent.go ParseStream), and what it did. */
type Sub = { id: string; name: string; type: string; start: Parsed; steps: Parsed[]; done?: Parsed };
type Item = { k: "ev"; e: Parsed } | { k: "tools"; list: Parsed[] } | { k: "agents"; list: Sub[] };

// Runs of tool calls collapse into one expandable group; subagents started together share one block, and their
// own steps go inside it instead of the main list.
function buildFeed(events: Parsed[]) {
  const items: Item[] = [];
  const subs = new Map<string, Sub>();
  for (const e of events) {
    if (e.b.parent) { subs.get(e.b.parent)?.steps.push(e); continue; }
    if (e.kind === "agent_done") { const s = subs.get(e.b.id); if (s) s.done = e; continue; }
    const last = items[items.length - 1];
    if (e.kind === "agent") {
      const s: Sub = { id: e.b.id, name: e.b.name, type: e.b.type, start: e, steps: [] };
      subs.set(s.id, s);
      if (last?.k === "agents") last.list.push(s); else items.push({ k: "agents", list: [s] });
    } else if (e.kind === "tool" || e.kind === "tool_error") {
      if (last?.k === "tools") last.list.push(e); else items.push({ k: "tools", list: [e] });
    } else items.push({ k: "ev", e });
  }
  return items;
}

export function Feed({ events, running, pid }: { events: Parsed[]; running: boolean; pid: number }) {
  const items = useMemo(() => buildFeed(events), [events]);
  return <>{items.map((it, i) =>
    it.k === "tools" ? <ToolGroup key={it.list[0].id} tools={it.list} open={running && i === items.length - 1} /> :
    it.k === "agents" ? <AgentGroup key={it.list[0].start.id} subs={it.list} running={running} /> :
    <EventView key={it.e.id} e={it.e} pid={pid} />
  )}</>;
}

/** What the agent is doing this turn: its latest step (named after the subagent doing it) and how many subagents work. */
export function liveStatus(events: Parsed[]) {
  const open = new Map<string, string>();
  let step: string | undefined;
  for (const e of events) {
    if (e.kind === "agent") open.set(e.b.id, e.b.name);
    else if (e.kind === "agent_done") open.delete(e.b.id);
    else if (e.kind === "user" || e.kind === "result") { step = undefined; if (e.kind === "result") open.clear(); }
    else if (e.kind === "tool") step = (e.b.parent && open.has(e.b.parent) ? open.get(e.b.parent) + ": " : "") + tidy(e.b.summary);
  }
  return { step, agents: open.size };
}

const toolLabel = (name: string) => name.replace(/^mcp__37studio__/, "").replaceAll("_", " ");

const toolIcon = (name: string) =>
  name === "Bash" ? TerminalWindowIcon :
  ["Edit", "Write", "MultiEdit", "NotebookEdit"].includes(name) ? PencilLineIcon :
  name === "Read" ? FileTextIcon :
  ["Glob", "Grep"].includes(name) ? MagnifyingGlassIcon :
  name === "Skill" ? SparkleIcon :
  name.startsWith("Web") ? GlobeIcon :
  name === "TodoWrite" ? ListChecksIcon : WrenchIcon;

// Absolute project paths are noise for an editor: show "comp/index.html", not "/home/…/projects/1/comp/index.html".
export const tidy = (s = "") => s.replace(/\/\S*?\/(projects|tenants)\/\d+\//g, "");

function Step({ t }: { t: Parsed }) {
  if (t.kind === "tool_error") return <div className="msg-tool err" title="The agent saw this and retried"><span className="mono">{tidy(t.b.text)}</span></div>;
  if (t.kind === "text") return <Md text={t.b.text} className="msg-agent sub-text" />;
  const I = toolIcon(t.b.name);
  return <div className="msg-tool"><I size={13} /><b>{toolLabel(t.b.name)}</b><span className="mono ellipsis" title={t.b.summary}>{tidy(t.b.summary)}</span></div>;
}

// A group only changes when a step is added or it opens or closes, so a poll that brings nothing new skips it.
const ToolGroup = memo(function ToolGroup({ tools, open }: { tools: Parsed[]; open: boolean }) {
  const last = [...tools].reverse().find((t) => t.kind === "tool") ?? tools[tools.length - 1];
  const steps = tools.filter((t) => t.kind === "tool").length;
  return (
    <details open={open || undefined}>
      <summary className="msg-tool" style={{ cursor: "pointer", listStyle: "none" }}>
        <WrenchIcon size={13} />
        <b>{steps} step{steps === 1 ? "" : "s"}</b>
        <span className="ellipsis">{last.b.name?.startsWith("mcp__") ? toolLabel(last.b.name) : tidy(last.b.summary ?? last.b.text)}</span>
      </summary>
      <div className="stack" style={{ gap: 4, marginTop: 6 }}>
        {tools.map((t) => <Step key={t.id} t={t} />)}
      </div>
    </details>
  );
}, (a, b) => a.open === b.open && a.tools.length === b.tools.length && a.tools[a.tools.length - 1] === b.tools[b.tools.length - 1]);

/** Subagents started together: one row each with a live status, its latest step, and everything it did inside. */
function AgentGroup({ subs, running }: { subs: Sub[]; running: boolean }) {
  const done = subs.filter((s) => s.done).length;
  // In parallel when every one had started before the first finished.
  const parallel = subs.length > 1 && Math.max(...subs.map((s) => s.start.id)) < Math.min(...subs.map((s) => s.done?.id ?? Infinity));
  return (
    <div className="agents">
      <div className="agents-h">
        <TreeStructureIcon size={14} />
        <b className="spacer">{subs.length === 1 ? "Subagent" : `${subs.length} subagents${parallel ? " in parallel" : ""}`}</b>
        {subs.length > 1 && <span>{done === subs.length ? "all done" : `${done} of ${subs.length} done`}</span>}
      </div>
      {subs.map((s) => <AgentRow key={s.id} s={s} live={running} />)}
    </div>
  );
}

const agentIcon = { running: CircleNotchIcon, done: CheckCircleIcon, failed: XCircleIcon, stopped: StopCircleIcon };

function AgentRow({ s, live }: { s: Sub; live: boolean }) {
  // No finish and the turn is over: it was stopped with the agent.
  const state = s.done ? (s.done.b.error ? "failed" : "done") : live ? "running" : "stopped";
  const steps = s.steps.filter((x) => x.kind === "tool").length;
  const lastTool = [...s.steps].reverse().find((x) => x.kind === "tool");
  const report = [...s.steps].reverse().find((x) => x.kind === "text")?.b.text.trim().split("\n")[0];
  const secs = s.done && (parseTime(s.done.at).getTime() - parseTime(s.start.at).getTime()) / 1000;
  const line = state === "running" ? (lastTool ? `${toolLabel(lastTool.b.name)} ${tidy(lastTool.b.summary)}` : "Starting…")
    : state === "failed" ? "Ended with an error" : state === "stopped" ? "Stopped" : report || "Finished";
  const Icon = agentIcon[state];
  return (
    <details className={`agent ${state}`}>
      <summary>
        <Icon size={16} className="agent-icon" weight={state === "running" ? "bold" : "fill"} aria-label={state} />
        <span className="agent-main">
          <span className="row" style={{ gap: 6, minWidth: 0 }}>
            <b className="ellipsis">{s.name}</b>
            {s.type && s.type !== "general-purpose" && <span className="agent-type">{s.type}</span>}
          </span>
          <span className={`agent-line ellipsis ${state === "running" ? "mono" : ""}`} title={line}>{line}</span>
        </span>
        <span className="small faint nowrap">{steps} step{steps === 1 ? "" : "s"}{secs ? ` · ${clock(secs)}` : ""}</span>
      </summary>
      <div className="agent-steps">
        {s.steps.length ? s.steps.map((t) => <Step key={t.id} t={t} />) : <span className="small faint">Nothing yet.</span>}
      </div>
    </details>
  );
}

const imageLine = /^\[image: (chat\/[^\]]+)\]$/;

const EventView = memo(function EventView({ e, pid }: { e: Parsed; pid: number }) {
  switch (e.kind) {
    case "user": {
      const lines: string[] = e.b.text.split("\n");
      const shots = lines.map((l) => l.match(imageLine)?.[1]).filter(Boolean);
      return (
        <div className="msg-user">
          {lines.filter((l) => !imageLine.test(l)).join("\n")}
          {shots.map((s) => <a key={s} href={`/files/projects/${pid}/${s}`} target="_blank" rel="noreferrer"><img className="msg-img" src={`/files/projects/${pid}/${s}`} alt="Attached screenshot" /></a>)}
          {e.b.queued && <span className="queued-tag">sent while working</span>}
        </div>
      );
    }
    case "text": return <Md text={e.b.text} />;
    case "result":
      return <div className="msg-done">{e.b.error ? "Turn ended with an error" : "Turn finished"}{e.b.model ? ` · ${modelName(e.b.model)}` : ""} · {cash(e.b.cost)} · {clock(e.b.ms / 1000)}</div>;
    case "render": return <div className="msg-render"><CheckCircleIcon size={15} /> {e.b.text}</div>;
    case "image":
      return (
        <a className="msg-image" href={e.b.url} target="_blank" rel="noreferrer" title={`The agent checked ${e.b.path}. Click for full size.`}>
          <img src={e.b.url} alt={`Frame the agent checked: ${e.b.path}`} loading="lazy" />
          <span><MagnifyingGlassIcon size={12} /> Checked {e.b.path}</span>
        </a>
      );
    case "error": return <div className="msg-error">{e.b.text}</div>;
    default: return e.b.text ? <div className="msg-info">{e.b.text}</div> : null;
  }
});

/** "claude-opus-5-5" -> "Opus 5.5" */
export const modelName = (id: string) => id.replace(/^.*\//, "").replace(/^claude-/, "").replace(/-(\d+)-(\d+)$/, " $1.$2").replace(/^\w/, (c) => c.toUpperCase());

/**
 * Dictation with the browser's built-in speech recognition (Chrome, Edge, Safari). No key, no cost.
 * ponytail: Chrome sends the audio to Google's recognizer; switch to ElevenLabs Scribe if that matters.
 */
export function useDictation(videoLang: string, onFinal: (text: string) => void) {
  const SR = (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition;
  const [lang, setLang] = useState(videoLang === "bn" ? "bn-BD" : "en-US");
  const [listening, setListening] = useState(false);
  const [interim, setInterim] = useState("");
  const rec = useRef<any>(null);
  const final = useRef(onFinal);
  final.current = onFinal;
  const stop = () => rec.current?.stop();
  const start = () => {
    if (!SR || rec.current) return;
    const r = new SR();
    r.lang = lang;
    r.continuous = true;
    r.interimResults = true;
    r.onresult = (e: any) => {
      let partial = "";
      for (let i = e.resultIndex; i < e.results.length; i++) {
        const t = e.results[i][0].transcript;
        if (e.results[i].isFinal) final.current(t.trim()); else partial += t;
      }
      setInterim(partial.trim());
    };
    r.onerror = (e: any) => { if (e.error !== "no-speech" && e.error !== "aborted") fail(new Error(`Dictation: ${e.error}`)); };
    r.onend = () => { rec.current = null; setListening(false); setInterim(""); };
    rec.current = r;
    r.start();
    setListening(true);
  };
  useEffect(() => () => rec.current?.abort(), []);
  return { supported: !!SR, listening, interim, lang, setLang, start, stop };
}
