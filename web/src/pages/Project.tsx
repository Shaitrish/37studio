import "@hyperframes/player";
import { CSSProperties, PointerEvent as ReactPointerEvent, ReactNode, RefObject, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { ArrowsOutCardinalIcon, ArrowsInLineHorizontalIcon, ArrowsOutLineHorizontalIcon, BookmarkSimpleIcon, CheckCircleIcon, CoinsIcon, ImagesSquareIcon, NotePencilIcon, PaperPlaneTiltIcon, StackIcon, TimerIcon, FolderOpenIcon, LinkIcon, ShareNetworkIcon, CaretRightIcon, FilmSlateIcon, DownloadSimpleIcon, FilmStripIcon, ArrowsClockwiseIcon, ArrowsOutIcon, MicrophoneIcon, MusicNotesIcon, SpeakerHighIcon, ImageIcon, PlayIcon, PauseIcon, SlidersHorizontalIcon, SparkleIcon, TrashIcon, MagicWandIcon, XIcon } from "@phosphor-icons/react";
import { Quality } from "../api";
import { api, bytes, canReveal, ChatEvent, clock, desktop, fail, FileInfo, go, languageName, Meta, nudge, openExternal, pref, Project, showInFolder, toast, upload, cash, useApi } from "../api";
import { Feed, liveStatus, parse, Parsed } from "../chat";
import { Composer, type Context } from "../composer";
import { shapeOf, SizesModal } from "./Sizes";
import { costTitle, DropZone, Empty, HelpButton, PlanNote, Md, FileTile, KindIcon, ModelOptions, ModelPicker, Progress, Seg, Status, projectStatus } from "../ui";
import { presetIcon } from "./Projects";
import { PostsCard, PublishModal } from "./Social";
import { SaveTemplate } from "./Library";
import { approve, skip } from "./Autopilot";

const tabHelp: Record<string, string> = { videos: "long-recordings", generate: "generated-media-and-voices", posts: "publish", cost: "costs-and-spending" };

/** The editor's left rail: each opens its panel beside the preview, the way video editors keep their tools. */
type PanelId = "videos" | "brief" | "sources" | "generate" | "posts" | "cost";
const panelInfo: Record<PanelId, { label: string; title: string; hint: string; Icon: typeof SparkleIcon }> = {
  videos: { label: "Videos", title: "Videos from this source", hint: "cut and queued by the agent", Icon: StackIcon },
  brief: { label: "Brief", title: "Brief", hint: "the agent reads it on the first turn", Icon: NotePencilIcon },
  sources: { label: "Media", title: "Media", hint: "your footage, and everything the video uses", Icon: ImagesSquareIcon },
  generate: { label: "Generate", title: "Generate media", hint: "images, clips, voice and music", Icon: MagicWandIcon },
  posts: { label: "Posts", title: "Posts", hint: "where this video went, and how it's doing", Icon: PaperPlaneTiltIcon },
  cost: { label: "Cost", title: "Cost", hint: "AI usage; rendering on this computer is free", Icon: CoinsIcon },
};

export default function ProjectPage({ id }: { id: number }) {
  const { data: p, error, reload } = useApi<Project>(`/projects/${id}`);
  const { data: meta } = useApi<Meta>("/meta");
  const [events, setEvents] = useState<Parsed[]>([]);
  // Chat width and full-width mode are this browser's choice, kept for every project.
  const ws = useRef<HTMLDivElement>(null);
  const [chatW, setChatW] = useState(() => Number(pref("chat-w")) || defaultChatW);
  const [chatFull, setChatFull] = useState(() => pref("chat-full") === "1");
  useEffect(() => { pref("chat-w", String(chatW)); }, [chatW]);
  useEffect(() => { pref("chat-full", chatFull ? "1" : "0"); }, [chatFull]);
  const [view, setView] = useState<FileInfo | "comp">();
  // The open panel is this browser's choice, kept across projects; closing it gives the preview the room.
  const [panel, setPanelState] = useState<PanelId | null>(() => (pref("editor-panel") as PanelId) || null);
  const setPanel = (v: PanelId | null) => { setPanelState(v); pref("editor-panel", v ?? ""); };
  const [moment, setMoment] = useState<Moment>();
  const [publishing, setPublishing] = useState(false);
  const [sizing, setSizing] = useState<FileInfo>();
  const [templating, setTemplating] = useState(false);
  // The player only reloads when asked: remounting on every agent save made the preview flicker and restart.
  const [shownComp, setShownComp] = useState<string>();
  const stage = useRef<HTMLDivElement>(null);
  // F toggles full screen when you're not typing.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement;
      if (e.key !== "f" || e.metaKey || e.ctrlKey || t.closest("input, textarea, select, [contenteditable]")) return;
      document.fullscreenElement ? document.exitFullscreen() : stage.current?.requestFullscreen();
    };
    addEventListener("keydown", onKey);
    return () => removeEventListener("keydown", onKey);
  }, []);

  // Pausing the preview marks a moment for the chat: the time, and the text on screen then, go with the next message.
  const playerKey = `${!!p}|${shownComp}|${view && view !== "comp" ? view.url : "comp"}`;
  useEffect(() => {
    setMoment(undefined);
    const el = stage.current?.querySelector<HTMLElement & { currentTime: number; paused: boolean }>("hyperframes-player, video");
    if (!el) return;
    let timer = 0, last = -1;
    const update = () => {
      const t = el.paused ? el.currentTime : -1;
      if (Math.abs(t - last) < 0.04) return;
      last = t;
      clearTimeout(timer);
      if (t < 0.05) { setMoment(undefined); return; }
      // Dragging the playhead passes many frames: read the screen once it settles.
      timer = window.setTimeout(() => setMoment({ t, live: el.tagName !== "VIDEO", screen: el.tagName === "VIDEO" ? [] : screenText(el) }), 250);
    };
    // Events for play and pause; a light poll too, since not every seek of the player fires one.
    const evs = ["pause", "play", "timeupdate", "seeked"];
    evs.forEach((e) => el.addEventListener(e, update));
    const poll = setInterval(update, 500);
    return () => { clearTimeout(timer); clearInterval(poll); evs.forEach((e) => el.removeEventListener(e, update)); };
  }, [playerKey]);

  // ponytail: polling, not SSE. Every 1.5 s while the agent works (its turns take minutes), every 5 s while it waits,
  // and not at all while the window is hidden. Sending a message polls at once.
  const working = useRef(false);
  working.current = !!p?.running;
  const poll = useRef(() => {});
  useEffect(() => {
    let last = 0, alive = true, timer = 0, inFlight = false;
    const tick = async () => {
      clearTimeout(timer);
      if (inFlight || !alive || document.hidden) return; // the request in flight schedules the next; showing the window restarts it
      inFlight = true;
      let more = false;
      try {
        const evs = await api<ChatEvent[]>(`/projects/${id}/events?after=${last}`);
        if (alive && evs.length) {
          last = evs[evs.length - 1].id;
          more = evs.length >= 500; // a long chat comes in pages of 500: fetch the rest straight away
          setEvents((e) => [...e, ...evs.map(parse)]);
          if (evs.some((e) => !chatOnly.has(e.kind))) reload();
        }
      } catch { /* server restarting; keep polling */ }
      inFlight = false;
      if (alive) timer = window.setTimeout(tick, more ? 0 : working.current ? 1500 : 5000);
    };
    poll.current = tick;
    document.addEventListener("visibilitychange", tick);
    tick();
    return () => { alive = false; clearTimeout(timer); document.removeEventListener("visibilitychange", tick); };
  }, [id, reload]);

  // A transcription carries on after the agent's turn, and a download runs before it starts: poll for those too.
  const busy = !!p?.running || p?.transcript?.state === "running" || (!!p?.import && !p.import.done);
  useEffect(() => {
    if (!busy) return;
    const t = setInterval(() => document.hidden || reload(), p?.render ? 1500 : 4000); // faster while a render reports progress
    return () => clearInterval(t);
  }, [busy, !!p?.render, reload]);

  useEffect(() => { if (p?.composition && !shownComp) setShownComp(p.composition); }, [p?.composition]);
  const hasVideos = !!p?.children?.length;
  useEffect(() => { if (hasVideos) setPanelState((t) => (t === "brief" ? "videos" : t)); }, [hasVideos]);

  // A new final video landing while the person watches is the happy moment to ask for a donation.
  const finalVideos = p?.outputs?.filter((f) => f.kind === "video").length;
  const seenFinals = useRef<number>(undefined);
  useEffect(() => {
    if (finalVideos === undefined) return;
    if (seenFinals.current !== undefined && finalVideos > seenFinals.current) nudge("Your video is rendered and ready to post.");
    seenFinals.current = finalVideos;
  }, [finalVideos]);

  // A new post opens straight on Publish: picking accounts and captions is all that's left to do.
  const autoPublish = useRef(false);
  useEffect(() => {
    if (p?.kind !== "post" || autoPublish.current || !p.outputs?.length || p.posts?.length) return;
    autoPublish.current = true;
    setPublishing(true);
  }, [p]);

  if (error) return <div className="page"><Empty icon={<FilmSlateIcon size={28} />} title="Project not found"><a className="btn" href="#/projects">Back to projects</a></Empty></div>;
  if (!p) return <EditorSkeleton chatW={chatW} chatFull={chatFull} />;

  const media = (f: FileInfo) => f.kind === "video" || f.kind === "image" || f.kind === "audio";
  const finals = (p.outputs ?? []).filter(media);
  // A post is a finished file from the user's computer (NewPost.tsx): no agent, timeline or export, just Publish.
  const post = p.kind === "post";
  const postable = finals.some((f) => f.kind === "video" || f.kind === "image");
  const drafts = (p.drafts ?? []).filter(media);
  const current: FileInfo | "comp" | undefined = view ?? (p.composition ? "comp" : finals[0] ?? drafts[0]);
  const stale = !!p.composition && !!shownComp && p.composition !== shownComp;
  const Icon = presetIcon[p.kind] ?? MagicWandIcon;
  const preset = meta?.presets.find((x) => x.id === p.kind);
  const panels: PanelId[] = [...(p.children?.length ? ["videos" as const] : []), "brief", ...(post ? [] : ["sources" as const, "generate" as const]), "posts", "cost"];
  const shown = panel && panels.includes(panel) ? panel : null;
  const act = (path: string, body: object = {}, msg?: string) =>
    api(`/projects/${id}/${path}`, { body }).then(() => { if (msg) toast(msg); reload(); }, fail);

  // Open the window before the await so popup blockers treat it as a direct click. The desktop app has no tabs:
  // the editor opens in the system browser, and the app's own window stays on the project.
  async function openTimeline() {
    const w = desktop ? null : window.open("", "_blank");
    try {
      const r = await api<{ port: number; project: string }>(`/projects/${id}/studio`, { body: {} });
      const url = `${location.protocol}//${location.hostname}:${r.port}/#project/${r.project}`;
      if (desktop) openExternal(url); else if (w) w.location.href = url; else location.href = url;
      reload();
    } catch (e) { w?.close(); fail(e); }
  }

  async function remove() {
    if (!confirm(`Delete "${p!.title}" and all of its files? This cannot be undone.`)) return;
    try { await api(`/projects/${id}`, { method: "DELETE" }); go("/projects"); } catch (e) { fail(e); }
  }
  const delFile = (dir: string) => (f: FileInfo) => act0(`/projects/${id}/files?path=${dir}/${encodeURIComponent(f.path)}`, reload);

  return (
    <div className="page wide editor">
      <div className="editor-head">
        <div className="editor-title">
          <div className="crumbs">
            <a href="#/projects">Projects</a><CaretRightIcon size={14} />
            <a href={`#/clients/${p.tenant_id}`}>{p.tenant_name}</a>
            {p.campaign_id && <><CaretRightIcon size={14} /><a href={`#/campaigns/${p.campaign_id}`}>{p.campaign_name}</a></>}
            {p.parent_id && <><CaretRightIcon size={14} /><a href={`#/projects/${p.parent_id}`} title="The project this video was cut from">{p.parent_title}</a></>}
          </div>
          <div className="row">
            <h1 className="ellipsis" title={p.title}>{p.title}</h1>
            <Status value={projectStatus(p)} />
          </div>
          <p className="row wrap small muted" style={{ marginTop: 2 }}>
            <Icon size={14} /> {preset?.label ?? (post ? "Post from your computer" : p.kind)} · {p.language ? languageName(p.language) + " · " : ""}{p.aspect}{p.seconds ? ` · ~${p.seconds}s` : ""} · <span title={costTitle()}>{cash(p.quote.cost)}</span>
          </p>
        </div>
        <div className="row wrap" style={{ justifyContent: "flex-end" }}>
          <HelpButton to={(panel && tabHelp[panel]) ?? "work-on-a-project"} />
          {!post && !p.running && p.status !== "delivered" && finals.length > 0 && (
            <button className="btn" onClick={() => api(`/projects/${id}`, { method: "PUT", body: { status: "delivered" } }).then(reload, fail)}>
              <CheckCircleIcon size={15} /> Mark delivered
            </button>
          )}
          {!post && <span className="row" style={{ gap: 2 }}>
            <button className="btn" disabled={!p.composition} onClick={openTimeline} title="Drag, trim and split clips, change text and audio by hand">
              <SlidersHorizontalIcon size={15} /> {p.studio_port ? "Open timeline" : "Edit timeline"}
            </button>
            {p.studio_port && (
              <button className="btn ghost icon" title="Close the timeline editor" aria-label="Close the timeline editor"
                onClick={() => api(`/projects/${id}/studio`, { method: "DELETE" }).then(reload, fail)}><XIcon size={15} /></button>
            )}
          </span>}
          {!post && <button className="btn" disabled={!p.composition} onClick={() => setTemplating(true)} title="Start later videos from this one's look">
            <BookmarkSimpleIcon size={15} /> Save as template
          </button>}
          <button className={`btn ${post ? "primary" : ""}`} disabled={!postable} onClick={() => setPublishing(true)}
            title={postable ? "Post to the brand's connected social accounts" : "Export a final video first"}>
            <ShareNetworkIcon size={15} /> Publish
          </button>
          {!post && <button className="btn primary" disabled={!p.composition || p.running} onClick={() => act("render", { quality: "high" }, "Export started; it appears in Versions")}
            title={p.running ? "Wait for the agent to finish" : "Render the final video"}>
            <DownloadSimpleIcon size={15} /> Export
          </button>}
        </div>
      </div>

      {p.autopilot_stage === "approve" && (
        <div className="approve-bar">
          <span className="spacer">
            <strong>Autopilot made this video.</strong> Watch it, check the captions in Posts, then approve to post to {p.posts?.filter((x) => x.status === "approval").length ?? 0} account(s).
          </span>
          <button className="btn" onClick={() => setPanel("posts")}>Check captions</button>
          <button className="btn ghost" onClick={() => skip(p.id, reload)}>Skip</button>
          <button className="btn primary" onClick={() => approve(p.id, reload)}>Approve</button>
        </div>
      )}

      <div className={`workspace ${chatFull && p.kind !== "post" ? "chat-full" : ""}`} ref={ws} style={{ "--chat-w": `${chatW}px` } as CSSProperties}>
        <div className="editor-main">
          <nav className="rail" aria-label="Project panels">
            {panels.map((k) => {
              const { label, Icon } = panelInfo[k];
              const count = k === "videos" ? p.children?.length : k === "generate" ? p.generated?.length : k === "posts" ? p.posts?.length : 0;
              return (
                <button key={k} className={shown === k ? "on" : ""} aria-pressed={shown === k} onClick={() => setPanel(shown === k ? null : k)}
                  title={k === "cost" ? costTitle() : panelInfo[k].hint}>
                  <Icon size={22} weight={shown === k ? "fill" : "regular"} />
                  <span>{k === "cost" ? cash(p.quote.cost) : label}</span>
                  {!!count && <i className="badge">{count}</i>}
                </button>
              );
            })}
          </nav>
          {shown && (
            <aside className="card panel" aria-label={panelInfo[shown].title}>
              <div className="panel-h">
                <div className="spacer" style={{ minWidth: 0 }}>
                  <h2>{panelInfo[shown].title}</h2>
                  <span className="small faint">{panelInfo[shown].hint}</span>
                </div>
                <button className="btn ghost sm icon" onClick={() => setPanel(null)} title="Close (the preview gets the room)" aria-label="Close panel"><XIcon size={15} /></button>
              </div>
              <div className="panel-b">
                {shown === "videos" && (
                  <div className="filelist">
                    <p className="small faint" style={{ margin: 0, padding: "10px 14px" }}>Cut from this source and queued by the agent. The app makes and exports them one at a time; finished ones are also in the Library.</p>
                    {p.children?.map((c) => (
                      <a className="item" key={c.id} href={`#/projects/${c.id}`}>
                        <span className="spacer ellipsis">{c.title}</span>
                        <Status value={projectStatus({ ...c, running: false })} />
                      </a>
                    ))}
                  </div>
                )}
                {shown === "brief" && <BriefCard p={p} meta={meta} reload={reload} onDelete={remove} />}
                {shown === "sources" && <>
                  <MediaSection title="In the video" hint="voice, music, sound and pictures the video uses"
                    files={(p.comp_assets ?? []).filter((f) => f.kind !== "font" && f.kind !== "other")} />
                  <Sources p={p} reload={reload} />
                </>}
                {shown === "generate" && <>
                  <Generate p={p} meta={meta} />
                  <MediaSection title="Generated for this project" hint="by you or the agent, newest first" empty="Nothing generated yet."
                    files={p.generated ?? []} onDelete={(f) => act0(`/projects/${p.id}/files?path=${encodeURIComponent(f.path)}`, reload)} />
                </>}
                {shown === "posts" && <PostsCard p={p} reload={reload} onPublish={() => setPublishing(true)} />}
                {shown === "cost" && <CostCard p={p} />}
              </div>
            </aside>
          )}
          <div className="card preview">
            <div className="stage" ref={stage}>
              <button className="btn sm icon stage-full" onClick={() => document.fullscreenElement ? document.exitFullscreen() : stage.current?.requestFullscreen()}
                title="Full screen (F)" aria-label="Full screen"><ArrowsOutIcon size={14} /></button>
              {current === "comp" && p.composition ? (
                <hyperframes-player key={shownComp} src={shownComp ?? p.composition} controls />
              ) : current && current !== "comp" ? (
                <div className="frame">
                  {current.kind === "video" ? <video key={current.url} src={current.url} controls playsInline /> :
                   current.kind === "image" ? <img src={current.url} alt={current.name} /> :
                   <audio key={current.url} src={current.url} controls />}
                </div>
              ) : (
                <Empty icon={<FilmStripIcon size={30} />} title={p.running ? "The agent is building the first preview" : "Nothing to preview yet"}>
                  {p.running ? "It shows up here the moment it exists. Follow along in the chat." : "Start the agent and the live preview appears here."}
                </Empty>
              )}
              {stale && current === "comp" && (
                <button className="btn sm primary stage-pill" onClick={() => setShownComp(p.composition)}><ArrowsClockwiseIcon size={13} /> New version ready, refresh preview</button>
              )}
            </div>
            {p.render && <RenderBar r={p.render} />}
            {p.import && !p.import.done && (
              <JobBar title="Downloading the video" pct={p.import.pct} line={p.import.line} onCancel={() => api(`/projects/${id}/import`, { method: "DELETE" }).then(reload, fail)} />
            )}
            {p.transcript?.state === "running" && <JobBar title="Transcribing" pct={p.transcript.pct ?? 0} line={p.transcript.line} onCancel={() => skipTranscript(p.id, reload)} />}
            <Versions p={p} finals={finals} drafts={drafts} current={current} onPick={setView}
              onDeleteFinal={delFile("out")} onDeleteDraft={delFile("out/drafts")}
              onDraft={() => act("render", { quality: "draft" }, "Draft render started")} onSizes={setSizing} />
          </div>
          {templating && <SaveTemplate projectId={p.id} title={p.title} onClose={() => setTemplating(false)} />}
          {sizing && <SizesModal p={p} file={sizing} onClose={() => setSizing(undefined)} onStarted={reload}
            onAskAgent={p.composition && p.session_id ? (text) => act("messages", { text }).then(() => poll.current()) : undefined} />}
          {publishing && <PublishModal p={p} onClose={() => setPublishing(false)} onQueued={() => { setPublishing(false); setPanel("posts"); reload(); }} />}
        </div>

        {post ? <PostSide p={p} onPublish={() => setPublishing(true)} postable={postable} /> : <>
        {!chatFull && <ChatResizer box={ws} width={chatW} onWidth={setChatW} />}
        <Chat p={p} events={events} qualities={meta?.qualities ?? []} full={chatFull} onFull={() => setChatFull(!chatFull)} viewing={current} moment={moment}
          onSend={(text, quality) => act("messages", { text, quality }).then(() => poll.current())} onStop={() => act("stop")} />
        </>}
      </div>
    </div>
  );
}

