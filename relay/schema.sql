-- WhatsApp messages waiting for your copy of 37Studio to pull them. Apply with:
--   npx wrangler d1 execute studio37-relay --remote --file schema.sql
create table if not exists wa (
  seq integer primary key autoincrement, -- the cursor a copy pulls from
  wamid text not null unique,            -- Meta retries webhooks; the message id dedupes them
  phone text not null,                   -- the business number's phone_number_id
  body text not null,                    -- the message as Meta sent it, plus the sender's profile name
  at integer not null                    -- received, ms; rows go once pulled, or after 7 days
);
create index if not exists wa_phone on wa(phone, seq);
