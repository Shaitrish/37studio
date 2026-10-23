import { useEffect, useState } from "react";
import { ArrowsClockwiseIcon, ArrowSquareOutIcon, ChatCircleTextIcon, ChatsCircleIcon, FacebookLogoIcon, InstagramLogoIcon, PaperPlaneRightIcon, SparkleIcon, WarningIcon, WhatsappLogoIcon } from "@phosphor-icons/react";
import { ago, api, fail, Tenant, toast, useApi } from "../api";
import { Empty, Seg } from "../ui";

interface Item {
  id: string; account: number; tenant: number; client: string; platform: "facebook" | "instagram" | "whatsapp"; page: string; kind: "dm" | "comment";
  from: string; to: string; text: string; context: string; url: string; time: string; pending: boolean; draft?: string;
}
interface Msg { from: string; text: string; time: string; mine: boolean }
interface InboxData { items: Item[]; errors: { page: string; platform: string; client: string; kind: string; error: string }[] }

const PlatformIcon = ({ p, size = 14 }: { p: string; size?: number }) =>
  p === "instagram" ? <InstagramLogoIcon size={size} /> : p === "whatsapp" ? <WhatsappLogoIcon size={size} /> : <FacebookLogoIcon size={size} />;
const kindLabel = { dm: "Message", comment: "Comment" };

/** One inbox for every connected Page, Instagram and WhatsApp number: messages and comments, replies drafted from the company's knowledge base. */
export default function Inbox() {
  const { data: tenants } = useApi<Tenant[]>("/tenants");
  const [tenant, setTenant] = useState("");
  const { data, reload } = useApi<InboxData>(`/inbox${tenant ? `?tenant=${tenant}` : ""}`);
  const [kind, setKind] = useState<"all" | "dm" | "comment">("all");
  const [open, setOpen] = useState(true);
  const [sel, setSel] = useState<Item>();
  const [loading, setLoading] = useState(false);
  // ponytail: polls while the page is open; Page webhooks through the Worker would make it instant.
  useEffect(() => { const t = setInterval(reload, 60_000); return () => clearInterval(t); }, [reload]);
  const refresh = () => { setLoading(true); reload().finally(() => setLoading(false)); };
  const items = (data?.items ?? []).filter((i) => (kind === "all" || i.kind === kind) && (!open || i.pending));
  const pending = data?.items.filter((i) => i.pending).length ?? 0;

  return (
    <div className="page wide">
      <div className="page-head">
        <div className="spacer">
          <h1>Inbox</h1>
          <p>Messages and comments from every connected Page, Instagram and WhatsApp number. Claude drafts each reply from the company's knowledge base; you check it and send.</p>
        </div>
        <select className="select" style={{ width: "auto" }} value={tenant} onChange={(e) => { setTenant(e.target.value); setSel(undefined); }} aria-label="Client">
          <option value="">All companies</option>
          {tenants?.map((t) => <option key={t.id} value={t.id}>{t.name}</option>)}
        </select>
        <button className="btn" onClick={refresh} disabled={loading}><ArrowsClockwiseIcon size={15} /> {loading ? "Checking…" : "Refresh"}</button>
      </div>

      {!!data?.errors.length && <AccessProblems errors={data.errors} />}

      <div className="inbox">
        <div className="card inbox-list">
          <div className="card-h row wrap" style={{ gap: 8 }}>
            <Seg value={kind} onChange={setKind} options={[{ value: "all", label: "All" }, { value: "dm", label: "Messages" }, { value: "comment", label: "Comments" }]} />
            <label className="row small muted" style={{ gap: 6 }}><input type="checkbox" checked={open} onChange={(e) => setOpen(e.target.checked)} /> Needs reply ({pending})</label>
          </div>
          {!data ? <p className="small muted card-b">Loading…</p> : items.length ? items.map((i) => (
            <button key={i.account + i.id} className={`inbox-item ${sel?.id === i.id ? "on" : ""}`} onClick={() => setSel(i)}>
              <span className="row" style={{ gap: 6, minWidth: 0 }}>
                <PlatformIcon p={i.platform} />
                <strong className="ellipsis spacer">{i.from || "Customer"}</strong>
                {i.draft && <span className="small nowrap" style={{ color: "var(--primary)" }} title="Claude drafted a reply: open it to check and send">Draft ready</span>}
                {i.pending && <span className="dot-new" title="Needs reply" />}
                <span className="small faint nowrap">{i.time ? ago(i.time) : ""}</span>
              </span>
              <span className="small muted ellipsis">{i.text || (i.kind === "dm" ? "(attachment)" : "")}</span>
              <span className="small faint ellipsis">{kindLabel[i.kind]} · {i.page}{tenant ? "" : ` · ${i.client}`}</span>
            </button>
          )) : (
            <Empty icon={<ChatsCircleIcon size={28} />} title={open ? "Nothing waiting" : "No messages or comments"}>
              {data.errors.length ? "Some accounts couldn't be read; see above." : "New messages and comments show up here."}
            </Empty>
          )}
        </div>
        <div className="card inbox-thread">
          {sel ? <Thread key={sel.account + sel.id} item={sel} onSent={reload} /> : (
            <Empty icon={<ChatCircleTextIcon size={28} />} title="Pick a conversation">Replies go out from the account it came in on.</Empty>
          )}
        </div>
      </div>
    </div>
  );
}

