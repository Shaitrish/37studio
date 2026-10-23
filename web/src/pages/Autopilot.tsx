import { useEffect, useState } from "react";
import { CheckIcon, LightningIcon, PencilSimpleIcon, PlayIcon, PlusIcon, TrashIcon, XIcon } from "@phosphor-icons/react";
import { ago, api, fail, languageName, languages, Meta, parseTime, SocialAccount, Tenant, toast, useApi } from "../api";
import { Empty, Field, Modal, Seg, Status } from "../ui";

export interface AutopilotVideo { id: number; title: string; autopilot_slot: string; autopilot_stage: string; created_at: string }
interface Plan {
  id: number; tenant_id: number; name: string; enabled: number; brief: string; kind: string; aspect: string; seconds: number; language: string;
  quality: string; days: string; at: string; accounts: number[]; approval: number; next?: string; videos: AutopilotVideo[];
}

const dayNames = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
const working = ["building", "rendering", "captions"];
const when = (t: string) => parseTime(t).toLocaleString(undefined, { weekday: "short", day: "numeric", month: "short", hour: "numeric", minute: "2-digit" });

export const approve = (id: number, then: () => void) => api(`/projects/${id}/approve`, { body: {} }).then(() => { toast("Approved: it posts on schedule"); then(); }, fail);
export const skip = (id: number, then: () => void) =>
  confirm("Skip this video? Its posts are cancelled; the video stays in the project.") && api(`/projects/${id}/skip`, { body: {} }).then(then, fail);

