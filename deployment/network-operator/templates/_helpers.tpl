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
{{- if .Values.operator.imagePullSecrets }}
{{- range .Values.operator.imagePullSecrets }}
  - name: {{ . }}
{{- end }}
{{- else }}
{{- if .Values.imagePullSecrets }}
{{- range .Values.imagePullSecrets }}
  - name: {{ . }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}

{{- define "network-operator.ofed.imagePullSecrets" }}
{{- if .Values.ofedDriver.imagePullSecrets }}
{{- range .Values.ofedDriver.imagePullSecrets }}
  - {{ . }}
{{- end }}
{{- else }}
{{- if .Values.imagePullSecrets }}
{{- range .Values.imagePullSecrets }}
  - {{ . }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}

{{- define "network-operator.nvPeerDriver.imagePullSecrets" }}
{{- if .Values.nvPeerDriver.imagePullSecrets }}
{{- range .Values.nvPeerDriver.imagePullSecrets }}
  - {{ . }}
{{- end }}
{{- else }}
{{- if .Values.imagePullSecrets }}
{{- range .Values.imagePullSecrets }}
  - {{ . }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}

{{- define "network-operator.rdmaSharedDevicePlugin.imagePullSecrets" }}
{{- if .Values.rdmaSharedDevicePlugin.imagePullSecrets }}
{{- range .Values.rdmaSharedDevicePlugin.imagePullSecrets }}
  - {{ . }}
{{- end }}
{{- else }}
{{- if .Values.imagePullSecrets }}
{{- range .Values.imagePullSecrets }}
  - {{ . }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}

{{- define "network-operator.sriovDevicePlugin.imagePullSecrets" }}
{{- if .Values.sriovDevicePlugin.imagePullSecrets }}
{{- range .Values.sriovDevicePlugin.imagePullSecrets }}
  - {{ . }}
{{- end }}
{{- else }}
{{- if .Values.imagePullSecrets }}
{{- range .Values.imagePullSecrets }}
  - {{ . }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}

{{- define "network-operator.ibKubernetes.imagePullSecrets" }}
{{- if .Values.ibKubernetes.imagePullSecrets }}
{{- range .Values.ibKubernetes.imagePullSecrets }}
  - {{ . }}
{{- end }}
{{- else }}
{{- if .Values.imagePullSecrets }}
{{- range .Values.imagePullSecrets }}
  - {{ . }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}

{{- define "network-operator.secondaryNetwork.cniPlugins.imagePullSecrets" }}
{{- if .Values.secondaryNetwork.cniPlugins.imagePullSecrets }}
{{- range .Values.secondaryNetwork.cniPlugins.imagePullSecrets }}
  - {{ . }}
{{- end }}
{{- else }}
{{- if .Values.imagePullSecrets }}
{{- range .Values.imagePullSecrets }}
  - {{ . }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}

{{- define "network-operator.secondaryNetwork.multus.imagePullSecrets" }}
{{- if .Values.secondaryNetwork.multus.imagePullSecrets }}
{{- range .Values.secondaryNetwork.multus.imagePullSecrets }}
  - {{ . }}
{{- end }}
{{- else }}
{{- if .Values.imagePullSecrets }}
{{- range .Values.imagePullSecrets }}
  - {{ . }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}

{{- define "network-operator.secondaryNetwork.ipamPlugin.imagePullSecrets" }}
{{- if .Values.secondaryNetwork.ipamPlugin.imagePullSecrets }}
{{- range .Values.secondaryNetwork.ipamPlugin.imagePullSecrets }}
  - {{ . }}
{{- end }}
{{- else }}
{{- if .Values.imagePullSecrets }}
{{- range .Values.imagePullSecrets }}
  - {{ . }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}

{{- define "network-operator.nvIpam.imagePullSecrets" }}
{{- if .Values.nvIpam.imagePullSecrets }}
{{- range .Values.nvIpam.imagePullSecrets }}
  - {{ . }}
{{- end }}
{{- else }}
{{- if .Values.imagePullSecrets }}
{{- range .Values.imagePullSecrets }}
  - {{ . }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}
