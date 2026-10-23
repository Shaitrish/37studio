import { useEffect, useState } from "react";
import { ArrowClockwiseIcon, ArrowSquareOutIcon, FacebookLogoIcon, Icon, InstagramLogoIcon, LinkBreakIcon, ShareNetworkIcon, SparkleIcon, TiktokLogoIcon, TrashIcon, WhatsappLogoIcon, YoutubeLogoIcon } from "@phosphor-icons/react";
import { ago, api, Captions, date, fail, parseTime, Platform, Post, Project, SocialAccount, toast, useApi } from "../api";
import { Empty, Field, Modal, Seg, Status } from "../ui";

const platforms: Record<Platform, { label: string; Icon: Icon; limit: number }> = {
  facebook: { label: "Facebook", Icon: FacebookLogoIcon, limit: 5000 },
  instagram: { label: "Instagram", Icon: InstagramLogoIcon, limit: 2200 },
  tiktok: { label: "TikTok", Icon: TiktokLogoIcon, limit: 2200 },
  youtube: { label: "YouTube", Icon: YoutubeLogoIcon, limit: 4900 },
  whatsapp: { label: "WhatsApp", Icon: WhatsappLogoIcon, limit: 4096 }, // Inbox only: videos never post here
};
const providers = [
  { id: "meta", label: "Facebook & Instagram", Icon: FacebookLogoIcon },
  { id: "google", label: "YouTube", Icon: YoutubeLogoIcon },
  { id: "tiktok", label: "TikTok", Icon: TiktokLogoIcon },
];
const tiktokPrivacy = [
  ["", "Choose who can see it"], ["PUBLIC_TO_EVERYONE", "Everyone"], ["FOLLOWER_OF_CREATOR", "Followers"],
  ["MUTUAL_FOLLOW_FRIENDS", "Friends"], ["SELF_ONLY", "Only me"], ["INBOX", "Send to TikTok drafts"],
];
interface SocialInfo { accounts: SocialAccount[]; ready: Record<string, boolean>; inbox_mode: string; replies: string; redirect: string; meta_app: string }
type Candidate = Pick<SocialAccount, "platform" | "account_id" | "name" | "picture">;

function AccountLabel({ a }: { a: Pick<SocialAccount, "platform" | "name" | "picture"> }) {
  const P = platforms[a.platform];
  return (
    <span className="row" style={{ gap: 10, minWidth: 0 }}>
      <span className="acct-pic">
        {/* Facebook's picture URLs expire after a while: fall back to the plain circle. */}
        {a.picture && <img src={a.picture} alt="" onError={(e) => { e.currentTarget.style.display = "none"; }} />}
        <P.Icon size={13} weight="fill" className="acct-badge" />
      </span>
      <span className="stack" style={{ gap: 0, minWidth: 0, textAlign: "left" }}>
        <span className="ellipsis" style={{ color: "var(--fg)" }}>{a.name}</span>
        <span className="small faint">{P.label}</span>
      </span>
    </span>
  );
}

