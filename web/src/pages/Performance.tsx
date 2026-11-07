import { type ReactNode, useEffect, useState } from "react";
import {
  ArrowSquareOutIcon, ArrowsClockwiseIcon, CaretDownIcon, CaretUpIcon, ChartLineUpIcon, FacebookLogoIcon, InstagramLogoIcon, TiktokLogoIcon, TrendDownIcon,
  TrendUpIcon, WarningIcon, YoutubeLogoIcon,
} from "@phosphor-icons/react";
import { ago, api, date, fail, openExternal, parseTime, Tenant, toast, useApi, useTenants } from "../api";
import { Empty, HelpButton, Md, Seg, TenantPicker } from "../ui";

/** One video on a connected account with the platform's own numbers (stats.go). null: the platform doesn't report it. */
interface Stat {
  media_id: string; platform: string; title: string; url: string; thumb: string; published_at: string; duration: number | null;
  views: number | null; reach: number | null; likes: number | null; comments: number | null; shares: number | null; saves: number | null;
  watch_avg: number | null; retention: number | null; skip_rate: number | null; curve: number[] | null;
  account: string; brand: string; tenant_id: number; engagement: number; engagement_rate: number | null; project_id: number | null; project: string | null;
}
interface PlatformSum { platform: string; videos: number; views: number; engagement: number; engagement_rate: number | null; retention: number | null }
interface StatsData {
  items: Stat[];
  summary: { videos: number; views: number; engagement: number; engagement_rate: number | null; retention: number | null; updated_at: string; platforms: PlatformSum[] };
  errors: { account: string; platform: string; brand: string; error: string }[];
  running: boolean;
  reviewing?: boolean; review_error?: string; // the brand's strategy review (strategy.go), when one brand is picked
}
type SortKey = "recent" | "views" | "retention" | "watch" | "engagement" | "rate";

const platformInfo: Record<string, { label: string; Icon: typeof FacebookLogoIcon }> = {
  facebook: { label: "Facebook", Icon: FacebookLogoIcon }, instagram: { label: "Instagram", Icon: InstagramLogoIcon },
  youtube: { label: "YouTube", Icon: YoutubeLogoIcon }, tiktok: { label: "TikTok", Icon: TiktokLogoIcon },
};
const compact = new Intl.NumberFormat(undefined, { notation: "compact", maximumFractionDigits: 1 });
const n = (v: number | null | undefined) => (v == null ? "—" : compact.format(v));
const pct = (v: number | null | undefined) => (v == null ? "—" : `${v.toFixed(v < 10 ? 1 : 0)}%`);
const secs = (v: number | null | undefined) => (v == null ? "—" : v < 60 ? `${v.toFixed(1)}s` : `${Math.floor(v / 60)}m ${Math.round(v % 60)}s`);
const watched = (v: Stat) => (v.retention != null ? `${pct(v.retention)} watched` : v.skip_rate != null ? `${(100 - v.skip_rate).toFixed(0)}% past 3s` : "— watched");
const old = (s: Stat) => Date.now() - parseTime(s.published_at).getTime() > 72 * 3600e3;

