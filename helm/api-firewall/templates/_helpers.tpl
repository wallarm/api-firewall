{{/*
Expand the name of the chart.
*/}}
{{- define "api-firewall.name" -}}
{{- default .Chart.Name .Values.apiFirewall.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "api-firewall.fullname" -}}
{{- $name := default .Chart.Name .Values.apiFirewall.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "api-firewall.serviceAccountName" -}}
{{- if not .Values.apiFirewall.serviceAccount.name -}}
{{ template "api-firewall.fullname" . }}
{{- else -}}
{{- .Values.apiFirewall.serviceAccount.name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{/*
Contruct and return the image to use
*/}}
{{- define "api-firewall.image" -}}
{{- if not .Values.apiFirewall.image.registry -}}
{{ printf "%s:%s" .Values.apiFirewall.image.name .Values.apiFirewall.image.tag }}
{{- else -}}
{{ printf "%s/%s:%s" .Values.apiFirewall.image.registry .Values.apiFirewall.image.name .Values.apiFirewall.image.tag }}
{{- end -}}
{{- end -}}

{{/*
Contruct and return the target service name to use
*/}}
{{- define "api-firewall.targetServiceName" -}}
{{- if .Values.apiFirewall.target.name -}}
{{ .Values.apiFirewall.target.name }}
{{- else -}}
{{- if eq .Values.apiFirewall.target.type "endpoints" -}}
{{ template "api-firewall.fullname" . }}-target
{{- else -}}
{{ fail "Value for target service name refers to existing service (.Values.apiFirewall.target.type), no name of service isn't defined (.Values.apiFirewall.target.name)" }}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Return the appropriate apiVersion for deployment.
*/}}
{{- define "deployment.apiVersion" -}}
{{- if .Capabilities.APIVersions.Has "apps/v1/Deployment" -}}
{{- print "apps/v1" -}}
{{- else -}}
{{- print "extensions/v1beta1" -}}
{{- end -}}
{{- end -}}

{{/*
Return the appropriate apiVersion for PodSecurityPolicy kind of objects.
*/}}
{{- define "podSecurityPolicy.apiVersion" -}}
{{- if .Capabilities.APIVersions.Has "policy/v1beta1/PodSecurityPolicy" -}}
{{- print "policy/v1beta1" -}}
{{- else -}}
{{- print "extensions/v1beta1" -}}
{{- end -}}
{{- end -}}

{{/*
Return the appropriate apiVersion for PodDisruptionBudget kind of objects.
*/}}
{{- define "podDisruptionBudget.apiVersion" -}}
{{- if .Capabilities.APIVersions.Has "policy/v1beta1/PodDisruptionBudget" -}}
{{- print "policy/v1beta1" -}}
{{- else -}}
{{- print "extensions/v1beta1" -}}
{{- end -}}
{{- end -}}

{{/*
Return the appropriate apiVersion for Role kind of objects.
*/}}
{{- define "role.apiVersion" -}}
{{- if .Capabilities.APIVersions.Has "rbac.authorization.k8s.io/v1/Role" -}}
{{- print "rbac.authorization.k8s.io/v1" -}}
{{- else -}}
{{- if .Capabilities.APIVersions.Has "rbac.authorization.k8s.io/v1beta1/Role" -}}
{{- print "rbac.authorization.k8s.io/v1beta1" -}}
{{- else -}}
{{- print "rbac.authorization.k8s.io/v1alpha1" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Return the appropriate apiVersion for RoleBinding kind of objects.
*/}}
{{- define "roleBinding.apiVersion" -}}
{{- if .Capabilities.APIVersions.Has "rbac.authorization.k8s.io/v1/RoleBinding" -}}
{{- print "rbac.authorization.k8s.io/v1" -}}
{{- else -}}
{{- if .Capabilities.APIVersions.Has "rbac.authorization.k8s.io/v1beta1/RoleBinding" -}}
{{- print "rbac.authorization.k8s.io/v1beta1" -}}
{{- else -}}
{{- print "rbac.authorization.k8s.io/v1alpha1" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Return the appropriate apiVersion for Ingress kind of objects.
*/}}
{{- define "ingress.apiVersion" -}}
{{- if .Capabilities.APIVersions.Has "networking.k8s.io/v1/Ingress" -}}
{{- print "networking.k8s.io/v1" -}}
{{- else -}}
{{- if .Capabilities.APIVersions.Has "networking.k8s.io/v1beta1/Ingress" -}}
{{- print "networking.k8s.io/v1beta1" -}}
{{- else -}}
{{- print "extensions/v1beta1" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Return the appropriate apiVersion for HorizontalPodAutoscaler kind of objects.
*/}}
{{- define "horizontalPodAutoscaler.apiVersion" -}}
{{- if .Capabilities.APIVersions.Has "autoscaling/v2beta2/HorizontalPodAutoscaler" -}}
{{- print "autoscaling/v2beta2" -}}
{{- else -}}
{{- if .Capabilities.APIVersions.Has "autoscaling/v2beta1/HorizontalPodAutoscaler" -}}
{{- print "autoscaling/v2beta1" -}}
{{- else -}}
{{- fail "Kubernetes Autoscaling API ti old. You need to upgrade your cluster for using this feature" -}}
{{- end -}}
{{- end -}}
{{- end -}}
