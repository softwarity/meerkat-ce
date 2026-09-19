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
