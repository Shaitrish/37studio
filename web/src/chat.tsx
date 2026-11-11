// The agent chat's feed, shared by the project editor and the Assistant: messages, tool steps and subagents.
import { KeyboardEvent, memo, useEffect, useMemo, useRef, useState } from "react";
import {
  CaretRightIcon, CheckCircleIcon, CircleNotchIcon, CopyIcon, FileIcon, MagnifyingGlassIcon, StopCircleIcon, TreeStructureIcon, XCircleIcon,
} from "@phosphor-icons/react";
import { cash, ChatEvent, clock, desktop, fail, parseTime, toast } from "./api";
import { flushSync } from "react-dom";
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

// Absolute project paths are noise for an editor: show "comp/index.html", not "/home/…/projects/1/comp/index.html".
export const tidy = (s = "") => s.replace(/\/\S*?\/(projects|tenants)\/\d+\//g, "");

// Steps read the way Claude Code prints them: "● Tool(what)", and an error the agent got back under it after "⎿".
function Step({ t, live }: { t: Parsed; live?: boolean }) {
  if (t.kind === "tool_error") return <div className="msg-tool err" title="The agent saw this and retried"><span className="tool-out">⎿</span><span className="mono">{tidy(t.b.text)}</span></div>;
  if (t.kind === "text") return <Md text={t.b.text} className="msg-agent sub-text" />;
  const what = tidy(t.b.summary);
  return (
    <div className="msg-tool">
      <i className={`tool-dot ${live ? "live" : ""}`} />
      <span className="ellipsis" title={what}><b>{toolLabel(t.b.name)}</b>{what && <span className="mono">({what})</span>}</span>
    </div>
  );
}

// A group only changes when a step is added or it opens or closes, so a poll that brings nothing new skips it.
const ToolGroup = memo(function ToolGroup({ tools, open }: { tools: Parsed[]; open: boolean }) {
  const last = [...tools].reverse().find((t) => t.kind === "tool") ?? tools[tools.length - 1];
  const steps = tools.filter((t) => t.kind === "tool").length;
  return (
    <details className="tools" open={open || undefined}>
      <summary className="msg-tool">
        <i className={`tool-dot ${open ? "live" : ""}`} />
        <b>{steps} step{steps === 1 ? "" : "s"}</b>
        <span className="mono ellipsis">{last.b.name?.startsWith("mcp__") ? toolLabel(last.b.name) : tidy(last.b.summary ?? last.b.text)}</span>
        <CaretRightIcon size={11} className="tool-caret" />
      </summary>
      <div className="tool-steps">
        {tools.map((t, i) => <Step key={t.id} t={t} live={open && i === tools.length - 1} />)}
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
/** A message you sent, without the [image: …] lines that carry its screenshots. */
const viewLine = /^\[viewing: ([^\]]+)\]$/; // the version picked under the preview, sent with the message
const promptText = (text: string) => text.split("\n").filter((l) => !imageLine.test(l) && !viewLine.test(l)).join("\n").trim();

// Hidden like the code blocks' Copy where the browser has no clipboard (an http:// address other than localhost).
function CopyButton({ text }: { text: string }) {
  if (!navigator.clipboard) return null;
  return (
    <button className="msg-copy" title="Copy" aria-label="Copy this message" onClick={() => navigator.clipboard.writeText(text).then(() => toast("Copied"), fail)}>
      <CopyIcon size={13} />
    </button>
  );
}

const kickoff = "Start the job."; // what agent.go sends for "Start agent": you never typed it, so Up skips it

/**
 * Up and Down in the composer walk through the messages you sent in this chat, like Claude Code: Up goes further back,
 * Down comes forward and ends on what you were typing. They only take over on the first line (Up) or the last (Down),
 * so moving around inside a long message still works. Returns the textarea's keydown handler.
 */
export function usePromptHistory(events: Parsed[], setText: (t: string) => void) {
  const sent = useMemo(() => {
    const out: string[] = [];
    for (const e of events) {
      if (e.kind !== "user") continue;
      const t = promptText(e.b.text);
      if (t && t !== kickoff && t !== out[out.length - 1]) out.push(t); // a quick reply sent twice is one entry, like a shell
    }
    return out;
  }, [events]);
  const back = useRef(-1); // how far back you are; -1 is your draft
  const draft = useRef("");
  const shown = useRef<string | undefined>(undefined);
  const step = (el: HTMLTextAreaElement, up: boolean) => {
    if (el.value !== shown.current) back.current = -1; // you typed since: that's your draft now
    const next = back.current + (up ? 1 : -1);
    if (next < -1 || next >= sent.length) return;
    if (back.current === -1) draft.current = el.value;
    back.current = next;
    const t = next === -1 ? draft.current : sent[sent.length - 1 - next];
    shown.current = t;
    flushSync(() => setText(t)); // the text must be in before the caret can move
    // Up leaves the caret at the start and Down at the end, so pressing again keeps going even past long messages.
    const caret = up ? 0 : t.length;
    el.setSelectionRange(caret, caret);
  };
  return (e: KeyboardEvent<HTMLTextAreaElement>) => {
    const up = e.key === "ArrowUp";
    if ((!up && e.key !== "ArrowDown") || e.shiftKey || e.altKey || e.ctrlKey || e.metaKey || e.nativeEvent.isComposing) return;
    const el = e.currentTarget, v = el.value;
    if (up ? v.slice(0, el.selectionStart).includes("\n") : v.slice(el.selectionEnd).includes("\n")) return;
    const edge = up ? 0 : v.length;
    if (el.selectionStart === edge && el.selectionEnd === edge) { e.preventDefault(); step(el, up); return; }
    // Only the browser knows where a long line wraps: let it move the caret, and if that lands on the very start (end),
    // the caret was on the first (last) row.
    setTimeout(() => { if (el.selectionStart === edge && el.selectionEnd === edge && el.value === v) step(el, up); });
  };
}

const EventView = memo(function EventView({ e, pid }: { e: Parsed; pid: number }) {
  switch (e.kind) {
    case "user": {
      const lines: string[] = e.b.text.split("\n");
      const shots = lines.map((l) => l.match(imageLine)?.[1]).filter(Boolean);
      const viewed = lines.map((l) => l.match(viewLine)?.[1]).find(Boolean);
      const said = promptText(e.b.text);
      // What you typed, the way Claude Code shows it: a full-width block behind "›".
      return (
        <div className="msg-prompt">
          <span className="prompt-mark" aria-hidden>›</span>
          <div className="prompt-body">
            {said}
            {viewed && <span className="prompt-file" title="Sent with this message"><FileIcon size={12} /> {viewed}</span>}
            {shots.map((s) => <a key={s} href={`/files/projects/${pid}/${s}`} target="_blank" rel="noreferrer"><img className="msg-img" src={`/files/projects/${pid}/${s}`} alt="Attached screenshot" /></a>)}
            {e.b.queued && <span className="queued-tag">sent while working</span>}
          </div>
          {said && <CopyButton text={said} />}
        </div>
      );
    }
    case "text": return <div className="msg-text"><Md text={e.b.text} /><CopyButton text={e.b.text} /></div>;
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
 * Dictation. In Chrome, Edge and Safari it uses the browser's own speech recognition (no key, no cost), which shows
 * words as you speak. The desktop app's window has none that works (WebKitGTK has no API; WebView2's needs Google's
 * keys), so there it records the microphone and the server transcribes the clip locally (POST /api/dictate).
 * ponytail: Chrome sends the audio to Google's recognizer; the local path could become the default if that matters.
 */
export function useDictation(videoLang: string, onFinal: (text: string) => void) {
  const SR = desktop ? undefined : (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition;
  const canRecord = typeof navigator.mediaDevices?.getUserMedia === "function" && !!(window as any).MediaRecorder;
  const [lang, setLang] = useState(videoLang === "bn" ? "bn-BD" : "en-US");
  const [listening, setListening] = useState(false);
  const [busy, setBusy] = useState(false); // the server is transcribing a recording
  const [interim, setInterim] = useState("");
  const rec = useRef<any>(null);
  const final = useRef(onFinal);
  final.current = onFinal;
  const stop = () => rec.current?.stop();
  const startRecording = async () => {
    let stream: MediaStream;
    try { stream = await navigator.mediaDevices.getUserMedia({ audio: true }); }
    catch { fail(new Error("Dictation: the microphone isn't available. Allow 37Studio to use it, or check it's plugged in.")); return; }
    const r = new MediaRecorder(stream);
    const chunks: Blob[] = [];
    r.ondataavailable = (e) => e.data.size && chunks.push(e.data);
    r.onstop = async () => {
      stream.getTracks().forEach((t) => t.stop());
      rec.current = null;
      setListening(false);
      if (!chunks.length) return;
      const form = new FormData();
      form.append("audio", new Blob(chunks, { type: r.mimeType }), r.mimeType.includes("mp4") ? "dictation.m4a" : "dictation.webm");
      form.append("lang", lang.slice(0, 2));
      setBusy(true);
      try {
        const res = await fetch("/api/dictate", { method: "POST", body: form });
        const out = await res.json().catch(() => ({}));
        if (!res.ok) throw new Error(out.error ?? "Dictation failed");
        if (out.text?.trim()) final.current(out.text.trim());
      } catch (e) { fail(e); } finally { setBusy(false); }
    };
    rec.current = r;
    r.start();
    setListening(true);
  };
  const start = () => {
    if (rec.current || busy) return;
    if (!SR) { if (canRecord) startRecording(); return; }
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
  useEffect(() => () => { const r = rec.current; if (r?.abort) r.abort(); else r?.stop(); }, []);
  return { supported: !!SR || canRecord, listening, busy, interim, lang, setLang, start, stop };
}