/** A paused moment in the preview: seconds from the start, and whether it's the live composition (not a rendered file). */
type Moment = { t: number; live: boolean; screen: string[] };

/** 12.43 → "0:12.4" */
const stamp = (t: number) => `${clock(Math.floor(t))}.${Math.floor((t % 1) * 10)}`;

/** The text visible in the paused live preview, one entry per block (a caption, a title, a label), so the agent can
 *  find "this line" in the composition without reading it all. Same-origin iframes, sub-compositions included. */
function screenText(player: HTMLElement): string[] {
  const docs: Document[] = [];
  const add = (d?: Document | null) => {
    if (!d?.body) return;
    docs.push(d);
    d.querySelectorAll("iframe").forEach((f) => { try { add(f.contentDocument); } catch { /* another origin */ } });
  };
  try { add((player as any).iframeElement?.contentDocument); } catch { /* not loaded */ }
  const blocks = new Map<Element, string[]>();
  for (const d of docs) {
    const w = d.defaultView!;
    const walk = d.createTreeWalker(d.body, NodeFilter.SHOW_TEXT);
    for (let n = walk.nextNode(); n; n = walk.nextNode()) {
      const text = n.textContent?.replace(/\s+/g, " ").trim();
      const el = n.parentElement;
      if (!text || !el || el.closest("script, style, noscript, template")) continue;
      const r = el.getBoundingClientRect();
      if (r.width < 1 || r.height < 1 || r.right < 0 || r.bottom < 0 || r.left > w.innerWidth || r.top > w.innerHeight) continue;
      if (el.checkVisibility && !el.checkVisibility({ opacityProperty: true, visibilityProperty: true })) continue;
      // Word-by-word captions are a span per word: gather words under their nearest block.
      let block: Element = el;
      while (block.parentElement && block !== d.body && w.getComputedStyle(block).display.startsWith("inline")) block = block.parentElement;
      blocks.set(block, [...(blocks.get(block) ?? []), text]);
    }
  }
  return [...blocks.values()].map((ws) => ws.join(" ").slice(0, 140)).filter((t, i, all) => all.indexOf(t) === i).slice(0, 8);
}

