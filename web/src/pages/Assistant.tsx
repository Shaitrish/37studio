import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { ArrowUpIcon, ChatCircleDotsIcon, PlusIcon, SquareIcon } from "@phosphor-icons/react";
import { api, ChatEvent, fail } from "../api";
import { Empty } from "../ui";
import { EventView, Parsed, ToolGroup } from "./Project";

interface AssistantState { events: ChatEvent[]; running: boolean; started: boolean }

const ideas = [
  "Add a company from its website: https://",
  "Make a 30-second reel for one of my companies about our best offer",
  "Plan three reels a week for a company, posted at 8 pm, with my approval",
  "What did we spend on AI this month, by company?",
  "Which videos are waiting for my review?",
];

/** Sidebar → Assistant: run the studio by chatting. Claude drives the same tools other agents get over MCP (internal/mcp). */
export default function Assistant() {
  const [events, setEvents] = useState<ChatEvent[]>([]);
  const [running, setRunning] = useState(false);
  const [text, setText] = useState("");
  const feed = useRef<HTMLDivElement>(null);
  const last = events[events.length - 1]?.id ?? 0;

  const load = (after = 0) => api<AssistantState>(`/assistant?after=${after}`).then((s) => {
    setEvents((ev) => (after ? [...ev, ...s.events] : s.events));
    setRunning(s.running);
  }, fail);
  useEffect(() => { load(); }, []);
  // ponytail: polls while a reply is coming in; a stream would be smoother, but turns are short.
  useEffect(() => {
    if (!running) return;
    const t = setInterval(() => load(last), 1500);
    return () => clearInterval(t);
  }, [running, last]);

  const items = useMemo(() => {
    const out: (Parsed | Parsed[])[] = [];
    for (const e of events) {
      const ev = { id: e.id, kind: e.kind, b: JSON.parse(e.body) };
      const prev = out[out.length - 1];
      if (ev.kind === "tool" || ev.kind === "tool_error") Array.isArray(prev) ? prev.push(ev) : out.push([ev]);
      else out.push(ev);
    }
    return out;
  }, [events]);
  useLayoutEffect(() => { feed.current?.scrollTo({ top: feed.current.scrollHeight }); }, [items.length, running]);

  const send = async (t = text) => {
    if (!t.trim() || running) return;
    setText("");
    try { await api("/assistant/messages", { body: { text: t.trim() } }); setRunning(true); load(last); } catch (e) { fail(e); setText(t); }
  };
  const stop = () => api("/assistant/stop", { body: {} }).then(() => load(last), fail);
  const reset = () => confirm("Start a new conversation? This one is cleared.") &&
    api("/assistant", { method: "DELETE" }).then(() => { setEvents([]); }, fail);

  return (
    <div className="page assistant">
      <div className="page-head">
        <div className="spacer">
          <h1>Assistant</h1>
          <p>Ask for anything in plain words: add companies, plan campaigns, start videos, publish, check costs. It uses the same tools as every button here.</p>
        </div>
        {events.length > 0 && <button className="btn" disabled={running} onClick={reset}><PlusIcon size={15} /> New conversation</button>}
      </div>
      <div className="card chat">
        <div className="feed" ref={feed}>
          {!events.length && (
            <Empty icon={<ChatCircleDotsIcon size={28} />} title="What should we make today?">
              <div className="chips" style={{ justifyContent: "center" }}>
                {ideas.map((i) => <button key={i} className="chip" onClick={() => setText(i)}>{i}</button>)}
              </div>
            </Empty>
          )}
          {items.map((it, i) =>
            Array.isArray(it) ? <ToolGroup key={it[0].id} tools={it} open={running && i === items.length - 1} /> : <EventView key={it.id} e={it} pid={0} />
          )}
          {running && <div className="typing"><i /><i /><i /> <span>Working…</span></div>}
        </div>
        <div className="composer">
          <div className="composer-box">
            <textarea value={text} rows={2} autoFocus placeholder={running ? "Wait for the reply, or stop it…" : "Ask the assistant…"} aria-label="Message the assistant"
              onChange={(e) => setText(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); send(); }
                if (e.key === "Escape" && running) stop();
              }} />
            <div className="composer-bar">
              <span className="small faint spacer">Asks before it deletes, publishes or replies to customers.</span>
              {running && <button className="cbtn stop" onClick={stop} title="Stop (Esc)" aria-label="Stop"><SquareIcon size={13} weight="fill" /></button>}
              <button className="cbtn send" disabled={!text.trim() || running} onClick={() => send()} title="Send (Enter)" aria-label="Send"><ArrowUpIcon size={16} /></button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
