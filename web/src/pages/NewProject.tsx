import { ReactNode, useEffect, useRef, useState } from "react";
import { ArrowUpIcon, MegaphoneIcon, PencilSimpleIcon, SparkleIcon, MagicWandIcon } from "@phosphor-icons/react";
import { api, bytes, Campaign, fail, go, languageName, languages, Meta, Project, Tenant, upload, useApi } from "../api";
import { DropZone, Modal, Progress, Seg } from "../ui";
import { presetIcon } from "./Projects";

/** Answers collected one question at a time. */
interface A {
  tenant?: number; campaign?: string; campaignText?: string; kind?: string; aspect?: string;
  seconds?: number; brief?: string; files?: File[]; quality?: string; focus?: string; language?: string; count?: string;
}
type Opt = { value: string; label: string; sub?: string; icon?: ReactNode };
interface Step {
  key: keyof A;
  ask: string | ((a: A) => string);
  options?: (a: A) => Opt[];
  text?: { placeholder: string; skip?: string };
  files?: boolean;
  cards?: boolean;
  skip?: (a: A) => boolean;
  show: (a: A) => string;
}

const platforms: Opt[] = [
  { value: "9:16", label: "Reels · TikTok · Shorts", sub: "vertical 9:16" },
  { value: "16:9", label: "YouTube · website", sub: "landscape 16:9" },
  { value: "4:5", label: "Instagram / Facebook feed", sub: "portrait 4:5" },
  { value: "1:1", label: "Square post", sub: "1:1" },
];
const lengths = [15, 30, 45, 60, 90, 180];
const secs = (n: number) => (n >= 60 ? `${n / 60} min` : `${n} seconds`);
const counts: Opt[] = [
  { value: "1", label: "One video" }, { value: "3", label: "3 videos" }, { value: "5", label: "5 videos" }, { value: "10", label: "10 videos" },
  { value: "all", label: "As many as it has", sub: "e.g. one per solved question or topic" },
];
const many = (a: A) => !!a.count && a.count !== "1";