// Events that only add to the chat; anything else (a reply, a render, an error) can change the project too.
const chatOnly = new Set(["text", "tool", "tool_error", "agent", "agent_done"]);

const defaultChatW = 420;

/** Drag the gap between the preview and the chat to size the chat; arrow keys work too, and a double-click resets it.
 *  While dragging, the width goes straight to the CSS variable (one layout per frame), and to React when you let go. */
function ChatResizer({ box, width, onWidth }: { box: RefObject<HTMLDivElement | null>; width: number; onWidth: (w: number) => void }) {
  // The preview keeps at least 360px (styles.css caps the column the same way when the window shrinks).
  const clamp = (w: number) => Math.round(Math.max(320, Math.min(w, (box.current?.clientWidth ?? 1400) - 380)));
  const down = (e: ReactPointerEvent<HTMLDivElement>) => {
    const ws = box.current;
    if (!ws || e.button !== 0) return;
    const handle = e.currentTarget, right = ws.getBoundingClientRect().right;
    handle.setPointerCapture(e.pointerId);
    document.body.classList.add("resizing");
    let w = width, frame = 0;
    const move = (ev: PointerEvent) => {
      w = clamp(right - ev.clientX - 10);
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => ws.style.setProperty("--chat-w", `${w}px`));
    };
    handle.addEventListener("pointermove", move);
    handle.addEventListener("lostpointercapture", () => {
      handle.removeEventListener("pointermove", move);
      cancelAnimationFrame(frame);
      ws.style.setProperty("--chat-w", `${w}px`);
      document.body.classList.remove("resizing");
      onWidth(w);
    }, { once: true });
  };
  return (
    <div className="chat-resize" role="separator" aria-orientation="vertical" aria-label="Chat width" aria-valuenow={width} tabIndex={0}
      title="Drag to resize the chat. Double-click to reset."
      onPointerDown={down} onDoubleClick={() => onWidth(defaultChatW)}
      onKeyDown={(e) => {
        if (e.key !== "ArrowLeft" && e.key !== "ArrowRight") return;
        e.preventDefault();
        onWidth(clamp(width + (e.key === "ArrowLeft" ? 32 : -32)));
      }} />
  );
}

