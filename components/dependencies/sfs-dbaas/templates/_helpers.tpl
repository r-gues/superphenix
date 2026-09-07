{{/*
Expand the name of the chart.
*/}}
{{- define "sfs-dbaas.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "sfs-dbaas.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "sfs-dbaas.labels" -}}
helm.sh/chart: {{ include "sfs-dbaas.chart" . }}
{{ include "sfs-dbaas.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.Version | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
superphenix.net/gitops: {{ .Values.gitops | quote }}
superphenix.net/organizationID: spx-{{ .Values.organizationID }}
superphenix.net/projectID: spx-{{ .Values.projectID }}
{{- if .Values.gitops }}
superphenix.net/organizationName: {{ .Values.organizationName | quote }}
superphenix.net/projectName: {{ .Values.projectName | quote }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "sfs-dbaas.selectorLabels" -}}
app.kubernetes.io/name: {{ include "sfs-dbaas.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Returns the SPX effective ID of a resource
*/}}
{{- define "sfs-dbaas.spxEID" -}}
{{- printf "spx-%s" (include "sfs-dbaas.getUUIDv5" (dict "NS" .projectID "NAME" .localID)) }}
{{- end }}

{{/*
Generate UUIDv5 through external templating
*/}}
{{- define "sfs-dbaas.getUUIDv5" -}}
{{- printf "<spx-uuidv5 %s %s>" .NS .NAME }}
{{- end }}

{{/*
Per-database identity labels, shared by every resource a database owns.
Expects a dict with "root" (the chart context), "localID", "effectiveID" and
"database".
*/}}
{{- define "sfs-dbaas.resourceLabels" -}}
{{- include "sfs-dbaas.labels" .root }}
superphenix.net/resourceEffectiveID: {{ .effectiveID | quote }}
superphenix.net/resourceLocalID: {{ .localID | quote }}
superphenix.net/resourceName: {{ .database.name | default .localID | quote }}
{{- end }}

{{/*
Validate one database entry and fail the render with an actionable message
rather than letting CloudNativePG reject a half-built Cluster later.
Expects a dict with "localID" and "database".
*/}}
{{- define "sfs-dbaas.validate" -}}
{{- $localID := .localID -}}
{{- $db := .database -}}
{{- $engine := $db.engine | default "postgresql" -}}
{{- if ne $engine "postgresql" -}}
{{- fail (printf "database %q: engine must be \"postgresql\", got %q" $localID $engine) -}}
{{- end -}}
{{- if not $db.version -}}
{{- fail (printf "database %q: version is required" $localID) -}}
{{- end -}}
{{- $instances := $db.instances | default 0 | int -}}
{{- if or (lt $instances 1) (gt $instances 5) -}}
{{- fail (printf "database %q: instances must be between 1 and 5, got %d" $localID $instances) -}}
{{- end -}}
{{- if not ($db.storage).size -}}
{{- fail (printf "database %q: storage.size is required" $localID) -}}
{{- end -}}
{{- if and (($db.network).publicAccess) (not ($db.network).vip) -}}
{{- fail (printf "database %q: network.publicAccess requires network.vip" $localID) -}}
{{- end -}}
{{- if and (($db.network).publicAccess) (not (($db.network).publicAccess).eipLocalId) -}}
{{- fail (printf "database %q: network.publicAccess requires an eipLocalId" $localID) -}}
{{- end -}}
{{- end }}

{{/*
Resolve the container image of a database: the explicit `image` when set,
otherwise the default repository tagged with the requested major version.
Expects a dict with "root" and "database".
*/}}
{{- define "sfs-dbaas.image" -}}
{{- if .database.image -}}
{{- .database.image -}}
{{- else -}}
{{- printf "%s:%s" .root.Values.databaseDefaults.imageRepository (.database.version | toString) -}}
{{- end -}}
{{- end }}
