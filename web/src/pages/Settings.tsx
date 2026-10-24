import { useEffect, useState } from "react";
import {
  ArrowSquareOutIcon, CaretRightIcon, ChartLineUpIcon, CheckCircleIcon, CircleIcon, CpuIcon, InfoIcon, KeyIcon, PlugsConnectedIcon,
  RocketLaunchIcon, ShareNetworkIcon, WarningCircleIcon, Icon,
} from "@phosphor-icons/react";
import { api, connectGuideURL, donateURL, fail, languageName, makerURL, Meta, money, repoURL, setDisplay, toast, useApi, Voice } from "../api";
import { Field, Seg } from "../ui";
import { Brand, BrandId } from "../brands";

interface KeyInfo { name: string; label: string; hint: string; set: boolean; last4?: string; source: "" | "app" | "env" }

type KeyField = { key: string; label: string; help?: string };
type Provider = {
  id: string; title: string; brands: BrandId[]; what: string; fields: KeyField[];
  get?: { label: string; url: string }; guide?: string; required?: boolean;
  ready: (set: (k: string) => boolean) => boolean;
};

// Plain words for every key; the backend's own labels are the fallback for keys added there first.
const aiProviders: Provider[] = [
  {
    id: "claude", title: "Claude", brands: ["claude"], required: true,
    what: "The brain of 37Studio: it edits your videos, writes captions and drafts replies. One of the two below is enough.",
    get: { label: "Get an API key", url: "https://console.anthropic.com/settings/keys" },
    fields: [
      { key: "CLAUDE_CODE_OAUTH_TOKEN", label: "Claude Pro or Max plan", help: "Uses your Claude subscription. In the 37Studio folder run docker compose exec studio claude setup-token, sign in, and paste the token it prints. (Claude Desktop's sign-in doesn't carry over.)" },
      { key: "ANTHROPIC_API_KEY", label: "Or an Anthropic API key", help: "Pay only for what you use. console.anthropic.com → API keys." },
    ],
    ready: (set) => set("ANTHROPIC_API_KEY") || set("CLAUDE_CODE_OAUTH_TOKEN"),
  },
  {
    id: "gemini", title: "Gemini", brands: ["gemini"],
    what: "Makes images and short video clips for your videos, and reads company websites better. Optional.",
    get: { label: "Get a key", url: "https://aistudio.google.com/apikey" },
    fields: [{ key: "GEMINI_API_KEY", label: "API key", help: "Google AI Studio → Get API key." }],
    ready: (set) => set("GEMINI_API_KEY"),
  },
  {
    id: "elevenlabs", title: "ElevenLabs", brands: ["elevenlabs"],
    what: "Voiceovers in many languages, sound effects and background music. Optional.",
    get: { label: "Get a key", url: "https://elevenlabs.io/app/settings/api-keys" },
    fields: [{ key: "ELEVENLABS_API_KEY", label: "API key", help: "elevenlabs.io → Developers → API keys." }],
    ready: (set) => set("ELEVENLABS_API_KEY"),
  },
];