/** The editor's own shape while the project loads, so opening one never shows a blank page. */
function EditorSkeleton({ chatW, chatFull }: { chatW: number; chatFull: boolean }) {
  const bar = (width: number | string, height: number, style?: CSSProperties) => <span className="sk" style={{ width, height, ...style }} />;
  return (
    <div className="page wide editor" aria-busy="true" aria-label="Opening the project">
      <div className="editor-head">
        <div className="editor-title">
          {bar(150, 12)}
          {bar("min(360px, 80%)", 22, { margin: "12px 0 9px" })}
          {bar(230, 12)}
        </div>
        <div className="row">{[92, 118, 86].map((w) => <span key={w}>{bar(w, 36)}</span>)}</div>
      </div>
      <div className={`workspace ${chatFull ? "chat-full" : ""}`} style={{ "--chat-w": `${chatW}px` } as CSSProperties}>
        <div className="stack editor-main" style={{ minWidth: 0, gap: 14 }}>
          <div className="card">
            <div className="stage"><div className="typing stage-wait"><i /><i /><i /> <span>Opening the project…</span></div></div>
            <div className="versions">{[0, 1, 2].map((i) => <span key={i}>{bar(112, 64)}</span>)}</div>
          </div>
          <div className="card"><div className="card-b stack" style={{ gap: 10 }}>{["35%", "90%", "70%"].map((w) => <span key={w}>{bar(w, 12)}</span>)}</div></div>
        </div>
        <div className="card chat">
          <div className="card-h"><SparkleIcon size={16} color="var(--primary)" /><h2 className="spacer">Editor agent</h2></div>
          <div className="feed">
            {bar("100%", 36)}
            {bar("90%", 12)}{bar("80%", 12)}{bar("55%", 12)}
            {bar("100%", 30)}
          </div>
        </div>
      </div>
    </div>
  );
}

function act0(path: string, then: () => void) {
  if (!confirm("Delete this file?")) return;
  api(path, { method: "DELETE" }).then(then, fail);
}