/** Company → Social: connect accounts through each platform's own sign-in, then pick which belong to this company. */
export function SocialTab({ tenantId }: { tenantId: number }) {
  const { data, reload } = useApi<SocialInfo>(`/tenants/${tenantId}/social`);
  const pendingKey = location.hash.split("/")[4]; // the OAuth callback lands on #/clients/<id>/social/<key>
  const connect = (prov: string) => {
    location.href = `/api/oauth/${prov}/start?tenant=${tenantId}&return=${encodeURIComponent(location.origin + location.pathname)}`;
  };
  const disconnect = (a: SocialAccount) =>
    confirm(`Disconnect ${a.name} (${platforms[a.platform].label})? Posts already scheduled to it will fail.`) &&
    api(`/social/${a.id}`, { method: "DELETE" }).then(reload, fail);
  const missing = providers.filter((p) => data && !data.ready[p.id]);
  const [whatsapp, setWhatsapp] = useState(false);
  const [metaApp, setMetaApp] = useState(false);
  return (
    <div className="stack">
      <div className="card">
        <div className="card-h"><h2>Social accounts</h2><span className="small faint spacer">post finished videos to any mix of these from a project's Publish button</span></div>
        {data?.accounts.length ? (
          <div className="filelist">
            {data.accounts.map((a) => (
              <div className="item" key={a.id}>
                <span className="spacer" style={{ minWidth: 0 }}><AccountLabel a={a} /></span>
                <span className="small faint nowrap">Connected {date(a.created_at)}</span>
                <button className="btn ghost sm icon" title="Disconnect" aria-label={`Disconnect ${a.name}`} onClick={() => disconnect(a)}><LinkBreakIcon size={14} /></button>
              </div>
            ))}
          </div>
        ) : (
          <Empty icon={<ShareNetworkIcon size={28} />} title="No accounts connected">
            Sign in once per platform with an account that manages the company's Pages or channels.
          </Empty>
        )}
        <div className="card-b row wrap" style={{ borderTop: "1px solid var(--border)" }}>
          {providers.map((p) => (
            <button key={p.id} className="btn" disabled={!data?.ready[p.id]} onClick={() => connect(p.id)}
              title={data?.ready[p.id] ? "Sign in, then pick the accounts that belong to this company" : "Not set up in this copy yet"}>
              <p.Icon size={15} /> Connect {p.label}
            </button>
          ))}
          <button className="btn" onClick={() => setWhatsapp(true)} title="For the Inbox: answer customers' WhatsApp messages">
            <WhatsappLogoIcon size={15} /> Connect WhatsApp
          </button>
          <span className="spacer" />
          <button className="btn ghost sm" onClick={() => setMetaApp(true)} title="Connect your own Pages before Meta approves 37Studio's app">
            {data?.meta_app ? `Your Meta app (${data.meta_app})` : "Use your own Meta app"}
          </button>
        </div>
      </div>

      <div className="card">
        <div className="card-h"><h2>Inbox replies</h2><span className="small faint spacer">Messenger, Instagram and WhatsApp messages, and comments</span></div>
        <div className="card-b stack" style={{ gap: 10 }}>
          <div style={{ alignSelf: "flex-start" }}>
            <Seg value={data?.inbox_mode || "off"} onChange={(m) => api(`/tenants/${tenantId}`, { method: "PUT", body: { inbox_mode: m } }).then(reload, fail)}
              options={[{ value: "off", label: "Off" }, { value: "draft", label: "Claude drafts, you approve" }, { value: "send", label: "Claude replies on its own" }]} />
          </div>
          <p className="small muted" style={{ margin: 0 }}>
            Checks every 2 minutes while 37Studio is running on this computer. Messages that came in while it was off get answered when
            it's back, if the platform still allows a reply (24 hours for messages, and comments up to 2 days old). Claude uses only the
            company's knowledge base; when the answer isn't there, it says a team member will reply.
          </p>
          {data && <ReplyRules tenantId={tenantId} value={data.replies} onSaved={reload} />}
        </div>
      </div>

      {missing.length > 0 && (
        <p className="small faint">{missing.map((p) => p.label).join(" and ")}: not set up in this copy yet (README → Social publishing).</p>
      )}
      <p className="small faint">Facebook Groups and personal profiles can't be posted to by any app: Meta removed group posting in 2024 and profile posting in 2018. Pages, Instagram, YouTube and TikTok work.</p>

      {whatsapp && <ConnectWhatsApp tenantId={tenantId} onDone={() => { setWhatsapp(false); reload(); }} />}
      {metaApp && data && <OwnMetaApp current={data.meta_app} redirect={data.redirect.replace("{provider}", "meta")} onDone={() => { setMetaApp(false); reload(); }} />}
      {pendingKey && <PickAccounts k={pendingKey} onDone={() => { location.hash = `/clients/${tenantId}/social`; reload(); }} />}
    </div>
  );
}

/** The company's own rules for replies: Claude follows them in every draft and auto-reply, ahead of its default style. */
function ReplyRules({ tenantId, value, onSaved }: { tenantId: number; value: string; onSaved: () => void }) {
  const [text, setText] = useState(value);
  useEffect(() => setText(value), [value]);
  const save = () => api(`/tenants/${tenantId}`, { method: "PUT", body: { replies: text } }).then(() => { toast("Reply rules saved"); onSaved(); }, fail);
  return (
    <div className="stack" style={{ gap: 8 }}>
      <Field label="How Claude should reply" hint="followed in every draft and auto-reply">
        <textarea className="textarea" rows={5} value={text} onChange={(e) => setText(e.target.value)}
          placeholder={"Sign off as \"Team Dhaka Bites\".\nReply in Bangla unless they write in English.\nNever promise delivery times on Fridays.\nComplaints and refunds: say our manager will call them today."} />
      </Field>
      <div className="row"><span className="small faint spacer">Facts still come only from the knowledge base.</span>
        <button className="btn sm" disabled={text === value} onClick={save}>Save rules</button></div>
    </div>
  );
}

