import { ReactNode, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { ArrowUpIcon, BookmarkSimpleIcon, CheckCircleIcon, LinkIcon, ShareNetworkIcon, CaretRightIcon, FilmSlateIcon, DownloadSimpleIcon, FileTextIcon, FilmStripIcon, GlobeIcon, ListChecksIcon, PencilLineIcon, ArrowsClockwiseIcon, ArrowsOutIcon, MicrophoneIcon, MicrophoneSlashIcon, PaperclipIcon, MagnifyingGlassIcon, SlidersHorizontalIcon, SparkleIcon, SquareIcon, TerminalWindowIcon, TrashIcon, MagicWandIcon, WrenchIcon, XIcon } from "@phosphor-icons/react";
import { Quality } from "../api";
import { api, bytes, ChatEvent, clock, fail, FileInfo, go, languageName, Meta, Project, toast, upload, cash, useApi } from "../api";
import { DropZone, Empty, FileTile, KindIcon, Progress, Seg, Status, projectStatus } from "../ui";
import { presetIcon } from "./Projects";
import { PostsCard, PublishModal } from "./Social";
import { SaveTemplate } from "./Library";
import { approve, skip } from "./Autopilot";

export default function ProjectPage({ id }: { id: number }) {
  const { data: p, error, reload } = useApi<Project>(`/projects/${id}`);
  const { data: meta } = useApi<Meta>("/meta");
  const [events, setEvents] = useState<ChatEvent[]>([]);
  const [view, setView] = useState<FileInfo | "comp">();
  const [tab, setTab] = useState("brief");
  const [publishing, setPublishing] = useState(false);
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

  // ponytail: polling, not SSE. 1.5s is plenty for a chat that waits on minutes-long agent turns.
  useEffect(() => {
    let last = 0, alive = true, timer = 0;
    const tick = async () => {
      try {
        const evs = await api<ChatEvent[]>(`/projects/${id}/events?after=${last}`);
        if (evs.length) {
          last = evs[evs.length - 1].id;
          setEvents((e) => [...e, ...evs]);
          if (evs.some((e) => e.kind !== "text" && e.kind !== "tool")) reload();
        }
      } catch { /* server restarting; keep polling */ }
      if (alive) timer = window.setTimeout(tick, 1500);
    };
    tick();
    return () => { alive = false; clearTimeout(timer); };
  }, [id, reload]);

  // A transcription carries on after the agent's turn, and a download runs before it starts: poll for those too.
  const busy = !!p?.running || p?.transcript?.state === "running" || (!!p?.import && !p.import.done);
  useEffect(() => {
    if (!busy) return;
    const t = setInterval(reload, p?.render ? 1500 : 4000); // faster while a render reports progress
    return () => clearInterval(t);
  }, [busy, !!p?.render, reload]);

  useEffect(() => { if (p?.composition && !shownComp) setShownComp(p.composition); }, [p?.composition]);
  const hasVideos = !!p?.children?.length;
  useEffect(() => { if (hasVideos) setTab((t) => (t === "brief" ? "videos" : t)); }, [hasVideos]);

  if (error) return <div className="page"><Empty icon={<FilmSlateIcon size={28} />} title="Project not found"><a className="btn" href="#/projects">Back to projects</a></Empty></div>;
  if (!p) return null;

  const media = (f: FileInfo) => f.kind === "video" || f.kind === "image" || f.kind === "audio";
  const finals = (p.outputs ?? []).filter(media);
  const drafts = (p.drafts ?? []).filter(media);
  const current: FileInfo | "comp" | undefined = view ?? (p.composition ? "comp" : finals[0] ?? drafts[0]);
  const stale = !!p.composition && !!shownComp && p.composition !== shownComp;
  const Icon = presetIcon[p.kind] ?? MagicWandIcon;
  const preset = meta?.presets.find((x) => x.id === p.kind);
  const act = (path: string, body: object = {}, msg?: string) =>
    api(`/projects/${id}/${path}`, { body }).then(() => { if (msg) toast(msg); reload(); }, fail);

  // Open the window before the await so popup blockers treat it as a direct click.
  async function openTimeline() {
    const w = window.open("", "_blank");
    try {
      const r = await api<{ port: number; project: string }>(`/projects/${id}/studio`, { body: {} });
      const url = `${location.protocol}//${location.hostname}:${r.port}/#project/${r.project}`;
      if (w) w.location.href = url; else location.href = url;
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
            <Icon size={14} /> {preset?.label ?? p.kind} · {p.language ? languageName(p.language) + " · " : ""}{p.aspect}{p.seconds ? ` · ~${p.seconds}s` : ""} · {cash(p.quote.cost)}
          </p>
        </div>
        <div className="row wrap" style={{ justifyContent: "flex-end" }}>
          {!p.running && p.status !== "delivered" && finals.length > 0 && (
            <button className="btn" onClick={() => api(`/projects/${id}`, { method: "PUT", body: { status: "delivered" } }).then(reload, fail)}>
              <CheckCircleIcon size={15} /> Mark delivered
            </button>
          )}
          <span className="row" style={{ gap: 2 }}>
            <button className="btn" disabled={!p.composition} onClick={openTimeline} title="Drag, trim and split clips, change text and audio by hand">
              <SlidersHorizontalIcon size={15} /> {p.studio_port ? "Open timeline" : "Edit timeline"}
            </button>
            {p.studio_port && (
              <button className="btn ghost icon" title="Close the timeline editor" aria-label="Close the timeline editor"
                onClick={() => api(`/projects/${id}/studio`, { method: "DELETE" }).then(reload, fail)}><XIcon size={15} /></button>
            )}
          </span>
          <button className="btn" disabled={!p.composition} onClick={() => setTemplating(true)} title="Start later videos from this one's look">
            <BookmarkSimpleIcon size={15} /> Save as template
          </button>
          <button className="btn" disabled={!finals.some((f) => f.kind === "video")} onClick={() => setPublishing(true)}
            title={finals.some((f) => f.kind === "video") ? "Post to the company's Facebook, Instagram, TikTok and YouTube" : "Export a final video first"}>
            <ShareNetworkIcon size={15} /> Publish
          </button>
          <button className="btn primary" disabled={!p.composition || p.running} onClick={() => act("render", { quality: "high" }, "Export started; it appears in Versions")}
            title={p.running ? "Wait for the agent to finish" : "Render the final video"}>
            <DownloadSimpleIcon size={15} /> Export
          </button>
        </div>
      </div>

      {p.autopilot_stage === "approve" && (
        <div className="approve-bar">
          <span className="spacer">
            <strong>Autopilot made this video.</strong> Watch it, check the captions in the Posts tab, then approve to post to {p.posts?.filter((x) => x.status === "approval").length ?? 0} account(s).
          </span>
          <button className="btn" onClick={() => setTab("posts")}>Check captions</button>
          <button className="btn ghost" onClick={() => skip(p.id, reload)}>Skip</button>
          <button className="btn primary" onClick={() => approve(p.id, reload)}>Approve</button>
        </div>
      )}

      <div className="workspace">
        <div className="stack editor-main" style={{ minWidth: 0, gap: 14 }}>
          <div className="card">
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
              onDraft={() => act("render", { quality: "draft" }, "Draft render started")} />
          </div>

          <div className="card tabbed">
            <div className="tabs" style={{ padding: "0 8px" }}>
              {[...(p.children?.length ? [["videos", `Videos (${p.children.length})`]] : []), ["brief", "Brief"], ["sources", "Media"], ["generate", `Generate${p.generated?.length ? ` (${p.generated.length})` : ""}`], ["posts", `Posts${p.posts?.length ? ` (${p.posts.length})` : ""}`], ["cost", `Cost · ${cash(p.quote.cost)}`]].map(([k, l]) => (
                <button key={k} className={tab === k ? "on" : ""} onClick={() => setTab(k)}>{l}</button>
              ))}
            </div>
            {tab === "videos" && (
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
            {tab === "brief" && <BriefCard p={p} meta={meta} reload={reload} onDelete={remove} />}
            {tab === "sources" && <>
              <Sources p={p} reload={reload} />
              <MediaSection title="In the video" hint="comp/assets: what the composition uses, including AI voice, music and sound"
                files={(p.comp_assets ?? []).filter((f) => f.kind !== "font" && f.kind !== "other")} />
            </>}
            {tab === "generate" && <>
              <Generate p={p} meta={meta} />
              <MediaSection title="Generated for this project" hint="by you or the agent, newest first" empty="Nothing generated yet."
                files={p.generated ?? []} />
            </>}
            {tab === "posts" && <PostsCard p={p} reload={reload} onPublish={() => setPublishing(true)} />}
            {tab === "cost" && <CostCard p={p} />}
          </div>
          {templating && <SaveTemplate projectId={p.id} title={p.title} onClose={() => setTemplating(false)} />}
          {publishing && <PublishModal p={p} onClose={() => setPublishing(false)} onQueued={() => { setPublishing(false); setTab("posts"); reload(); }} />}
        </div>

        <Chat p={p} events={events} qualities={meta?.qualities ?? []} onSend={(text, quality) => act("messages", { text, quality })} onStop={() => act("stop")} />
      </div>
    </div>
  );
}

function act0(path: string, then: () => void) {
  if (!confirm("Delete this file?")) return;
  api(path, { method: "DELETE" }).then(then, fail);
}

/** Every version of the video in one strip: the live composition, final exports, then drafts and generated media. */
function Versions({ p, finals, drafts, current, onPick, onDeleteFinal, onDeleteDraft, onDraft }: {
  p: Project; finals: FileInfo[]; drafts: FileInfo[]; current?: FileInfo | "comp"; onPick: (v: FileInfo | "comp") => void;
  onDeleteFinal: (f: FileInfo) => void; onDeleteDraft: (f: FileInfo) => void; onDraft: () => void;
}) {
  const item = (f: FileInfo, tag: string, onDelete: (f: FileInfo) => void) => (
    <div key={f.url} className={`version ${current !== "comp" && current?.url === f.url ? "on" : ""}`}>
      <button className="cover" onClick={() => onPick(f)} title={f.path}>
        <div className="vthumb">
          {f.kind === "image" ? <img src={f.url} alt="" loading="lazy" /> : f.kind === "video" ? <video src={f.url + "#t=0.5"} preload="metadata" muted /> : <KindIcon kind={f.kind} size={20} />}
          <span className={`vtag ${tag === "Final" ? "final" : ""}`}>{tag}</span>
        </div>
        <span className="ellipsis">{f.name}</span>
      </button>
      <span className="vacts">
        <a href={f.url} download={f.name} title="Download" aria-label={`Download ${f.name}`}><DownloadSimpleIcon size={13} /></a>
        <button onClick={() => onDelete(f)} title="Delete" aria-label={`Delete ${f.name}`}><TrashIcon size={13} /></button>
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
      </div>
    </div>
  );
}

function FragmentRow({ left, right, cls = "" }: { left: ReactNode; right: ReactNode; cls?: string }) {
  return <><span className={cls}>{left}</span><span className={`${cls} right nowrap`}>{right}</span></>;
}

const genKinds = [
  { value: "image", label: "Image", provider: "gemini", hint: "Thumbnail background: glowing equations on a dark violet gradient…" },
  { value: "video", label: "Video clip", provider: "gemini", hint: "Slow push-in on a chalkboard full of equations, warm light…" },
  { value: "voice", label: "Voiceover", provider: "eleven", hint: "The exact script to speak, in the video's language…" },
  { value: "sfx", label: "Sound effect", provider: "eleven", hint: "Soft whoosh transition, short and clean" },
  { value: "music", label: "Music", provider: "eleven", hint: "Upbeat lo-fi bed, no vocals, 90 bpm, builds at the end" },
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
      <div className="card-h"><h2>Generate media</h2><span className="small faint">Gemini & ElevenLabs · counted in this project's cost</span></div>
      <div className="card-b stack" style={{ gap: 12 }}>
        <Seg value={kind} options={genKinds.map(({ value, label }) => ({ value, label }))} onChange={(v) => { setKind(v); setSeconds(v === "music" ? p.seconds || 30 : 2); }} />
        <textarea className="textarea" rows={3} value={prompt} onChange={(e) => setPrompt(e.target.value)} placeholder={k.hint} />
        <div className="row wrap">
          {visual && <Seg value={aspect} options={["9:16", "16:9", ...(kind === "image" ? ["1:1", "4:5"] : [])].map((v) => ({ value: v, label: v }))} onChange={setAspect} />}
          {(kind === "sfx" || kind === "music") && (
            <label className="row small">Length <input className="input" type="number" min={kind === "music" ? 3 : 0.5} step={0.5} value={seconds} onChange={(e) => setSeconds(Number(e.target.value))} style={{ width: 90 }} /> s</label>
          )}
          {kind === "voice" && <span className="small muted">Voice for {languageName(p.language || "en")} from Settings</span>}
          <span className="spacer" />
          {!ready && <span className="small muted">Set {k.provider === "gemini" ? "GEMINI_API_KEY" : "ELEVENLABS_API_KEY"} to enable</span>}
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
          <Seg value={quality} options={(meta?.qualities ?? []).map((q) => ({ value: q.id, label: q.label }))} onChange={setQuality} />
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

// ---- agent chat

const quickReplies = ["Looks good, render the final", "Make the hook punchier", "Make it shorter", "Try a different hook moment", "Bigger, bolder captions"];

const toolIcon = (name: string) =>
  name === "Bash" ? TerminalWindowIcon :
  ["Edit", "Write", "MultiEdit", "NotebookEdit"].includes(name) ? PencilLineIcon :
  name === "Read" ? FileTextIcon :
  ["Glob", "Grep"].includes(name) ? MagnifyingGlassIcon :
  name === "Skill" ? SparkleIcon :
  name.startsWith("Web") ? GlobeIcon :
  name === "TodoWrite" ? ListChecksIcon : WrenchIcon;

type Parsed = { id: number; kind: string; b: any };

// Absolute project paths are noise for an editor: show "comp/index.html", not "/home/…/projects/1/comp/index.html".
const tidy = (s = "") => s.replace(/\/\S*?\/(projects|tenants)\/\d+\//g, "");

function Chat({ p, events, qualities, onSend, onStop }: {
  p: Project; events: ChatEvent[]; qualities: Quality[]; onSend: (text: string, quality?: string) => Promise<void>; onStop: () => void;
}) {
  // Model for the next message: cheap Haiku for small fixes, Opus for heavy edits. Defaults to the project's quality.
  const [quality, setQuality] = useState(p.quality);
  const [text, setText] = useState("");
  const [imgs, setImgs] = useState<string[]>([]); // attached screenshots, as project paths (chat/…)
  const feed = useRef<HTMLDivElement>(null);
  const started = !!p.session_id;

  // Collapse runs of tool calls into one expandable group so the conversation stays readable.
  const items = useMemo(() => {
    const out: (Parsed | Parsed[])[] = [];
    for (const e of events) {
      const ev = { id: e.id, kind: e.kind, b: JSON.parse(e.body) };
      const last = out[out.length - 1];
      if (ev.kind === "tool" || ev.kind === "tool_error") Array.isArray(last) ? last.push(ev) : out.push([ev]);
      else out.push(ev);
    }
    return out;
  }, [events]);

  // Opening a project lands on the latest message; after that, follow new messages only if you're already at the bottom.
  const opened = useRef(false);
  useLayoutEffect(() => {
    const el = feed.current;
    if (!el || !items.length) return;
    if (!opened.current || el.scrollHeight - el.scrollTop - el.clientHeight < 240) el.scrollTop = el.scrollHeight;
    opened.current = true;
  }, [items.length, p.running]);

  // While the agent works, sending queues the message server-side; it goes out when the turn ends.
  // Images ride along as [image: chat/<file>] lines; the agent views them with Read, the feed shows thumbnails.
  const send = async (t = text) => {
    if (!t.trim() && !imgs.length) return;
    setText("");
    setImgs([]);
    await onSend([t.trim(), ...imgs.map((i) => `[image: ${i}]`)].filter(Boolean).join("\n"), quality);
  };
  const attach = async (list: FileList | File[]) => {
    const files = [...list].filter((f) => f.type.startsWith("image/"))
      .map((f, i) => new File([f], `${Date.now()}-${i}.${f.name.split(".").pop() || "png"}`, { type: f.type })); // pasted screenshots are all "image.png"
    if (!files.length) return;
    try { await upload(`/projects/${p.id}/attachments`, files); setImgs((v) => [...v, ...files.map((f) => "chat/" + f.name)]); } catch (e) { fail(e); }
  };
  // The current turn's latest step: stop at the last user message or finished turn, so an old step never shows.
  const liveStep = (() => {
    for (let i = events.length - 1; i >= 0; i--) {
      const e = events[i];
      if (e.kind === "tool") return JSON.parse(e.body).summary as string;
      if (e.kind === "user" || e.kind === "result") return undefined;
    }
  })();
  const mic = useDictation(p.language || "en", (said) => setText((t) => (t ? t.trimEnd() + " " : "") + said));

  return (
    <div className="card chat">
      <div className="card-h">
        <SparkleIcon size={16} color="var(--primary)" />
        <h2 className="spacer">Editor agent</h2>
        <span className="small faint">{p.running ? "working" : started ? "waiting for you" : "not started"}</span>
      </div>
      <div className="feed" ref={feed}>
        {!started && !p.running && (
          <Empty icon={<SparkleIcon size={28} />} title="Ready when you are">
            The agent reads the brief and footage, asks what it needs, builds a live preview, then exports.
            <button className="btn primary" onClick={() => onSend("")}><SparkleIcon size={15} /> Start agent</button>
          </Empty>
        )}
        {items.map((it, i) =>
          Array.isArray(it) ? <ToolGroup key={it[0].id} tools={it} open={p.running && i === items.length - 1} /> : <EventView key={it.id} e={it} pid={p.id} />
        )}
        {p.running && (
          <div className="typing"><i /><i /><i /> <span className="ellipsis">{p.render ? `Rendering · ${p.render.pct}% · ${p.render.stage}` : liveStep ? tidy(liveStep) : "Working…"}</span></div>
        )}
      </div>
      {started && (
        <div className="composer">
          {!p.running && (
            <div className="chips">
              {quickReplies.map((q) => <button key={q} className="chip" onClick={() => send(q)}>{q}</button>)}
            </div>
          )}
          <div className={`composer-box ${mic.listening ? "listening" : ""}`}
            onDragOver={(e) => e.preventDefault()} onDrop={(e) => { e.preventDefault(); attach(e.dataTransfer.files); }}>
            {imgs.length > 0 && (
              <div className="attachments">
                {imgs.map((i) => (
                  <span key={i} className="attachment">
                    <img src={`/files/projects/${p.id}/${i}`} alt="" />
                    <button onClick={() => setImgs((v) => v.filter((x) => x !== i))} aria-label="Remove image"><XIcon size={12} /></button>
                  </span>
                ))}
              </div>
            )}
            <textarea
              value={mic.listening ? text + (mic.interim ? (text ? " " : "") + mic.interim : "") : text}
              rows={2}
              placeholder={mic.listening ? "Listening…" : p.running ? "Queue your next message…" : "Ask for changes…"}
              aria-label="Message the agent"
              onChange={(e) => setText(e.target.value)}
              onPaste={(e) => { if (e.clipboardData.files.length) { e.preventDefault(); attach(e.clipboardData.files); } }}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); mic.stop(); send(); }
                if (e.key === "Escape" && p.running) onStop(); // like Claude Code: Esc interrupts
              }}
            />
            <div className="composer-bar">
              <label className="cbtn" title="Attach screenshots (or paste / drop them)" aria-label="Attach images">
                <PaperclipIcon size={15} />
                <input type="file" accept="image/*" multiple hidden onChange={(e) => { attach(e.target.files!); e.target.value = ""; }} />
              </label>
              {mic.supported && (
                <button className={`cbtn ${mic.listening ? "rec" : ""}`} onClick={mic.listening ? mic.stop : mic.start}
                  title={mic.listening ? "Stop dictation" : "Dictate"} aria-label={mic.listening ? "Stop dictation" : "Dictate"}>
                  {mic.listening ? <MicrophoneSlashIcon size={15} /> : <MicrophoneIcon size={15} />}
                </button>
              )}
              {mic.supported && (
                <select className="cselect" value={mic.lang} onChange={(e) => mic.setLang(e.target.value)} aria-label="Dictation language" title="Dictation language">
                  <option value="bn-BD">বাংলা</option>
                  <option value="en-US">EN</option>
                </select>
              )}
              <select className="cselect" value={quality} onChange={(e) => setQuality(e.target.value)} aria-label="Model for this message"
                title={qualities.find((q) => q.id === quality)?.note}>
                <optgroup label="Claude Code">
                  {qualities.filter((q) => !q.harness).map((q) => <option key={q.id} value={q.id}>{modelName(q.model)}{q.effort === "xhigh" ? " max" : ""}</option>)}
                </optgroup>
                <optgroup label="OpenCode">
                  {qualities.filter((q) => q.harness === "opencode").map((q) => <option key={q.id} value={q.id}>{q.label}</option>)}
                </optgroup>
              </select>
              <span className="spacer" />
              {p.running && (
                <button className="cbtn stop" onClick={onStop} title="Stop the agent (Esc)" aria-label="Stop the agent"><SquareIcon size={13} weight="fill" /></button>
              )}
              <button className="cbtn send" disabled={!text.trim() && !imgs.length} onClick={() => { mic.stop(); send(); }}
                title={p.running ? "Queue: sends when this turn ends" : "Send (Enter)"} aria-label={p.running ? "Queue message" : "Send"}>
                <ArrowUpIcon size={16} />
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function ToolGroup({ tools, open }: { tools: Parsed[]; open: boolean }) {
  const last = [...tools].reverse().find((t) => t.kind === "tool") ?? tools[tools.length - 1];
  const steps = tools.filter((t) => t.kind === "tool").length;
  return (
    <details open={open || undefined}>
      <summary className="msg-tool" style={{ cursor: "pointer", listStyle: "none" }}>
        <WrenchIcon size={13} />
        <b>{steps} step{steps === 1 ? "" : "s"}</b>
        <span className="ellipsis">{tidy(last.b.summary ?? last.b.text)}</span>
      </summary>
      <div className="stack" style={{ gap: 4, marginTop: 6 }}>
        {tools.map((t) => {
          if (t.kind === "tool_error") return <div key={t.id} className="msg-tool err" title="The agent saw this and retried"><span className="mono">{tidy(t.b.text)}</span></div>;
          const I = toolIcon(t.b.name);
          return (
            <div key={t.id} className="msg-tool"><I size={13} /><b>{t.b.name}</b><span className="mono ellipsis" title={t.b.summary}>{tidy(t.b.summary)}</span></div>
          );
        })}
      </div>
    </details>
  );
}

const imageLine = /^\[image: (chat\/[^\]]+)\]$/;

function EventView({ e, pid }: { e: Parsed; pid: number }) {
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
          <img src={e.b.url} alt={`Frame the agent checked: ${e.b.path}`} />
          <span><MagnifyingGlassIcon size={12} /> Checked {e.b.path}</span>
        </a>
      );
    case "error": return <div className="msg-error">{e.b.text}</div>;
    default: return <div className="msg-info">{e.b.text}</div>;
  }
}

/** Just enough markdown for agent replies: paragraphs, lists, headings, **bold**, `code`. */
function Md({ text }: { text: string }) {
  const blocks: ReactNode[] = [];
  let para: string[] = [];
  let items: string[] = [], ordered = false;
  const flushPara = () => { if (para.length) blocks.push(<p key={blocks.length}>{inline(para.join(" "))}</p>); para = []; };
  const flushList = () => {
    if (!items.length) return;
    const lis = items.map((t, i) => <li key={i}>{inline(t)}</li>);
    blocks.push(ordered ? <ol key={blocks.length}>{lis}</ol> : <ul key={blocks.length}>{lis}</ul>);
    items = [];
  };
  for (const line of text.split("\n")) {
    const li = line.match(/^\s*([-*•]|\d+[.)])\s+(.*)/);
    const h = line.match(/^#{1,6}\s+(.*)/);
    if (li) {
      flushPara();
      const isOrdered = /\d/.test(li[1]);
      if (items.length && isOrdered !== ordered) flushList();
      ordered = isOrdered;
      items.push(li[2]);
    } else if (h) {
      flushPara(); flushList();
      blocks.push(<h4 key={blocks.length}>{inline(h[1])}</h4>);
    } else if (!line.trim()) {
      flushPara(); flushList();
    } else {
      flushList();
      para.push(line.trim());
    }
  }
  flushPara(); flushList();
  return <div className="msg-agent">{blocks}</div>;
}

function inline(s: string): ReactNode[] {
  return s.split(/(\*\*[^*]+\*\*|`[^`]+`)/g).map((part, i) =>
    part.length > 4 && part.startsWith("**") && part.endsWith("**") ? <strong key={i}>{part.slice(2, -2)}</strong> :
    part.length > 2 && part.startsWith("`") && part.endsWith("`") ? <code key={i}>{part.slice(1, -1)}</code> : part
  );
}

/**
 * Dictation with the browser's built-in speech recognition (Chrome, Edge, Safari). No key, no cost.
 * ponytail: Chrome sends the audio to Google's recognizer; switch to ElevenLabs Scribe if that matters.
 */
function useDictation(videoLang: string, onFinal: (text: string) => void) {
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

/** "claude-opus-5-5" -> "Opus 5.5" */
const modelName = (id: string) => id.replace(/^.*\//, "").replace(/^claude-/, "").replace(/-(\d+)-(\d+)$/, " $1.$2").replace(/^\w/, (c) => c.toUpperCase());

/** Inline-playable media list: audio gets a player, images and video a thumbnail. */
function MediaSection({ title, hint, files, empty }: { title: string; hint: string; files: (FileInfo & { item?: string; cost?: number })[]; empty?: string }) {
  if (!files.length && !empty) return null;
  return (
    <div className="card-b stack" style={{ gap: 10, borderTop: "1px solid var(--border)" }}>
      <div className="row"><h3>{title}</h3><span className="small faint">{hint}</span></div>
      {!files.length ? <p className="small muted">{empty}</p> : (
        <div className="media-grid">
          {files.map((f) => (
            <div key={f.url} className="media-item">
              {f.kind === "audio" ? <audio src={f.url} controls preload="none" /> :
               f.kind === "image" ? <a href={f.url} target="_blank" rel="noreferrer"><img src={f.url} alt={f.name} loading="lazy" /></a> :
               <video src={f.url} controls preload="metadata" />}
              <div className="small row" style={{ gap: 6 }}>
                <span className="ellipsis spacer" title={f.path}>{f.path}</span>
                <span className="faint nowrap">{f.seconds ? clock(f.seconds) : bytes(f.size)}{f.cost ? ` · ${cash(f.cost)}` : ""}</span>
              </div>
              {f.item && <div className="small faint">{f.item}</div>}
            </div>
          ))}
        </div>
      )}
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
