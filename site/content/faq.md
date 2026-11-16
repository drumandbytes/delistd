---
title: "Questions and answers"
description: "Plain answers about Delistd: what it does, what it needs from you, where your data goes, what to expect from brokers, and what to prepare for a paid setup session."
---

## The basics

### What does Delistd do?

It sends data removal requests to the data brokers in its list, from your own email account, worded for where you live (GDPR Article 17 for the EU, UK and EEA; CCPA for California). It reads the replies, sorts them, and tells you which ones need something from you. It keeps a record of what was sent and when.

### Is it really free?

Yes. The tool, the broker list and the docs are MIT licensed. [Paid setup help](/support/) exists for people who would rather not do it alone, and nothing in the tool is held back for customers.

### Where does my data go?

Your details stay on your computer. The requests go from your mail account to the brokers, which is the point of the tool. Nothing goes to the Delistd project: no account, no sync, no telemetry. The [privacy page](/privacy/) has the details.

### Do I need to be technical?

No. The [step-by-step guide](/docs/) is written for people who have never used a command line, and takes about 15 minutes. If you would rather have someone with you, see [paid setup](/support/).

## What it needs

### What do I need?

A computer running macOS, Windows or Linux (it is not a phone app), an email address to send from, and your name and, ideally, your postal address, because brokers match requests to records using them.

### Why an app password and not my normal password?

An app password is a separate, 16-character password your provider makes for one app. It can send and read mail only, and you can delete it at any time. Your real password is never given to Delistd. Gmail, iCloud and Fastmail work this way; Outlook.com, Hotmail and Live use a Microsoft sign-in; Proton Mail needs its Bridge app.

### Should I use my main email address?

You can, but a dedicated address for removal requests keeps replies and any new spam away from your main inbox. We recommend one, and we create it with you in a setup session if you like.

### I don't want to give any tool access to my email.

Then use the "send them yourself" option. Delistd prepares each email and you send it from your own mail app, then mark it as sent. It is slower, which is why many people hand-send a few and let Delistd send the rest.

## What to expect

### Will brokers delete everything?

Not all of them, and nobody can promise they will. Some act quickly, some ask for a form or ID, some never reply. Delistd records what each one answered and re-sends on a schedule, because brokers re-list people. If a broker ignores the deadline, the export report is your evidence for a complaint to your data protection authority. See [when a broker ignores you](/guides/when-a-broker-ignores-you/).

### How long does it take?

Under GDPR a broker must answer within a month, which can be extended in complex cases. California's law allows 45 days. Replies arrive over weeks, so the first send is the start, not the end.

### Does Delistd find where my data is?

No. It sends requests to the brokers in its list. It does not search the web for your details and cannot verify that a broker really deleted anything, only record what they said.

## Paid setup sessions

### What do I need ready for a session?

- A computer you can install software on (macOS, Linux or Windows).
- An email account you are happy to dedicate to removal requests, or we create one together.
- Your phone, if your email provider sends a code when you sign in.
- Your name and the postal address you want brokers to use.

You do not need an ID document or any technical knowledge.

### What do you see and keep?

During the session we can see your screen. Nothing is recorded. You type your mailbox password yourself on your own machine, and we never receive it or keep any access afterwards.

### How do I book, pay and change a booking?

You get a quote with the final price, then book a slot and pay by card. The calendar offers slots at least 24 hours ahead. Anything sooner is only by agreement, for an extra €10. You can move a session free of charge with 24 hours' notice. The full wording is in the [terms](/support/terms/).

### Do you offer a hosted version?

No, and that is deliberate. A hosted copy would mean holding your mailbox credentials and your personal details. Doing it on your own machine keeps both with you.

Something missing from this page, or a few words on how Delistd worked for you? Use the [feedback form on GitHub](https://github.com/drumandbytes/delistd/issues/new?template=feedback.yml), or just write to [{{< param supportEmail >}}](mailto:{{< param supportEmail >}}) if you don't use GitHub. If Delistd has been useful, a star on [GitHub](https://github.com/drumandbytes/delistd) or a share helps it reach more people.
