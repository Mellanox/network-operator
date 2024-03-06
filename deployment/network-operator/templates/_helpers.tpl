{{/*
  Copyright 2020 NVIDIA

  Licensed under the Apache License, Version 2.0 (the "License");
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at

      http://www.apache.org/licenses/LICENSE-2.0

  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an "AS IS" BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific language governing permissions and
  limitations under the License.
*/}}
{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "network-operator.name" -}}
{{- default .Chart.Name .Values.operator.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "network-operator.fullname" -}}
{{- if .Values.operator.fullnameOverride }}
{{- .Values.operator.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.operator.nameOverride }}
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
{{- define "network-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "network-operator.labels" -}}
helm.sh/chart: {{ include "network-operator.chart" . }}
{{ include "network-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "network-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "network-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
imagePullSecrets helpers
*/}}
{{- define "network-operator.operator.imagePullSecrets" }}
{{- $imagePullSecrets := list }}
{{- if .Values.operator.imagePullSecrets }}
{{- range .Values.operator.imagePullSecrets }}
{{- $imagePullSecrets  = append $imagePullSecrets  (dict "name" . ) }}
{{- end }}
{{- else }}
{{- if .Values.imagePullSecrets }}
{{- range .Values.imagePullSecrets }}
{{- $imagePullSecrets  = append $imagePullSecrets  (dict "name" . ) }}
{{- end }}
{{- end }}
{{- end }}
{{- $imagePullSecrets | toJson }}
{{- end }}

{{- define "network-operator.ofed.imagePullSecrets" }}
{{- $imagePullSecrets := list }}
{{- if .Values.ofedDriver.imagePullSecrets }}
{{- range .Values.ofedDriver.imagePullSecrets }}
{{- $imagePullSecrets  = append $imagePullSecrets  . }}
{{- end }}
{{- else }}
{{- if .Values.imagePullSecrets }}
{{- range .Values.imagePullSecrets }}
{{- $imagePullSecrets  = append $imagePullSecrets  . }}
{{- end }}
{{- end }}
{{- end }}
{{- $imagePullSecrets | toJson }}
{{- end }}

{{- define "network-operator.rdmaSharedDevicePlugin.imagePullSecrets" }}
{{- $imagePullSecrets := list }}
{{- if .Values.rdmaSharedDevicePlugin.imagePullSecrets }}
{{- range .Values.rdmaSharedDevicePlugin.imagePullSecrets }}
{{- $imagePullSecrets  = append $imagePullSecrets  . }}
{{- end }}
{{- else }}
{{- if .Values.imagePullSecrets }}
{{- range .Values.imagePullSecrets }}
{{- $imagePullSecrets  = append $imagePullSecrets  . }}
{{- end }}
{{- end }}
{{- end }}
{{- $imagePullSecrets | toJson }}
{{- end }}

{{- define "network-operator.sriovDevicePlugin.imagePullSecrets" }}
{{- $imagePullSecrets := list }}
{{- if .Values.sriovDevicePlugin.imagePullSecrets }}
{{- range .Values.sriovDevicePlugin.imagePullSecrets }}
{{- $imagePullSecrets  = append $imagePullSecrets  . }}
{{- end }}
{{- else }}
{{- if .Values.imagePullSecrets }}
{{- range .Values.imagePullSecrets }}
{{- $imagePullSecrets  = append $imagePullSecrets  . }}
{{- end }}
{{- end }}
{{- end }}
{{- $imagePullSecrets | toJson }}
{{- end }}

{{- define "network-operator.ibKubernetes.imagePullSecrets" }}
{{- $imagePullSecrets := list }}
{{- if .Values.ibKubernetes.imagePullSecrets }}
{{- range .Values.ibKubernetes.imagePullSecrets }}
{{- $imagePullSecrets  = append $imagePullSecrets  . }}
{{- end }}
{{- else }}
{{- if .Values.imagePullSecrets }}
{{- range .Values.imagePullSecrets }}
{{- $imagePullSecrets  = append $imagePullSecrets  . }}
{{- end }}
{{- end }}
{{- end }}
{{- $imagePullSecrets | toJson }}
{{- end }}

{{- define "network-operator.secondaryNetwork.cniPlugins.imagePullSecrets" }}
{{- $imagePullSecrets := list }}
{{- if .Values.secondaryNetwork.cniPlugins.imagePullSecrets }}
{{- range .Values.secondaryNetwork.cniPlugins.imagePullSecrets }}
{{- $imagePullSecrets  = append $imagePullSecrets  . }}
{{- end }}
{{- else }}
{{- if .Values.imagePullSecrets }}
{{- range .Values.imagePullSecrets }}
{{- $imagePullSecrets  = append $imagePullSecrets  . }}
{{- end }}
{{- end }}
{{- end }}
{{- $imagePullSecrets | toJson }}
{{- end }}

{{- define "network-operator.secondaryNetwork.multus.imagePullSecrets" }}
{{- $imagePullSecrets := list }}
{{- if .Values.secondaryNetwork.multus.imagePullSecrets }}
{{- range .Values.secondaryNetwork.multus.imagePullSecrets }}
{{- $imagePullSecrets  = append $imagePullSecrets  . }}
{{- end }}
{{- else }}
{{- if .Values.imagePullSecrets }}
{{- range .Values.imagePullSecrets }}
{{- $imagePullSecrets  = append $imagePullSecrets  . }}
{{- end }}
{{- end }}
{{- end }}
{{- $imagePullSecrets | toJson }}
{{- end }}

{{- define "network-operator.secondaryNetwork.ipamPlugin.imagePullSecrets" }}
{{- $imagePullSecrets := list }}
{{- if .Values.secondaryNetwork.ipamPlugin.imagePullSecrets }}
{{- range .Values.secondaryNetwork.ipamPlugin.imagePullSecrets }}
{{- $imagePullSecrets  = append $imagePullSecrets  . }}
{{- end }}
{{- else }}
{{- if .Values.imagePullSecrets }}
{{- range .Values.imagePullSecrets }}
{{- $imagePullSecrets  = append $imagePullSecrets  . }}
{{- end }}
{{- end }}
{{- end }}
{{- $imagePullSecrets | toJson }}
{{- end }}

{{- define "network-operator.nvIpam.imagePullSecrets" }}
{{- $imagePullSecrets := list }}
{{- if .Values.nvIpam.imagePullSecrets }}
{{- range .Values.nvIpam.imagePullSecrets }}
{{- $imagePullSecrets  = append $imagePullSecrets  . }}
{{- end }}
{{- else }}
{{- if .Values.imagePullSecrets }}
{{- range .Values.imagePullSecrets }}
{{- $imagePullSecrets  = append $imagePullSecrets  . }}
{{- end }}
{{- end }}
{{- end }}
{{- $imagePullSecrets | toJson }}
{{- end }}

{{- define "network-operator.nicFeatureDiscovery.imagePullSecrets" }}
{{- $imagePullSecrets := list }}
{{- if .Values.nicFeatureDiscovery.imagePullSecrets }}
{{- range .Values.nicFeatureDiscovery.imagePullSecrets }}
{{- $imagePullSecrets  = append $imagePullSecrets  . }}
{{- end }}
{{- else }}
{{- if .Values.imagePullSecrets }}
{{- range .Values.imagePullSecrets }}
{{- $imagePullSecrets  = append $imagePullSecrets  . }}
{{- end }}
{{- end }}
{{- end }}
{{- $imagePullSecrets | toJson }}
{{- end }}

{{- define "network-operator.docaTelemetryService.imagePullSecrets" }}
{{- $imagePullSecrets := list }}
{{- if .Values.docaTelemetryService.imagePullSecrets }}
{{- range .Values.docaTelemetryService.imagePullSecrets }}
{{- $imagePullSecrets  = append $imagePullSecrets  . }}
{{- end }}
{{- else }}
{{- if .Values.imagePullSecrets }}
{{- range .Values.imagePullSecrets }}
{{- $imagePullSecrets  = append $imagePullSecrets  . }}
{{- end }}
{{- end }}
{{- end }}
{{- $imagePullSecrets | toJson }}
{{- end }}

