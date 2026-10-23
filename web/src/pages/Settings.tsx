import { useEffect, useState } from "react";
import { CheckCircleIcon, XCircleIcon } from "@phosphor-icons/react";
import { api, fail, languageName, Meta, money, setDisplay, toast, useApi, Voice } from "../api";
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
          <p>Keys, AI providers, voice, rendering, currency and your license.</p>
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
        <Field label="Currency" hint="BDT for taka, or USD">{input("currency", { maxLength: 3 })}</Field>
        <Field label="Exchange rate" hint="taka per 1 USD">{input("fx_rate", { type: "number", min: 0, step: 0.01 })}</Field>
      </Section>

      <LicenseSection />
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

/** Provider keys, saved on their own (not with "Save changes"): encrypted on this computer, shown only as their last 4. */
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
  return (
    <div className="card">
      <div className="card-h" style={{ display: "block" }}>
        <h2>Keys</h2>
        <p className="small muted">Encrypted on this computer and sent only to the provider each belongs to. The agent needs a Claude key or token; the rest are optional.</p>
      </div>
      <div className="filelist">
        {data?.map((k) => (
          <div className="item wrap-row" key={k.name}>
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
        ))}
      </div>
    </div>
  );
}

interface Update { current: string; latest?: string; notes?: string; url?: string; newer?: boolean; error?: string }

function LicenseSection() {
  const { data: lic, setData } = useApi<{ valid: boolean; name?: string; email?: string }>("/license");
  const { data: upd } = useApi<Update>("/update");
  const [key, setKey] = useState("");
  const activate = () => api<{ valid: boolean; name?: string }>("/license", { method: "PUT", body: { key } })
    .then((l) => { setData(l); setKey(""); toast(`Activated for ${l.name}`); }, fail);
  return (
    <div className="card">
      <div className="card-h" style={{ display: "block" }}><h2>License & updates</h2></div>
      <div className="card-b stack" style={{ gap: 12 }}>
        {lic?.valid ? (
          <p className="row" style={{ gap: 8 }}><CheckCircleIcon size={16} color="var(--success)" /> Licensed to <strong>{lic.name}</strong>{lic.email && <span className="muted">· {lic.email}</span>}</p>
        ) : (
          <div className="row wrap">
            <input className="input mono spacer" value={key} onChange={(e) => setKey(e.target.value)} placeholder="Paste your license key (37S1.…)" aria-label="License key" style={{ minWidth: 260 }} />
            <button className="btn primary" disabled={!key.trim()} onClick={activate}>Activate</button>
          </div>
        )}
        <p className="small muted">
          37Studio {upd?.current}
          {upd?.newer ? <> · <strong>version {upd.latest} is available.</strong> Close 37Studio and open it again from the launcher to update.{upd.notes && <> {upd.notes}</>}</> :
            upd?.error ? " · couldn't check for updates right now" : upd?.latest ? " · up to date" : ""}
        </p>
      </div>
    </div>
  );
}
