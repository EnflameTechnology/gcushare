{{- define "schedulerConfig.apiVersion" -}}
{{- if semverCompare ">=1.24-0" .Capabilities.KubeVersion.Version -}}
kubescheduler.config.k8s.io/v1
{{- else -}}
kubescheduler.config.k8s.io/v1beta1
{{- end -}}
{{- end -}}