/** Company → Autopilot: content plans that make, caption and schedule videos on their own. */
export function AutopilotTab({ tenant }: { tenant: Tenant }) {
  const { data: plans, reload } = useApi<Plan[]>(`/autopilots?tenant=${tenant.id}`);
  const { data: social } = useApi<{ accounts: SocialAccount[] }>(`/tenants/${tenant.id}/social`);
  const [editing, setEditing] = useState<Partial<Plan>>();
  const busy = plans?.some((p) => p.videos?.some((v) => working.includes(v.autopilot_stage)));
  useEffect(() => {
    if (!busy) return;
    const t = setInterval(reload, 10_000);
    return () => clearInterval(t);
  }, [busy, reload]);
  const accountName = (id: number) => social?.accounts.find((a) => a.id === id)?.name ?? "disconnected account";
  const toggle = (p: Plan) => api(`/autopilots/${p.id}`, { method: "PUT", body: { enabled: !p.enabled } }).then(reload, fail);
  const remove = (p: Plan) => confirm(`Delete the plan "${p.name || "Autopilot"}"? Videos it already made stay.`) && api(`/autopilots/${p.id}`, { method: "DELETE" }).then(reload, fail);
  const run = (p: Plan) => api(`/autopilots/${p.id}/run`, { body: {} }).then(() => { toast("Autopilot is making a video; it takes a while"); reload(); }, fail);

  return (
    <div className="stack">
      <div className="row">
        <p className="small muted spacer">Autopilot makes a fresh video before each time slot: the agent picks a topic from your brief and knowledge base, builds and checks it, renders it and writes the captions. You approve with one click, or let it post on its own. It runs while 37Studio is open.</p>
        <button className="btn primary" onClick={() => setEditing({})}><PlusIcon size={15} /> New plan</button>
      </div>

      {plans && !plans.length && (
        <div className="card">
          <Empty icon={<LightningIcon size={28} />} title="No content plans yet">
            For example: "3 reels a week with one exam tip each, in Bangla, posted at 8pm to our Page and Instagram."
            <button className="btn primary" onClick={() => setEditing({})}><PlusIcon size={15} /> New plan</button>
          </Empty>
        </div>
      )}

      {plans?.map((p) => (
        <div className="card" key={p.id}>
          <div className="card-h row wrap" style={{ gap: 8 }}>
            <LightningIcon size={16} color={p.enabled ? "var(--primary)" : "var(--faint-fg)"} />
            <h2 className="spacer">{p.name || "Autopilot"}</h2>
            <Seg value={p.enabled ? "on" : "off"} onChange={() => toggle(p)} options={[{ value: "on", label: "On" }, { value: "off", label: "Paused" }]} />
            <button className="btn sm" onClick={() => run(p)} title="Make one video now"><PlayIcon size={13} /> Run now</button>
            <button className="btn ghost sm icon" onClick={() => setEditing(p)} aria-label="Edit plan" title="Edit"><PencilSimpleIcon size={14} /></button>
            <button className="btn ghost sm icon" onClick={() => remove(p)} aria-label="Delete plan" title="Delete"><TrashIcon size={14} /></button>
          </div>
          <div className="card-b stack" style={{ gap: 10 }}>
            <p className="small" style={{ whiteSpace: "pre-wrap" }}>{p.brief}</p>
            <p className="small muted">
              {p.days.split(",").filter(Boolean).map((d) => dayNames[Number(d)]).join(", ") || "No days"} at {p.at} · {p.aspect} · ~{p.seconds}s · {languageName(p.language || tenant.language)}
              {" · "}{p.accounts.length ? p.accounts.map(accountName).join(", ") : "no accounts picked"}
              {" · "}{p.approval ? "waits for your approval" : "posts automatically"}
              {p.enabled && p.next ? <> · next video for <strong>{when(p.next)}</strong></> : null}
            </p>
          </div>
          {!!p.videos?.length && (
            <div className="filelist" style={{ borderTop: "1px solid var(--border)" }}>
              {p.videos.map((v) => (
                <div className="item" key={v.id}>
                  <a className="spacer ellipsis" href={`#/projects/${v.id}`}>{v.title}</a>
                  <span className="small faint nowrap">{v.autopilot_slot ? when(v.autopilot_slot) : ago(v.created_at)}</span>
                  <Status value={v.autopilot_stage || "draft"} />
                  {v.autopilot_stage === "approve" && <>
                    <button className="btn sm primary" onClick={() => approve(v.id, reload)}><CheckIcon size={13} /> Approve</button>
                    <button className="btn ghost sm icon" onClick={() => skip(v.id, reload)} aria-label="Skip" title="Skip"><XIcon size={13} /></button>
                  </>}
                </div>
              ))}
            </div>
          )}
        </div>
      ))}

      {editing && <PlanModal tenant={tenant} plan={editing} accounts={(social?.accounts ?? []).filter((a) => a.platform !== "whatsapp")} onClose={() => setEditing(undefined)} onSaved={() => { setEditing(undefined); reload(); }} />}
    </div>
  );
}

