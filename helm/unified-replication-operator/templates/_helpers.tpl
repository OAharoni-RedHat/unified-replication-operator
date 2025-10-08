{{/*
Expand the name of the chart.
*/}}
{{- define "unified-replication-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "unified-replication-operator.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "unified-replication-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "unified-replication-operator.labels" -}}
helm.sh/chart: {{ include "unified-replication-operator.chart" . }}
{{ include "unified-replication-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "unified-replication-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "unified-replication-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "unified-replication-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "unified-replication-operator.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the webhook certificate secret
*/}}
{{- define "unified-replication-operator.webhookCertSecret" -}}
{{- if .Values.webhook.certificate.existingSecret }}
{{- .Values.webhook.certificate.existingSecret }}
{{- else }}
{{- printf "%s-webhook-cert" (include "unified-replication-operator.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Create the webhook service name
*/}}
{{- define "unified-replication-operator.webhookServiceName" -}}
{{- printf "%s-webhook-service" (include "unified-replication-operator.fullname" .) }}
{{- end }}