const socialProviders: Provider[] = [
  {
    id: "meta", title: "Facebook, Instagram & WhatsApp", brands: ["facebook", "instagram", "whatsapp"], guide: "meta",
    what: "One Meta app of your own posts to your Pages and Instagram, and brings their comments and messages to the Inbox.",
    fields: [
      { key: "META_APP_ID", label: "App ID" },
      { key: "META_APP_SECRET", label: "App secret" },
      { key: "META_CONFIG_ID", label: "Login configuration ID", help: "Needed to post videos to Instagram." },
      { key: "META_VERIFY_TOKEN", label: "WhatsApp webhook verify token", help: "Only for WhatsApp: any long random text, entered again in your Meta app's webhook settings." },
    ],
    ready: (set) => set("META_APP_ID") && set("META_APP_SECRET"),
  },
  {
    id: "youtube", title: "YouTube", brands: ["youtube"], guide: "youtube-steps", what: "Uploads to your channels, through your own Google Cloud project.",
    fields: [{ key: "GOOGLE_CLIENT_ID", label: "Client ID" }, { key: "GOOGLE_CLIENT_SECRET", label: "Client secret" }],
    ready: (set) => set("GOOGLE_CLIENT_ID") && set("GOOGLE_CLIENT_SECRET"),
  },
  {
    id: "tiktok", title: "TikTok", brands: ["tiktok"], guide: "tiktok-steps", what: "Posts to TikTok, or sends videos to the creator's drafts.",
    fields: [{ key: "TIKTOK_CLIENT_KEY", label: "Client key" }, { key: "TIKTOK_CLIENT_SECRET", label: "Client secret" }],
    ready: (set) => set("TIKTOK_CLIENT_KEY") && set("TIKTOK_CLIENT_SECRET"),
  },
  {
    id: "linkedin", title: "LinkedIn", brands: ["linkedin"], guide: "linkedin-steps", what: "Posts videos to your LinkedIn profile.",
    fields: [{ key: "LINKEDIN_CLIENT_ID", label: "Client ID" }, { key: "LINKEDIN_CLIENT_SECRET", label: "Client secret" }],
    ready: (set) => set("LINKEDIN_CLIENT_ID") && set("LINKEDIN_CLIENT_SECRET"),
  },
  {
    id: "x", title: "X", brands: ["x"], guide: "x-steps", what: "Posts videos to X.",
    fields: [{ key: "X_CLIENT_ID", label: "OAuth 2.0 client ID" }, { key: "X_CLIENT_SECRET", label: "OAuth 2.0 client secret" }],
    ready: (set) => set("X_CLIENT_ID") && set("X_CLIENT_SECRET"),
  },
  {
    id: "pinterest", title: "Pinterest", brands: ["pinterest"], guide: "pinterest-steps", what: "Pins videos to the boards you pick.",
    fields: [{ key: "PINTEREST_APP_ID", label: "App ID" }, { key: "PINTEREST_APP_SECRET", label: "App secret" }],
    ready: (set) => set("PINTEREST_APP_ID") && set("PINTEREST_APP_SECRET"),
  },
];

const tabs: { id: string; label: string; Icon: Icon }[] = [
  { id: "start", label: "Get started", Icon: RocketLaunchIcon },
  { id: "ai", label: "AI services", Icon: KeyIcon },
  { id: "social", label: "Social apps", Icon: ShareNetworkIcon },
  { id: "spending", label: "Spending", Icon: ChartLineUpIcon },
  { id: "performance", label: "Performance", Icon: CpuIcon },
  { id: "agents", label: "Other AI apps", Icon: PlugsConnectedIcon },
  { id: "about", label: "About", Icon: InfoIcon },
];

