import { useState } from "react";
import { MicrophoneIcon, PencilSimpleIcon, PlusIcon, StarIcon, TrashIcon, WarningIcon } from "@phosphor-icons/react";
import { api, BrandVoice, bytes, fail, languageName, languages, Meta, Tenant, toast, upload, useApi, Voice } from "../api";
import { DropZone, Empty, Field, Modal, Progress } from "../ui";

const spoken = (v: Pick<BrandVoice, "language">) => (v.language ? languageName(v.language) : "Any language");

/** The brand's voices: the editor agent speaks voiceovers in them, and can switch a finished video to one. */
export function VoicesTab({ tenant }: { tenant: Tenant }) {
  const { data: voices, reload } = useApi<BrandVoice[]>(`/tenants/${tenant.id}/voices`);
  const { data: meta } = useApi<Meta>("/meta");
  const [open, setOpen] = useState<"clone" | "pick" | BrandVoice>();
  const toggleDefault = (v: BrandVoice) => api(`/brand-voices/${v.id}`, { method: "PUT", body: { is_default: !v.is_default } }).then(reload, fail);
  async function remove(v: BrandVoice) {
    if (!confirm(v.cloned
      ? `Delete the voice "${v.name}"? It is also deleted from your ElevenLabs account. Videos already made keep their audio.`
      : `Remove "${v.name}" from ${tenant.name}? It stays in your ElevenLabs account.`)) return;
    api(`/brand-voices/${v.id}`, { method: "DELETE" }).then(reload, fail);
  }
  return (
    <div className="stack">
      {meta && !meta.env.eleven && (
        <div className="card card-b row" style={{ background: "var(--danger-soft)", borderColor: "var(--border)" }}>
          <WarningIcon size={16} color="var(--danger)" style={{ flex: "none" }} />
          <span className="spacer small">Voices need an ElevenLabs key. Add it in Settings → AI services, then come back here.</span>
          <a className="btn sm" href="#/settings">Open Settings</a>
        </div>
      )}
      <div className="card">
        <div className="card-h">
          <h2>Voices</h2>
          <span className="small faint spacer">the agent speaks voiceovers in the default for the video's language; name another in chat to use it, even on a finished video</span>
        </div>
        {voices?.length ? (
          <div className="filelist">
            {voices.map((v) => (
              <div className="item" key={v.id} style={{ flexWrap: "wrap" }}>
                <div className="spacer" style={{ minWidth: 180 }}>
                  <div className="row" style={{ gap: 8 }}>
                    <strong className="ellipsis">{v.name}</strong>
                    {!!v.is_default && <span className="pill green">Default for {v.language ? languageName(v.language) : "every language"}</span>}
                  </div>
                  <div className="small muted ellipsis">{[spoken(v), v.cloned ? "cloned here" : "from ElevenLabs", v.notes].filter(Boolean).join(" · ")}</div>
                </div>
                {v.samples[0] && <audio src={v.samples[0].url} controls preload="none" style={{ height: 32, maxWidth: 260 }} aria-label={`Sample of ${v.name}`} />}
                <button className="btn ghost sm icon" title={v.is_default ? "Stop using as default" : `Make the default for ${spoken(v).toLowerCase()}`}
                  aria-label={v.is_default ? `Stop using ${v.name} as default` : `Make ${v.name} the default`} onClick={() => toggleDefault(v)}>
                  <StarIcon size={14} weight={v.is_default ? "fill" : "regular"} />
                </button>
                <button className="btn ghost sm icon" title="Edit" aria-label={`Edit ${v.name}`} onClick={() => setOpen(v)}><PencilSimpleIcon size={14} /></button>
                <button className="btn ghost sm icon" title="Delete" aria-label={`Delete ${v.name}`} onClick={() => remove(v)}><TrashIcon size={14} /></button>
              </div>
            ))}
          </div>
        ) : (
          <Empty icon={<MicrophoneIcon size={28} />} title="No voices for this brand yet">
            Clone the founder's or presenter's voice from a short recording, or pick a voice from your ElevenLabs account. Until then, videos use the voice in Settings.
          </Empty>
        )}
        <div className="card-b row wrap" style={{ borderTop: "1px solid var(--border)" }}>
          <button className="btn primary" disabled={!meta?.env.eleven} onClick={() => setOpen("clone")}><MicrophoneIcon size={15} /> Clone a voice</button>
          <button className="btn" disabled={!meta?.env.eleven} onClick={() => setOpen("pick")}><PlusIcon size={15} /> Add an ElevenLabs voice</button>
        </div>
      </div>
      {open && <VoiceModal tenant={tenant} mode={open} onClose={() => setOpen(undefined)} onSaved={() => { setOpen(undefined); reload(); }} />}
    </div>
  );
}