/** Every version of the video in one strip: the live composition, final exports, then drafts and generated media. */
function Versions({ p, finals, drafts, current, onPick, onDeleteFinal, onDeleteDraft, onDraft, onSizes }: {
  p: Project; finals: FileInfo[]; drafts: FileInfo[]; current?: FileInfo | "comp"; onPick: (v: FileInfo | "comp") => void;
  onDeleteFinal: (f: FileInfo) => void; onDeleteDraft: (f: FileInfo) => void; onDraft: () => void; onSizes: (f: FileInfo) => void;
}) {
  const item = (f: FileInfo, tag: string, onDelete: (f: FileInfo) => void) => (
    <div key={f.url} className={`version ${current !== "comp" && current?.url === f.url ? "on" : ""}`}>
      <button className="cover" onClick={() => onPick(f)} title={f.path}>
        <div className="vthumb">
          {f.kind === "image" ? <img src={f.url} alt="" loading="lazy" /> : f.kind === "video" ? <video src={f.url + "#t=0.5"} preload="metadata" muted /> : <KindIcon kind={f.kind} size={20} />}
          <span className={`vtag ${tag === "Final" ? "final" : ""}`}>{tag}{tag === "Final" && f.kind === "video" ? ` · ${shapeOf(f, p)}` : ""}</span>
        </div>
        <span className="ellipsis">{f.name}</span>
      </button>
      <span className="vacts">
        {tag === "Final" && f.kind === "video" && <button onClick={() => onSizes(f)} title="Make another size: 9:16 for Reels, TikTok and Shorts, 16:9 for Facebook and YouTube" aria-label={`Make another size of ${f.name}`}><ArrowsOutCardinalIcon size={13} /></button>}
        {canReveal && <button onClick={() => showInFolder(f.url)} title="Show in folder" aria-label={`Show ${f.name} in its folder`}><FolderOpenIcon size={13} /></button>}
        <a href={f.url} download={f.name} title="Download" aria-label={`Download ${f.name}`}><DownloadSimpleIcon size={13} /></a>
        <button className="del" onClick={() => onDelete(f)} title="Delete" aria-label={`Delete ${f.name}`}><TrashIcon size={13} /></button>
      </span>
    </div>
  );
  return (
    <div className="versions">
      {p.composition && (
        <div className={`version ${current === "comp" ? "on" : ""}`}>
          <button className="cover" onClick={() => onPick("comp")}>
            <div className="vthumb live"><SparkleIcon size={18} /><span className="vtag">Live</span></div>
            <span className="ellipsis">Live preview</span>
          </button>
        </div>
      )}
      {finals.map((f) => item(f, "Final", onDeleteFinal))}
      {drafts.map((f) => item(f, f.kind === "audio" ? "Audio" : f.name.startsWith("gen-") ? "AI" : "Draft", onDeleteDraft))}
      {p.composition && (
        <button className="version add" onClick={onDraft} disabled={p.running} title="Quick low-quality render to share or check timing">
          <div className="vthumb"><FilmStripIcon size={18} /></div><span>Draft render</span>
        </button>
      )}
    </div>
  );
}

function Sources({ p, reload }: { p: Project; reload: () => void }) {
  const [pct, setPct] = useState<number>();
  async function add(files: File[]) {
    if (!files.length) return;
    setPct(0);
    try { await upload(`/projects/${p.id}/inputs`, files, setPct); toast("Uploaded"); reload(); } catch (e) { fail(e); }
    setPct(undefined);
  }
  return (
    <div className="card">
      <div className="card-h"><h2>Sources</h2><span className="small faint">input/ · read-only for the agent</span></div>
      <div className="filelist">
        {p.inputs?.map((f) => (
          <div className="item" key={f.path}>
            <KindIcon kind={f.kind} size={16} />
            <a href={f.url} target="_blank" rel="noreferrer" className="ellipsis spacer">{f.path}</a>
            <span className="small faint nowrap">{f.seconds ? clock(f.seconds) + " · " : ""}{bytes(f.size)}</span>
            <button className="btn ghost sm icon" aria-label={`Delete ${f.name}`} onClick={() => act0(`/projects/${p.id}/files?path=input/${encodeURIComponent(f.path)}`, reload)}><TrashIcon size={14} /></button>
          </div>
        ))}
      </div>
      <div className="card-b stack" style={{ gap: 12 }}>
        {pct === undefined ? <DropZone compact onFiles={add} accept="video/*,audio/*,image/*,.srt,.vtt" label="Add footage, or subtitles (.srt) to skip transcribing" /> : <Progress pct={pct} />}
        <ImportLink p={p} reload={reload} />
        <TranscriptLine p={p} reload={reload} />
      </div>
    </div>
  );
}

function CostCard({ p }: { p: Project }) {
  const q = p.quote;
  return (
    <div className="card">
      <div className="card-h"><h2>Cost</h2><span className="small faint">AI usage; rendering on this computer is free</span></div>
      <div className="card-b">
        <div className="kv">
          {q.lines.length === 0 && <span className="muted small" style={{ gridColumn: "1 / -1" }}>No usage yet. Cost builds up from agent runs and AI media.</span>}
          {q.lines.map((l, i) => (
            <FragmentRow key={i} left={<>{l.label} <span className="faint small">{l.detail}</span></>} right={cash(l.cost)} />
          ))}
          <FragmentRow left={<strong>Total</strong>} right={<strong>{cash(q.cost)}</strong>} cls="total" />
        </div>
        <PlanNote style={{ marginTop: 14 }} />
      </div>
    </div>
  );
}

function FragmentRow({ left, right, cls = "" }: { left: ReactNode; right: ReactNode; cls?: string }) {
  return <><span className={cls}>{left}</span><span className={`${cls} right nowrap`}>{right}</span></>;
}

const genKinds = [
  { value: "image", label: "Image", Icon: ImageIcon, provider: "gemini", hint: "Thumbnail background: glowing equations on a dark violet gradient…" },
  { value: "video", label: "Video clip", Icon: FilmStripIcon, provider: "gemini", hint: "Slow push-in on a chalkboard full of equations, warm light…" },
  { value: "voice", label: "Voiceover", Icon: MicrophoneIcon, provider: "eleven", hint: "The exact script to speak, in the video's language…" },
  { value: "sfx", label: "Sound effect", Icon: SpeakerHighIcon, provider: "eleven", hint: "Soft whoosh transition, short and clean" },
  { value: "music", label: "Music", Icon: MusicNotesIcon, provider: "eleven", hint: "Upbeat lo-fi bed, no vocals, 90 bpm, builds at the end" },
] as const;

