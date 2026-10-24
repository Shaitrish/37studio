-- WhatsApp messages waiting for the copy of 37Studio that owns the number. Apply with:
--   npx wrangler d1 execute studio37 --remote --file schema.sql
create table if not exists wa (
  seq integer primary key autoincrement, -- the cursor a copy pulls from
  wamid text not null unique,            -- Meta retries webhooks; the message id dedupes them
  phone text not null,                   -- the business number's phone_number_id
  body text not null,                    -- the message as Meta sent it, plus the sender's profile name
  at integer not null                    -- received, ms; rows go once pulled, or after 7 days
);
create index if not exists wa_phone on wa(phone, seq);

-- Donations reported on /donate (bKash Send Money to a personal number has no API). Each waits as pending until checked
-- against the bKash app, then: update donations set status='confirmed' where trx='…'   (or 'rejected')
create table if not exists donations (
  trx text primary key,                    -- the bKash TrxID the donor reported; also stops double reports
  amount integer not null,                 -- taka, as reported; correct it when confirming if bKash shows otherwise
  name text not null default '',
  message text not null default '',
  public integer not null default 0,       -- 1: show the name and message on /donate
  status text not null default 'pending',  -- pending | confirmed | rejected
  at integer not null                      -- reported, ms
);