function VoiceModal({ tenant, mode, onClose, onSaved }: { tenant: Tenant; mode: "clone" | "pick" | BrandVoice; onClose: () => void; onSaved: () => void }) {
  const editing = typeof mode === "object" ? mode : undefined;
  const { data: account } = useApi<Voice[]>(mode === "pick" ? "/voices" : null);
  const [name, setName] = useState(editing?.name ?? "");
  const [language, setLanguage] = useState(editing?.language ?? tenant.language ?? "");
  const [notes, setNotes] = useState(editing?.notes ?? "");
  const [isDefault, setIsDefault] = useState(editing ? !!editing.is_default : true);
  const [voiceId, setVoiceId] = useState("");
  const [samples, setSamples] = useState<File[]>([]);
  const [noise, setNoise] = useState(false);
  const [pct, setPct] = useState<number>();

  async function save() {
    try {
      const fields = { name: name.trim(), language, notes: notes.trim(), is_default: isDefault };
      if (editing) {
        await api(`/brand-voices/${editing.id}`, { method: "PUT", body: fields });
        toast("Voice saved");
      } else if (mode === "pick") {
        await api(`/tenants/${tenant.id}/voices`, { body: { ...fields, voice_id: voiceId } });
        toast("Voice added");
      } else {
        setPct(0);
        const q = new URLSearchParams({ name: fields.name, language, notes: fields.notes, default: isDefault ? "1" : "", remove_noise: noise ? "1" : "" });
        await upload(`/tenants/${tenant.id}/voices/clone?${q}`, samples, setPct);
        toast(`Cloned ${fields.name}'s voice`);
      }
      onSaved();
    } catch (e) { fail(e); setPct(undefined); }
  }

  const ready = editing ? !!name.trim() : mode === "pick" ? !!voiceId : !!name.trim() && samples.length > 0;
  const title = editing ? `Edit ${editing.name}` : mode === "pick" ? "Add an ElevenLabs voice" : "Clone a voice";
  const sub = editing ? undefined : mode === "pick" ? "Any voice in your ElevenLabs account: premade, from their voice library, or cloned there."
    : "ElevenLabs makes a copy of the voice from a recording. It's made once and reused in every video.";
  return (
    <Modal title={title} sub={sub} onClose={onClose}
      footer={<><button className="btn" onClick={onClose}>Cancel</button>
        <button className="btn primary" disabled={!ready || pct !== undefined} onClick={save}>{pct !== undefined ? "Cloning…" : editing ? "Save" : mode === "pick" ? "Add voice" : "Clone voice"}</button></>}>
      <div className="stack" style={{ gap: 14 }}>
        {mode === "pick" && (
          <Field label="Voice">
            <select className="select" value={voiceId} onChange={(e) => {
              setVoiceId(e.target.value);
              const v = account?.find((x) => x.id === e.target.value);
              if (v && !name) setName(v.name);
            }}>
              <option value="">{account ? "Pick a voice" : "Loading your voices…"}</option>
              {account?.map((v) => <option key={v.id} value={v.id}>{[v.name, v.language, v.gender, v.use].filter(Boolean).join(" · ")}</option>)}
            </select>
          </Field>
        )}
        <Field label="Name" hint="ask the agent for it by this name">
          <input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="Rahim" autoFocus={mode !== "pick"} />
        </Field>
        <div className="grid cols-2">
          <Field label="Speaks">
            <select className="select" value={language} onChange={(e) => setLanguage(e.target.value)}>
              <option value="">Any language</option>
              {languages.map(([c, l]) => <option key={c} value={c}>{l}</option>)}
            </select>
          </Field>
          <Field label="Notes" hint="optional">
            <input className="input" value={notes} onChange={(e) => setNotes(e.target.value)} placeholder="Founder, warm and calm" />
          </Field>
        </div>
        {mode === "clone" && (
          <div className="stack" style={{ gap: 8 }}>
            <div className="small muted">Recordings · 1–3 minutes of only this person speaking clearly, without music. Audio or video files both work.</div>
            {samples.length > 0 && (
              <div className="filelist">
                {samples.map((f, i) => (
                  <div className="item" key={i}>
                    <span className="spacer ellipsis">{f.name}</span><span className="small faint">{bytes(f.size)}</span>
                    <button className="btn ghost sm icon" aria-label={`Remove ${f.name}`} onClick={() => setSamples(samples.filter((_, j) => j !== i))}><TrashIcon size={14} /></button>
                  </div>
                ))}
              </div>
            )}
            {pct === undefined ? <DropZone compact accept="audio/*,video/*" label="Add recordings" onFiles={(l) => setSamples([...samples, ...l])} /> : <Progress pct={pct} />}
            <label className="row small"><input type="checkbox" checked={noise} onChange={(e) => setNoise(e.target.checked)} /> Remove background noise first</label>
            <p className="small faint">Only clone a voice you have the speaker's permission to use. Cloning needs a paid ElevenLabs plan.</p>
          </div>
        )}
        <label className="row small"><input type="checkbox" checked={isDefault} onChange={(e) => setIsDefault(e.target.checked)} /> Use it by default for {language ? languageName(language) : "every language"} videos</label>
      </div>
    </Modal>
  );
}
