import { ReactNode, useEffect, useRef, useState } from "react";
import { ArrowUpIcon, PencilSimpleIcon, SparkleIcon, MagicWandIcon } from "@phosphor-icons/react";
import { api, bytes, Campaign, fail, go, languageName, languages, Meta, Project, Template, upload, useApi, useTenants } from "../api";
import { Avatar, DropZone, Field, Modal, ModelPicker, Progress, Seg } from "../ui";
import { presetIcon } from "./Projects";

/** Answers collected one question at a time. */
interface A {
  tenant?: number; campaign?: string; campaignText?: string; kind?: string; aspect?: string;
  seconds?: number; brief?: string; files?: File[]; quality?: string; focus?: string; language?: string; count?: string;
  link?: string; transcribe?: string; template?: string;
}
type Opt = { value: string; label: string; sub?: string; icon?: ReactNode };
interface Step {
  key: keyof A;
  label?: string; // shown in front of the answer once it collapses into a chip
  ask: string | ((a: A) => string);
  options?: (a: A) => Opt[];
  cards?: boolean;
  skip?: (a: A) => boolean;
  show: (a: A) => ReactNode;
}

const shapes: Opt[] = [
  { value: "9:16", label: "Vertical 9:16", sub: "Reels, TikTok, Shorts" },
  { value: "16:9", label: "Wide 16:9", sub: "YouTube, website" },
  { value: "4:5", label: "Portrait 4:5", sub: "Instagram and Facebook feed" },
  { value: "1:1", label: "Square 1:1", sub: "feed post" },
];
const lengths = [15, 30, 45, 60, 90, 180];
const secs = (n: number) => (n >= 60 ? `${n / 60} min` : `${n} sec`);
const counts = [{ value: "1", label: "1" }, { value: "3", label: "3" }, { value: "5", label: "5" }, { value: "10", label: "10" }, { value: "all", label: "As many as it has" }];
const many = (a: A) => !!a.count && a.count !== "1";
const askText = (s: Step, a: A) => (typeof s.ask === "function" ? s.ask(a) : s.ask);

