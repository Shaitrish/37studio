import { useEffect, useState } from "react";
import { CaretRightIcon, MegaphoneIcon, PlusIcon, TrashIcon } from "@phosphor-icons/react";
import { api, Campaign, fail, go, Tenant, toast, cash, useApi } from "../api";
import { Empty, Field, Modal } from "../ui";
import { ProjectTable } from "./Projects";
import { NewProjectModal } from "./NewProject";

export default function Campaigns() {
  const { data: campaigns } = useApi<Campaign[]>("/campaigns");
  const [creating, setCreating] = useState(false);
  return (
    <div className="page">
      <div className="page-head">
        <div className="spacer">
          <h1>Campaigns</h1>
          <p>A launch, offer or product push for one of your companies, with all the videos made for it.</p>
        </div>
        <button className="btn primary" onClick={() => setCreating(true)}><PlusIcon size={16} /> New campaign</button>
      </div>
      <CampaignGrid campaigns={campaigns} onNew={() => setCreating(true)} />
      {creating && <CampaignModal onClose={() => setCreating(false)} />}
    </div>
  );
}

export function CampaignGrid({ campaigns, onNew, hideClient }: { campaigns?: Campaign[]; onNew: () => void; hideClient?: boolean }) {
  if (campaigns && !campaigns.length) {
    return (
      <div className="card">
        <Empty icon={<MegaphoneIcon size={28} />} title="No campaigns yet">
          Group videos around one product, launch or offer, so every video shares the same message.
          <button className="btn primary" onClick={onNew}><PlusIcon size={16} /> New campaign</button>
        </Empty>
      </div>
    );
  }
  return (
    <div className="grid cards">
      {campaigns?.map((c) => (
        <a key={c.id} href={`#/campaigns/${c.id}`} className="card client-card">
          <div className="row">
            <div className="avatar" style={{ background: "var(--primary-soft)", color: "var(--primary)" }}><MegaphoneIcon size={18} /></div>
            <div style={{ minWidth: 0 }}>
              <h2 className="ellipsis">{c.name}</h2>
              {!hideClient && <div className="small muted ellipsis">{c.tenant_name}</div>}
            </div>
          </div>
          <p className="small muted clamp">{c.brief || "No brief yet."}</p>
          <div className="small faint">{c.video_count} video{c.video_count === 1 ? "" : "s"}</div>
        </a>
      ))}
    </div>
  );
}

export function CampaignModal({ tenantId, onClose }: { tenantId?: number; onClose: () => void }) {
  const { data: tenants } = useApi<Tenant[]>("/tenants");
  const [tenant, setTenant] = useState(tenantId ?? 0);
  const [name, setName] = useState("");
  const [brief, setBrief] = useState("");
  const tid = tenant || tenants?.[0]?.id;
  async function save() {
    try {
      const c = await api<Campaign>("/campaigns", { body: { tenant_id: tid, name, brief } });
      go(`/campaigns/${c.id}`);
    } catch (e) { fail(e); }
  }
  return (
    <Modal title="New campaign" sub="Its brief goes to the agent on every video in the campaign." onClose={onClose}
      footer={<><button className="btn" onClick={onClose}>Cancel</button><button className="btn primary" disabled={!name.trim() || !tid} onClick={save}>Create campaign</button></>}>
      <div className="stack" style={{ gap: 14 }}>
        {!tenantId && (
          <Field label="Company">
            <select className="select" value={tid ?? ""} onChange={(e) => setTenant(Number(e.target.value))}>
              {tenants?.map((t) => <option key={t.id} value={t.id}>{t.name}</option>)}
            </select>
          </Field>
        )}
        <Field label="Campaign name"><input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="Eid offer 2027" autoFocus /></Field>
        <Field label="Brief" hint="what it promotes, offer, audience, CTA, dates">
          <textarea className="textarea" rows={5} value={brief} onChange={(e) => setBrief(e.target.value)}
            placeholder="30% off the HSC physics course until 15 April. Audience: HSC candidates and parents. CTA: enroll at course37.com. Tone: urgent but friendly." />
        </Field>
      </div>
    </Modal>
  );
}

export function CampaignPage({ id }: { id: number }) {
  const { data: c, error, reload } = useApi<Campaign>(`/campaigns/${id}`);
  const [name, setName] = useState("");
  const [brief, setBrief] = useState("");
  const [creating, setCreating] = useState(false);
  useEffect(() => { if (c) { setName(c.name); setBrief(c.brief); } }, [c?.name, c?.brief]);

  if (error) return <div className="page"><Empty icon={<MegaphoneIcon size={28} />} title="Campaign not found"><a className="btn" href="#/campaigns">Back to campaigns</a></Empty></div>;
  if (!c) return null;
  const dirty = name !== c.name || brief !== c.brief;
  const save = () => api(`/campaigns/${id}`, { method: "PUT", body: { name, brief } }).then(() => { toast("Campaign saved"); reload(); }, fail);
  async function remove() {
    if (!confirm(`Delete the campaign "${c!.name}"? Its videos are kept as one-off projects.`)) return;
    try { await api(`/campaigns/${id}`, { method: "DELETE" }); go("/campaigns"); } catch (e) { fail(e); }
  }

  return (
    <div className="page">
      <div className="page-head">
        <div className="spacer">
          <div className="crumbs"><a href="#/campaigns">Campaigns</a><CaretRightIcon size={14} /><a href={`#/clients/${c.tenant_id}`}>{c.tenant_name}</a></div>
          <h1>{c.name}</h1>
          <p>{c.video_count} video{c.video_count === 1 ? "" : "s"} · cost {cash(c.cost ?? 0)}</p>
        </div>
        <button className="btn primary" onClick={() => setCreating(true)}><PlusIcon size={16} /> New video</button>
      </div>

      <div className="card">
        <div className="card-h"><h2>Brief</h2><span className="small faint spacer">every video's agent reads this</span><button className="btn sm" disabled={!dirty} onClick={save}>Save</button></div>
        <div className="card-b stack" style={{ gap: 12 }}>
          <input className="input" value={name} onChange={(e) => setName(e.target.value)} aria-label="Campaign name" />
          <textarea className="textarea" rows={4} value={brief} onChange={(e) => setBrief(e.target.value)} placeholder="What it promotes, offer, audience, CTA, dates, tone." />
        </div>
      </div>

      <div className="card">
        <div className="card-h"><h2>Videos</h2></div>
        {c.projects?.length ? <ProjectTable projects={c.projects} hideClient /> : (
          <Empty icon={<MegaphoneIcon size={28} />} title="No videos in this campaign yet">
            <button className="btn primary" onClick={() => setCreating(true)}><PlusIcon size={16} /> New video</button>
          </Empty>
        )}
      </div>
      <div className="row"><span className="spacer" /><button className="btn ghost danger sm" onClick={remove}><TrashIcon size={14} /> Delete campaign</button></div>
      {creating && <NewProjectModal tenantId={c.tenant_id} campaignId={c.id} onClose={() => setCreating(false)} />}
    </div>
  );
}
