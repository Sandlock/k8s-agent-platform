{{/*
Expand the name of the chart.
*/}}
{{- define "sandlock.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Full release name.
*/}}
{{- define "sandlock.fullname" -}}
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
Common labels.
*/}}
{{- define "sandlock.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
app.kubernetes.io/name: {{ include "sandlock.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels for a given component.
*/}}
{{- define "sandlock.selectorLabels" -}}
app.kubernetes.io/name: {{ include "sandlock.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: {{ . }}
{{- end }}

{{/*
Resolve image for a given component (operator, controlplane, supervisor).
Usage: include "sandlock.image" (list . "operator")
*/}}
{{- define "sandlock.image" -}}
{{- $ctx := index . 0 -}}
{{- $component := index . 1 -}}
{{- $override := index $ctx.Values $component -}}
{{- if and $override (not (empty $override.image)) -}}
{{ $override.image }}:{{ $ctx.Values.imageTag }}
{{- else -}}
{{ $ctx.Values.registry }}/{{ $ctx.Values.owner }}/sandlock-{{ $component }}:{{ $ctx.Values.imageTag }}
{{- end -}}
{{- end }}

{{/*
System namespace.
*/}}
{{- define "sandlock.systemNamespace" -}}
{{ .Values.systemNamespace | default "sandlock-system" }}
{{- end }}

{{/*
Sandbox namespace.
*/}}
{{- define "sandlock.sandboxNamespace" -}}
{{ .Values.sandboxNamespace | default "sandboxes" }}
{{- end }}
