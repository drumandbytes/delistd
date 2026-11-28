{{- /* The body of a "how to opt out" page; data comes from `delistd guides` (site/data/guides.json). */ -}}
**{{ .name }}** is a {{ .category_title }} data broker{{ if eq .region "eu" }} (EU){{ else if eq .region "us" }} (US){{ end }}.
{{ with .notes }}
> {{ . }}
{{ end }}

## At a glance

- **Type:** {{ .category_label }}
- **Law that applies:** {{ .law }}
- **How to opt out:** {{ .route }}
- **Response deadline:** {{ .timeline }}
{{- if .parent }}
- **Operated by:** {{ .parent }}{{ with .hq }}, {{ . }}{{ end }}
{{- else if .hq }}
- **Based in:** {{ .hq }}
{{- end }}
{{- with .holds }}
- **Data they say they hold:** {{ . }}
{{- end }}
{{- if .verification }}
- **Identity check:** {{ .verification }}
{{- else if .needs_id }}
- **Identity check:** expect to be asked for proof of identity
{{- end }}
{{- with .response_time }}
- **Response time in their policy:** {{ . }}
{{- end }}

## Opt out

{{ if .opt_out_url -}}
1. Go to **[{{ .opt_out_url }}]({{ .opt_out_url }})**.
2. Follow their removal / suppression process. You may need to confirm by email or verify your identity.
{{ with .email }}3. If the form fails, email **{{ . }}** with the request below.
{{ end -}}
{{ else if .email -}}
Email **{{ .email }}** with the request below.
{{ else -}}
No public opt-out URL or email is on file for this broker yet. Try their website's privacy page, or contact them and cite the laws below.
{{ end }}
{{ if or .gdpr .ccpa }}
<details>
<summary>Request letter templates</summary>
{{ with .gdpr }}
### GDPR request (EU/EEA residents)

```
{{ . }}
```
{{ end }}
{{ with .ccpa }}
### CCPA request (California residents)

```
{{ . }}
```
{{ end }}
</details>
{{ end }}
{{ with .category_intro }}
## About {{ lower $.category_label }}s

{{ . }}
{{ end }}
{{ with .sources }}
## Sources

Facts above come from the broker's own pages{{ with $.researched }}, read on {{ . }}{{ end }}:

{{ range . }}- <{{ . }}>
{{ end }}{{ end }}
{{ with .related }}
## Related guides

{{ range . }}- [How to opt out of {{ .name }}](/brokers/{{ .id }}/)
{{ end }}{{ end }}
## Related reading

- [Sending a GDPR Article 17 request](/guides/sending-a-gdpr-article-17-request/)
- [When a broker ignores you](/guides/when-a-broker-ignores-you/)
- [Opting out of advertising data brokers](/guides/opting-out-of-advertising-data-brokers/)
- [All brokers Delistd covers](/brokers/)

---

*Fill in the bracketed placeholders before sending. [Delistd](https://github.com/drumandbytes/delistd) can send this and track the reply for you, or you can send it yourself.*
