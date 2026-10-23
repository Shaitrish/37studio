// Buying a license online. In Bangladesh through SSLCommerz (bKash, Nagad, Rocket, cards): the key is issued the moment
// the payment validates, signed here with the LICENSE_PRIVATE_KEY secret. Other countries pay by bank transfer (buy.html)
// and get their key by email from us.
//   POST /pay/start     the form on /buy: plan, name, email, phone -> an order row -> SSLCommerz checkout
//   POST /pay/return    SSLCommerz sends the buyer back here (success, fail or cancel) -> their order page
//   POST /pay/ipn       SSLCommerz's server-to-server notice, validated the same way, so a closed tab still gets its key
//   GET  /pay/order     the order page: the key once paid. The random tran_id in the link is what keeps it private.
// Secrets: SSLCZ_STORE_ID/SSLCZ_STORE_PASS (sandbox), SSLCZ_LIVE_STORE_ID/SSLCZ_LIVE_STORE_PASS, LICENSE_PRIVATE_KEY.
// Var SSLCZ_MODE: "live" takes real money; anything else uses the sandbox.

const SITE = "https://studio.shaitrish.com";
export const PLANS = {
  "6m": { months: 6, amount: 5000, label: "6 months" },
  "12m": { months: 12, amount: 8000, label: "1 year" },
};

const gateway = (env) =>
  env.SSLCZ_MODE === "live"
    ? { host: "https://securepay.sslcommerz.com", id: env.SSLCZ_LIVE_STORE_ID, pass: env.SSLCZ_LIVE_STORE_PASS }
    : { host: "https://sandbox.sslcommerz.com", id: env.SSLCZ_STORE_ID, pass: env.SSLCZ_STORE_PASS };

export async function buy(req, env) {
  const f = await req.formData();
  const planId = f.get("plan");
  const plan = PLANS[planId];
  const name = String(f.get("name") || "").trim().slice(0, 80);
  const email = String(f.get("email") || "").trim().slice(0, 120);
  const phone = String(f.get("phone") || "").trim().slice(0, 20);
  // Until SSLCZ_MODE is "live", only our own test purchases (test=1) reach the sandbox; buyers are pointed elsewhere.
  if (env.SSLCZ_MODE !== "live" && f.get("test") !== "1") {
    return page("Online payment opens soon", `<p>We're finishing the online checkout. Until then, message us on <a href="https://wa.me/8801635191148?text=Hi%2C%20I%27d%20like%20a%2037Studio%20license">WhatsApp</a> and we'll send you a bKash or Nagad number, or pay by <a href="/buy#abroad">bank transfer</a>.</p>`);
  }
  if (!plan || !name || !/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(email) || !/^\+?[0-9 -]{8,16}$/.test(phone)) {
    return page("Check your details", `<p>Please go back and fill in your name, a valid email address and your phone number.</p><p><a class="btn ink" href="/buy">Back</a></p>`, 400);
  }
  const tran = "37S" + crypto.randomUUID().replace(/-/g, "").slice(0, 24);
  await env.DB.prepare("insert into orders(tran_id, plan, amount, name, email, phone, created_at) values(?,?,?,?,?,?,?)")
    .bind(tran, planId, plan.amount, name, email, phone, Date.now()).run();
  const g = gateway(env);
  const body = new URLSearchParams({
    store_id: g.id, store_passwd: g.pass, total_amount: String(plan.amount), currency: "BDT", tran_id: tran,
    success_url: SITE + "/pay/return", fail_url: SITE + "/pay/return", cancel_url: SITE + "/pay/return", ipn_url: SITE + "/pay/ipn",
    cus_name: name, cus_email: email, cus_phone: phone, cus_add1: "Dhaka", cus_city: "Dhaka", cus_postcode: "1200", cus_country: "Bangladesh",
    shipping_method: "NO", num_of_item: "1", product_name: "37Studio license, " + plan.label, product_category: "Software",
    product_profile: "non-physical-goods", emi_option: "0",
  });
  const r = await fetch(g.host + "/gwprocess/v4/api.php", { method: "POST", body });
  const s = await r.json().catch(() => ({}));
  if (s.status !== "SUCCESS" || !s.GatewayPageURL) {
    return page("Payment couldn't start", `<p>${esc(s.failedreason || "The payment gateway didn't answer.")} Please try again in a minute, or message us on <a href="https://wa.me/8801635191148">WhatsApp</a>.</p>`, 502);
  }
  return Response.redirect(s.GatewayPageURL, 303);
}

// payBack handles both the buyer coming back and SSLCommerz's IPN: validate, then issue the key once.
export async function payBack(req, env, ipn) {
  const f = await req.formData();
  const tran = String(f.get("tran_id") || "");
  await settle(env, tran, f.get("val_id"), String(f.get("status") || ""));
  if (ipn) return new Response("ok");
  return Response.redirect(SITE + "/pay/order?t=" + encodeURIComponent(tran), 303);
}