export function NewProjectModal({ onClose, tenantId, campaignId }: { onClose: () => void; tenantId?: number; campaignId?: number }) {
  const { data: meta } = useApi<Meta>("/meta");
  const { data: tenants } = useApi<Tenant[]>("/tenants");
  const [a, setA] = useState<A>({ tenant: tenantId, campaign: campaignId ? String(campaignId) : undefined, quality: "standard" });
  const [done, setDone] = useState<(keyof A)[]>([]);
  const { data: campaigns } = useApi<Campaign[]>(a.tenant ? `/campaigns?tenant=${a.tenant}` : null);
  const [title, setTitle] = useState("");
  const [busy, setBusy] = useState<string>();
  const [pct, setPct] = useState(0);
  const end = useRef<HTMLDivElement>(null);

  const tenantName = tenants?.find((t) => t.id === a.tenant)?.name ?? "";
  const campaignName = a.campaign === "new" ? a.campaignText?.split(/[.:\n]/)[0].trim().slice(0, 60) : campaigns?.find((c) => String(c.id) === a.campaign)?.name;
  const preset = meta?.presets.find((p) => p.id === a.kind);

  const steps: Step[] = [
    {
      key: "tenant", ask: "Hi! Who is this for?", skip: () => !!tenantId,
      options: () => (tenants ?? []).map((t) => ({ value: String(t.id), label: t.name, sub: t.website?.replace(/^https?:\/\//, "") || t.teacher })),
      show: () => tenantName,
    },
    {
      key: "campaign", ask: (x) => `Is this part of a campaign for ${tenantName}?`, skip: () => !!campaignId,
      options: () => [
        ...(campaigns ?? []).map((c) => ({ value: String(c.id), label: c.name, sub: `${c.video_count} video${c.video_count === 1 ? "" : "s"}`, icon: <MegaphoneIcon size={14} /> })),
        { value: "new", label: "Start a new campaign", sub: "a launch, offer or product push" },
        { value: "none", label: "Just a one-off video" },
      ],
      show: (x) => (x.campaign === "none" ? "One-off video" : x.campaign === "new" ? "New campaign" : campaignName ?? ""),
    },
    {
      key: "campaignText", ask: "What's the campaign? Give it a name and say what it promotes.", skip: (x) => x.campaign !== "new",
      text: { placeholder: "Eid offer: 30% off the HSC physics course until 15 April. Audience: HSC candidates. CTA: enroll at course37.com" },
      show: (x) => x.campaignText ?? "",
    },
    {
      key: "kind", ask: "What are we making?", cards: true,
      options: () => (meta?.presets ?? []).map((p) => {
        const I = presetIcon[p.id] ?? MagicWandIcon;
        return { value: p.id, label: p.label, sub: p.blurb, icon: <I size={15} /> };
      }),
      show: () => preset?.label ?? "",
    },
    {
      key: "language", ask: "What language is the video in?",
      options: () => {
        const own = tenants?.find((t) => t.id === a.tenant)?.language ?? "en";
        return [...languages.slice(0, 2), ...languages.slice(2).filter(([c]) => c === own)].map(([value, label]) => ({ value, label, sub: value === own ? "company's language" : undefined }));
      },
      show: (x) => languageName(x.language ?? ""),
    },
    {
      key: "focus", ask: "Is it about a specific page or feature of their site?",
      text: { placeholder: "Paste the page URL (https://…/features/live-class), or name the feature and the agent finds its page.", skip: "No, the brand in general" },
      show: (x) => x.focus || "The brand in general",
    },
    {
      key: "aspect", ask: "Where will it run?", options: () => platforms,
      show: (x) => platforms.find((p) => p.value === x.aspect)?.label ?? x.aspect ?? "",
    },
    {
      key: "count", ask: "One video, or several from the same footage?", skip: (x) => x.kind === "thumbnail",
      options: () => counts, show: (x) => counts.find((c) => c.value === x.count)?.label ?? "One video",
    },
    {
      key: "seconds", ask: (x) => (many(x) ? "How long should each video be?" : "How long should it be?"), skip: (x) => x.kind === "thumbnail",
      options: () => lengths.map((n) => ({ value: String(n), label: secs(n), sub: n === preset?.seconds ? "suggested" : undefined })),
      show: (x) => secs(x.seconds ?? 0),
    },
    {
      key: "brief", ask: (x) => (x.kind === "thumbnail" ? "What should the thumbnails say or show?" : many(x) ? "What should the videos be about?" : "What's the video about?"),
      text: {
        placeholder: "Topic, the moment you want as the hook, the call to action, anything to avoid. No footage? Paste a landing page URL and the agent builds from it.",
        skip: "Let the agent decide from the knowledge base",
      },
      show: (x) => x.brief || "Agent decides from the knowledge base",
    },
    {
      key: "files", ask: "Any footage to work from?", files: true,
      show: (x) => (x.files?.length ? `${x.files.length} file${x.files.length > 1 ? "s" : ""} · ${bytes(x.files.reduce((n, f) => n + f.size, 0))}` : "No footage, use brand assets"),
    },
  ];
  const live = steps.filter((s) => !s.skip?.(a));
  const current = live.find((s) => !done.includes(s.key));
  const history = live.filter((s) => done.includes(s.key));

  useEffect(() => { end.current?.scrollIntoView({ behavior: "smooth", block: "end" }); }, [done.length, current?.key]);
  useEffect(() => {
    if (!current) setTitle((t) => t || (a.brief?.split("\n")[0].slice(0, 60).trim() || `${preset?.label} · ${campaignName ?? tenantName}`));
  }, [current?.key]);

  function answer(key: keyof A, value: A[keyof A]) {
    const next = { ...a, [key]: value };
    // Picking a format sets sensible defaults for the questions after it.
    if (key === "kind") {
      const p = meta?.presets.find((x) => x.id === value);
      next.aspect ??= p?.aspect;
      if (p) next.seconds = p.seconds || next.seconds;
    }
    setA(next);
    setDone((d) => [...d.filter((k) => k !== key), key]);
  }
  // Clicking an earlier answer re-asks from there; later answers stay as defaults.
  const rewind = (key: keyof A) => setDone((d) => d.slice(0, d.indexOf(key)));

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
        body: { tenant_id: a.tenant, campaign_id, title: title.trim() || "Untitled", kind: a.kind, aspect: a.aspect, seconds: a.kind === "thumbnail" ? 0 : a.seconds, quality: a.quality, brief: split + (a.brief ?? ""), focus: a.focus ?? "", language: a.language ?? "" },
      });
      if (a.files?.length) {
        setBusy(`Uploading ${bytes(a.files.reduce((n, f) => n + f.size, 0))}…`);
        await upload(`/projects/${p.id}/inputs`, a.files, setPct);
      }
      await api(`/projects/${p.id}/messages`, { body: { text: "" } });
      go(`/projects/${p.id}`);
    } catch (e) { fail(e); setBusy(undefined); }
  }

  return (
    <Modal title="New project" sub="A few quick questions, then the agent takes it from there." size="lg" onClose={busy ? () => {} : onClose}>
      <div className="wizard">
        {history.map((s) => (
          <div key={s.key} className="stack" style={{ gap: 8 }}>
            <Ask>{typeof s.ask === "function" ? s.ask(a) : s.ask}</Ask>
            <button className="msg-user answer" onClick={() => !busy && rewind(s.key)} title="Change this answer">
              {s.show(a)} <PencilSimpleIcon size={12} />
            </button>
          </div>
        ))}

        {current ? (
          <div className="stack" style={{ gap: 10 }}>
            <Ask>{typeof current.ask === "function" ? current.ask(a) : current.ask}</Ask>
            {current.options && (current.key === "tenant" && tenants?.length === 0 ? (
              <p className="small muted">No companies yet. <a className="link" href="#/clients" onClick={onClose}>Add one from its website</a> first.</p>
            ) : (
              <div className={current.cards ? "choices" : "opts"}>
                {current.options(a).map((o) => {
                  const on = String(a[current.key] ?? "") === o.value;
                  return (
                    <button key={o.value} className={`${current.cards ? "choice" : "opt"} ${on ? "on" : ""}`}
                      onClick={() => answer(current.key, current.key === "tenant" || current.key === "seconds" ? Number(o.value) : o.value)}>
                      <strong>{o.icon}{o.label}</strong>
                      {o.sub && <span>{o.sub}</span>}
                    </button>
                  );
                })}
              </div>
            ))}
            {current.text && <TextAnswer key={current.key} initial={String(a[current.key] ?? "")} placeholder={current.text.placeholder} skip={current.text.skip} onAnswer={(v) => answer(current.key, v)} />}
            {current.files && <FilesAnswer initial={a.files ?? []} onAnswer={(f) => answer("files", f)} />}
          </div>
        ) : (
          <div className="card card-b stack" style={{ gap: 12 }}>
            <Ask>Great, here's the plan. Name it and I'll start.</Ask>
            <label className="field"><span>Project name</span><input className="input" value={title} onChange={(e) => setTitle(e.target.value)} /></label>
            <div className="field">
              <span>Quality <span className="hint">· {meta?.qualities.find((q) => q.id === a.quality)?.note}</span></span>
              <Seg value={a.quality ?? "standard"} options={(meta?.qualities ?? []).map((q) => ({ value: q.id, label: q.label }))} onChange={(quality) => setA({ ...a, quality })} />
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

function Ask({ children }: { children: ReactNode }) {
  return <div className="ask"><span className="ask-mark"><SparkleIcon size={13} /></span><span>{children}</span></div>;
}

function TextAnswer({ initial, placeholder, skip, onAnswer }: { initial: string; placeholder: string; skip?: string; onAnswer: (v: string) => void }) {
  const [v, setV] = useState(initial);
  return (
    <div className="stack" style={{ gap: 8 }}>
      <textarea className="textarea" rows={3} value={v} autoFocus placeholder={placeholder} onChange={(e) => setV(e.target.value)}
        onKeyDown={(e) => { if (e.key === "Enter" && !e.shiftKey && v.trim()) { e.preventDefault(); onAnswer(v.trim()); } }} />
      <div className="row">
        {skip && <button className="opt" onClick={() => onAnswer("")}><strong>{skip}</strong></button>}
        <span className="spacer" />
        <button className="btn primary sm" disabled={!v.trim()} onClick={() => onAnswer(v.trim())}><ArrowUpIcon size={14} /> Continue</button>
      </div>
    </div>
  );
}

function FilesAnswer({ initial, onAnswer }: { initial: File[]; onAnswer: (f: File[]) => void }) {
  const [files, setFiles] = useState(initial);
  return (
    <div className="stack" style={{ gap: 8 }}>
      <DropZone onFiles={(f) => setFiles((x) => [...x, ...f])} accept="video/*,audio/*,image/*" label="Drop long videos, b-roll or music, or click to browse" />
      {files.length > 0 && (
        <div className="row wrap small">
          {files.map((f, i) => <span key={i} className="pill">{f.name} · {bytes(f.size)}</span>)}
        </div>
      )}
      <div className="row">
        <button className="opt" onClick={() => onAnswer([])}><strong>No footage, use brand assets</strong></button>
        <span className="spacer" />
        <button className="btn primary sm" disabled={!files.length} onClick={() => onAnswer(files)}><ArrowUpIcon size={14} /> Continue</button>
      </div>
    </div>
  );
}
