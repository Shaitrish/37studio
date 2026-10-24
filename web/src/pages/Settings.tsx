import { useEffect, useState } from "react";
import {
  CaretRightIcon, CheckCircleIcon, FacebookLogoIcon, GoogleLogoIcon, Icon, LinkedinLogoIcon, PinterestLogoIcon, SparkleIcon, TiktokLogoIcon,
  WaveformIcon, XCircleIcon, XLogoIcon, YoutubeLogoIcon,
} from "@phosphor-icons/react";
import { api, connectGuideURL, donateURL, fail, languageName, makerURL, Meta, money, repoURL, setDisplay, toast, useApi, Voice } from "../api";
import { Field, Seg } from "../ui";

export default function Settings() {
  const { data, setData } = useApi<Record<string, string>>("/settings");
  const { data: meta } = useApi<Meta>("/meta");
  const { data: voices, error: voiceErr } = useApi<Voice[]>(meta?.env.eleven ? "/voices" : null);
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
  const [s, setS] = useState<Record<string, string>>({});
  useEffect(() => { if (data) setS(data); }, [data]);
  const dirty = data && Object.keys(s).some((k) => s[k] !== data[k]);
  const input = (k: string, props: React.InputHTMLAttributes<HTMLInputElement> = {}) => (
    <input className="input" value={s[k] ?? ""} onChange={(e) => setS({ ...s, [k]: e.target.value })} {...props} />
  );

  async function save() {
    try {
      const saved = await api<Record<string, string>>("/settings", { method: "PUT", body: s });
      setData(saved);
      setDisplay(saved);
      toast("Settings saved");
    } catch (e) { fail(e); }
  }

  // Live preview with the form's currency and rate, before saving.
  const local = (usdAmount: number) => money(usdAmount * (Number(s.fx_rate) || 1), s.currency || "USD");

  return (
    <div className="page">
      <div className="page-head">
        <div className="spacer">
          <h1>Settings</h1>
          <p>Keys, AI providers, voice, rendering, currency and updates.</p>
        </div>
        <button className="btn primary" disabled={!dirty} onClick={save}>Save changes</button>
      </div>

      <KeysSection />

      <Section title="AI providers" sub="Claude's cost comes from its own usage report. Gemini prices are per unit you set here.">
        <Field label="Agent budget per turn" hint={`USD, hard stop for runaway runs · ≈ ${local(Number(s.run_budget_usd) || 0)}`}>{input("run_budget_usd", { type: "number", min: 1 })}</Field>
        <Field label="Image model">{input("image_model")}</Field>
        <Field label="Image price" hint="USD per image">{input("image_price", { type: "number", min: 0, step: 0.001 })}</Field>
        <Field label="Video model">{input("video_model")}</Field>
        <Field label="Video price" hint="USD per second">{input("video_price_per_sec", { type: "number", min: 0, step: 0.01 })}</Field>
        <div className="full row wrap" style={{ gap: 16 }}>
          {meta && [
            ["Claude Code CLI", meta.env.claude], ["ffmpeg", meta.env.ffmpeg], ["GEMINI_API_KEY", meta.env.gemini], ["ELEVENLABS_API_KEY", meta.env.eleven],
            ["Claude token or API key (else CLI login)", meta.env.anthropic], ["Agent full autonomy (bypass)", meta.env.bypass], ["App password", meta.env.password],
          ].map(([label, ok]) => (
            <span key={String(label)} className="row small">
              {ok ? <CheckCircleIcon size={15} color="var(--success)" /> : <XCircleIcon size={15} color="var(--faint-fg)" />} {label}
            </span>
          ))}
          {meta && <span className="small faint">HyperFrames {meta.hf_version} · data in <span className="mono">{meta.data_dir}</span></span>}
        </div>
      </Section>

      <Section title="Voice, sound & music (ElevenLabs)" sub={meta?.env.eleven ? "The agent narrates with the voice for the video's language. Prices are what your ElevenLabs plan works out to." : "Add your ElevenLabs key under Keys to enable voiceover, sound effects and music."}>
        <Field label="Bangla voice" hint="for videos in Bangla">{voicePicker("voice_bn", "bn")}</Field>
        <Field label="Default voice" hint="English and other languages">{voicePicker("voice_default", "en")}</Field>
        <Field label="Voice model" hint="eleven_v4 speaks Bangla">{input("voice_model")}</Field>
        <Field label="Voiceover price" hint="USD per 1,000 characters">{input("voice_price_per_1k", { type: "number", min: 0, step: 0.01 })}</Field>
        <Field label="Sound effect price" hint="USD per second">{input("sfx_price_per_sec", { type: "number", min: 0, step: 0.001 })}</Field>
        <Field label="Music price" hint="USD per minute">{input("music_price_per_min", { type: "number", min: 0, step: 0.01 })}</Field>
        {voiceErr && <p className="small full" style={{ color: "var(--danger)" }}>{voiceErr}</p>}
      </Section>

      <Section title="Rendering" sub="On this machine Chrome has no GPU, so frame capture runs on the CPU and is the bottleneck. A 1-minute 1080×1920 reel takes about 3 min to export, 2 min as a draft.">
        <Field label="Parallel workers" hint="auto measures your machine; more is not faster on CPU-only capture">{input("render_workers", { placeholder: "auto" })}</Field>
        <Field label="Draft frame rate" hint="fps for draft renders; 15 is ~40% faster, finals stay 30">{input("render_draft_fps", { type: "number", min: 0, max: 60 })}</Field>
        <Field label="GPU encoding" hint="VAAPI/NVENC: ~9% faster here, but files ~50% bigger">
          <Seg value={s.render_gpu ?? "false"} options={[{ value: "false", label: "Off" }, { value: "true", label: "On" }]} onChange={(v) => setS({ ...s, render_gpu: v })} />
        </Field>
        <Field label="Fast capture" hint="experimental; needs a hardware GPU in Chrome (no gain here)">
          <Seg value={s.render_fast_capture ?? "false"} options={[{ value: "false", label: "Off" }, { value: "true", label: "On" }]} onChange={(v) => setS({ ...s, render_fast_capture: v })} />
        </Field>
      </Section>

      <Section title="Currency" sub="Costs are recorded in USD, which is what AI providers charge, and shown in this currency.">
        <Field label="Currency" hint="a code such as USD, EUR, GBP, INR or BDT">{input("currency", { maxLength: 3 })}</Field>
        <Field label="Exchange rate" hint="units of that currency per 1 USD (1 for USD)">{input("fx_rate", { type: "number", min: 0, step: 0.01 })}</Field>
      </Section>

      <McpSection password={!!meta?.env.password} />
      <UpdatesSection />
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

interface KeyInfo { name: string; label: string; hint: string; set: boolean; last4?: string; source: "" | "app" | "env" }

/** Which keys belong together on the page. `guide` is the section of the connect guide that explains where to get them. */
const keyGroups: { title: string; sub: string; Icon: Icon; keys: string[]; social?: boolean; guide?: string }[] = [
  { title: "Claude", sub: "Runs the editor agent and writes captions and replies. One of the two is enough.", Icon: SparkleIcon, keys: ["ANTHROPIC_API_KEY", "CLAUDE_CODE_OAUTH_TOKEN"] },
  { title: "Gemini", sub: "Images, video clips and smarter website scans. Optional.", Icon: GoogleLogoIcon, keys: ["GEMINI_API_KEY"] },
  { title: "ElevenLabs", sub: "Voiceover, sound effects and music. Optional.", Icon: WaveformIcon, keys: ["ELEVENLABS_API_KEY"] },
  { title: "Facebook, Instagram & WhatsApp", sub: "Your Meta app.", Icon: FacebookLogoIcon, social: true, guide: "meta", keys: ["META_APP_ID", "META_APP_SECRET", "META_CONFIG_ID", "META_VERIFY_TOKEN"] },
  { title: "YouTube", sub: "Your Google Cloud OAuth client.", Icon: YoutubeLogoIcon, social: true, guide: "youtube-steps", keys: ["GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET"] },
  { title: "TikTok", sub: "Your TikTok developer app.", Icon: TiktokLogoIcon, social: true, guide: "tiktok-steps", keys: ["TIKTOK_CLIENT_KEY", "TIKTOK_CLIENT_SECRET"] },
  { title: "LinkedIn", sub: "Your LinkedIn developer app.", Icon: LinkedinLogoIcon, social: true, guide: "linkedin-steps", keys: ["LINKEDIN_CLIENT_ID", "LINKEDIN_CLIENT_SECRET"] },
  { title: "X", sub: "Your X developer app.", Icon: XLogoIcon, social: true, guide: "x-steps", keys: ["X_CLIENT_ID", "X_CLIENT_SECRET"] },
  { title: "Pinterest", sub: "Your Pinterest developer app.", Icon: PinterestLogoIcon, social: true, guide: "pinterest-steps", keys: ["PINTEREST_APP_ID", "PINTEREST_APP_SECRET"] },
];

/** Provider keys, saved on their own (not with "Save changes"): encrypted on this computer, shown only as their last 4.
 *  Grouped by provider; a social platform that isn't set up stays folded, since most people use only a few. */
function KeysSection() {
  const { data, setData } = useApi<KeyInfo[]>("/keys");
  const [editing, setEditing] = useState<Record<string, string>>({});
  const stop = (name: string) => setEditing(({ [name]: _, ...rest }) => rest);
  async function save(name: string, value: string) {
    try {
      setData(await api<KeyInfo[]>("/keys", { method: "PUT", body: { [name]: value.trim() } }));
      stop(name);
      toast(value ? "Key saved" : "Key removed");
    } catch (e) { fail(e); }
  }
  const byName = new Map(data?.map((k) => [k.name, k]));
  const grouped = new Set(keyGroups.flatMap((g) => g.keys));
  const others = data?.filter((k) => !grouped.has(k.name)) ?? []; // a key added in the backend before it's given a group here
  const row = (k: KeyInfo) => (
    <div className="item wrap-row" key={k.name} style={{ paddingLeft: 52 }}>
      <div className="spacer" style={{ minWidth: 220 }}>
        <strong>{k.label}</strong>
        <div className="small faint">{k.hint}</div>
      </div>
      {editing[k.name] !== undefined ? (
        <span className="row">
          <input className="input mono" type="password" autoFocus value={editing[k.name]} placeholder="Paste the key" style={{ width: 260 }} aria-label={k.label}
            onChange={(e) => setEditing({ ...editing, [k.name]: e.target.value })}
            onKeyDown={(e) => { if (e.key === "Enter" && editing[k.name].trim()) save(k.name, editing[k.name]); if (e.key === "Escape") stop(k.name); }} />
          <button className="btn primary sm" disabled={!editing[k.name].trim()} onClick={() => save(k.name, editing[k.name])}>Save</button>
          <button className="btn ghost sm" onClick={() => stop(k.name)}>Cancel</button>
        </span>
      ) : (
        <span className="row">
          <span className="small mono muted nowrap">{k.set ? `••••${k.last4}${k.source === "env" ? " · from .env" : ""}` : "not set"}</span>
          <button className="btn sm" onClick={() => setEditing({ ...editing, [k.name]: "" })}>{k.set ? "Replace" : "Add"}</button>
          {k.source === "app" && <button className="btn ghost sm danger" onClick={() => confirm(`Remove the ${k.label}?`) && save(k.name, "")}>Remove</button>}
        </span>
      )}
    </div>
  );
  const group = (g: (typeof keyGroups)[number]) => {
    const keys = g.keys.map((n) => byName.get(n)).filter((k): k is KeyInfo => !!k);
    const set = keys.filter((k) => k.set).length;
    const ready = g.title === "Claude" ? set > 0 : g.title.startsWith("Facebook") ? !!byName.get("META_APP_ID")?.set && !!byName.get("META_APP_SECRET")?.set : set === keys.length;
    const head = (
      <>
        <span style={{ width: 30, height: 30, borderRadius: 6, background: "var(--muted)", display: "grid", placeItems: "center", flex: "none" }}><g.Icon size={18} weight="fill" style={{ color: "var(--fg)" }} /></span>
        <div className="spacer">
          <strong>{g.title}</strong>
          <div className="small faint">
            {g.sub}
            {g.guide && <> <a href={`${connectGuideURL}#${g.guide}`} target="_blank" rel="noreferrer" style={{ color: "var(--primary)" }} onClick={(e) => e.stopPropagation()}>How to get these</a></>}
          </div>
        </div>
        {ready ? <span className="pill green"><span className="dot" />Ready</span> : set ? <span className="pill amber"><span className="dot" />Incomplete</span> : <span className="small faint nowrap">Not set up</span>}
      </>
    );
    if (!g.social) {
      return <div key={g.title}><div className="item">{head}</div>{keys.map(row)}</div>;
    }
    return (
      <details key={g.title} open={set > 0} className="key-group">
        <summary className="item" style={{ cursor: "pointer", listStyle: "none" }}>{head}<CaretRightIcon size={14} className="caret" /></summary>
        {keys.map(row)}
      </details>
    );
  };
  return (
    <div className="card">
      <div className="card-h" style={{ display: "block" }}>
        <h2>Keys</h2>
        <p className="small muted">Encrypted on this computer and sent only to the provider each belongs to. The agent needs a Claude key or token; the rest are optional.</p>
      </div>
      <div className="filelist">
        <div className="item eyebrow" style={{ background: "var(--muted)" }}>AI providers</div>
        {keyGroups.filter((g) => !g.social).map(group)}
        <div className="item eyebrow" style={{ background: "var(--muted)" }}>Social platforms: your own developer apps</div>
        {keyGroups.filter((g) => g.social).map(group)}
        {others.map(row)}
      </div>
    </div>
  );
}

interface Update { current: string; latest?: string; notes?: string; url?: string; newer?: boolean; error?: string }

function UpdatesSection() {
  const { data: upd } = useApi<Update>("/update");
  return (
    <div className="card">
      <div className="card-h" style={{ display: "block" }}><h2>Updates</h2></div>
      <div className="card-b stack" style={{ gap: 12 }}>
        <p className="small muted">
          37Studio {upd?.current}
          {upd?.newer ? <> · <strong>version {upd.latest} is available.</strong> Close 37Studio and open it again from the launcher to update.{upd.notes && <> {upd.notes}</>}</> :
            upd?.error ? " · couldn't check for updates right now" : upd?.latest ? " · up to date" : ""}
        </p>
        <p className="small muted">37Studio is free and open source under the MIT License. Report a bug, suggest a feature or send a pull request on <a href={repoURL} target="_blank" rel="noreferrer">GitHub</a>. It's made by <a href={makerURL} target="_blank" rel="noreferrer">Shaitrish</a> and funded by <a href={donateURL} target="_blank" rel="noreferrer">donations</a>.</p>
      </div>
    </div>
  );
}

/** Other agents (Claude Code, Claude Desktop, Cursor…) can run 37Studio through its MCP server: the tools the Assistant uses. */
function McpSection({ password }: { password: boolean }) {
  const url = `${location.origin}/mcp`;
  const cmd = `claude mcp add --transport http 37studio ${url}` + (password ? ` --header "Authorization: Basic <base64 of 37studio:your-password>"` : "");
  return (
    <div className="card">
      <div className="card-h" style={{ display: "block" }}>
        <h2>Connect other AI agents (MCP)</h2>
        <p className="small muted">Claude Code, Claude Desktop, Cursor and other MCP clients on this computer can manage companies, campaigns, videos, publishing and settings here, with the same tools as the Assistant.</p>
      </div>
      <div className="card-b stack" style={{ gap: 10 }}>
        <Field label="Server URL"><input className="input mono" readOnly value={url} onFocus={(e) => e.target.select()} /></Field>
        <Field label="Claude Code" hint="run in a terminal"><input className="input mono" readOnly value={cmd} onFocus={(e) => e.target.select()} /></Field>
        <p className="small faint" style={{ margin: 0 }}>
          It accepts connections from this computer only{password ? ", with the app's password" : ""}. Agents connected here can spend on your AI keys and post to your accounts, so connect only ones you trust.
        </p>
      </div>
    </div>
  );
}