function AccessProblems({ errors }: { errors: InboxData["errors"] }) {
  const perms = errors.some((e) => /permission|Page Public Content/i.test(e.error));
  const igOff = errors.some((e) => /disabled access to instagram direct messages/i.test(e.error));
  return (
    <div className="card card-b stack small" style={{ gap: 6, background: "var(--warn-soft)" }}>
      <strong className="row" style={{ gap: 6 }}><WarningIcon size={15} /> Some accounts can't be read yet</strong>
      {perms && <span>The Meta app needs the inbox permissions, then reconnect each company on its Social tab. See README → Social inbox.</span>}
      {igOff && <span>Instagram: in the Instagram app, Settings → Messages and story replies → Message controls → Connected tools, turn on <b>Allow access to messages</b>.</span>}
      <details><summary className="muted" style={{ cursor: "pointer" }}>Details ({errors.length})</summary>
        <ul style={{ margin: "6px 0 0", paddingLeft: 18 }}>
          {errors.map((e, n) => <li key={n}><b>{e.page}</b> {kindLabel[e.kind as "dm"]}s: <span className="muted">{e.error}</span></li>)}
        </ul>
      </details>
    </div>
  );
}

function Thread({ item, onSent }: { item: Item; onSent: () => void }) {
  const q = `account=${item.account}&kind=${item.kind}&id=${encodeURIComponent(item.id)}`;
  const { data: msgs, error, reload } = useApi<Msg[]>(`/inbox/thread?${q}`);
  const [text, setText] = useState(item.draft ?? "");
  const [busy, setBusy] = useState("");
  const target = { account: item.account, kind: item.kind, id: item.id };

  async function draft() {
    setBusy("draft");
    try { setText((await api<{ text: string }>("/inbox/draft", { body: { ...target, context: item.context } })).text); } catch (e) { fail(e); }
    setBusy("");
  }
  async function send() {
    setBusy("send");
    try {
      await api("/inbox/reply", { body: { ...target, to: item.to, text } });
      toast(item.kind === "dm" ? `Sent to ${item.from}` : "Reply posted");
      setText("");
      reload();
      onSent();
    } catch (e) { fail(e); }
    setBusy("");
  }

  return (
    <div className="thread">
      <div className="card-h row" style={{ gap: 8 }}>
        <PlatformIcon p={item.platform} size={16} />
        <div className="spacer" style={{ minWidth: 0 }}>
          <h2 className="ellipsis">{item.from || "Customer"}</h2>
          <p className="small faint ellipsis">{kindLabel[item.kind]} on {item.page} · {item.client}</p>
        </div>
        {item.url && <a className="btn ghost sm icon" href={item.url} target="_blank" rel="noreferrer" title="Open on the platform" aria-label="Open on the platform"><ArrowSquareOutIcon size={14} /></a>}
      </div>
      <div className="thread-msgs">
        {item.context && <p className="small muted thread-post"><b>Post:</b> {item.context}</p>}
        {error && <p className="msg-error">{error}</p>}
        {msgs?.map((m, n) => (
          <div key={n} className={m.mine ? "msg-user" : "msg-them"} title={m.time ? new Date(m.time).toLocaleString() : ""}>
            {!m.mine && <span className="small faint">{m.from}</span>}
            {m.text || <span className="faint">(attachment)</span>}
          </div>
        ))}
      </div>
      <div className="thread-reply stack" style={{ gap: 8 }}>
        <textarea className="textarea" rows={3} value={text} onChange={(e) => setText(e.target.value)}
          placeholder={item.kind === "dm" ? "Write a reply, or let Claude draft one" : "Write a public reply, or let Claude draft one"}
          onKeyDown={(e) => { if (e.key === "Enter" && (e.ctrlKey || e.metaKey) && text.trim()) { e.preventDefault(); send(); } }} />
        <div className="row">
          <span className="small faint spacer">{item.kind === "dm" ? "Messages can only be answered within 24 hours of the customer's last message." : "Comment replies are public."}</span>
          <button className="btn" disabled={!!busy} onClick={draft}><SparkleIcon size={15} /> {busy === "draft" ? "Drafting…" : "Draft with AI"}</button>
          <button className="btn primary" disabled={!!busy || !text.trim()} onClick={send} title="Ctrl+Enter">
            <PaperPlaneRightIcon size={15} /> {busy === "send" ? "Sending…" : "Send"}
          </button>
        </div>
      </div>
    </div>
  );
}