export function NewProjectModal({ onClose, tenantId, campaignId }: { onClose: () => void; tenantId?: number; campaignId?: number }) {
  const { data: meta } = useApi<Meta>("/meta");
  const tenants = useTenants();
  const [a, setA] = useState<A>({ tenant: tenantId, campaign: campaignId ? String(campaignId) : "none", quality: "standard" });
  const [done, setDone] = useState<(keyof A)[]>([]);
  const { data: campaigns } = useApi<Campaign[]>(a.tenant ? `/campaigns?tenant=${a.tenant}` : null);
  const { data: templates } = useApi<Template[]>(a.tenant ? `/templates?tenant=${a.tenant}` : null);
  const template = templates?.find((t) => String(t.id) === a.template);
  const [title, setTitle] = useState("");
  const [busy, setBusy] = useState<string>();
  const [pct, setPct] = useState(0);
  const end = useRef<HTMLDivElement>(null);

  // With a single brand there's nothing to ask.
  const only = tenants?.length === 1 ? tenants[0] : undefined;
  useEffect(() => { if (only && !a.tenant) setA((x) => ({ ...x, tenant: only.id })); }, [only?.id]);
  const tenant = tenants?.find((t) => t.id === a.tenant);
  const campaignName = a.campaign === "new" ? a.campaignText?.split(/[.:\n]/)[0].trim().slice(0, 60) : campaigns?.find((c) => String(c.id) === a.campaign)?.name;
  const preset = meta?.presets.find((p) => p.id === a.kind);

  const steps: Step[] = [
    {
      key: "tenant", ask: "Hi! Who is this for?", skip: () => !!tenantId || !!only,
      options: () => (tenants ?? []).map((t) => ({ value: String(t.id), label: t.name, sub: t.website?.replace(/^https?:\/\//, "") || t.teacher, icon: <Avatar tenant={t} size="xs" /> })),
      show: () => tenant && <><Avatar tenant={tenant} size="xs" />{tenant.name}</>,
    },
    {
      // A saved look: the new project's comp/ starts as a copy of it, and the format follows it.
      key: "template", label: "Template", ask: "Start from a template?", skip: () => !templates?.length,
      options: () => [
        ...(templates ?? []).map((t) => ({ value: String(t.id), label: t.name, sub: [t.tenant_name ?? "All brands", t.aspect, t.notes].filter(Boolean).join(" · ") })),
        { value: "none", label: "No, start fresh" },
      ],
      show: () => template?.name ?? "Start fresh",
    },
    {
      key: "kind", ask: "What are we making?", cards: true,
      options: () => (meta?.presets ?? []).map((p) => {
        const I = presetIcon[p.id] ?? MagicWandIcon;
        return { value: p.id, label: p.label, sub: p.blurb, icon: <I size={15} /> };
      }),
      show: () => { const I = presetIcon[a.kind ?? ""] ?? MagicWandIcon; return <><I size={13} />{preset?.label}</>; },
    },
    {
      // Shape, length, count and language in one go: each has a good default, so most people just press Continue.
      key: "aspect", ask: (x) => (x.kind === "thumbnail" ? "What shape, and which language?" : "Where will it run, and how long?"),
      show: (x) => [
        shapes.find((s) => s.value === x.aspect)?.label ?? x.aspect,
        x.kind !== "thumbnail" && secs(x.seconds ?? 0),
        x.kind !== "thumbnail" && many(x) && (x.count === "all" ? "as many videos as it has" : `${x.count} videos`),
        languageName(x.language ?? ""),
      ].filter(Boolean).join(" · "),
    },
    {
      key: "brief", label: "About", ask: (x) => (x.kind === "thumbnail" ? "What should the thumbnails say or show?" : many(x) ? "What should the videos be about?" : "What's the video about?"),
      show: (x) => [x.brief || "Agent decides", x.focus].filter(Boolean).join(" · "),
    },
    {
      key: "files", label: "Footage", ask: "Any footage to work from?",
      show: (x) => [x.files?.length && `${x.files.length} file${x.files.length > 1 ? "s" : ""} · ${bytes(x.files.reduce((n, f) => n + f.size, 0))}`, x.link, x.transcribe === "no" && "no transcript"]
        .filter(Boolean).join(" · ") || "None, use brand assets",
    },
  ];
  const live = steps.filter((s) => !s.skip?.(a));
  const current = live.find((s) => !done.includes(s.key));
  const history = live.filter((s) => done.includes(s.key));
  const fixed = tenant && steps[0].skip?.(a) ? tenant : undefined;

  useEffect(() => { end.current?.scrollIntoView({ behavior: "smooth", block: "end" }); }, [done.length, current?.key]);
  useEffect(() => {
    if (!current) setTitle((t) => t || (a.brief?.split("\n")[0].slice(0, 60).trim() || `${preset?.label} · ${tenant?.name ?? ""}`));
  }, [current?.key]);

  function answer(key: keyof A, value: A[keyof A], also: Partial<A> = {}) {
    const next = { ...a, ...also, [key]: value };
    // Another brand has other campaigns and templates.
    if (key === "tenant" && value !== a.tenant) Object.assign(next, { campaign: "none", template: undefined });
    // Picking a template or a format fills the format step's defaults, until that step has been answered.
    const tpl = key === "template" ? templates?.find((t) => String(t.id) === value) : undefined;
    if (tpl) Object.assign(next, { kind: tpl.kind || next.kind, aspect: tpl.aspect || next.aspect, seconds: tpl.seconds || next.seconds });
    const p = key === "kind" && !template && !done.includes("aspect") ? meta?.presets.find((x) => x.id === value) : undefined;
    if (p) Object.assign(next, { aspect: p.aspect || next.aspect, seconds: p.seconds || next.seconds });
    setA(next);
    setDone((d) => [...d.filter((k) => k !== key), key]);
  }
  // Clicking a chip re-asks just that question; a different brand starts over, keeping the old answers as defaults.
  const rewind = (key: keyof A) => setDone((d) => (key === "tenant" ? [] : d.filter((k) => k !== key)));

  async function create() {
    try {
      setBusy("Creating…");
      let campaign_id: number | null = a.campaign && a.campaign !== "new" && a.campaign !== "none" ? Number(a.campaign) : null;
      if (a.campaign === "new") {
        const c = await api<Campaign>("/campaigns", { body: { tenant_id: a.tenant, name: campaignName || "New campaign", brief: a.campaignText } });
        campaign_id = c.id;
      }
      // Several videos: the agent plans them from the footage and queues one project per video (clip.go).
      const split = many(a) ? `Make ${a.count === "all" ? "a separate video for every strong moment in this footage (for example each solved question or topic)" : `${a.count} separate videos from this footage`}, each about ${secs(a.seconds ?? 60)}. Plan them from the transcript, then queue them without waiting for me.\n\n` : "";
      const p = await api<Project>("/projects", {
        body: { tenant_id: a.tenant, campaign_id, title: title.trim() || "Untitled", kind: a.kind, aspect: a.aspect, seconds: a.kind === "thumbnail" ? 0 : a.seconds, quality: a.quality, brief: split + (a.brief ?? ""), focus: a.focus ?? "", language: a.language ?? "", template_id: template?.id ?? null },
      });
      if (a.files?.length) {
        setBusy(`Uploading ${bytes(a.files.reduce((n, f) => n + f.size, 0))}…`);
        await upload(`/projects/${p.id}/inputs`, a.files, setPct);
      }
      if (a.transcribe === "no") await api(`/projects/${p.id}/transcript`, { body: { action: "skip" } });
      // A link downloads first; the server starts the agent once the video is in.
      if (a.link) await api(`/projects/${p.id}/import`, { body: { url: a.link, start: true } });
      else await api(`/projects/${p.id}/messages`, { body: { text: "" } });
      go(`/projects/${p.id}`);
    } catch (e) { fail(e); setBusy(undefined); }
  }

  return (
    <Modal title="New video or image" sub="A few quick questions, then the agent takes it from there." size="lg" onClose={busy ? () => {} : onClose}>
      <div className="wizard">
        {(fixed || history.length > 0) && (
          <div className="wiz-sum">
            {fixed && <span className="wiz-chip" style={{ cursor: "default" }}><Avatar tenant={fixed} size="xs" />{fixed.name}</span>}
            {history.map((s) => (
              <button key={s.key} className="wiz-chip" onClick={() => !busy && rewind(s.key)} title={`Change: ${askText(s, a)}`}>
                {s.label && <span className="k">{s.label}</span>}<span className="ellipsis">{s.show(a)}</span><PencilSimpleIcon size={11} />
              </button>
            ))}
          </div>
        )}

        {current ? (
          <div className="stack" style={{ gap: 10 }}>
            <Ask step={`${history.length + 1} of ${live.length}`}>{askText(current, a)}</Ask>
            {current.options && (current.key === "tenant" && tenants?.length === 0 ? (
              <p className="small muted">No brands yet. <a className="link" href="#/clients" onClick={onClose}>Add one from its website</a> first.</p>
            ) : (
              <div className={current.cards ? "choices" : "opts"}>
                {current.options(a).map((o) => {
                  const on = String(a[current.key] ?? "") === o.value;
                  return (
                    <button key={o.value} className={`${current.cards ? "choice" : "opt"} ${on ? "on" : ""}`}
                      onClick={() => answer(current.key, current.key === "tenant" ? Number(o.value) : o.value)}>
                      <strong>{o.icon}{o.label}</strong>
                      {o.sub && <span>{o.sub}</span>}
                    </button>
                  );
                })}
              </div>
            ))}
            {current.key === "aspect" && <FormatAnswer a={a} own={tenant?.language ?? "en"} suggested={preset?.seconds} onAnswer={(f) => answer("aspect", f.aspect, f)} />}
            {current.key === "brief" && <BriefAnswer a={a} onAnswer={(brief, focus) => answer("brief", brief, { focus })} />}
            {current.key === "files" && <FilesAnswer a={a} onAnswer={(f, link, transcribe) => answer("files", f, { link, transcribe })} />}
          </div>
        ) : (
          <div className="card card-b stack" style={{ gap: 14 }}>
            <Ask>That's everything. Name it and I'll start.</Ask>
            <div className="form">
              <Field label="Name"><input className="input" value={title} onChange={(e) => setTitle(e.target.value)} /></Field>
              <Field label="Campaign" hint="optional">
                <select className="select" value={a.campaign ?? "none"} onChange={(e) => setA({ ...a, campaign: e.target.value })}>
                  <option value="none">None, a one-off video</option>
                  {campaigns?.map((c) => <option key={c.id} value={c.id}>{c.name} · {c.video_count} video{c.video_count === 1 ? "" : "s"}</option>)}
                  <option value="new">New campaign…</option>
                </select>
              </Field>
              {a.campaign === "new" && (
                <Field label="New campaign" hint="its name, what it promotes, audience, call to action" full>
                  <textarea className="textarea" rows={2} autoFocus value={a.campaignText ?? ""} onChange={(e) => setA({ ...a, campaignText: e.target.value })}
                    placeholder="Eid offer: 30% off the HSC physics course until 15 April. Audience: HSC candidates. CTA: enroll at course37.com" />
                </Field>
              )}
              <div className="field full">
                <span>Quality <span className="hint">· {meta?.qualities.find((q) => q.id === a.quality)?.note}</span></span>
                <ModelPicker value={a.quality ?? "standard"} qualities={meta?.qualities ?? []} onChange={(quality) => setA({ ...a, quality })} />
              </div>
            </div>
            {busy ? (
              <div className="stack" style={{ gap: 6 }}><span className="small muted">{busy}</span>{busy.startsWith("Uploading") && <Progress pct={pct} />}</div>
            ) : (
              <div className="row"><span className="small muted spacer">The agent may ask a question or two before the first preview.</span>
                <button className="btn primary" onClick={create}><SparkleIcon size={15} /> Create and start</button></div>
            )}
          </div>
        )}
        <div ref={end} />
      </div>
    </Modal>
  );
}

function Ask({ children, step }: { children: ReactNode; step?: string }) {
  return <div className="ask"><span className="ask-mark"><SparkleIcon size={13} /></span><span>{children}</span>{step && <span className="step">{step}</span>}</div>;
}

function FormatAnswer({ a, own, suggested, onAnswer }: { a: A; own: string; suggested?: number; onAnswer: (f: Required<Pick<A, "aspect" | "seconds" | "count" | "language">>) => void }) {
  const [f, setF] = useState({ aspect: a.aspect ?? "9:16", seconds: a.seconds ?? suggested ?? 30, count: a.count ?? "1", language: a.language ?? own });
  const video = a.kind !== "thumbnail";
  const opts = [...new Set([...lengths, f.seconds])].sort((x, y) => x - y).map((n) => ({ value: String(n), label: secs(n) }));
  const langs = languages.filter(([c], i) => i < 2 || c === own).map(([value, label]) => ({ value, label }));
  return (
    <div className="stack" style={{ gap: 14 }}>
      <div className="fmt">
        <span>Shape</span>
        <div><Seg value={f.aspect} options={shapes} onChange={(aspect) => setF({ ...f, aspect })} /><span className="hint">{shapes.find((s) => s.value === f.aspect)?.sub}</span></div>
        {video && <>
          <span>Length</span>
          <div><Seg value={String(f.seconds)} options={opts} onChange={(v) => setF({ ...f, seconds: Number(v) })} />{f.seconds === suggested && <span className="hint">suggested</span>}</div>
          <span>Videos</span>
          <div><Seg value={f.count} options={counts} onChange={(count) => setF({ ...f, count })} />{f.count !== "1" && <span className="hint">cut from the same footage</span>}</div>
        </>}
        <span>Language</span>
        <div><Seg value={f.language} options={langs} onChange={(language) => setF({ ...f, language })} />{f.language === own && <span className="hint">the brand's</span>}</div>
      </div>
      <div className="row"><span className="spacer" /><button className="btn primary sm" autoFocus onClick={() => onAnswer(f)}><ArrowUpIcon size={14} /> Continue</button></div>
    </div>
  );
}

function BriefAnswer({ a, onAnswer }: { a: A; onAnswer: (brief: string, focus: string) => void }) {
  const [v, setV] = useState(a.brief ?? "");
  const [focus, setFocus] = useState(a.focus ?? "");
  const send = (brief: string) => onAnswer(brief, focus.trim());
  return (
    <div className="stack" style={{ gap: 8 }}>
      <textarea className="textarea" rows={3} value={v} autoFocus onChange={(e) => setV(e.target.value)}
        placeholder="Topic, the moment you want as the hook, the call to action, anything to avoid. No footage? Paste a landing page URL and the agent builds from it."
        onKeyDown={(e) => { if (e.key === "Enter" && !e.shiftKey && v.trim()) { e.preventDefault(); send(v.trim()); } }} />
      <input className="input" value={focus} onChange={(e) => setFocus(e.target.value)} aria-label="Page or feature to focus on"
        placeholder="Optional: one page or feature to focus on (https://…/features/live-class, or just its name)" />
      <div className="row">
        <button className="opt" onClick={() => send("")}><strong>Let the agent decide from the knowledge base</strong></button>
        <span className="spacer" />
        <button className="btn primary sm" disabled={!v.trim()} onClick={() => send(v.trim())}><ArrowUpIcon size={14} /> Continue</button>
      </div>
    </div>
  );
}

function FilesAnswer({ a, onAnswer }: { a: A; onAnswer: (f: File[], link: string, transcribe: string) => void }) {
  const [files, setFiles] = useState(a.files ?? []);
  const [link, setLink] = useState(a.link ?? "");
  const [transcribe, setTranscribe] = useState(a.transcribe !== "no");
  const url = /^https?:\/\/\S+$/.test(link.trim());
  const ok = !!files.length || url;
  // Transcribing takes about as long as the video on most PCs: let the editor skip it, or bring subtitles instead.
  const media = url || files.some((f) => /^(video|audio)\//.test(f.type));
  const subs = files.some((f) => /\.(srt|vtt)$/i.test(f.name)) || link.includes("youtu");
  return (
    <div className="stack" style={{ gap: 8 }}>
      <DropZone onFiles={(f) => setFiles((x) => [...x, ...f])} accept="video/*,audio/*,image/*,.srt,.vtt" label="Drop long videos, b-roll, music or subtitles (.srt), or click to browse" />
      <input className="input" value={link} onChange={(e) => setLink(e.target.value)} placeholder="Or paste a YouTube or Google Drive link; YouTube's subtitles come along" aria-label="Video link" />
      {files.length > 0 && (
        <div className="row wrap small">
          {files.map((f, i) => <span key={i} className="pill">{f.name} · {bytes(f.size)}</span>)}
        </div>
      )}
      {media && (
        <div className="row small">
          <label className="row"><input type="checkbox" checked={transcribe} onChange={(e) => setTranscribe(e.target.checked)} /> Transcribe what's said</label>
          <span className="faint">· {subs ? "uses the subtitles, which is instant" : "takes about as long as the video; untick to start sooner"}</span>
        </div>
      )}
      <div className="row">
        <button className="opt" onClick={() => onAnswer([], "", "")}><strong>No footage, use brand assets</strong></button>
        <span className="spacer" />
        <button className="btn primary sm" disabled={!ok} onClick={() => onAnswer(files, link.trim(), media && !transcribe ? "no" : "")}><ArrowUpIcon size={14} /> Continue</button>
      </div>
    </div>
  );
}
