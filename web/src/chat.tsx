// The agent chat's feed, shared by the project editor and the Assistant: messages, tool steps and subagents.
import { KeyboardEvent, memo, useEffect, useMemo, useRef, useState } from "react";
import {
  CaretRightIcon, CheckCircleIcon, CircleNotchIcon, CopyIcon, FileIcon, MagnifyingGlassIcon, StopCircleIcon, TreeStructureIcon, XCircleIcon,
} from "@phosphor-icons/react";
import { cash, ChatEvent, claudePlan, clock, desktop, fail, parseTime, toast } from "./api";
import { flushSync } from "react-dom";
import { costTitle, Md } from "./ui";

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
      return <div className="msg-done">{e.b.error ? "Turn ended with an error" : "Turn finished"}{e.b.model ? ` · ${modelName(e.b.model)}` : ""} · <span title={costTitle()}>{cash(e.b.cost)}{claudePlan && e.b.cost ? " at API prices" : ""}</span> · {clock(e.b.ms / 1000)}</div>;
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
 * Dictation, always in English. In Chrome, Edge and Safari it uses the browser's own speech recognition, which shows
 * words as you speak. Everywhere else (the desktop app's WebKitGTK and WebView2, Firefox) it streams the microphone to
 * the speech model on this computer (POST /api/dictate, see streamMic).
 * ponytail: Chrome sends the audio to Google's recognizer; the local path could become the default if that matters.
 */
export function useDictation(onFinal: (text: string) => void) {
  const SR = desktop ? undefined : (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition;
  const canStream = typeof navigator.mediaDevices?.getUserMedia === "function" && !!window.AudioContext;
  const [listening, setListening] = useState(false);
  const [pending, setPending] = useState(0); // phrases the local model is still writing down
  const [interim, setInterim] = useState("");
  const rec = useRef<{ stop: () => void; abort: () => void } | null>(null);
  const final = useRef(onFinal);
  final.current = onFinal;
  const stop = () => { rec.current?.stop(); setListening(false); };
  const start = () => {
    if (rec.current || pending) return;
    if (!SR) {
      if (!canStream) return;
      let opening = true; // stop pressed while the microphone opens
      rec.current = { stop: () => { opening = false; }, abort: () => { opening = false; } };
      setListening(true);
      streamMic({
        text: (t) => final.current(t), draft: setInterim, pending: setPending,
        ended: () => { rec.current = null; setListening(false); setInterim(""); },
      }).then((r) => { rec.current = r; if (!opening) r.stop(); }, (e) => { rec.current = null; setListening(false); fail(e); });
      return;
    }
    const r = new SR();
    r.lang = "en-US";
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
    rec.current = { stop: () => r.stop(), abort: () => r.abort() };
    r.start();
    setListening(true);
  };
  useEffect(() => () => rec.current?.abort(), []);
  return { supported: !!SR || canStream, listening, busy: pending > 0 && !listening, interim, start, stop };
}

/**
 * Streams the microphone to this computer's speech model as 16 kHz 16-bit PCM. A pause of about half a second ends a
 * phrase, which goes off to be written down properly (small.en) while you carry on talking; the phrase being spoken
 * shows as a quick draft (tiny.en) a few times a second. Phrases land in the box in order as each is done, so
 * stopping only waits for the last one. Raw PCM rather than MediaRecorder, which WebKitGTK doesn't support.
 */
async function streamMic(on: { text: (t: string) => void; draft: (t: string) => void; pending: (n: number) => void; ended: () => void }) {
  const post = async (pcm: Int16Array | null, final: boolean) => {
    const res = await fetch(`/api/dictate${final ? "?final=1" : ""}`, { method: "POST", body: pcm as BodyInit | null, headers: { "Content-Type": "application/octet-stream" } });
    const out = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(out.error ?? "Dictation failed");
    return out as { text?: string; busy?: boolean };
  };
  const warm = post(null, false); // loads the models while the microphone opens
  let stream: MediaStream;
  try { stream = await navigator.mediaDevices.getUserMedia({ audio: { echoCancellation: true, noiseSuppression: true } }); }
  catch { throw new Error("Dictation: the microphone isn't available. Allow 37Studio to use it, or check it's plugged in."); }
  await warm.catch((e) => { stream.getTracks().forEach((t) => t.stop()); throw e; });

  const rate = 16000;
  type Part = { text: string; done: boolean; sent: boolean; pcm: Int16Array[]; samples: number; voiced: number };
  const parts: Part[] = [];
  let cur: Part | null = null, pre: Int16Array[] = [], quiet = 0, floor = 0.01, pending = 0, stopped = false, failed = false, drafting = false;
  const join = (p: Part) => { const out = new Int16Array(p.samples); let o = 0; for (const c of p.pcm) { out.set(c, o); o += c.length; } return out; };
  const show = () => {
    while (parts.length && parts[0].done) { const t = parts.shift()!.text; if (t) on.text(t); }
    on.draft(parts.map((p) => p.text).filter(Boolean).join(" "));
    if (stopped && !pending) on.ended();
  };
  const end = (p: Part) => {
    cur = null;
    if (p.voiced < rate / 4) { parts.splice(parts.indexOf(p), 1); show(); return; } // a click or a cough
    p.sent = true;
    on.pending(++pending);
    post(join(p), true)
      .then((o) => { p.text = o.text ?? ""; }, (e) => { if (!failed) fail(e); failed = true; })
      .finally(() => { p.done = true; on.pending(--pending); show(); });
  };

  const ac = new AudioContext();
  ac.resume();
  const src = ac.createMediaStreamSource(stream);
  const node = ac.createScriptProcessor(4096, 1, 1);
  const step = ac.sampleRate / rate;
  node.onaudioprocess = (e) => {
    const x = e.inputBuffer.getChannelData(0);
    let sum = 0;
    for (let i = 0; i < x.length; i++) sum += x[i] * x[i];
    const rms = Math.sqrt(sum / x.length);
    floor = rms < floor ? rms : floor * 1.003; // the room's noise, followed slowly upward and at once downward
    const loud = rms > Math.max(0.008, floor * 3);
    // Down to 16 kHz by averaging each window of input samples.
    const s16 = new Int16Array(Math.floor(x.length / step));
    for (let i = 0; i < s16.length; i++) {
      const a = Math.floor(i * step), b = Math.min(x.length, Math.floor((i + 1) * step));
      let v = 0;
      for (let j = a; j < b; j++) v += x[j];
      s16[i] = Math.max(-1, Math.min(1, v / Math.max(1, b - a))) * 32767;
    }
    if (!cur) {
      if (!loud) { pre.push(s16); if (pre.length > 3) pre.shift(); return; } // a little lead-in for the first word
      cur = { text: "", done: false, sent: false, pcm: pre, samples: pre.reduce((n, c) => n + c.length, 0), voiced: 0 };
      pre = [];
      parts.push(cur);
    }
    cur.pcm.push(s16);
    cur.samples += s16.length;
    if (loud) { cur.voiced += s16.length; quiet = 0; } else quiet += s16.length;
    if (quiet >= rate * 0.6 || cur.samples >= rate * 20) end(cur);
  };
  src.connect(node);
  node.connect(ac.destination); // a script node only runs when connected; it outputs silence

  const drafts = setInterval(() => {
    const p = cur;
    if (!p || drafting || p.voiced < rate * 0.3) return;
    drafting = true;
    post(join(p), false).then((o) => { if (!o.busy && !p.done && o.text) { p.text = o.text; show(); } }, () => {}).finally(() => { drafting = false; });
  }, 600);
  const close = () => {
    clearInterval(drafts);
    node.onaudioprocess = null;
    src.disconnect(); node.disconnect();
    stream.getTracks().forEach((t) => t.stop());
    ac.close();
  };
  return {
    stop: () => { if (stopped) return; stopped = true; close(); if (cur) end(cur); show(); },
    abort: () => { stopped = true; close(); },
  };
}