function PlanModal({ tenant, plan, accounts, onClose, onSaved }: {
  tenant: Tenant; plan: Partial<Plan>; accounts: SocialAccount[]; onClose: () => void; onSaved: () => void;
}) {
  const { data: meta } = useApi<Meta>("/meta");
  const [f, setF] = useState({
    name: plan.name ?? "", brief: plan.brief ?? "", kind: plan.kind ?? "promo", aspect: plan.aspect ?? "9:16", seconds: plan.seconds ?? 30,
    language: plan.language ?? "", quality: plan.quality ?? "standard", at: plan.at ?? "20:00", approval: plan.approval ?? 1,
  });
  const [days, setDays] = useState(new Set((plan.days ?? "0,2,4").split(",").filter(Boolean).map(Number)));
  const [picked, setPicked] = useState(new Set(plan.accounts ?? accounts.map((a) => a.id)));
  const set = <K extends keyof typeof f>(k: K, v: (typeof f)[K]) => setF({ ...f, [k]: v });
  const flip = <T,>(s: Set<T>, v: T) => { const n = new Set(s); n.has(v) ? n.delete(v) : n.add(v); return n; };

  async function save() {
    const body = { ...f, tenant_id: tenant.id, days: [...days].sort(), accounts: [...picked], approval: !!f.approval, tz: Intl.DateTimeFormat().resolvedOptions().timeZone };
    try {
      await api(plan.id ? `/autopilots/${plan.id}` : "/autopilots", { method: plan.id ? "PUT" : "POST", body });
      toast("Plan saved");
      onSaved();
    } catch (e) { fail(e); }
  }

  return (
    <Modal title={plan.id ? "Edit content plan" : "New content plan"} sub="Autopilot follows this to make and schedule videos." size="lg" onClose={onClose}
      footer={<><button className="btn" onClick={onClose}>Cancel</button><button className="btn primary" disabled={!f.brief.trim() || !days.size} onClick={save}>Save plan</button></>}>
      <div className="form">
        <Field label="Name" hint="optional"><input className="input" value={f.name} onChange={(e) => set("name", e.target.value)} placeholder="Exam tips" /></Field>
        <Field label="Video type">
          <select className="select" value={f.kind} onChange={(e) => set("kind", e.target.value)}>
            {meta?.presets.filter((p) => p.id !== "thumbnail").map((p) => <option key={p.id} value={p.id}>{p.label}</option>)}
          </select>
        </Field>
        <Field label="What the videos are about" hint="topics, angle, call to action" full>
          <textarea className="textarea" rows={4} value={f.brief} onChange={(e) => set("brief", e.target.value)} autoFocus
            placeholder="One practical tip per video for BCS candidates, from our question bank and blog. Friendly, fast, Bangla captions. End with: practise daily in the BCS Mate app." />
        </Field>
        <Field label="Days">
          <div className="toggles">
            {dayNames.map((d, i) => (
              <button key={d} type="button" className={`chip ${days.has(i) ? "on" : ""}`} aria-pressed={days.has(i)} onClick={() => setDays(flip(days, i))}>{d}</button>
            ))}
          </div>
        </Field>
        <Field label="Post at" hint="your time"><input className="input" type="time" value={f.at} onChange={(e) => set("at", e.target.value)} /></Field>
        <Field label="Format">
          <Seg value={f.aspect} onChange={(v) => set("aspect", v)} options={["9:16", "1:1", "4:5", "16:9"].map((v) => ({ value: v, label: v }))} />
        </Field>
        <Field label="Length" hint="seconds"><input className="input" type="number" min={10} max={180} value={f.seconds} onChange={(e) => set("seconds", Number(e.target.value))} /></Field>
        <Field label="Language">
          <select className="select" value={f.language} onChange={(e) => set("language", e.target.value)}>
            <option value="">Company default ({languageName(tenant.language)})</option>
            {languages.map(([v, l]) => <option key={v} value={v}>{l}</option>)}
          </select>
        </Field>
        <Field label="Quality" hint={meta?.qualities.find((q) => q.id === f.quality)?.note}>
          <select className="select" value={f.quality} onChange={(e) => set("quality", e.target.value)}>
            {meta?.qualities.map((q) => <option key={q.id} value={q.id}>{q.label}</option>)}
          </select>
        </Field>
        <Field label="Post to" full>
          {accounts.length ? (
            <div className="toggles">
              {accounts.map((a) => (
                <button key={a.id} type="button" className={`chip ${picked.has(a.id) ? "on" : ""}`} aria-pressed={picked.has(a.id)} onClick={() => setPicked(flip(picked, a.id))}>
                  {a.name} · {a.platform}
                </button>
              ))}
            </div>
          ) : <span className="small muted">No social accounts yet: connect them on the Social tab. Videos are still made.</span>}
        </Field>
        <Field label="Before posting" full>
          <Seg value={f.approval ? "approve" : "auto"} onChange={(v) => set("approval", v === "approve" ? 1 : 0)}
            options={[{ value: "approve", label: "Wait for my approval" }, { value: "auto", label: "Post automatically" }]} />
        </Field>
      </div>
    </Modal>
  );
}
