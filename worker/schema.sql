-- WhatsApp messages waiting for the copy of 37Studio that owns the number. Apply with:
--   npx wrangler d1 execute studio37 --remote --file schema.sql
create table if not exists wa (
  seq integer primary key autoincrement, -- the cursor a copy pulls from
  wamid text not null unique,            -- Meta retries webhooks; the message id dedupes them
  phone text not null,                   -- the business number's phone_number_id
  body text not null,                    -- the message as Meta sent it, plus the sender's profile name
  at integer not null                    -- received, ms; rows go after 30 days
);
create index if not exists wa_phone on wa(phone, seq);

-- License purchases through SSLCommerz (pay.js). Apply the same way as above.
create table if not exists orders (
  tran_id text primary key,        -- random; the order page link is /pay/order?t=<tran_id>
  plan text not null,              -- 6m | 12m
  amount integer not null,         -- taka
  name text not null,
  email text not null,
  phone text not null,
  status text not null default 'pending', -- pending | paid | review | failed | cancelled
  val_id text,
  license text,
  created_at integer not null,
  paid_at integer
);