/** Each section has its own address (#/settings/social), so other pages can link straight to it. */
export default function Settings({ tab: tabParam }: { tab?: string }) {
  const { data, setData } = useApi<Record<string, string>>("/settings");
  const { data: meta } = useApi<Meta>("/meta");
  const { data: keys, setData: setKeys } = useApi<KeyInfo[]>("/keys");
  const { data: voices, error: voiceErr } = useApi<Voice[]>(meta?.env.eleven ? "/voices" : null);
  const tab = tabs.some((t) => t.id === tabParam) ? tabParam! : "start";
  const setTab = (t: string) => { location.hash = `#/settings/${t}`; };
  const [s, setS] = useState<Record<string, string>>({});
  useEffect(() => { if (data) setS(data); }, [data]);
  const dirty = !!data && Object.keys(s).some((k) => s[k] !== data[k]);

  const isSet = (k: string) => !!keys?.find((x) => x.name === k)?.set;
  const claudeReady = aiProviders[0].ready(isSet);
  const socialReady = socialProviders.filter((p) => p.ready(isSet)).length;

  const input = (k: string, props: React.InputHTMLAttributes<HTMLInputElement> = {}) => (
    <input className="input" value={s[k] ?? ""} onChange={(e) => setS({ ...s, [k]: e.target.value })} {...props} />
  );
  const voicePicker = (k: string, lang?: string) => {
    const list = [...(voices ?? [])].sort((a, b) => Number(b.language === lang) - Number(a.language === lang));
    const cur = voices?.find((v) => v.id === s[k]);
    return (
      <div className="row">
        <select className="select" value={s[k] ?? ""} onChange={(e) => setS({ ...s, [k]: e.target.value })}>
          {!cur && <option value={s[k]}>{s[k] || "Pick a voice"}</option>}
          {list.map((v) => <option key={v.id} value={v.id}>{v.name} · {languageName(v.language ?? "?")}{v.gender ? `, ${v.gender}` : ""}</option>)}
        </select>
        {cur?.preview && <button className="btn icon" type="button" title="Play a sample" aria-label={`Play ${cur.name}`} onClick={() => new Audio(cur.preview).play()}>▶</button>}
      </div>
    );
  };
  async function save() {
    try {
      const saved = await api<Record<string, string>>("/settings", { method: "PUT", body: s });
      setData(saved);
      setDisplay(saved);
      toast("Settings saved");
    } catch (e) { fail(e); }
  }
  async function saveKey(name: string, value: string) {
    try {
      setKeys(await api<KeyInfo[]>("/keys", { method: "PUT", body: { [name]: value.trim() } }));
      toast(value ? "Saved" : "Removed");
      return true;
    } catch (e) { fail(e); return false; }
  }
  const local = (usd: number) => money(usd * (Number(s.fx_rate) || 1), s.currency || "USD"); // live preview before saving
  const card = (p: Provider, extra?: React.ReactNode, advanced?: React.ReactNode) =>
    <ProviderCard key={p.id} p={p} keys={keys ?? []} isSet={isSet} onSave={saveKey} extra={extra} advanced={advanced} />;
  const grouped = new Set([...aiProviders, ...socialProviders].flatMap((p) => p.fields.map((f) => f.key)));
  const otherKeys = (keys ?? []).filter((k) => !grouped.has(k.name));

  return (
    <div className="page">
      <div className="page-head">
        <div className="spacer">
          <h1>Settings</h1>
          <p>Connect your AI services and social apps, and tune how 37Studio works. Everything is stored on this computer.</p>
        </div>
      </div>

      <div className="settings">
        <nav className="settings-nav" aria-label="Settings sections">
          {tabs.map(({ id, label, Icon: I }) => (
            <button key={id} className={tab === id ? "on" : ""} onClick={() => setTab(id)} aria-current={tab === id ? "page" : undefined}>
              <I size={18} weight={tab === id ? "duotone" : "regular"} /> <span>{label}</span>
              {id === "ai" && keys && !claudeReady && <span className="settings-dot" title="Claude isn't connected yet" />}
            </button>
          ))}
        </nav>

        <div className="settings-body stack">
          {tab === "start" && (
            <>
              <div className="card">
                <div className="card-h" style={{ display: "block" }}>
                  <h2>Set up 37Studio</h2>
                  <p className="small muted">Only Claude is required. Add the rest when you need them; you can come back here any time.</p>
                </div>
                <div className="filelist">
                  <Step done={claudeReady} title="Connect Claude" required text="Edits your videos, writes captions and drafts replies." action="Connect" onClick={() => setTab("ai")} brands={["claude"]} />
                  <Step done={isSet("GEMINI_API_KEY")} title="Images and video clips" text="Add a Gemini key to make pictures and short clips for your videos." action="Add" onClick={() => setTab("ai")} brands={["gemini"]} />
                  <Step done={isSet("ELEVENLABS_API_KEY")} title="Voiceovers and music" text="Add an ElevenLabs key for narration, sound effects and music." action="Add" onClick={() => setTab("ai")} brands={["elevenlabs"]} />
                  <Step done={socialReady > 0} title="Social apps" text={`${socialReady} of ${socialProviders.length} set up. Bluesky and Mastodon need no app: connect them on a company's Social tab.`}
                    action="Set up" onClick={() => setTab("social")} brands={["facebook", "instagram", "youtube", "tiktok"]} />
                </div>
              </div>
              <p className="small faint" style={{ margin: 0 }}>Keys are encrypted on this computer and only ever sent to the service they belong to. 37Studio's makers never see them.</p>
            </>
          )}

          {tab === "ai" && (
            <>
              {card(aiProviders[0])}
              {card(aiProviders[1], null, (
                <div className="form">
                  <Field label="Image model">{input("image_model")}</Field>
                  <Field label="Price per image" hint="USD, for the Costs page">{input("image_price", { type: "number", min: 0, step: 0.001 })}</Field>
                  <Field label="Video model">{input("video_model")}</Field>
                  <Field label="Video price" hint="USD per second">{input("video_price_per_sec", { type: "number", min: 0, step: 0.01 })}</Field>
                </div>
              ))}
              {card(aiProviders[2], meta?.env.eleven && (
                <div className="form">
                  <Field label="Voice for Bangla videos">{voicePicker("voice_bn", "bn")}</Field>
                  <Field label="Voice for other languages">{voicePicker("voice_default", "en")}</Field>
                  {voiceErr && <p className="small full" style={{ color: "var(--danger)" }}>{voiceErr}</p>}
                </div>
              ), (
                <div className="form">
                  <Field label="Voice model" hint="eleven_v4 speaks Bangla">{input("voice_model")}</Field>
                  <Field label="Voiceover price" hint="USD per 1,000 characters">{input("voice_price_per_1k", { type: "number", min: 0, step: 0.01 })}</Field>
                  <Field label="Sound effect price" hint="USD per second">{input("sfx_price_per_sec", { type: "number", min: 0, step: 0.001 })}</Field>
                  <Field label="Music price" hint="USD per minute">{input("music_price_per_min", { type: "number", min: 0, step: 0.01 })}</Field>
                </div>
              ))}
            </>
          )}

          {tab === "social" && (
            <>
              <div className="note-box small">
                Each platform needs a free developer app of your own; you make it once, and every company uses it. Then connect accounts on a company's <b>Social</b> tab.{" "}
                <a href={connectGuideURL} target="_blank" rel="noreferrer">Step-by-step guide <ArrowSquareOutIcon size={12} /></a>
              </div>
              {socialProviders.map((p) => card(p))}
              {otherKeys.length > 0 && <div className="card"><div className="filelist">{otherKeys.map((k) => <KeyRow key={k.name} k={k} f={{ key: k.name, label: k.label, help: k.hint }} onSave={saveKey} />)}</div></div>}
            </>
          )}

          {tab === "spending" && (
            <Section title="Spending and currency" sub="AI services charge in US dollars. 37Studio records costs in dollars and shows them in your currency.">
              <Field label="Spending limit per agent turn" hint={`USD; the agent stops if one turn would cost more · about ${local(Number(s.run_budget_usd) || 0)}`}>{input("run_budget_usd", { type: "number", min: 1 })}</Field>
              <div />
              <Field label="Your currency" hint="a code such as USD, EUR, GBP, INR or BDT">{input("currency", { maxLength: 3 })}</Field>
              <Field label="Exchange rate" hint="how much of your currency 1 US dollar buys (1 for USD)">{input("fx_rate", { type: "number", min: 0, step: 0.01 })}</Field>
            </Section>
          )}

          {tab === "performance" && (
            <Section title="Video export speed" sub="The defaults suit most computers. Exporting happens on the processor; a 1-minute vertical video takes about 3 minutes, or 2 as a draft.">
              <Field label="Parallel workers" hint="leave on auto unless exports overload your computer">{input("render_workers", { placeholder: "auto" })}</Field>
              <Field label="Draft frame rate" hint="frames per second for quick drafts; final videos stay at 30">{input("render_draft_fps", { type: "number", min: 0, max: 60 })}</Field>
              <Field label="Use the graphics card to encode" hint="a little faster, but files are about 50% bigger">
                <Seg value={s.render_gpu ?? "false"} options={[{ value: "false", label: "Off" }, { value: "true", label: "On" }]} onChange={(v) => setS({ ...s, render_gpu: v })} />
              </Field>
              <Field label="Fast capture (experimental)" hint="only helps with a graphics card Chrome can use">
                <Seg value={s.render_fast_capture ?? "false"} options={[{ value: "false", label: "Off" }, { value: "true", label: "On" }]} onChange={(v) => setS({ ...s, render_fast_capture: v })} />
              </Field>
            </Section>
          )}

          {tab === "agents" && <McpSection password={!!meta?.env.password} />}
          {tab === "about" && <AboutSection meta={meta} />}
        </div>
      </div>

      {dirty && (
        <div className="savebar" role="status">
          <span className="spacer">You have unsaved changes.</span>
          <button className="btn ghost" onClick={() => data && setS(data)}>Discard</button>
          <button className="btn primary" onClick={save}>Save changes</button>
        </div>
      )}
    </div>
  );
}

