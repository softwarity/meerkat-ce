{{- define "meerkat.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "meerkat.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name (include "meerkat.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "meerkat.labels" -}}
app.kubernetes.io/name: {{ include "meerkat.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
{{- end -}}

{{- define "meerkat.selectorLabels" -}}
app.kubernetes.io/name: {{ include "meerkat.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "meerkat.adminSecretName" -}}
{{- if .Values.admin.existingSecret -}}
{{- .Values.admin.existingSecret -}}
{{- else -}}
{{- printf "%s-admin" (include "meerkat.fullname" .) -}}
{{- end -}}
{{- end -}}

{{/* A gateway is CLUSTERED as soon as it has a shared database: that is what
     turns several pods into one installation, and what makes the local
     directory disposable. */}}
{{- define "meerkat.clustered" -}}
{{- if or .Values.database.url .Values.database.existingSecret -}}yes{{- end -}}
{{- end -}}

{{- define "meerkat.stateSecretName" -}}
{{- if .Values.database.existingSecret -}}
{{- .Values.database.existingSecret -}}
{{- else -}}
{{- printf "%s-state" (include "meerkat.fullname" .) -}}
{{- end -}}
{{- end -}}

{{- define "meerkat.vaultSecretName" -}}
{{- if .Values.vault.existingSecret -}}
{{- .Values.vault.existingSecret -}}
{{- else -}}
{{- printf "%s-state" (include "meerkat.fullname" .) -}}
{{- end -}}
{{- end -}}

{{/*
The Secret holding the internal plane's certificate, if any: the one named,
or the one OpenShift's service CA writes for the internal Service.
*/}}
{{- define "meerkat.internalTlsSecret" -}}
{{- if .Values.internalTls.secretName -}}
{{ .Values.internalTls.secretName }}
{{- else if .Values.internalTls.openshiftServingCert -}}
{{ include "meerkat.fullname" . }}-internal-tls
{{- end -}}
{{- end }}

{{/*
The Certificate, and its Secret, the gateway asks cert-manager for itself
(internalTls.certManager, SSL-13); empty when it may not.
*/}}
{{- define "meerkat.issuedSecret" -}}
{{- if ((.Values.internalTls | default dict).certManager) -}}
{{ include "meerkat.fullname" . }}-internal-issued
{{- end -}}
{{- end }}
