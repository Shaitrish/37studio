// 37Studio's public endpoints on studio.shaitrish.com, as one Cloudflare Worker:
//   GET  /version.json          the latest release (public/version.json), read by every copy's update check
//   POST /oauth/meta/token      finishes a Facebook login for copies that don't hold the Meta app secret
//   GET  /webhooks/meta         Meta's webhook check (hub.challenge)
//   POST /webhooks/meta         Meta's webhooks: WhatsApp messages are held in D1 for 30 days
//   GET  /whatsapp/messages     a copy pulls its number's messages; its WhatsApp token proves the number is its own
//   anything else               static files in public/ (download page, launcher bundle)
// Secrets (set with `npx wrangler secret put NAME`): META_APP_SECRET, META_VERIFY_TOKEN. Vars in wrangler.toml: META_APP_ID,
// REVOKED_LICENSES (comma-separated first 16 characters of a key's signature, for leaked keys).
// The Facebook login and the WhatsApp relay only serve licensed copies: a patched app can't get past this.

const GRAPH = "https://graph.facebook.com/v26.0";
const LICENSE_PUB = "/4H+3jLcxlI0cnvC94wpnc4Doqa2iActEj2Nq5XoKn8="; // same key as edition.go
const NEED_LICENSE = "This needs a 37Studio license. Add your key in Settings, or get one at https://studio.shaitrish.com/#pricing";
const KEEP_MS = 30 * 864e5;

export default {
  async fetch(req, env) {
    const url = new URL(req.url);
    if (url.pathname === "/oauth/meta/token" && req.method === "POST") return metaToken(req, env);
    if (url.pathname === "/webhooks/meta") return webhook(req, env, url);
    if (url.pathname === "/whatsapp/messages" && req.method === "GET") return waMessages(req, env, url);
    return env.ASSETS.fetch(req);
  },
};

async function metaToken(req, env) {
  if (!(await licensed(req, env))) return json({ error: { message: NEED_LICENSE } }, 402);
  const { code, redirect_uri } = await req.json().catch(() => ({}));
  // Only local installs log in through here; anything else is someone else's redirect.
  if (!code || !/^http:\/\/(localhost|127\.0\.0\.1):\d+\/api\/oauth\/meta\/callback$/.test(redirect_uri || "")) {
    return json({ error: { message: "bad request" } }, 400);
  }
  const app = { client_id: env.META_APP_ID, client_secret: env.META_APP_SECRET };
  const short = await graph("/oauth/access_token", { ...app, redirect_uri, code });
  if (!short.access_token) return json(short, 400);
  const long = await graph("/oauth/access_token", { ...app, grant_type: "fb_exchange_token", fb_exchange_token: short.access_token });
  return json(long.access_token ? { access_token: long.access_token } : long, long.access_token ? 200 : 400);
}

// WhatsApp only reaches an app through webhooks, and copies on people's PCs have no public address, so messages wait here.
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
  if (!(await licensed(req, env))) return json({ error: { message: NEED_LICENSE } }, 402);
  if (!/^\d+$/.test(phone) || !token || !(await canRead(token, phone))) {
    return json({ error: { message: "this WhatsApp token can't read that number" } }, 403);
  }
  const { results } = await env.DB.prepare("select seq, body from wa where phone=? and seq>? order by seq limit 500").bind(phone, after).all();
  return json({ messages: results.map((r) => ({ seq: r.seq, ...JSON.parse(r.body) })) });
}

// licensed checks the X-37Studio-License header: 37S1.<payload>.<signature>, both base64url, Ed25519-signed.
async function licensed(req, env) {
  const parts = (req.headers.get("x-37studio-license") || "").trim().split(".");
  if (parts.length !== 3 || parts[0] !== "37S1") return false;
  if ((env.REVOKED_LICENSES || "").split(",").map((s) => s.trim()).filter(Boolean).includes(parts[2].slice(0, 16))) return false;
  try {
    const b64 = (s) => Uint8Array.from(atob(s.replace(/-/g, "+").replace(/_/g, "/") + "===".slice((s.length + 3) % 4)), (c) => c.charCodeAt(0));
    const key = await crypto.subtle.importKey("raw", b64(LICENSE_PUB), { name: "Ed25519" }, false, ["verify"]);
    return await crypto.subtle.verify({ name: "Ed25519" }, key, b64(parts[2]), b64(parts[1]));
  } catch {
    return false;
  }
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