function Step({ done, title, text, action, onClick, brands, required }: {
  done: boolean; title: string; text: string; action: string; onClick: () => void; brands: BrandId[]; required?: boolean;
}) {
  return (
    <div className="item wrap-row">
      {done ? <CheckCircleIcon size={22} weight="fill" color="var(--success)" /> : required ? <WarningCircleIcon size={22} weight="fill" color="var(--warn)" /> : <CircleIcon size={22} />}
      <div className="spacer" style={{ minWidth: 200 }}>
        <strong>{title}</strong> {required && !done && <span className="pill amber" style={{ marginLeft: 6 }}>Required</span>}
        <div className="small muted">{text}</div>
      </div>
      <span className="row brand-row">{brands.map((b) => <Brand key={b} id={b} size={18} />)}</span>
      <button className={`btn sm ${done ? "ghost" : required ? "primary" : ""}`} onClick={onClick}>{done ? "Change" : action}</button>
    </div>
  );
}

function ProviderCard({ p, keys, isSet, onSave, extra, advanced }: {
  p: Provider; keys: KeyInfo[]; isSet: (k: string) => boolean; onSave: (name: string, value: string) => Promise<boolean>;
  extra?: React.ReactNode; advanced?: React.ReactNode;
}) {
  const ready = p.ready(isSet);
  const any = p.fields.some((f) => isSet(f.key));
  const [open, setOpen] = useState(ready || any || !!p.required);
  return (
    <div className="card provider">
      <button className="provider-head" onClick={() => setOpen(!open)} aria-expanded={open}>
        <span className="brand-tile">{p.brands.map((b) => <Brand key={b} id={b} size={p.brands.length > 1 ? 18 : 24} />)}</span>
        <span className="spacer" style={{ textAlign: "left" }}>
          <strong>{p.title}</strong>
          <span className="small muted" style={{ display: "block" }}>{p.what}</span>
        </span>
        {ready ? <span className="pill green"><span className="dot" />Connected</span>
          : any ? <span className="pill amber"><span className="dot" />Incomplete</span>
          : p.required ? <span className="pill amber"><span className="dot" />Required</span>
          : <span className="small faint nowrap">Not set up</span>}
        <CaretRightIcon size={14} className="caret" style={{ transform: open ? "rotate(90deg)" : undefined }} />
      </button>
      {open && (
        <>
          <div className="filelist" style={{ borderTop: "1px solid var(--border)" }}>
            {p.fields.map((f) => { const k = keys.find((x) => x.name === f.key); return k && <KeyRow key={f.key} k={k} f={f} onSave={onSave} />; })}
          </div>
          {(p.get || p.guide) && (
            <div className="provider-foot small">
              {p.get && <a href={p.get.url} target="_blank" rel="noreferrer">{p.get.label} <ArrowSquareOutIcon size={12} /></a>}
              {p.guide && <a href={`${connectGuideURL}#${p.guide}`} target="_blank" rel="noreferrer">How to make this app, step by step <ArrowSquareOutIcon size={12} /></a>}
            </div>
          )}
          {extra && <div className="card-b" style={{ borderTop: "1px solid var(--border)" }}>{extra}</div>}
          {advanced && (
            <details className="advanced">
              <summary className="small">Advanced: models and prices</summary>
              <div className="card-b">{advanced}</div>
            </details>
          )}
        </>
      )}
    </div>
  );
}

