import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { ChatCircleDotsIcon, PlusIcon } from "@phosphor-icons/react";
import { api, ChatEvent, fail, Meta, useApi } from "../api";
import { Empty, HelpButton } from "../ui";
import { Feed, liveStatus, parse, Parsed } from "../chat";
import { Composer } from "../composer";

interface AssistantState { events: ChatEvent[]; running: boolean; started: boolean }

const ideas = [
  "Add a brand from its website: https://",
  "Make a 30-second reel for one of my brands about our best offer",
  "Plan three reels a week for a brand, posted at 8 pm, with my approval",
  "What did we spend on AI this month, by brand?",
  "Which videos are waiting for my review?",
];

/** Sidebar → Assistant: run the studio by chatting. Claude drives the same tools other agents get over MCP (internal/mcp). */
export default function Assistant() {
  const [events, setEvents] = useState<Parsed[]>([]);
  const [running, setRunning] = useState(false);
  // The video chat's models: Claude through Claude Code, the rest through OpenCode (assistant.go). Sonnet is plenty.
  // OpenCode's free models are left out: they refuse an OpenCode whose tools are locked down to this app's.
  const { data: meta } = useApi<Meta>("/meta");
  const [quality, setQuality] = useState("draft");
  const feed = useRef<HTMLDivElement>(null);
  const last = events[events.length - 1]?.id ?? 0;

  const [loaded, setLoaded] = useState(false);
  const load = (after = 0) => api<AssistantState>(`/assistant?after=${after}`).then((s) => {
    const got = s.events.map(parse);
    setEvents((ev) => (after ? [...ev, ...got] : got));
    setRunning(s.running);
    setLoaded(true);
  }, (e) => { if (after) fail(e); });
  // Retry the first load: right after an update or restart the server may not be up yet, and an empty page would
  // look like the conversation was lost.
  useEffect(() => {
    if (loaded) return;
    load();
    const t = setInterval(() => load(), 3000);
    return () => clearInterval(t);
  }, [loaded]);
  // ponytail: polls while a reply is coming in; a stream would be smoother, but turns are short.
  useEffect(() => {
    if (!running) return;
    const t = setInterval(() => load(last), 1500);
    return () => clearInterval(t);
  }, [running, last]);

  const live = useMemo(() => (running ? liveStatus(events) : undefined), [events, running]);
  useLayoutEffect(() => { feed.current?.scrollTo({ top: feed.current.scrollHeight }); }, [events.length, running]);

  const send = (t: string, q: string) => api("/assistant/messages", { body: { text: t, quality: q } }).then(() => { setRunning(true); load(last); }, (e) => { fail(e); throw e; });
  const stop = () => api("/assistant/stop", { body: {} }).then(() => load(last), fail);
  const reset = () => confirm("Start a new conversation? This one is cleared.") &&
    api("/assistant", { method: "DELETE" }).then(() => { setEvents([]); }, fail);

  return (
    <div className="page assistant">
      <div className="page-head">
        <div className="spacer">
          <h1>Assistant</h1>
          <p>Ask for anything in plain words: add brands, plan campaigns, start videos, publish, check costs. It uses the same tools as every button here.</p>
        </div>
        <HelpButton to="assistant" />
        {events.length > 0 && <button className="btn" disabled={running} onClick={reset}><PlusIcon size={15} /> New conversation</button>}
      </div>
      <div className="card chat">
        <div className="feed" ref={feed}>
          {!loaded && <div className="typing"><i /><i /><i /> <span>Loading the conversation…</span></div>}
          {loaded && !events.length && (
            <Empty icon={<ChatCircleDotsIcon size={28} />} title="What should we make today?">
              <div className="chips" style={{ justifyContent: "center" }}>
                {ideas.map((i) => <button key={i} className="chip" onClick={() => send(i, quality).catch(() => {})}>{i}</button>)}
              </div>
            </Empty>
          )}
          <Feed events={events} running={running} pid={0} />
          {live && <div className="typing"><i /><i /><i /> <span className="ellipsis">{[live.agents && `${live.agents} subagent${live.agents === 1 ? "" : "s"} working`, live.step].filter(Boolean).join(" · ") || "Working…"}</span></div>}
        </div>
        <Composer events={events} running={running} onSend={send} onStop={stop}
          qualities={(meta?.qualities ?? []).filter((q) => !(q.harness && q.id.endsWith("-free")))} quality={quality} onQuality={setQuality} lang="en"
          placeholder={running ? "Wait for the reply, or stop it…" : "Ask the assistant, or / for ideas. It asks before it deletes, publishes or replies."}
          actions={ideas.map((label) => ({ label }))} />
      </div>
    </div>
  );
}
