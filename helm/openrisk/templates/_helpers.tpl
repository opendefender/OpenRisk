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

{{/*
Optional keys that older values files set as backend.env.<NAME> (#1317). The
inline value wins; the legacy one is moved into the Secret, so an upgrade keeps
the operator's key and it never sits in clear in the Deployment.
*/}}
{{- define "openrisk.mfaEncryptionPreviousKey" -}}
{{- .Values.secrets.inline.mfaEncryptionPreviousKey | default ((.Values.backend.env | default dict).MFA_ENCRYPTION_PREVIOUS_KEY) | default "" -}}
{{- end -}}

{{- define "openrisk.auditExportKey" -}}
{{- .Values.secrets.inline.auditExportKey | default ((.Values.backend.env | default dict).AUDIT_EXPORT_KEY) | default "" -}}
{{- end -}}

{{/*
Who runs this instance (D-081): backend.deployment, rendered into
OPENRISK_DEPLOYMENT. No default on purpose. "hosted" read by mistake caps a
self-hoster at Free, "self_hosted" read by mistake opens the SaaS to every
tenant, so the render refuses an empty or unknown value instead of guessing.
*/}}
{{- define "openrisk.deployment" -}}
{{- $d := required "backend.deployment is required: set it to self_hosted for your own install, or hosted for the OpenDefender service (D-081)" .Values.backend.deployment -}}
{{- if not (has $d (list "self_hosted" "hosted")) -}}
{{- fail (printf "backend.deployment is %q: set it to self_hosted for your own install, or hosted for the OpenDefender service (D-081)" $d) -}}
{{- end -}}
{{- if hasKey (.Values.backend.env | default dict) "OPENRISK_DEPLOYMENT" -}}
{{- fail "backend.env.OPENRISK_DEPLOYMENT is not read: set backend.deployment instead (D-081)" -}}
{{- end -}}
{{- $d -}}
{{- end -}}
