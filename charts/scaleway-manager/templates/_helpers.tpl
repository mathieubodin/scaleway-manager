{{/*
Expand the name of the chart.
*/}}
{{- define "scaleway-manager.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "scaleway-manager.fullname" -}}
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
{{- define "scaleway-manager.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "scaleway-manager.labels" -}}
helm.sh/chart: {{ include "scaleway-manager.chart" . }}
{{ include "scaleway-manager.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "scaleway-manager.selectorLabels" -}}
app.kubernetes.io/name: {{ include "scaleway-manager.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
The port of listenAddr: the text after its last ":", decimal digits with no sign and
no leading zero, from 1 to 65535. Any other value fails the render. The length check
comes first: atoi gives 0 on overflow.
*/}}
{{- define "scaleway-manager.port" -}}
{{- $addr := toString .Values.listenAddr }}
{{- if not (contains ":" $addr) }}
{{- fail (printf "listenAddr %q: no \":\" before the port" $addr) }}
{{- end }}
{{- $port := splitList ":" $addr | last }}
{{- if or (not (regexMatch "^[1-9][0-9]*$" $port)) (gt (len $port) 5) (gt (atoi $port) 65535) }}
{{- fail (printf "listenAddr %q: the port after the last \":\" must be a decimal number from 1 to 65535, without sign or leading zero" $addr) }}
{{- end }}
{{- $port }}
{{- end }}
