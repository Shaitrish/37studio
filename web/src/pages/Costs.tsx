import { ChartPieSliceIcon } from "@phosphor-icons/react";
import { ReactNode } from "react";
import { cash, useApi, useTenants } from "../api";
import { Empty, TenantTag } from "../ui";

interface Row { provider: string; item: string; n: number; qty: number; cost: number }
interface Costs {
  by_provider: Row[];
  by_tenant: { tenant_id: number; name: string; cost: number; projects: number }[];
  by_month: { month: string; provider: string; cost: number }[];
  total: number; this_month: number;
}

const providerName: Record<string, string> = { anthropic: "Claude", gemini: "Gemini", elevenlabs: "ElevenLabs", opencode: "OpenCode", codex: "Codex (ChatGPT plan)", fal: "fal.ai" };

export default function Costs() {
  const { data: c } = useApi<Costs>("/costs");
  const tenants = useTenants();
  if (!c) return null;

  const drivers = [
    ...c.by_provider.map((r) => ({
      label: `${providerName[r.provider] ?? r.provider} · ${r.item}`,
      detail: ["anthropic", "opencode", "codex"].includes(r.provider) ? `${r.n} runs · ${(r.qty / 1e6).toFixed(1)}M tokens` : `${r.n} generations`,
      cost: r.cost,
    })),
  ].filter((d) => d.cost > 0).sort((a, b) => b.cost - a.cost);
  const projects = c.by_tenant.reduce((n, t) => n + t.projects, 0);
  const months = [...new Set(c.by_month.map((m) => m.month))].reverse();
  const providers = [...new Set(c.by_month.map((m) => m.provider))];

  return (
    <div className="page">
      <div className="page-head">
        <div className="spacer">
          <h1>Costs</h1>
          <p>What your videos, captions and replies cost in AI usage, and where it goes. Rendering runs on this computer, so it is free.</p>
        </div>
      </div>

      <div className="grid stats">
        <div className="card stat"><div className="label">This month</div><div className="value">{cash(c.this_month)}</div><div className="sub">since the 1st</div></div>
        <div className="card stat"><div className="label">All time</div><div className="value">{cash(c.total)}</div><div className="sub">{projects} project{projects === 1 ? "" : "s"}</div></div>
        <div className="card stat"><div className="label">Per project</div><div className="value">{cash(projects ? c.total / projects : 0)}</div><div className="sub">average, including inbox replies</div></div>
      </div>

      <div className="grid cols-2">
        <div className="card">
          <div className="card-h"><h2>Where the money goes</h2></div>
          <div className="card-b">
            {drivers.length ? <Bars rows={drivers.map((d) => ({ label: d.label, sub: d.detail, value: d.cost }))} /> :
              <Empty icon={<ChartPieSliceIcon size={28} />} title="No spend yet" />}
          </div>
        </div>
        <div className="card">
          <div className="card-h"><h2>Spend by brand</h2></div>
          <div className="card-b">
            {c.by_tenant.length ? <Bars rows={c.by_tenant.map((t) => ({ label: <TenantTag tenants={tenants} id={t.tenant_id} name={t.name} />, key: String(t.tenant_id), sub: `${t.projects} project${t.projects === 1 ? "" : "s"}`, value: t.cost }))} /> :
              <p className="small muted">No usage yet.</p>}
          </div>
        </div>
      </div>

      {months.length > 0 && (
        <div className="card">
          <div className="card-h"><h2>By month</h2></div>
          <div className="table-wrap">
            <table className="table">
              <thead><tr><th>Month</th>{providers.map((p) => <th key={p} className="num">{providerName[p] ?? p}</th>)}<th className="num">Total</th></tr></thead>
              <tbody>
                {months.map((m) => {
                  const rows = c.by_month.filter((x) => x.month === m);
                  return (
                    <tr key={m}>
                      <td>{new Date(m + "-01T00:00:00").toLocaleDateString(undefined, { month: "long", year: "numeric" })}</td>
                      {providers.map((p) => <td key={p} className="num">{cash(rows.find((r) => r.provider === p)?.cost ?? 0)}</td>)}
                      <td className="num"><strong>{cash(rows.reduce((n, r) => n + r.cost, 0))}</strong></td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}

function Bars({ rows }: { rows: { label: ReactNode; key?: string; sub: string; value: number }[] }) {
  const max = Math.max(...rows.map((r) => r.value), 0.0001);
  return (
    <div className="bars">
      {rows.map((r) => (
        <div className="bar-row" key={r.key ?? String(r.label)}>
          <div style={{ minWidth: 0 }}><div className="ellipsis">{r.label}</div><div className="small faint ellipsis">{r.sub}</div></div>
          <div className="bar-track"><div className="bar-fill" style={{ width: `${(r.value / max) * 100}%` }} /></div>
          <strong className="nowrap" style={{ fontVariantNumeric: "tabular-nums" }}>{cash(r.value)}</strong>
        </div>
      ))}
    </div>
  );
}
