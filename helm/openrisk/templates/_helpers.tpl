{{/*
The TOTP encryption key (#1057): secrets.inline.mfaEncryptionKey, or, for a
values file written before it existed, backend.env.MFA_ENCRYPTION_KEY. Falling
back keeps an upgrade on the SAME key: a new one would leave every enrolled
second factor undecryptable. Empty when neither is set, which secrets.yaml
refuses (#1062).
*/}}
{{- define "openrisk.mfaEncryptionKey" -}}
{{- $inline := .Values.secrets.inline.mfaEncryptionKey | default "" -}}
{{- $legacy := (.Values.backend.env | default dict).MFA_ENCRYPTION_KEY | default "" -}}
{{- $inline | default $legacy -}}
{{- end -}}
