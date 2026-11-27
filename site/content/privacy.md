---
title: "Privacy"
description: "What this website collects (cookieless, aggregate visit counts), what the Delistd tool on your machine doesn't (nothing about your use reaches us), and what Drumandbytes OÜ keeps when you ask for paid setup help."
---

Three different things live under this name, and they have very different answers: the tool and this website, which collect next to nothing, and the optional paid setup service, which does keep some of your details.

## The tool

Delistd runs on your own machine. Your profile, the brokers you've contacted, and
every response you've received stay in a local database on that machine. Removal
requests go straight from your mail account to the broker.

Nothing is sent to us. There is no account, no sync, no telemetry, and no
server of ours in the path. We could not tell you how many people use Delistd, or
which brokers they've written to, because we never receive any of it.

That isn't a promise about our intentions; it's a property of how the thing is
built, and you can check it in the source.

### Signing in to Outlook.com

Microsoft only accepts OAuth for Outlook.com, Hotmail and Live accounts. If you use that sign-in, you approve Delistd at Microsoft's own page and your computer talks to Microsoft directly. The sign-in (a refresh token) is stored on your computer, encrypted like a password, and used only to send your requests and read replies. Drumandbytes registered the "Delistd" app with Microsoft but receives no tokens, no mail and no list of who signed in. Microsoft, as the provider of your account, processes the sign-in under its own privacy terms. Microsoft marks the app as from an unverified publisher on its approval page. Revoke Delistd at any time in your Microsoft account's app permissions.

## This website

The site is static pages served by Cloudflare Pages. Cloudflare sits in front
of it and keeps two kinds of aggregate counts:

- Page views (Cloudflare Web Analytics): which pages get visited, roughly
  how many times, and referrers.
- Referrals: which site or link sent a visit: the referring site's name (or
  a short tag we add to our own links) and the page path, without the query
  string. Kept for three months.

Neither sets cookies, uses a cross-site identifier, or stores your IP
address. There's no way for us to single you out in them, which is also why
there's no cookie banner to click through.

One processor is involved in serving you this page:

| Who | What they see | Why |
| --- | --- | --- |
| Cloudflare | Your IP address and request headers, as any web server does | Serving the site, aggregate visitor counts |

## Paid setup and support

The tool and this website never see your credentials or your broker replies. If you ask [Drumandbytes OÜ](/support/) for a quote, or pay for setup help, that is a separate relationship, and Drumandbytes OÜ (Estonia, details below) is the controller of what it involves.

What we collect from you: your name and email address, what you tell us about your setup (operating system, country, mail provider), your chosen time, and the billing details on an invoice. You need to give us these for us to quote and do the work.

| What | Why | How long |
| --- | --- | --- |
| Name, email address and what you wrote to us | To answer you, give a quote and do the work | While the enquiry or the work is open; if you don't become a customer, then deleted |
| Correspondence with customers | To support you and deal with any claim | Up to three years after the work, then deleted |
| Billing details on an invoice | To bill you and meet our accounting duties | Seven years, as the Estonian Accounting Act requires |

Live setup sessions are not recorded. During a session we can see your screen, which may show your profile or broker replies, but we keep none of it. You type your mailbox password on your own computer; we never receive it, write it down or keep any access to your machine afterwards.

Who else handles it: We use a few providers to run this, and they act on our instructions:

| Who | What they handle | Why |
| --- | --- | --- |
| Stripe | Your payment details and the invoice | Taking payment, receipts and any tax |
| Our email provider | Your messages to and from us | Answering you |

Some of these providers may process data outside the EEA. Where they do, they rely on safeguards the law recognises, such as an adequacy decision or standard contractual clauses. We don't sell your details or use them for marketing, and we make no automated decisions about you.

The [terms for paid setup](/support/terms/) cover the rest.

## Lawful basis

Aggregate, cookieless analytics on a public site: legitimate interest,
GDPR Article 6(1)(f). Knowing roughly how many people find the project is
proportionate, and it can't identify anyone.

Paid setup and support: taking steps you ask for before a contract and
performing it (Article 6(1)(b)), and keeping invoices because the law requires
it (Article 6(1)(c)).

## Your rights

We hold no personal data about you from this website; nothing is stored beyond
the aggregate counts and whatever the processors above keep in their own logs.
So there's nothing of yours for us to export or delete.

If you have written to us or been a customer, you can ask us what we hold, have
it corrected or deleted, ask us to restrict how we use it, receive it in a usable
format, or object to our using it. Deletion does not reach invoices we must keep
by law. Where we rely on your consent, you can withdraw it at any time. Write to
<privacy@drumandbytes.com> and we answer within a month.

If you use the tool, the data is on your computer, under your control. Delete
the database and it's gone.

## Who is responsible

Delistd is run by Drumandbytes OÜ (registry code 17599202), Uus-Sadama tn 21-207,
10120 Tallinn, Estonia. Write to <privacy@drumandbytes.com> about anything on
this page, including any of the rights above.

You can also raise anything publicly as a
[GitHub issue](https://github.com/drumandbytes/delistd/issues/new).

If you think we've got this wrong, you can complain to the Estonian Data
Protection Inspectorate (Andmekaitse Inspektsioon, <https://www.aki.ee>) or, if
you're elsewhere in the EU or EEA, to your national data protection authority; see [Privacy authorities](/authorities/).