function Generate({ p, meta }: { p: Project; meta?: Meta }) {
  const [kind, setKind] = useState<(typeof genKinds)[number]["value"]>("image");
  const [prompt, setPrompt] = useState("");
  const [aspect, setAspect] = useState(p.aspect === "16:9" ? "16:9" : "9:16");
  const [seconds, setSeconds] = useState(kind === "music" ? p.seconds || 30 : 2);
  const k = genKinds.find((x) => x.value === kind)!;
  const ready = meta?.env[k.provider];
  const visual = kind === "image" || kind === "video";
  async function submit() {
    try {
      await api(`/projects/${p.id}/generate`, { body: { kind, prompt, aspect, seconds } });
      toast(`Generating ${k.label.toLowerCase()}; it lands in Drafts`);
      setPrompt("");
    } catch (e) { fail(e); }
  }
  return (
    <div className="card">
      <div className="card-h"><h2>Generate media</h2><span className="small faint">images, clips, voice and music · counted in this project's cost</span></div>
      <div className="card-b stack" style={{ gap: 12 }}>
        <div className="gen-kinds" role="radiogroup" aria-label="What to generate">
          {genKinds.map((g) => (
            <button key={g.value} role="radio" aria-checked={kind === g.value} className={kind === g.value ? "on" : ""}
              onClick={() => { setKind(g.value); setSeconds(g.value === "music" ? p.seconds || 30 : 2); }}>
              <g.Icon size={20} /><span>{g.label}</span>
            </button>
          ))}
        </div>
        <textarea className="textarea" rows={3} value={prompt} onChange={(e) => setPrompt(e.target.value)} placeholder={k.hint} />
        <div className="row wrap">
          {visual && <Seg value={aspect} options={["9:16", "16:9", ...(kind === "image" ? ["1:1", "4:5"] : [])].map((v) => ({ value: v, label: v }))} onChange={setAspect} />}
          {(kind === "sfx" || kind === "music") && (
            <label className="row small">Length <input className="input" type="number" min={kind === "music" ? 3 : 0.5} step={0.5} value={seconds} onChange={(e) => setSeconds(Number(e.target.value))} style={{ width: 90 }} /> s</label>
          )}
          {kind === "voice" && <span className="small muted">Voice for {languageName(p.language || "en")} from Settings</span>}
          <span className="spacer" />
          {!ready && <span className="small muted">Add a {k.provider === "gemini" ? "Gemini" : "ElevenLabs"} key in Settings → AI services to enable</span>}
          <button className="btn" disabled={!ready || !prompt.trim()} onClick={submit}><SparkleIcon size={15} /> Generate</button>
        </div>
      </div>
    </div>
  );
}

function BriefCard({ p, meta, reload, onDelete }: { p: Project; meta?: Meta; reload: () => void; onDelete: () => void }) {
  const [brief, setBrief] = useState(p.brief);
  const [focus, setFocus] = useState(p.focus);
  const [quality, setQuality] = useState(p.quality);
  const dirty = brief !== p.brief || quality !== p.quality || focus !== p.focus;
  const save = () => api(`/projects/${p.id}`, { method: "PUT", body: { brief, quality, focus } }).then(() => { toast("Saved"); reload(); }, fail);
  return (
    <div className="card">
      <div className="card-h"><h2>Brief</h2><span className="small faint">the agent reads it on the first turn</span></div>
      <div className="card-b stack" style={{ gap: 12 }}>
        <textarea className="textarea" rows={4} value={brief} onChange={(e) => setBrief(e.target.value)} placeholder="No brief." />
        <input className="input" value={focus} onChange={(e) => setFocus(e.target.value)} placeholder="Page or feature this is about (URL or name), optional" aria-label="Focus page or feature" />
        <div className="row wrap">
          <ModelPicker value={quality} qualities={meta?.qualities ?? []} onChange={setQuality} />
          <span className="spacer" />
          <button className="btn" disabled={!dirty} onClick={save}>Save</button>
        </div>
        <div className="row">
          <span className="small faint spacer">{meta?.qualities.find((q) => q.id === quality)?.note}</span>
          <button className="btn ghost danger sm" onClick={onDelete}><TrashIcon size={14} /> Delete project</button>
        </div>
      </div>
    </div>
  );
}