/** WhatsApp has no "sign in with" for numbers outside a Meta Tech Provider, so it takes the values from Meta's API Setup page. */
function ConnectWhatsApp({ tenantId, onDone }: { tenantId: number; onDone: () => void }) {
  const [f, setF] = useState({ phone_id: "", waba_id: "", token: "" });
  const [busy, setBusy] = useState(false);
  const set = (k: keyof typeof f) => (e: { target: { value: string } }) => setF({ ...f, [k]: e.target.value.trim() });
  const save = () => {
    setBusy(true);
    api(`/tenants/${tenantId}/whatsapp`, { body: f }).then(() => { toast("WhatsApp connected: its messages show up in the Inbox"); onDone(); }, (e) => { fail(e); setBusy(false); });
  };
  return (
    <Modal title="Connect WhatsApp" sub="Customers' WhatsApp messages show up in the Inbox, where Claude drafts the replies." onClose={onDone}
      footer={<><button className="btn" onClick={onDone}>Cancel</button><button className="btn primary" disabled={busy || !f.phone_id || !f.waba_id || !f.token} onClick={save}>{busy ? "Checking…" : "Connect"}</button></>}>
      <div className="stack">
        <p className="small muted" style={{ margin: 0 }}>
          In the Meta app dashboard, open <b>WhatsApp → API Setup</b> and copy the two IDs. For a token that doesn't expire, go to
          Business settings → Users → System users, add one, then <b>Generate token</b> for the 37Studio app with
          whatsapp_business_messaging and whatsapp_business_management.
        </p>
        <Field label="Phone number ID"><input className="input" inputMode="numeric" value={f.phone_id} onChange={set("phone_id")} /></Field>
        <Field label="WhatsApp Business Account ID"><input className="input" inputMode="numeric" value={f.waba_id} onChange={set("waba_id")} /></Field>
        <Field label="Access token"><input className="input" type="password" autoComplete="off" value={f.token} onChange={set("token")} /></Field>
      </div>
    </Modal>
  );
}

/** The user's own Meta app. Until Meta approves 37Studio's app for everyone, Facebook only lets people listed on an app
 *  sign in to it; on their own app they are the admin. Saved as keys (Settings → Keys), so it applies to every company. */
