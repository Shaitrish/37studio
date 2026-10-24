// 37Studio's WhatsApp relay: a Cloudflare Worker you deploy to your own account (docs/platforms.md → WhatsApp).
// Meta delivers WhatsApp messages only by webhook to a public address, which a copy on your computer doesn't have,
// so they wait here until the copy pulls them:
//   GET  /webhooks/meta         Meta's webhook check (hub.challenge)
//   POST /webhooks/meta         Meta's webhooks: messages wait in D1 until pulled (7 days at most)
//   GET  /whatsapp/messages     the copy pulls its number's messages; its WhatsApp token proves the number is its own, and
//                               asking for messages after N confirms it has stored 1..N, which are then deleted
// Secrets (set with `npx wrangler secret put NAME`): META_APP_SECRET, META_VERIFY_TOKEN.

const GRAPH = "https://graph.facebook.com/v26.0";
const KEEP_MS = 7 * 864e5; // past WhatsApp's 24-hour reply window, so an unpulled message is of little use by then

export default {
  async fetch(req, env) {
    const url = new URL(req.url);
    if (url.pathname === "/webhooks/meta") return webhook(req, env, url);
    if (url.pathname === "/whatsapp/messages" && req.method === "GET") return waMessages(req, env, url);
    return new Response("not found", { status: 404 });
  },
};

async function webhook(req, env, url) {
  if (req.method === "GET") {
    const q = url.searchParams;
    const ok = !!env.META_VERIFY_TOKEN && q.get("hub.mode") === "subscribe" && q.get("hub.verify_token") === env.META_VERIFY_TOKEN;
    return new Response(ok ? q.get("hub.challenge") : "forbidden", { status: ok ? 200 : 403 });
  }
  const raw = await req.arrayBuffer();
  if (!(await signedByMeta(raw, req.headers.get("x-hub-signature-256"), env.META_APP_SECRET))) return new Response("bad signature", { status: 401 });
  const body = JSON.parse(new TextDecoder().decode(raw));
  if (body.object !== "whatsapp_business_account") return new Response("ok"); // Pages and Instagram are read live
  const now = Date.now();
  const writes = [env.DB.prepare("delete from wa where at < ?").bind(now - KEEP_MS)];
  for (const entry of body.entry || []) {
    for (const { value: v = {} } of entry.changes || []) {
      const phone = v.metadata?.phone_number_id;
      const names = Object.fromEntries((v.contacts || []).map((c) => [c.wa_id, c.profile?.name || ""]));
      for (const m of phone ? v.messages || [] : []) {
        writes.push(env.DB.prepare("insert or ignore into wa(wamid, phone, body, at) values(?,?,?,?)")
          .bind(m.id, phone, JSON.stringify({ ...m, name: names[m.from] || "" }), now));
      }
    }
  }
  await env.DB.batch(writes);
  return new Response("ok");
}

async function signedByMeta(raw, header, secret) {
  if (!secret || !/^sha256=[0-9a-f]{64}$/.test(header || "")) return false;
  const key = await crypto.subtle.importKey("raw", new TextEncoder().encode(secret), { name: "HMAC", hash: "SHA-256" }, false, ["verify"]);
  const sig = new Uint8Array(header.slice(7).match(/../g).map((h) => parseInt(h, 16)));
  return crypto.subtle.verify("HMAC", key, sig, raw);
}

async function waMessages(req, env, url) {
  const phone = url.searchParams.get("phone") || "";
  const after = Number(url.searchParams.get("after")) || 0;
  const token = (req.headers.get("authorization") || "").replace(/^Bearer /, "");
  if (!/^\d+$/.test(phone) || !token || !(await canRead(token, phone))) {
    return json({ error: { message: "this WhatsApp token can't read that number" } }, 403);
  }
  const [, { results }] = await env.DB.batch([
    env.DB.prepare("delete from wa where (phone=? and seq<=?) or at<?").bind(phone, after, Date.now() - KEEP_MS),
    env.DB.prepare("select seq, body from wa where phone=? and seq>? order by seq limit 500").bind(phone, after),
  ]);
  return json({ messages: results.map((r) => ({ seq: r.seq, ...JSON.parse(r.body) })) });
}

// ponytail: per-isolate memory, so a fresh isolate asks Meta again; fine at one pull a minute per number.
const readers = new Map();
async function canRead(token, phone) {
  const key = phone + " " + token;
  if (readers.get(key) > Date.now()) return true;
  const r = await graph(`/${phone}`, { fields: "id", access_token: token });
  if (r.id !== phone) return false;
  readers.set(key, Date.now() + 10 * 60e3);
  return true;
}

async function graph(path, params) {
  const r = await fetch(`${GRAPH}${path}?${new URLSearchParams(params)}`);
  return r.json();
}

const json = (body, status = 200) => new Response(JSON.stringify(body), { status, headers: { "content-type": "application/json" } });
