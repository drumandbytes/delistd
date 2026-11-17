# Support session runbook

For whoever runs a paid setup session ([delistd.dev/support](https://delistd.dev/support/)). It is a map of the tool for explaining it, not a user guide: the user guide is [getting-started.md](getting-started.md), and this points into it. Not published on the website.

## Rules of the session

- The customer types their own mailbox password. Never ask for it, never type it, never take control of their machine. Screen share and talk them through it.
- Nothing is recorded. Take no screenshots of their screen. Keep no notes with their details beyond what the invoice needs.
- Don't promise removals. Say what Delistd does and what it can't (it doesn't search for their data and can't verify deletion).
- Standard scope is in the support page. Anything beyond it (server, NAS, second person, evidence pack): stop, quote, continue only once agreed.
- The web UI is local only. Never expose it to the internet, even on request. Remote reach for a server install means a VPN (Tailscale) or an SSH tunnel.

## Before the session

1. They have the quote and have paid. The booking confirmation carried the consent to start now and to English.
2. Send the prep email: link to the [FAQ](https://delistd.dev/faq/#paid-setup-sessions) and the checklist (a computer they can install on, a dedicated mailbox or willingness to create one, their phone for a sign-in code, name and postal address).
3. Ask for OS, mail provider and country. These decide the path:
   - Gmail, iCloud, Fastmail: app password (2-Step Verification must be on).
   - Outlook.com, Hotmail, Live: Microsoft sign-in (`delistd oauth login`, or the wizard).
   - Proton Mail: needs the Bridge app running (a non-standard mailbox extra).
   - Locked-down work or school Google accounts often can't make app passwords: use a fresh personal mailbox instead.

## The tool in one pass

Order to explain it, with where each part lives:

1. **Profile** (wizard step 2): name, email, address, country. Brokers match on these. Optional: phone, date of birth, former names, extra emails. Country also hides brokers that only handle other countries.
2. **Mail** (wizard step 3): provider preset, address, app password. Test email to themselves. Stored encrypted on their machine. "Skip" means send by hand.
3. **Brokers page**: the list (700+, region and category filters). **Send to All** sends up to the daily cap (450/day default), skips anyone emailed in the last 25 days, resumes on the next run. Dry run exists on the CLI.
4. **Dashboard**: counts of brokers, requests sent, failed, pending, and the reply pipeline.
5. **Pipeline / tasks**: replies that need them: confirmation links, forms, ID requests, questions. Read [broker-replies.md](broker-replies.md) for each type. ID requests: the tool never supplies ID; they decide.
6. **Replies**: read over IMAP and classified automatically. A scan runs from the UI; `delistd monitor` is the CLI.
7. **Evidence**: History, Export evidence. This is what goes to a data protection authority if a broker ignores the deadline.
8. **Automation**: Settings, Automation. Install the OS schedule (every 6 hours) or use the in-app scheduler while the app is open. Brokers re-list people, so this is what makes it keep working.
9. **Profiles for others**: a second person is a second profile (see [multi-profile.md](multi-profile.md)), sharing or overriding the mailbox.
10. **MCP**: experimental agent integration. Skip it unless they ask.

## Where setups stall

- App passwords page unavailable: 2-Step Verification is off, Advanced Protection is on, or the account is Workspace. Fix or switch mailbox.
- "Authentication failed" on the test email: wrong or revoked app password, or the Google password was used by mistake.
- macOS "can't be opened": unsigned file, right-click Open, or the `xattr` command in the troubleshooting section.
- `localhost:8080` doesn't load: the terminal window was closed, or the port is taken (`delistd serve -p 3000`).
- Window closed, Delistd stopped: just start it again, the data is kept.
- Test email never arrives: spam folder, then the provider's Sent folder.

Everything above is in [getting-started.md](getting-started.md#troubleshooting).

## Server, NAS and Docker variant

Follow [docker.md](docker.md) (compose, volume, optional Chromium container) or [kubernetes.md](kubernetes.md). Rules that matter: publish the port as `127.0.0.1:8080:8080`, never `8080:8080`; keep the data volume off NFS (SQLite); one replica. For access from other devices use Tailscale or an SSH tunnel. If they insist on public access, that is a different, quoted job with an authentication layer in front, never the bare UI.

## Wrap-up

1. Show them the Dashboard and where "needs action" items appear.
2. Say what they do next and when: check the Pipeline every week or two, run again after a few weeks (or leave the schedule on).
3. Follow-up message within a day: a short summary of what was set up, the three things to check, and how to reach us.
4. Ask for feedback in the same message (the feedback form, or a reply by email if they don't use GitHub), invite a star on the GitHub repo, and ask for a testimonial only with their explicit yes ("may we quote you, with or without your name?"). Never quote anyone without it.
5. Offer a check-in period if they wanted ongoing help.

## Admin

Calendar: only slots 24 hours or more ahead. Anything sooner is by prior email agreement only, at +€10. Moving or cancelling with at least 24 hours' notice is free. Keep only the invoice and the correspondence needed for it (seven years for invoices, see the privacy page).

### Order flow

Each step is an email, because the proof duty sits with us: keep every one.

1. **Quote** (we send). Final price in euros, what is included, the model withdrawal wording, a link to the [terms](https://delistd.dev/support/terms/), our name, address, phone and email. No tax is added: we are not VAT-registered, so the price is final.
2. **Acceptance** (they reply). Accepts the quote and agrees to English. To start within the 14 days they add the early-start statement from the terms, word for word. Without it, only schedule a session after the 14 days.
3. **Confirmation** (we send). This makes the contract. Repeat the quote's information, the agreed time, and the payment link. Slots are 24 hours or more ahead.
4. **Payment** (they pay by card through the Stripe link, before the session). The Stripe receipt shows our name and address.

### Refunds and withdrawal

- They may withdraw within 14 days of the confirmation, with any clear statement (the form is optional). Reply by email to confirm we received it.
- Refund everything within 14 days of learning of the decision, to the same card, in the Stripe dashboard (Payments, the payment, Refund). Never ask them to pay a fee.
- If they asked to start early and the session already happened in part, keep only the share of the price for what was done and refund the rest as a partial refund. If the session was carried out in full, after they asked for it and acknowledged it, the right to withdraw has ended: no refund on that ground.
- If we cancel or move a session, offer a new time or a full refund. If setup cannot work with their mailbox, refund the part not delivered.
- Check in the dashboard whether Stripe returns its processing fee on a refund; assume it does not, and treat it as a cost of the offer.

### Stripe set-up checklist

Do this in test mode first, then repeat in live mode. One Stripe account for Drumandbytes OÜ, not one per project.

- [ ] Business details: OÜ name, registry code, address; a representative identity check; the OÜ's bank IBAN for payouts.
- [ ] Public details and receipts: our name and address show on the receipt; customer emails are switched on.
- [ ] Products: "Delistd setup" €39 and each add-on from the [support page](https://delistd.dev/support/) as its own product, one payment link per plan.
- [ ] Tax: Stripe Tax off while we are not VAT-registered; revisit when we register.
- [ ] Terms: the link requires acceptance of the terms page before paying.
- [ ] Button wording: the pay button, or a line above it, says it is an order with an obligation to pay (VÕS § 62²). Check what the payment link lets you set.
- [ ] Statement descriptor recognisable on a card statement, for example `DRUMANDBYTES SETUP`.
- [ ] Test: pay €39 in test mode, then refund; read the receipt for our name and address and that no VAT is shown.
- [ ] Live: repeat once with a real card, refund it, and check the payout lands.

### Before the first sale

- [ ] A test mail to the support address reaches the inbox, and a reply from it is not marked as spam.
- [ ] Booking calendar live, offering only slots 24 hours or more ahead.
- [ ] Stripe checklist above done in live mode.
- [ ] Quote, confirmation and refund wording reviewed once against the terms page.