function OwnMetaApp({ current, redirect, onDone }: { current: string; redirect: string; onDone: () => void }) {
  const [f, setF] = useState({ META_APP_ID: current, META_APP_SECRET: "", META_CONFIG_ID: "" });
  const [busy, setBusy] = useState(false);
  const set = (k: keyof typeof f) => (e: { target: { value: string } }) => setF({ ...f, [k]: e.target.value.trim() });
  // Blank fields are left as they are; only "Go back" clears them.
  const save = (keys: Record<string, string>, msg: string) => {
    setBusy(true);
    api("/keys", { method: "PUT", body: keys }).then(() => { toast(msg); onDone(); }, (e) => { fail(e); setBusy(false); });
  };
  return (
    <Modal title="Use your own Meta app" size="lg" onClose={onDone}
      sub="Until Meta approves 37Studio's app for everyone, Facebook only lets the people listed on an app sign in. On your own app you're the admin, so your Pages and Instagram work today."
      footer={<>
        {current && <button className="btn ghost danger" disabled={busy} onClick={() => save({ META_APP_ID: "", META_APP_SECRET: "", META_CONFIG_ID: "" }, "Back to 37Studio's Meta app")}>Go back to 37Studio's app</button>}
        <span className="spacer" />
        <button className="btn" onClick={onDone}>Cancel</button>
        <button className="btn primary" disabled={busy || !f.META_APP_ID || (!current && !f.META_APP_SECRET)}
          onClick={() => save(Object.fromEntries(Object.entries(f).filter(([, v]) => v)), "Saved. Now connect Facebook & Instagram again")}>Save</button>
      </>}>
      <div className="stack">
        <ol className="small muted" style={{ margin: 0, paddingLeft: 18, display: "grid", gap: 6 }}>
          <li>At <a href="https://developers.facebook.com/apps" target="_blank" rel="noreferrer">developers.facebook.com/apps</a>, click <b>Create app</b>. Pick the use cases <b>Manage everything on your Page</b> and <b>Manage messaging &amp; content on Instagram</b>, plus <b>Engage with customers on Messenger</b> for the Inbox. Connect your business portfolio; it doesn't need to be verified.</li>
          <li>In each use case, click <b>Customize</b> and add pages_show_list, pages_read_engagement, pages_manage_posts, business_management, instagram_basic and instagram_content_publish. For the Inbox, also add pages_read_user_content, pages_manage_engagement, pages_manage_metadata, pages_messaging, instagram_manage_comments and instagram_manage_messages.</li>
          <li><b>Facebook Login for Business → Configurations → Create</b>: User access token, your Pages and Instagram accounts, the same permissions. Copy its ID; Instagram video uploads need it.</li>
          <li><b>App settings → Basic</b>: copy the App ID and App secret. If Meta asks for a redirect URI, use <code style={{ overflowWrap: "anywhere" }}>{redirect}</code>.</li>
          <li>Leave the app in <b>Development</b> mode. Anyone else who connects Pages needs a role under <b>App roles</b>.</li>
        </ol>
        <Field label="App ID"><input className="input" inputMode="numeric" value={f.META_APP_ID} onChange={set("META_APP_ID")} /></Field>
        <Field label="App secret" hint={current ? "leave blank to keep it" : undefined}><input className="input" type="password" autoComplete="off" value={f.META_APP_SECRET} onChange={set("META_APP_SECRET")} /></Field>
        <Field label="Login configuration ID" hint={current ? "leave blank to keep it" : "for Instagram videos"}><input className="input" inputMode="numeric" value={f.META_CONFIG_ID} onChange={set("META_CONFIG_ID")} /></Field>
        <p className="small faint" style={{ margin: 0 }}>
          This covers posting to your Pages and Instagram, and comments in the Inbox. Messenger and Instagram messages from customers
          still need Meta's App Review on any app; until then only people with a role on your app show up. After saving, connect
          Facebook &amp; Instagram again on each company. The secret is stored encrypted on this computer and never shown to the agents.
        </p>
      </div>
    </Modal>
  );
}

function PickAccounts({ k, onDone }: { k: string; onDone: () => void }) {
  const { data, error } = useApi<{ accounts: Candidate[] | null; error?: string }>(`/social/pending/${k}`);
  const [off, setOff] = useState<Set<string>>(new Set());
  const problem = error ?? data?.error;
  useEffect(() => { if (problem) { fail(problem); onDone(); } }, [problem]);
  if (!data?.accounts?.length) return null;
  const id = (a: Candidate) => `${a.platform}:${a.account_id}`;
  const picked = data.accounts.filter((a) => !off.has(id(a)));
  const toggle = (a: Candidate) => setOff((s) => { const n = new Set(s); n.has(id(a)) ? n.delete(id(a)) : n.add(id(a)); return n; });
  const save = () => api<{ connected: number }>(`/social/pending/${k}`, { body: { accounts: picked.map(id) } })
    .then((r) => { toast(`Connected ${r.connected} account${r.connected === 1 ? "" : "s"}`); onDone(); }, fail);
  return (
    <Modal title="Which accounts belong to this company?" sub="Untick any that belong to your other companies." onClose={onDone}
      footer={<><button className="btn" onClick={onDone}>Cancel</button><button className="btn primary" disabled={!picked.length} onClick={save}>Connect {picked.length}</button></>}>
      <div className="stack" style={{ gap: 4 }}>
        {data.accounts.map((a) => (
          <label key={id(a)} className="row acct-pick">
            <input type="checkbox" checked={!off.has(id(a))} onChange={() => toggle(a)} />
            <span className="spacer" style={{ minWidth: 0 }}><AccountLabel a={a} /></span>
          </label>
        ))}
      </div>
    </Modal>
  );
}

/** Project → Publish: one video, any mix of the company's accounts, a caption per platform, now or scheduled. */
export function PublishModal({ p, onClose, onQueued }: { p: Project; onClose: () => void; onQueued: () => void }) {
  const videos = (p.outputs ?? []).filter((f) => f.kind === "video");
  const { data } = useApi<SocialInfo>(`/tenants/${p.tenant_id}/social`);
  const [file, setFile] = useState(videos[0]?.path ?? "");
  const [picked, setPicked] = useState<Set<number>>(new Set());
  const [caps, setCaps] = useState<Captions>(p.social ?? {});
  const [privacy, setPrivacy] = useState({ youtube: "public", tiktok: "" });
  const [when, setWhen] = useState<"now" | "later">("now");
  const [at, setAt] = useState("");
  const [busy, setBusy] = useState("");
  const accounts = (data?.accounts ?? []).filter((a) => a.platform !== "whatsapp");
  const chosen = accounts.filter((a) => picked.has(a.id));
  const plats = (Object.keys(platforms) as Platform[]).filter((pl) => chosen.some((a) => a.platform === pl));
  const toggle = (id: number) => setPicked((s) => { const n = new Set(s); n.has(id) ? n.delete(id) : n.add(id); return n; });
  const cap = (k: keyof Captions) => ({ value: caps[k] ?? "", onChange: (e: { target: { value: string } }) => setCaps({ ...caps, [k]: e.target.value }) });

  async function write() {
    setBusy("ai");
    try { setCaps(await api<Captions>(`/projects/${p.id}/captions`, { body: {} })); } catch (e) { fail(e); }
    setBusy("");
  }
  async function submit() {
    setBusy("post");
    try {
      const r = await api<{ queued: number }>(`/projects/${p.id}/posts`, {
        body: { file: "out/" + file, accounts: [...picked], captions: caps, privacy, at: when === "later" ? new Date(at).toISOString() : "" },
      });
      toast(when === "later" ? `Scheduled ${r.queued} post${r.queued === 1 ? "" : "s"}` : `Posting to ${r.queued} account${r.queued === 1 ? "" : "s"}; follow it in Posts`);
      onQueued();
    } catch (e) { fail(e); setBusy(""); }
  }

  return (
    <Modal title="Publish" sub="Uploads straight from this machine to each platform." size="lg" onClose={onClose}
      footer={<>
        <span className="small faint spacer">{when === "later" ? "37Studio must be running at that time." : ""}</span>
        <button className="btn" onClick={onClose}>Cancel</button>
        <button className="btn primary" disabled={!!busy || !file || !picked.size || (when === "later" && !at)} onClick={submit}>
          <ShareNetworkIcon size={15} /> {busy === "post" ? "Queuing…" : `${when === "later" ? "Schedule" : "Post"} to ${picked.size || ""} account${picked.size === 1 ? "" : "s"}`}
        </button>
      </>}>
      <div className="stack" style={{ gap: 18 }}>
        <Field label="Video">
          <select className="select" value={file} onChange={(e) => setFile(e.target.value)}>
            {videos.map((v) => <option key={v.path} value={v.path}>{v.name}</option>)}
          </select>
        </Field>

        <div className="stack" style={{ gap: 8 }}>
          <span className="row" style={{ fontSize: 13, fontWeight: 500 }}>
            <span className="spacer">Accounts</span>
            {accounts.length > 1 && <button className="btn ghost sm" onClick={() => setPicked(picked.size === accounts.length ? new Set() : new Set(accounts.map((a) => a.id)))}>
              {picked.size === accounts.length ? "Clear" : "Select all"}
            </button>}
          </span>
          {data && !accounts.length ? (
            <p className="small muted">No accounts connected for {p.tenant_name}. <a href={`#/clients/${p.tenant_id}/social`}>Connect them on the client's Social tab</a>.</p>
          ) : (
            <div className="choices">
              {accounts.map((a) => (
                <button key={a.id} type="button" className={`choice ${picked.has(a.id) ? "on" : ""}`} aria-pressed={picked.has(a.id)} onClick={() => toggle(a.id)}>
                  <AccountLabel a={a} />
                </button>
              ))}
            </div>
          )}
        </div>

        {plats.length > 0 && (
          <div className="stack" style={{ gap: 12 }}>
            <div className="row">
              <span className="spacer" style={{ fontSize: 13, fontWeight: 500 }}>Captions</span>
              <button className="btn sm" disabled={!!busy} onClick={write} title="Claude writes each caption from the video, brief and company knowledge base">
                <SparkleIcon size={14} /> {busy === "ai" ? "Writing…" : "Write with AI"}
              </button>
            </div>
            {plats.map((pl) => pl === "youtube" ? (
              <div key={pl} className="stack" style={{ gap: 8 }}>
                <Field label="YouTube title" hint={`${(caps.youtube_title ?? "").length}/100`}><input className="input" maxLength={100} {...cap("youtube_title")} /></Field>
                <Field label="YouTube description"><textarea className="textarea" rows={3} {...cap("youtube")} /></Field>
                <div className="field">
                  <span>YouTube visibility <span className="hint">· private until Google audits the app</span></span>
                  <div className="row"><Seg value={privacy.youtube} onChange={(v) => setPrivacy({ ...privacy, youtube: v })}
                    options={[{ value: "public", label: "Public" }, { value: "unlisted", label: "Unlisted" }, { value: "private", label: "Private" }]} /></div>
                </div>
              </div>
            ) : (
              <div key={pl} className="stack" style={{ gap: 8 }}>
                <Field label={`${platforms[pl].label} caption`} hint={`${(caps[pl] ?? "").length}/${platforms[pl].limit}`}>
                  <textarea className="textarea" rows={3} maxLength={platforms[pl].limit} {...cap(pl)} />
                </Field>
                {pl === "tiktok" && (
                  <Field label="TikTok visibility" hint="until TikTok audits the app, only “Only me” or drafts work">
                    <select className="select" value={privacy.tiktok} onChange={(e) => setPrivacy({ ...privacy, tiktok: e.target.value })}>
                      {tiktokPrivacy.map(([v, l]) => <option key={v} value={v} disabled={!v}>{l}</option>)}
                    </select>
                  </Field>
                )}
              </div>
            ))}
          </div>
        )}

        <div className="field">
          <span>When</span>
          <div className="row wrap">
            <Seg value={when} onChange={setWhen} options={[{ value: "now", label: "Post now" }, { value: "later", label: "Schedule" }]} />
            {when === "later" && <input className="input" type="datetime-local" value={at} onChange={(e) => setAt(e.target.value)} style={{ width: "auto" }} aria-label="Publish at" />}
          </div>
        </div>
      </div>
    </Modal>
  );
}

/** Project → Posts: every post of this project and where it stands. Polls while something is uploading. */
export function PostsCard({ p, reload, onPublish }: { p: Project; reload: () => void; onPublish: () => void }) {
  const posts = p.posts ?? [];
  const live = posts.some((x) => x.status === "posting" || (x.status === "scheduled" && parseTime(x.publish_at).getTime() < Date.now() + 60_000));
  useEffect(() => {
    if (!live) return;
    const t = setInterval(reload, 4000);
    return () => clearInterval(t);
  }, [live, reload]);
  const retry = (x: Post) => api(`/posts/${x.id}/retry`, { body: {} }).then(reload, fail);
  const remove = (x: Post) =>
    confirm(x.status === "scheduled" ? "Cancel this scheduled post?" : `Remove from this list? It stays on ${platforms[x.platform].label}.`) &&
    api(`/posts/${x.id}`, { method: "DELETE" }).then(reload, fail);
  if (!posts.length) {
    return (
      <Empty icon={<ShareNetworkIcon size={28} />} title="Not posted anywhere yet">
        <button className="btn" onClick={onPublish}><ShareNetworkIcon size={15} /> Publish</button>
      </Empty>
    );
  }
  return (
    <div className="table-wrap">
      <table className="table">
        <thead><tr><th>Account</th><th>Status</th><th>When</th><th /></tr></thead>
        <tbody>
          {posts.map((x) => (
            <tr key={x.id}>
              <td style={{ maxWidth: 220 }}>
                <AccountLabel a={{ platform: x.platform, name: x.account_name, picture: "" }} />
                <div className="small faint ellipsis" style={{ marginTop: 4 }} title={x.file}>{x.file.replace(/^out\//, "")}</div>
              </td>
              <td style={{ minWidth: 160 }}>
                <Status value={x.status} />
                {x.error && <div className="small" style={{ color: "var(--danger)", marginTop: 4 }}>{x.error}</div>}
              </td>
              <td className="small muted nowrap">{x.status === "scheduled" ? parseTime(x.publish_at).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" }) : ago(x.publish_at)}</td>
              <td className="num">
                <span className="row" style={{ gap: 2, justifyContent: "flex-end" }}>
                  {x.url && <a className="btn ghost sm icon" href={x.url} target="_blank" rel="noreferrer" title="Open post" aria-label="Open post"><ArrowSquareOutIcon size={14} /></a>}
                  {x.status === "failed" && <button className="btn ghost sm icon" title="Retry now" aria-label="Retry" onClick={() => retry(x)}><ArrowClockwiseIcon size={14} /></button>}
                  {x.status !== "posting" && <button className="btn ghost sm icon" title={x.status === "scheduled" ? "Cancel" : "Remove from list"} aria-label="Remove" onClick={() => remove(x)}><TrashIcon size={14} /></button>}
                </span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