async function settle(env, tran, valId, status) {
  const o = await env.DB.prepare("select * from orders where tran_id=?").bind(tran).first();
  if (!o || o.status === "paid") return;
  if (!valId) { // failed and cancelled payments come back without a val_id
    if (/FAIL|CANCEL|EXPIRE/i.test(status)) {
      await env.DB.prepare("update orders set status=? where tran_id=? and status='pending'").bind(/CANCEL/i.test(status) ? "cancelled" : "failed", tran).run();
    }
    return;
  }
  // Never trust the redirect: ask SSLCommerz, and check it's this order, in taka, for the full price.
  const g = gateway(env);
  const q = new URLSearchParams({ val_id: valId, store_id: g.id, store_passwd: g.pass, format: "json" });
  const v = await fetch(`${g.host}/validator/api/validationserverAPI.php?${q}`).then((r) => r.json()).catch(() => ({}));
  const ok = (v.status === "VALID" || v.status === "VALIDATED") && v.tran_id === tran && v.currency === "BDT" && Math.round(Number(v.amount)) === o.amount;
  if (!ok) return;
  if (String(v.risk_level) === "1") { // SSLCommerz flags it as risky: a person checks before a key goes out
    await env.DB.prepare("update orders set status='review', val_id=? where tran_id=? and status<>'paid'").bind(valId, tran).run();
    return;
  }
  const key = await issue(env, o);
  await env.DB.prepare("update orders set status='paid', val_id=?, license=?, paid_at=? where tran_id=? and status<>'paid'")
    .bind(valId, key, Date.now(), tran).run();
}

// issue signs the same 37S1.<payload>.<signature> key as `37studio license --months N`.
export async function issue(env, o) {
  const seed = fromB64(env.LICENSE_PRIVATE_KEY.trim()); // Go's ed25519 private key: 32-byte seed, then the public key
  const key = await crypto.subtle.importKey("jwk", { kty: "OKP", crv: "Ed25519", d: toB64url(seed.slice(0, 32)), x: toB64url(seed.slice(32)) },
    { name: "Ed25519" }, false, ["sign"]);
  const now = new Date();
  const end = new Date(now);
  end.setUTCMonth(end.getUTCMonth() + PLANS[o.plan].months);
  const day = (d) => d.toISOString().slice(0, 10);
  const lic = { name: o.name, email: o.email, issued: day(now), expires: day(end) };
  if (env.SSLCZ_MODE !== "live") lic.test = true; // sandbox money isn't real, so neither is the key
  const payload = new TextEncoder().encode(JSON.stringify(lic));
  const sig = new Uint8Array(await crypto.subtle.sign({ name: "Ed25519" }, key, payload));
  return `37S1.${toB64url(payload)}.${toB64url(sig)}`;
}

export async function orderPage(env, url) {
  const t = url.searchParams.get("t") || "";
  const o = await env.DB.prepare("select plan, name, status, license from orders where tran_id=?").bind(t).first();
  if (!o) return page("Order not found", `<p>We couldn't find that order. If you paid, message us on <a href="https://wa.me/8801635191148">WhatsApp</a> with your phone number.</p>`, 404);
  const plan = PLANS[o.plan]?.label || o.plan;
  if (o.status === "paid") {
    return page("Thank you! Here's your license", `
      <p>Your 37Studio license for <b>${esc(plan)}</b>, made out to <b>${esc(o.name)}</b>.</p>
      <textarea id="key" class="keybox" readonly rows="4" onclick="this.select()">${esc(o.license)}</textarea>
      <p><button class="btn ink" type="button" onclick="navigator.clipboard.writeText(document.getElementById('key').value).then(()=>this.textContent='Copied')">Copy key</button>
        <a class="btn" href="/setup">Setup guide</a></p>
      <ol><li>Open 37Studio → <b>Settings</b>.</li><li>Paste the key under <b>License</b> and click <b>Activate</b>.</li></ol>
      <p class="muted">Keep this page or the key somewhere safe. If you lose it, message us with your order number <code>${esc(t)}</code>.</p>`);
  }
  if (o.status === "pending") {
    return page("Confirming your payment…", `<p>This usually takes a few seconds. The page refreshes by itself.</p>`, 200, '<meta http-equiv="refresh" content="4">');
  }
  if (o.status === "review") {
    return page("Your payment is being checked", `<p>The payment gateway asked for a manual check on this payment. We'll email your license to you within one working day. Order number <code>${esc(t)}</code>.</p>`);
  }
  return page("The payment didn't go through", `<p>No money was taken for this order. You can <a href="/buy">try again</a>, or message us on <a href="https://wa.me/8801635191148">WhatsApp</a>.</p>`);
}

function page(title, body, status = 200, head = "") {
  return new Response(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">${head}<title>${esc(title)} · 37Studio</title><link rel="icon" href="/favicon.svg">
<link rel="preconnect" href="https://fonts.googleapis.com"><link href="https://fonts.googleapis.com/css2?family=Geist:wght@400;500;600;700&family=Geist+Mono:wght@400;500&display=swap" rel="stylesheet">
<link rel="stylesheet" href="/site.css"></head><body>
<header class="nav scrolled"><div class="wrap"><a class="brand" href="/"><img src="/favicon.svg" alt="">37Studio <small>by Shaitrish</small></a></div></header>
<main class="wrap" style="max-width:720px;padding-top:64px;padding-bottom:96px"><h1 style="font-size:clamp(30px,4vw,42px)">${esc(title)}</h1><div class="prose" style="margin-top:20px">${body}</div></main>
</body></html>`, { status, headers: { "content-type": "text/html; charset=utf-8", "cache-control": "no-store" } });
}

const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);
const toB64url = (u8) => btoa(String.fromCharCode(...u8)).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
export const fromB64 = (s) => Uint8Array.from(atob(s.replace(/-/g, "+").replace(/_/g, "/") + "===".slice((s.length + 3) % 4)), (c) => c.charCodeAt(0));