function KeyRow({ k, f, onSave }: { k: KeyInfo; f: KeyField; onSave: (name: string, value: string) => Promise<boolean> }) {
  const [value, setValue] = useState<string | null>(null);
  const submit = async () => { if (value?.trim() && await onSave(k.name, value)) setValue(null); };
  return (
    <div className="item wrap-row">
      <div className="spacer" style={{ minWidth: 220 }}>
        <strong>{f.label}</strong>
        {f.help && <div className="small faint">{f.help}</div>}
      </div>
      {value !== null ? (
        <span className="row">
          <input className="input mono" type="password" autoFocus value={value} placeholder="Paste it here" style={{ width: 240 }} aria-label={f.label}
            onChange={(e) => setValue(e.target.value)} onKeyDown={(e) => { if (e.key === "Enter") submit(); if (e.key === "Escape") setValue(null); }} />
          <button className="btn primary sm" disabled={!value.trim()} onClick={submit}>Save</button>
          <button className="btn ghost sm" onClick={() => setValue(null)}>Cancel</button>
        </span>
      ) : (
        <span className="row">
          {k.set
            ? <span className="small muted nowrap"><CheckCircleIcon size={14} weight="fill" color="var(--success)" style={{ verticalAlign: -2 }} /> Saved <span className="mono">••••{k.last4}</span>{k.source === "env" && " (from the .env file)"}</span>
            : <span className="small faint nowrap">Not added</span>}
          <button className="btn sm" onClick={() => setValue("")}>{k.set ? "Replace" : "Add"}</button>
          {k.source === "app" && <button className="btn ghost sm danger" onClick={() => confirm(`Remove the ${f.label}?`) && onSave(k.name, "")}>Remove</button>}
        </span>
      )}
    </div>
  );
}