/** How published videos did on each platform, so the next ones repeat what worked. The editor agent reads the same numbers. */
export default function Performance() {
  const tenants = useTenants();
  const [tenant, setTenant] = useState<number>();
  const [platform, setPlatform] = useState("");
  const [sort, setSort] = useState<{ key: SortKey; asc: boolean }>({ key: "views", asc: false });
  const q = new URLSearchParams({ sort: sort.key, order: sort.asc ? "asc" : "desc" });
  if (tenant) q.set("tenant", String(tenant));
  if (platform) q.set("platform", platform);
  const { data, reload } = useApi<StatsData>(`/stats?${q}`);
  // The platform filter's choices come from the unfiltered list, so picking Instagram doesn't hide TikTok.
  const [seen, setSeen] = useState<string[]>([]);
  useEffect(() => { if (data && !platform) setSeen(data.summary.platforms.map((p) => p.platform)); }, [data, platform]);
  useEffect(() => { // a refresh and a strategy review run in the background; check back until they're done
    if (!data?.running && !data?.reviewing) return;
    const t = setTimeout(reload, 4000);
    return () => clearTimeout(t);
  }, [data, reload]);
  const refresh = () => api("/stats/refresh", { method: "POST", body: { tenant_id: tenant ?? 0 } }).then(reload, fail);
  const by = (key: SortKey) => setSort((s) => ({ key, asc: s.key === key ? !s.asc : false }));

  const items = data?.items ?? [];
  const s = data?.summary;
  const ranked = [...items].filter((i) => i.views != null).sort((a, b) => (b.views ?? 0) - (a.views ?? 0));
  const best = ranked[0];
  const weakest = [...ranked].reverse().find(old) ?? (ranked.length > 1 ? ranked[ranked.length - 1] : undefined);

  return (
    <div className="page wide">
      <div className="page-head">
        <div className="spacer">
          <h1>Performance</h1>
          <p>How your published videos did: views, how much of each video people watched, and how many liked, commented, shared or saved it. 37Studio reads the numbers from Facebook, Instagram, YouTube and TikTok every 6 hours, and the editor agent uses them when it plans new videos.</p>
        </div>
        <HelpButton to="performance" />
        <TenantPicker tenants={tenants} value={tenant} all="All brands" onChange={setTenant} />
        <button className="btn" onClick={refresh} disabled={data?.running}><ArrowsClockwiseIcon size={15} /> {data?.running ? "Reading…" : "Refresh"}</button>
      </div>

      {!!data?.errors.length && (
        <div className="card card-b stack small" style={{ gap: 6, background: "var(--warn-soft)" }}>
          <strong className="row" style={{ gap: 6 }}><WarningIcon size={15} /> Some accounts couldn't be read</strong>
          {data.errors.some((e) => /permission|insights|scope|#10\b|#200\b/i.test(e.error)) &&
            <span>The numbers need extra permissions: Facebook and Instagram need read_insights and instagram_manage_insights in your Meta app, YouTube needs the YouTube Analytics API, TikTok needs video.list. Then connect the account again on the brand's Social tab. See <a className="link" href="#/guide/performance">Guide → Performance</a>.</span>}
          <ul style={{ margin: 0, paddingLeft: 18 }}>
            {data.errors.map((e, i) => <li key={i}><b>{e.account}</b> ({platformInfo[e.platform]?.label ?? e.platform}{tenant ? "" : `, ${e.brand}`}): <span className="muted">{e.error}</span></li>)}
          </ul>
        </div>
      )}

      {!data ? <p className="small muted">Loading…</p> : !s?.videos ? (
        <div className="card">
          <Empty icon={<ChartLineUpIcon size={28} />} title={data.running ? "Reading the numbers…" : "No numbers yet"}>
            {data.running ? "This takes a minute or two." : <>Connect the brand's Facebook Page, Instagram, YouTube or TikTok on its Social tab, then click <b>Refresh</b>.</>}
          </Empty>
        </div>
      ) : (
        <>
          <div className="grid stats">
            <div className="card stat"><div className="label">Videos</div><div className="value">{s.videos}</div><div className="sub">{s.platforms.map((p) => `${p.videos} ${platformInfo[p.platform]?.label ?? p.platform}`).join(" · ")}</div></div>
            <div className="card stat"><div className="label">Views</div><div className="value">{n(s.views)}</div><div className="sub">{s.videos ? `${n(s.views / s.videos)} per video` : ""}</div></div>
            <div className="card stat"><div className="label">Watched</div><div className="value">{pct(s.retention)}</div><div className="sub">of each video, on average</div></div>
            <div className="card stat"><div className="label">Engagement</div><div className="value">{pct(s.engagement_rate)}</div><div className="sub">{n(s.engagement)} likes, comments, shares and saves</div></div>
          </div>

          {best && weakest && best !== weakest && (
            <div className="grid cols-2">
              <Highlight label="Most viewed" icon={<TrendUpIcon size={15} />} v={best} />
              <Highlight label="Least viewed" icon={<TrendDownIcon size={15} />} v={weakest} />
            </div>
          )}

          {s.platforms.length > 1 && <PlatformCompare platforms={s.platforms} />}
          {tenant ? <Strategy tenant={tenant} reviewing={!!data.reviewing} error={data.review_error}
            onReview={() => api("/stats/review", { method: "POST", body: { tenant_id: tenant } }).then(reload, fail)} /> : null}

          <div className="card">
            <div className="card-h row wrap" style={{ gap: 8 }}>
              <h2 className="spacer">Videos</h2>
              {seen.length > 1 || platform ? (
                <Seg value={platform} onChange={setPlatform} options={[{ value: "", label: "All" }, ...Object.keys(platformInfo).filter((p) => p === platform || seen.includes(p)).map((p) => ({ value: p, label: platformInfo[p].label }))]} />
              ) : null}
              <span className="small faint">{s.updated_at ? `Updated ${ago(s.updated_at)}` : ""}</span>
            </div>
            <div className="table-wrap">
              <table className="table perf">
                <thead>
                  <tr>
                    <SortTh label="Video" k="recent" sort={sort} by={by} left />
                    <SortTh label="Views" k="views" sort={sort} by={by} />
                    <SortTh label="Watched" k="retention" sort={sort} by={by} title="Average share of the video people watched" />
                    <SortTh label="Avg watch" k="watch" sort={sort} by={by} title="Average time each view lasted" />
                    <th className="num">Likes</th><th className="num">Comments</th><th className="num">Shares</th><th className="num">Saves</th>
                    <SortTh label="Engagement" k="rate" sort={sort} by={by} title="Likes, comments, shares and saves per 100 views" />
                  </tr>
                </thead>
                <tbody>
                  {items.map((v) => (
                    <tr key={v.platform + v.media_id}>
                      <td><VideoCell v={v} brand={!tenant} /></td>
                      <td className="num"><strong>{n(v.views)}</strong></td>
                      <td className="num"><Retention v={v} /></td>
                      <td className="num">{secs(v.watch_avg)}</td>
                      <td className="num">{n(v.likes)}</td>
                      <td className="num">{n(v.comments)}</td>
                      <td className="num">{n(v.shares)}</td>
                      <td className="num">{n(v.saves)}</td>
                      <td className="num">{pct(v.engagement_rate)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </>
      )}
    </div>
  );
}

/** Each platform side by side: where the brand's videos get watched and engaged with most. */
function PlatformCompare({ platforms }: { platforms: PlatformSum[] }) {
  const perVideo = (p: PlatformSum) => p.views / Math.max(p.videos, 1);
  const top = (f: (p: PlatformSum) => number | null) => platforms.reduce<PlatformSum | undefined>((a, p) => (f(p) ?? -1) > (a ? f(a) ?? -1 : -1) ? p : a, undefined)?.platform;
  const most = top(perVideo), engaged = top((p) => p.engagement_rate), held = top((p) => p.retention);
  const star = (on: boolean) => (on ? <strong style={{ color: "var(--success)" }}> ★</strong> : null);
  return (
    <div className="card">
      <div className="card-h"><h2 className="spacer">By platform</h2><span className="small faint">★ best on that measure</span></div>
      <div className="table-wrap">
        <table className="table">
          <thead><tr><th>Platform</th><th className="num">Videos</th><th className="num">Views per video</th><th className="num">Watched</th><th className="num">Engagement</th></tr></thead>
          <tbody>
            {platforms.map((p) => {
              const P = platformInfo[p.platform]?.Icon ?? ChartLineUpIcon;
              return (
                <tr key={p.platform}>
                  <td><span className="row" style={{ gap: 6 }}><P size={15} />{platformInfo[p.platform]?.label ?? p.platform}</span></td>
                  <td className="num">{p.videos}</td>
                  <td className="num">{n(Math.round(perVideo(p)))}{star(p.platform === most)}</td>
                  <td className="num">{pct(p.retention)}{star(p.retention != null && p.platform === held)}</td>
                  <td className="num">{pct(p.engagement_rate)}{star(p.engagement_rate != null && p.platform === engaged)}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

/** The brand's strategy.md: what Claude and the team learned from the numbers. Every new video's agent follows it. */
function Strategy({ tenant, reviewing, error, onReview }: { tenant: number; reviewing: boolean; error?: string; onReview: () => void }) {
  const { data: t, reload } = useApi<Tenant & { strategy?: string }>(`/tenants/${tenant}`);
  const [text, setText] = useState<string>();
  useEffect(() => { if (!reviewing) reload(); }, [reviewing, reload]); // pick up what a review just wrote
  const save = () => api(`/tenants/${tenant}`, { method: "PUT", body: { strategy: text } }).then(() => { toast("Strategy saved"); setText(undefined); reload(); }, fail);
  const doc = t?.strategy?.trim() ?? "";
  return (
    <div className="card">
      <div className="card-h row wrap" style={{ gap: 8 }}>
        <div className="spacer">
          <h2>Strategy</h2>
          <div className="small faint">What the numbers taught {t?.name ?? "this brand"}. The editor agent follows it in every new video and adds lessons as it goes.</div>
        </div>
        {text === undefined && <button className="btn" onClick={() => setText(doc)}>Edit</button>}
        <button className="btn" onClick={onReview} disabled={reviewing} title="Claude reads these numbers and rewrites the What the numbers say section, about $0.05">
          <ChartLineUpIcon size={15} /> {reviewing ? "Claude is reading the numbers…" : "Update from the numbers"}
        </button>
      </div>
      <div className="card-b stack" style={{ gap: 8 }}>
        {error && <p className="small" style={{ color: "var(--danger)", margin: 0 }}>{error}</p>}
        {text !== undefined ? <>
          <textarea className="textarea" rows={12} value={text} onChange={(e) => setText(e.target.value)}
            placeholder={"## What we post\nExam-tip reels in Bangla, under 60 seconds.\n\n## Notes\n- Map explainers do best on Instagram."} />
          <div className="row" style={{ justifyContent: "flex-end", gap: 8 }}>
            <button className="btn" onClick={() => setText(undefined)}>Cancel</button>
            <button className="btn primary" onClick={save} disabled={text === doc}>Save</button>
          </div>
        </> : doc ? <Md text={doc} className="msg-agent md-doc" /> : (
          <p className="small muted" style={{ margin: 0 }}>Nothing yet. Click <b>Update from the numbers</b> and Claude writes down what works for this brand, or click <b>Edit</b> to write your own plan.</p>
        )}
      </div>
    </div>
  );
}

function SortTh({ label, k, sort, by, left, title }: { label: string; k: SortKey; sort: { key: SortKey; asc: boolean }; by: (k: SortKey) => void; left?: boolean; title?: string }) {
  const on = sort.key === k;
  return (
    <th className={left ? "" : "num"} title={title} aria-sort={on ? (sort.asc ? "ascending" : "descending") : undefined}>
      <button className={`sort-th ${on ? "on" : ""}`} onClick={() => by(k)}>
        {label}{on ? (sort.asc ? <CaretUpIcon size={11} /> : <CaretDownIcon size={11} />) : null}
      </button>
    </th>
  );
}

function Thumb({ v }: { v: Stat }) {
  const [broken, setBroken] = useState(false); // platform thumbnail links expire between refreshes
  const P = platformInfo[v.platform]?.Icon ?? ChartLineUpIcon;
  return v.thumb && !broken ? <img className="perf-thumb" src={v.thumb} alt="" loading="lazy" referrerPolicy="no-referrer" onError={() => setBroken(true)} />
    : <span className="perf-thumb"><P size={18} /></span>;
}

function VideoCell({ v, brand }: { v: Stat; brand: boolean }) {
  const P = platformInfo[v.platform]?.Icon;
  return (
    <div className="row" style={{ gap: 10, minWidth: 260, maxWidth: 440 }}>
      <Thumb v={v} />
      <div style={{ minWidth: 0 }}>
        <a className="perf-title" href={v.url} onClick={(e) => { e.preventDefault(); if (v.url) openExternal(v.url); }} title={v.title}>
          {v.title || "Untitled"} {v.url && <ArrowSquareOutIcon size={12} className="faint" />}
        </a>
        <div className="small faint row wrap" style={{ gap: 6 }}>
          {P && <P size={13} />}<span>{v.account}{brand ? ` · ${v.brand}` : ""}</span><span>{date(v.published_at)}</span>
          {v.project_id && <a className="link" href={`#/projects/${v.project_id}`}>{v.project || "37Studio project"}</a>}
        </div>
      </div>
    </div>
  );
}

function Retention({ v }: { v: Stat }) {
  const tip = [
    v.curve?.length ? `Still watching: ${v.curve.filter((_, i, a) => i % Math.ceil(a.length / 5) === 0).map((x) => `${Math.round(x)}%`).join(" → ")}` : "",
    v.skip_rate != null ? `${v.skip_rate.toFixed(0)}% of views left in the first 3 seconds` : "",
  ].filter(Boolean).join("\n");
  return (
    <span className="perf-ret" title={tip || undefined}>
      {v.retention != null && <span className="bar-track"><span className="bar-fill" style={{ width: `${Math.min(100, v.retention)}%` }} /></span>}
      {v.retention != null ? pct(v.retention) : v.skip_rate != null ? <span className="faint">{`${(100 - v.skip_rate).toFixed(0)}% past 3s`}</span> : "—"}
    </span>
  );
}

function Highlight({ label, icon, v }: { label: string; icon: ReactNode; v: Stat }) {
  return (
    <div className="card card-b row" style={{ gap: 12 }}>
      <Thumb v={v} />
      <div className="spacer" style={{ minWidth: 0 }}>
        <div className="small muted row" style={{ gap: 6 }}>{icon} {label}</div>
        <div className="perf-title" title={v.title}>{v.title || "Untitled"}</div>
        <div className="small muted">{n(v.views)} {v.views === 1 ? "view" : "views"} · {watched(v)} · {pct(v.engagement_rate)} engagement · {platformInfo[v.platform]?.label ?? v.platform}, {date(v.published_at)}</div>
      </div>
    </div>
  );
}