/** Beside a post's preview, where the agent's chat would be: what happens next, and Publish. */
function PostSide({ p, postable, onPublish }: { p: Project; postable: boolean; onPublish: () => void }) {
  const sent = p.posts?.length ?? 0;
  return (
    <div className="card chat post-side">
      <div className="card-h"><ShareNetworkIcon size={16} color="var(--primary)" /><h2 className="spacer">Post</h2></div>
      <div className="stack" style={{ padding: 16, gap: 12 }}>
        <p style={{ margin: 0 }}>{postable ? "Your file is ready. Pick the accounts, let the AI write a caption for each platform (it looks at the file), then post now or schedule it." : "The file is still uploading, or it isn't a video or photo."}</p>
        <button className="btn primary" disabled={!postable} onClick={onPublish}><ShareNetworkIcon size={15} /> {sent ? "Post it again" : "Publish"}</button>
        {sent > 0 && <p className="small muted" style={{ margin: 0 }}>{sent} post{sent === 1 ? "" : "s"} so far: see Posts for each one's status and numbers.</p>}
      </div>
    </div>
  );
}

// ---- agent chat

const quickReplies = ["Looks good, render the final", "Make the hook punchier", "Make it shorter", "Try a different hook moment", "Bigger, bolder captions"];

function Chat({ p, events, qualities, onSend, onStop, full, onFull, viewing, moment }: {
  p: Project; events: Parsed[]; qualities: Quality[]; onSend: (text: string, quality?: string) => Promise<void>; onStop: () => void;
  full: boolean; onFull: () => void; viewing?: FileInfo | "comp"; moment?: Moment;
}) {
  // Model for the next message: cheap Haiku for small fixes, Opus for heavy edits. Defaults to the project's quality.
  const [quality, setQuality] = useState(p.quality);
  const feed = useRef<HTMLDivElement>(null);
  const started = !!p.session_id;

  // Opening a project lands on the latest message; after that, follow new messages only if you're already at the bottom.
  const opened = useRef(false);
  useLayoutEffect(() => {
    const el = feed.current;
    if (!el || !events.length) return;
    if (!opened.current || el.scrollHeight - el.scrollTop - el.clientHeight < 240) el.scrollTop = el.scrollHeight;
    opened.current = true;
  }, [events.length, p.running]);

  // Images ride along as [image: chat/<file>] lines; the agent views them with Read, the feed shows thumbnails.
  const attach = async (list: File[]) => {
    const files = list.map((f, i) => new File([f], `${Date.now()}-${i}.${f.name.split(".").pop() || "png"}`, { type: f.type })); // pasted screenshots are all "image.png"
    try { await upload(`/projects/${p.id}/attachments`, files); return files.map((f) => "chat/" + f.name); } catch (e) { fail(e); return []; }
  };
  // A version picked in the strip under the preview goes with the message, like the open file in Claude Code.
  const viewPath = viewing && viewing !== "comp" ? decodeURIComponent(viewing.url.replace(/^\/files\/projects\/\d+\//, "")) : "";
  const live = useMemo(() => (p.running ? liveStatus(events) : undefined), [events, p.running]);

  return (
    <div className="card chat">
      <div className="card-h">
        <SparkleIcon size={16} color="var(--primary)" />
        <h2 className="spacer">Editor agent</h2>
        <span className="small faint">{p.running ? "working" : started ? "waiting for you" : "not started"}</span>
        <button className="btn ghost sm icon" onClick={onFull} aria-pressed={full}
          title={full ? "Show the preview again" : "Widen the chat to the whole page"} aria-label={full ? "Show the preview" : "Widen the chat"}>
          {full ? <ArrowsInLineHorizontalIcon size={15} /> : <ArrowsOutLineHorizontalIcon size={15} />}
        </button>
      </div>
      <div className="feed" ref={feed}>
        {!started && !p.running && (
          <Empty icon={<SparkleIcon size={28} />} title="Ready when you are">
            The agent reads the brief and footage, asks what it needs, builds a live preview, then exports.
            <button className="btn primary" onClick={() => onSend("")}><SparkleIcon size={15} /> Start agent</button>
          </Empty>
        )}
        <Feed events={events} running={p.running} pid={p.id} />
        {live && (
          <div className="typing"><i /><i /><i /> <span className="ellipsis">{p.render ? `Rendering · ${p.render.pct}% · ${p.render.stage}` :
            [live.agents && `${live.agents} subagent${live.agents === 1 ? "" : "s"} working`, live.step].filter(Boolean).join(" · ") || "Working…"}</span></div>
        )}
      </div>
      {started && (
        <Composer events={events} running={p.running} queue onSend={(t, q) => onSend(t, q)} onStop={onStop}
          qualities={qualities} quality={quality} onQuality={setQuality}
          placeholder={p.running ? "Queue your next message…" : "Ask for changes, or / for quick actions"}
          actions={quickReplies.map((label) => ({ label, send: true }))}
          attach={attach} fileUrl={(f) => `/files/projects/${p.id}/${f}`}
          context={[
            ...(viewPath ? [{ label: viewing && viewing !== "comp" ? viewing.name : viewPath, line: `[viewing: ${viewPath}]`, title: `${viewPath} goes with your message, so the agent knows which version you mean` }] : []),
            ...(moment ? [{
              label: stamp(moment.t), icon: <TimerIcon size={14} />,
              line: `[paused at ${stamp(moment.t)} ${moment.live ? "in the live preview" : "in the version I'm viewing"}${moment.screen.length ? `; on screen: ${moment.screen.map((t) => JSON.stringify(t)).join(" | ")}` : ""}]`,
              title: `Where you paused goes with your message${moment.screen.length ? `, with what's on screen:\n${moment.screen.join("\n")}` : ""}. Say "here" or "this line" and the agent goes straight to it.`,
            } satisfies Context] : []),
          ]} />
      )}
    </div>
  );
}

/** Inline-playable media list: audio gets a player, images and video a thumbnail. */
type MediaFile = FileInfo & { item?: string; cost?: number };

/** Media sorted into what a person thinks in: voiceover, music, sound effects, pictures and clips. Paths are the agent's
 *  own (comp/assets/audio/vo/hook.mp3), so the guess goes by folder and file name; anything unclear is "Other audio". */
const mediaGroups: { id: string; label: string; Icon: typeof SparkleIcon; match: (f: MediaFile) => boolean }[] = [
  { id: "vo", label: "Voiceover", Icon: MicrophoneIcon, match: (f) => f.kind === "audio" && /(^|\/)(vo|voice|voiceover|narration)(\/|[-_.])/i.test(f.path) },
  { id: "music", label: "Music", Icon: MusicNotesIcon, match: (f) => f.kind === "audio" && /music|bgm|bed/i.test(f.path) },
  { id: "sfx", label: "Sound effects", Icon: SpeakerHighIcon, match: (f) => f.kind === "audio" && /sfx|sound|whoosh|hit|click|swoosh/i.test(f.path) },
  { id: "audio", label: "Other audio", Icon: SpeakerHighIcon, match: (f) => f.kind === "audio" },
  { id: "image", label: "Images", Icon: ImageIcon, match: (f) => f.kind === "image" },
  { id: "video", label: "Video clips", Icon: FilmStripIcon, match: (f) => f.kind === "video" },
];

/** "audio/vo/s-review.mp3" → "S review": the file name a person would say. */
const mediaName = (path: string) => {
  const base = path.split("/").pop()!.replace(/\.[^.]+$/, "").replace(/[-_]+/g, " ").trim();
  return base.charAt(0).toUpperCase() + base.slice(1);
};

function MediaSection({ title, hint, files, empty, onDelete }: { title: string; hint: string; files: MediaFile[]; empty?: string; onDelete?: (f: MediaFile) => void }) {
  const [playing, setPlaying] = useState<string>();
  const [pos, setPos] = useState(0); // how far through the playing track, 0 to 1
  const player = useRef<HTMLAudioElement>(null);
  useEffect(() => () => player.current?.pause(), []);
  if (!files.length && !empty) return null;
  const used = new Set<MediaFile>();
  const groups = mediaGroups.map((g) => {
    const list = files.filter((f) => !used.has(f) && g.match(f));
    list.forEach((f) => used.add(f));
    return { ...g, list };
  }).filter((g) => g.list.length);
  // One player for the whole list, so starting one track stops the last. Clicking the bar starts from there.
  const play = (f: MediaFile, at?: number) => {
    const a = player.current!;
    if (playing === f.url && at === undefined) { a.pause(); setPlaying(undefined); return; }
    if (playing !== f.url) { a.src = f.url; setPos(0); }
    if (at !== undefined && a.duration) a.currentTime = at * a.duration;
    else if (at !== undefined) a.addEventListener("loadedmetadata", () => { a.currentTime = at * a.duration; }, { once: true });
    a.play().then(() => setPlaying(f.url), fail);
  };
  const meta = (f: MediaFile) => <span className="small faint nowrap">{f.seconds ? clock(f.seconds) : bytes(f.size)}{f.cost ? ` · ${cash(f.cost)}` : ""}</span>;
  // Only out/ files: anything the agent put in comp/assets may be in the video, and the server refuses it anyway.
  const del = (f: MediaFile) => onDelete && f.path.startsWith("out/") &&
    <button className="btn ghost sm icon" onClick={() => onDelete(f)} aria-label={`Delete ${mediaName(f.path)}`} title="Delete"><TrashIcon size={14} /></button>;
  return (
    <div className="media-sec">
      <div className="media-sec-h"><h3>{title}</h3><span className="small faint">{hint}</span></div>
      <audio ref={player} hidden onEnded={() => { setPlaying(undefined); setPos(0); }}
        onTimeUpdate={(e) => { const a = e.currentTarget; if (a.duration) setPos(a.currentTime / a.duration); }} />
      {!files.length ? <p className="small muted">{empty}</p> : groups.map((g) => (
        <section key={g.id} className="media-group">
          <h4><g.Icon size={16} /> {g.label} <span className="count">{g.list.length}</span></h4>
          {g.list[0].kind === "audio" ? (
            <div className="tracks">
              {g.list.map((f) => {
                const on = playing === f.url;
                return (
                  <div key={f.url} className={`track ${on ? "on" : ""}`}>
                    <button className="play" onClick={() => play(f)} aria-label={`${on ? "Pause" : "Play"} ${mediaName(f.path)}`}>
                      {on ? <PauseIcon size={15} weight="fill" /> : <PlayIcon size={15} weight="fill" />}
                    </button>
                    <div className="track-main">
                      <div className="row" style={{ gap: 8 }}><span className="spacer ellipsis track-name" title={f.path}>{mediaName(f.path)}</span>{meta(f)}</div>
                      <div className="track-bar" title="Click to play from here"
                        onClick={(e) => { const r = e.currentTarget.getBoundingClientRect(); play(f, (e.clientX - r.left) / r.width); }}>
                        <i style={{ width: `${on ? pos * 100 : 0}%` }} />
                      </div>
                      {f.item && <div className="small faint ellipsis" title={f.item}>{f.item}</div>}
                    </div>
                    <span className="track-acts">
                      <a className="btn ghost sm icon" href={f.url} download aria-label={`Download ${mediaName(f.path)}`} title="Download"><DownloadSimpleIcon size={14} /></a>
                      {del(f)}
                    </span>
                  </div>
                );
              })}
            </div>
          ) : (
            <div className="media-grid">
              {g.list.map((f) => (
                <div key={f.url} className="media-item">
                  {f.kind === "image" ? <a href={f.url} target="_blank" rel="noreferrer"><img src={f.url} alt={mediaName(f.path)} loading="lazy" /></a> : <video src={f.url} controls preload="metadata" />}
                  <div className="small row" style={{ gap: 6 }}>
                    <span className="ellipsis spacer" title={f.path}>{mediaName(f.path)}</span>
                    {meta(f)}
                    {del(f)}
                  </div>
                  {f.item && <div className="small faint ellipsis" title={f.item}>{f.item}</div>}
                </div>
              ))}
            </div>
          )}
        </section>
      ))}
    </div>
  );
}

/** Live render progress from hyperframes' own @hf-progress reports, with elapsed time and a rough ETA. */
/** A download or transcription in progress. Both can be stopped: nobody should have to sit through a slow one. */
function JobBar({ title, pct, line, onCancel }: { title: string; pct: number; line?: string; onCancel: () => void }) {
  return (
    <div className="render-bar" role="progressbar" aria-valuenow={pct} aria-valuemin={0} aria-valuemax={100} aria-label={title}>
      <div className="row small">
        <strong>{title}</strong>
        <span className="muted ellipsis spacer">{line}</span>
        <span className="nowrap">{Math.round(pct)}%</span>
        <button className="btn ghost sm" onClick={onCancel}>Cancel</button>
      </div>
      <div className="progress"><div style={{ width: `${pct}%` }} /></div>
    </div>
  );
}

const skipTranscript = (id: number, then: () => void) =>
  confirm("Stop transcribing? The agent works from the footage and the brief instead. You can add a subtitle file (.srt) any time.") &&
  api(`/projects/${id}/transcript`, { body: { action: "skip" } }).then(then, fail);

/** The transcript, where it came from, and switching it off or on. The agent's `37studio transcribe` reads the same state. */
function TranscriptLine({ p, reload }: { p: Project; reload: () => void }) {
  const t = p.transcript;
  const resume = () => api(`/projects/${p.id}/transcript`, { body: { action: "resume" } }).then(() => { toast("Transcription is on again; ask the agent to transcribe"); reload(); }, fail);
  const from = t?.from?.startsWith("subtitles: ") ? `from ${t.from.slice(11)}` : t?.from ? `transcribed with ${t.from}` : "ready";
  return (
    <div className="row wrap small">
      <span className="muted spacer">
        <strong>Transcript:</strong>{" "}
        {!t ? "not made yet. The agent transcribes speech when it starts, which takes about as long as the video. Add subtitles (.srt) to skip that."
          : t.state === "running" ? `transcribing, ${t.pct ?? 0}%. ${t.line ?? ""}`
          : t.state === "done" ? from
          : t.state === "skipped" ? "switched off. The agent works without one, or from subtitles you add."
          : `failed. ${t.line ?? ""}`}
      </span>
      {(!t || t.state === "failed") && <button className="btn ghost sm" onClick={() => api(`/projects/${p.id}/transcript`, { body: { action: "skip" } }).then(reload, fail)}>Don't transcribe</button>}
      {t?.state === "running" && <button className="btn ghost sm" onClick={() => skipTranscript(p.id, reload)}>Stop</button>}
      {t?.state === "skipped" && <button className="btn ghost sm" onClick={resume}>Turn back on</button>}
    </div>
  );
}

/** YouTube or Google Drive link → input/, downloaded on this computer (import.go). */
function ImportLink({ p, reload }: { p: Project; reload: () => void }) {
  const [url, setUrl] = useState("");
  const busy = !!p.import && !p.import.done;
  const start = () => api(`/projects/${p.id}/import`, { body: { url } }).then(() => { setUrl(""); reload(); }, fail);
  return (
    <div className="stack" style={{ gap: 6 }}>
      <div className="row">
        <input className="input spacer" value={url} disabled={busy} onChange={(e) => setUrl(e.target.value)} onKeyDown={(e) => e.key === "Enter" && url.trim() && start()}
          placeholder="Or paste a YouTube or Google Drive link" aria-label="Video link" />
        <button className="btn" disabled={busy || !url.trim()} onClick={start}><LinkIcon size={15} /> Import</button>
      </div>
      {p.import?.error && <p className="small" style={{ margin: 0, color: "var(--danger)" }}>{p.import.error}</p>}
    </div>
  );
}

function RenderBar({ r }: { r: NonNullable<Project["render"]> }) {
  const elapsed = (Date.now() - new Date(r.started).getTime()) / 1000;
  const eta = r.pct >= 15 && r.pct < 100 ? (elapsed * (100 - r.pct)) / r.pct : 0;
  return (
    <div className="render-bar" role="progressbar" aria-valuenow={r.pct} aria-valuemin={0} aria-valuemax={100} aria-label="Render progress">
      <div className="row small">
        <strong>Rendering {r.out.replace(/^out\/(drafts\/)?/, "")}</strong>
        <span className="muted ellipsis spacer">{r.stage}</span>
        <span className="nowrap">{r.pct}% · {clock(elapsed)}{eta ? ` · ~${clock(eta)} left` : ""}</span>
      </div>
      <div className="progress"><div style={{ width: `${r.pct}%` }} /></div>
    </div>
  );
}