function Section({ title, sub, children }: { title: string; sub: string; children: React.ReactNode }) {
  return (
    <div className="card">
      <div className="card-h" style={{ display: "block" }}><h2>{title}</h2><p className="small muted">{sub}</p></div>
      <div className="card-b form">{children}</div>
    </div>
  );
}

interface Update { current: string; latest?: string; notes?: string; url?: string; newer?: boolean; error?: string }

function AboutSection({ meta }: { meta?: Meta }) {
  const { data: upd } = useApi<Update>("/update");
  const checks: [string, boolean | undefined][] = [
    ["Claude Code is installed", meta?.env.claude], ["ffmpeg is installed (video processing)", meta?.env.ffmpeg],
    ["Claude is connected", meta?.env.anthropic], ["The agent runs with full permissions inside the container", meta?.env.bypass],
    ["The app has a password (for servers)", meta?.env.password],
  ];
  return (
    <>
      <div className="card">
        <div className="card-h" style={{ display: "block" }}><h2>Version</h2></div>
        <div className="card-b stack" style={{ gap: 10 }}>
          <p style={{ margin: 0 }}>
            37Studio <strong>{upd?.current}</strong>
            {upd?.newer ? <> · <strong style={{ color: "var(--primary)" }}>version {upd.latest} is available.</strong> Close 37Studio and open it again from the launcher to update.</>
              : upd?.error ? <span className="muted"> · couldn't check for updates right now</span> : upd?.latest ? <span className="muted"> · up to date</span> : null}
          </p>
          {upd?.newer && upd.notes && <p className="small muted" style={{ margin: 0 }}>{upd.notes}</p>}
          <p className="small muted" style={{ margin: 0 }}>
            Free and open source under the MIT License, made by <a href={makerURL} target="_blank" rel="noreferrer">Shaitrish</a>. Report a bug or suggest a feature
            on <a href={repoURL} target="_blank" rel="noreferrer">GitHub</a>. If 37Studio helps you, <a href={donateURL} target="_blank" rel="noreferrer">a donation</a> keeps it going.
          </p>
        </div>
      </div>
      <div className="card">
        <div className="card-h" style={{ display: "block" }}><h2>System check</h2></div>
        <div className="card-b stack" style={{ gap: 8 }}>
          {checks.map(([label, ok]) => (
            <span key={label} className="row small">{ok ? <CheckCircleIcon size={16} weight="fill" color="var(--success)" /> : <CircleIcon size={16} color="var(--faint-fg)" />} {label}</span>
          ))}
          {meta && <p className="small faint" style={{ margin: "6px 0 0" }}>HyperFrames {meta.hf_version} · your data is in <span className="mono">{meta.data_dir}</span></p>}
        </div>
      </div>
    </>
  );
}

/** Other agents (Claude Code, Claude Desktop, Cursor…) can run 37Studio through its MCP server: the tools the Assistant uses. */
function McpSection({ password }: { password: boolean }) {
  const url = `${location.origin}/mcp`;
  const cmd = `claude mcp add --transport http 37studio ${url}` + (password ? ` --header "Authorization: Basic <base64 of 37studio:your-password>"` : "");
  return (
    <div className="card">
      <div className="card-h" style={{ display: "block" }}>
        <h2>Use 37Studio from other AI apps</h2>
        <p className="small muted">Claude Code, Claude Desktop, Cursor and other apps that support MCP can manage your companies, videos and posts from their own chat, with the same tools as the Assistant.</p>
      </div>
      <div className="card-b stack" style={{ gap: 10 }}>
        <Field label="Server address"><input className="input mono" readOnly value={url} onFocus={(e) => e.target.select()} /></Field>
        <Field label="For Claude Code" hint="run this in a terminal"><input className="input mono" readOnly value={cmd} onFocus={(e) => e.target.select()} /></Field>
        <p className="small faint" style={{ margin: 0 }}>
          Only apps on this computer can connect{password ? ", with the app's password" : ""}. Connected apps can spend on your AI keys and post to your accounts, so connect only ones you trust.
        </p>
      </div>
    </div>
  );
}
