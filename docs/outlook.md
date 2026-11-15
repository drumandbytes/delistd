# Outlook.com, Hotmail and Live

Microsoft no longer accepts passwords or app passwords for IMAP and SMTP on personal accounts ([Microsoft's settings page](https://support.microsoft.com/en-us/outlook/pop-imap-and-smtp-settings-for-outlook-com) says Outlook.com requires OAuth2). Delistd signs in the way Microsoft requires, with a short code you enter in a browser.

A user has reported this working ([issue #293](https://github.com/drumandbytes/delistd/issues/293)). If it works for you, or doesn't, please say so there.

It is for **personal** accounts (outlook.com, hotmail.com, live.com). Microsoft 365 work and school accounts are not covered. If you don't want to set this up, the simpler route is a separate Gmail or other app-password mailbox just for Delistd; see [getting-started.md](getting-started.md).

## 1. The Delistd app, or your own (optional)

Microsoft only allows sign-in through a registered app. Delistd ships a shared one, so you only click **Sign in with Microsoft** and can skip the rest of this section.

Microsoft's approval page will call the app an **unverified** publisher. That is expected: Drumandbytes has not completed Microsoft's publisher verification yet. The app asks only for `IMAP.AccessAsUser.All`, `SMTP.Send` and `offline_access`: read and send mail for the account you approve, and stay signed in. It has no secret, and Delistd never sees your Microsoft password. You can see and revoke it any time in your Microsoft account's app permissions.

If you would rather not depend on the shared app, or it ever stops working, register your own (free, no secret) and use it with `client_id:` in the config or `--client-id`. `DELISTD_OAUTH_CLIENT_ID` overrides the shared app for accounts without their own `client_id` (see [oauth-app.md](oauth-app.md)). Steps:

1. Open [entra.microsoft.com](https://entra.microsoft.com) → **App registrations** → **New registration**, signed in with your Microsoft account.
2. Name: anything (e.g. `Delistd`). Supported account types: **Personal Microsoft accounts only**. Leave the redirect URI empty. **Register**.
3. **Authentication** → **Advanced settings** → **Allow public client flows** → **Yes** → **Save**.
4. **API permissions**: if the sign-in in step 3 below complains that the app lacks permission, add the delegated permissions `IMAP.AccessAsUser.All` and `SMTP.Send` (under *Office 365 Exchange Online*) and `offline_access`.
5. From **Overview**, copy the **Application (client) ID**.

**This step can fail for a personal account.** Microsoft gives a personal account no directory of its own, and some accounts are refused at the Entra sign-in itself (one tester got `AADSTS90123: access_denied`). If that happens, the ways in are a free Azure account (Microsoft may ask for a phone number and a payment method to verify it) or a free [Microsoft 365 Developer Program](https://developer.microsoft.com/microsoft-365/dev-program) tenant, registering the app there with the account type set to personal accounts. If neither is acceptable, use a separate Gmail or other app-password mailbox instead. We are looking at removing this step.

## 2. Sign in

**In the web UI:** pick **Outlook.com / Hotmail / Live (OAuth)** as the provider, in the setup wizard's email step, on Settings → Inbox, or in a profile's own mail account. Enter your address and click **Sign in with Microsoft**: it shows a code and a link, you approve it at Microsoft, and the form then saves like any other account. The collapsed "Use my own Microsoft app" option is where your own client ID goes.

**From the command line:** run `delistd init` first if you haven't (any provider; the next step replaces the mail account). Then:

```bash
delistd oauth login --email you@outlook.com
```

Add `--client-id <your client id>` to use your own Microsoft app.

Delistd prints a link and a code. Open the link on any device, enter the code, and approve. Only enter a code you started yourself: a code someone else sends you would give them access to your mailbox. Delistd then stores the sign-in, encrypted like a password, as both your sending account and your inbox, with `smtp-mail.outlook.com:587` and `outlook.office365.com:993`. It writes `auth: oauth2` and your `client_id` next to them in `config.yaml`.

Check it works with `delistd monitor` (signs in to the inbox) and a test send from the web UI's Test Your Setup step or `delistd send` for one broker.

## What to expect

- **Primary address.** Right after you approve the code, Delistd logs in to the inbox and to SMTP to check the sign-in. Outlook's SMTP server only lets the account's *primary* address log in. That's normally your Outlook address. If your Microsoft account signs in with another address (say a Gmail address with an Outlook alias added), enter that primary address under **Server settings → Login username** (or `username:` under `email.smtp` and `email:` under `inbox`) and keep the Outlook address as the From address; Delistd says so if the check fails. A Microsoft account that only has a Gmail address and no Outlook mailbox can't be used: there is no inbox to read or send from.
- **Spam folder.** A brand-new Outlook account that sends short messages with no history can land in the recipient's junk folder at first. Delivery to brokers is the same as from any new account; sending from an address with some history helps.

- The sign-in is refreshed automatically on every run and saved back to the config. It stops working if you don't use it for about 90 days, change your Microsoft password, or revoke it; Delistd then says to run `delistd oauth login` again.
- Unattended runs (`delistd schedule install`, `delistd auto`) work, with the usual limit that a passphrase-protected config can't be unlocked by the OS scheduler.
- The command line works in Docker too, if you prefer it: `docker exec -it <container> delistd oauth login --client-id ...`.
- The refresh token never reaches your browser: the form only carries a one-time id for the finished sign-in.
- If sending fails with an authentication error while the inbox works, Microsoft may be refusing SMTP for the account. Tell us in the issue above with the exact error; this part is the least verified.
