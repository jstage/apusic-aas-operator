package resources

import (
	"fmt"
	v1 "github.com/jstage/apusic-aas-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	defaultConsulImage = "consul:v1.8.5"

	defaultConsulImagePolicy = corev1.PullIfNotPresent

	statefulsetTemplate = `
	# StatefulSet to run the actual Consul server cluster.
		apiVersion: apps/v1
		kind: StatefulSet
		metadata:
		  name: {{ template "consul.fullname" . }}-server
		  namespace: {{ .Release.Namespace }}
		  labels:
			app: {{ template "consul.name" . }}
			chart: {{ template "consul.chart" . }}
			heritage: {{ .Release.Service }}
			release: {{ .Release.Name }}
			component: server
		spec:
		  serviceName: {{ template "consul.fullname" . }}-server
		  podManagementPolicy: Parallel
		  replicas: {{ .Values.server.replicas }}
		  {{- if (gt (int .Values.server.updatePartition) 0) }}
		  updateStrategy:
			type: RollingUpdate
			rollingUpdate:
			  partition: {{ .Values.server.updatePartition }}
		  {{- end }}
		  selector:
			matchLabels:
			  app: {{ template "consul.name" . }}
			  chart: {{ template "consul.chart" . }}
			  release: {{ .Release.Name }}
			  component: server
			  hasDNS: "true"
		  template:
			metadata:
			  labels:
				app: {{ template "consul.name" . }}
				chart: {{ template "consul.chart" . }}
				release: {{ .Release.Name }}
				component: server
				hasDNS: "true"
				{{- if .Values.server.extraLabels }}
				  {{- toYaml .Values.server.extraLabels | nindent 8 }}
				{{- end }}
			  annotations:
				"consul.hashicorp.com/connect-inject": "false"
				"consul.hashicorp.com/config-checksum": {{ include (print $.Template.BasePath "/server-config-configmap.yaml") . | sha256sum }}
				{{- if .Values.server.annotations }}
				  {{- tpl .Values.server.annotations . | nindent 8 }}
				{{- end }}
			spec:
			{{- if .Values.server.affinity }}
			  affinity:
				{{ tpl .Values.server.affinity . | nindent 8 | trim }}
			{{- end }}
			{{- if .Values.server.tolerations }}
			  tolerations:
				{{ tpl .Values.server.tolerations . | nindent 8 | trim }}
			{{- end }}
			  terminationGracePeriodSeconds: 30
			  serviceAccountName: {{ template "consul.fullname" . }}-server
			  {{- if not .Values.global.openshift.enabled }}
			  securityContext:
				{{- toYaml .Values.server.securityContext | nindent 8 }}
			  {{- end }}
			  volumes:
				- name: config
				  configMap:
					name: {{ template "consul.fullname" . }}-server-config
				{{- if .Values.global.tls.enabled }}
				- name: consul-ca-cert
				  secret:
					{{- if .Values.global.tls.caCert.secretName }}
					secretName: {{ .Values.global.tls.caCert.secretName }}
					{{- else }}
					secretName: {{ template "consul.fullname" . }}-ca-cert
					{{- end }}
					items:
					- key: {{ default "tls.crt" .Values.global.tls.caCert.secretKey }}
					  path: tls.crt
				- name: consul-server-cert
				  secret:
					secretName: {{ template "consul.fullname" . }}-server-cert
				{{- end }}
				{{- range .Values.server.extraVolumes }}
				- name: userconfig-{{ .name }}
				  {{ .type }}:
					{{- if (eq .type "configMap") }}
					name: {{ .name }}
					{{- else if (eq .type "secret") }}
					secretName: {{ .name }}
					{{- end }}
					{{- with .items }}
					items:
					{{- range . }}
					- key: {{.key}}
					  path: {{.path}}
					{{- end }}
					{{- end }}
				{{- end }}
			  {{- if .Values.server.priorityClassName }}
			  priorityClassName: {{ .Values.server.priorityClassName | quote }}
			  {{- end }}
			  containers:
				- name: consul
				  image: "{{ default .Values.global.image .Values.server.image }}"
				  env:
					- name: ADVERTISE_IP
					  valueFrom:
						fieldRef:
						  {{- if .Values.server.exposeGossipAndRPCPorts }}
						  {{- /* Server gossip and RPC ports will be exposed as a hostPort
						  on the hostIP, so they need to advertise their host ip
						  instead of their pod ip. This is to support external client
						  agents. */}}
						  fieldPath: status.hostIP
						  {{- else }}
						  fieldPath: status.podIP
						  {{- end }}
					- name: POD_IP
					  valueFrom:
						fieldRef:
						  fieldPath: status.podIP
					- name: NAMESPACE
					  valueFrom:
						fieldRef:
						  fieldPath: metadata.namespace
					{{- if (and .Values.global.gossipEncryption.secretName .Values.global.gossipEncryption.secretKey) }}
					- name: GOSSIP_KEY
					  valueFrom:
						secretKeyRef:
						  name: {{ .Values.global.gossipEncryption.secretName }}
						  key: {{ .Values.global.gossipEncryption.secretKey }}
					{{- end }}
					{{- if .Values.global.tls.enabled }}
					- name: CONSUL_HTTP_ADDR
					  value: https://localhost:8501
					- name: CONSUL_CACERT
					  value: /consul/tls/ca/tls.crt
					{{- end }}
					{{- if (and .Values.global.acls.replicationToken.secretName .Values.global.acls.replicationToken.secretKey) }}
					- name: ACL_REPLICATION_TOKEN
					  valueFrom:
						secretKeyRef:
						  name: {{ .Values.global.acls.replicationToken.secretName | quote }}
						  key: {{ .Values.global.acls.replicationToken.secretKey | quote }}
					{{- end }}
					{{- include "consul.extraEnvironmentVars" .Values.server | nindent 12 }}
				  command:
					- "/bin/sh"
					- "-ec"
					- |
					  CONSUL_FULLNAME="{{template "consul.fullname" . }}"
					  exec /bin/consul agent \
						-advertise="${ADVERTISE_IP}" \
						-bind=0.0.0.0 \
						-bootstrap-expect={{ if .Values.server.bootstrapExpect }}{{ .Values.server.bootstrapExpect }}{{ else }}{{ .Values.server.replicas }}{{ end }} \
						{{- if .Values.global.tls.enabled }}
						-hcl='ca_file = "/consul/tls/ca/tls.crt"' \
						-hcl='cert_file = "/consul/tls/server/tls.crt"' \
						-hcl='key_file = "/consul/tls/server/tls.key"' \
						{{- if .Values.global.tls.enableAutoEncrypt }}
						-hcl='auto_encrypt = {allow_tls = true}' \
						{{- end }}
						{{- if .Values.global.tls.verify }}
						-hcl='verify_incoming_rpc = true' \
						-hcl='verify_outgoing = true' \
						-hcl='verify_server_hostname = true' \
						{{- end }}
						-hcl='ports { https = 8501 }' \
						{{- if .Values.global.tls.httpsOnly }}
						-hcl='ports { http = -1 }' \
						{{- end }}
						{{- end }}
						-client=0.0.0.0 \
						-config-dir=/consul/config \
						{{- /* Always include the extraVolumes at the end so that users can
							override other Consul settings. The last -config-dir takes
							precedence. */}}
						{{- range .Values.server.extraVolumes }}
						{{- if .load }}
						-config-dir=/consul/userconfig/{{ .name }} \
						{{- end }}
						{{- end }}
						-datacenter={{ .Values.global.datacenter }} \
						-data-dir=/consul/data \
						-domain={{ .Values.global.domain }} \
						{{- if (and .Values.global.gossipEncryption.secretName .Values.global.gossipEncryption.secretKey) }}
						-encrypt="${GOSSIP_KEY}" \
						{{- end }}
						{{- if .Values.server.connect }}
						-hcl="connect { enabled = true }" \
						{{- end }}
						{{- if .Values.global.federation.enabled }}
						-hcl="connect { enable_mesh_gateway_wan_federation = true }" \
						{{- end }}
						{{- if (and .Values.global.acls.replicationToken.secretName .Values.global.acls.replicationToken.secretKey) }}
						-hcl="acl { tokens { agent = \"${ACL_REPLICATION_TOKEN}\", replication = \"${ACL_REPLICATION_TOKEN}\" } }" \
						{{- end }}
						{{- if .Values.ui.enabled }}
						-ui \
						{{- end }}
						{{- $serverSerfLANPort  := .Values.server.ports.serflan.port -}}
						{{- range $index := until (.Values.server.replicas | int) }}
						-retry-join="${CONSUL_FULLNAME}-server-{{ $index }}.${CONSUL_FULLNAME}-server.${NAMESPACE}.svc:{{ $serverSerfLANPort }}" \
						{{- end }}
						-serf-lan-port={{ .Values.server.ports.serflan.port }} \
						-server
				  volumeMounts:
					- name: data-{{ .Release.Namespace }}
					  mountPath: /consul/data
					- name: config
					  mountPath: /consul/config
					{{- if .Values.global.tls.enabled }}
					- name: consul-ca-cert
					  mountPath: /consul/tls/ca/
					  readOnly: true
					- name: consul-server-cert
					  mountPath: /consul/tls/server
					  readOnly: true
					{{- end }}
					{{- range .Values.server.extraVolumes }}
					- name: userconfig-{{ .name }}
					  readOnly: true
					  mountPath: /consul/userconfig/{{ .name }}
					{{- end }}
				  ports:
					{{- if (or (not .Values.global.tls.enabled) (not .Values.global.tls.httpsOnly)) }}
					- containerPort: 8500
					  name: http
					{{- end }}
					{{- if .Values.global.tls.enabled }}
					- containerPort: 8501
					  name: https
					{{- end }}
					- containerPort: {{ .Values.server.ports.serflan.port }}
					  {{- if .Values.server.exposeGossipAndRPCPorts }}
					  hostPort: {{ .Values.server.ports.serflan.port }}
					  {{- end }}
					  protocol: "TCP"
					  name: serflan-tcp
					- containerPort: {{ .Values.server.ports.serflan.port }}
					  {{- if .Values.server.exposeGossipAndRPCPorts }}
					  hostPort: {{ .Values.server.ports.serflan.port }}
					  {{- end }}
					  protocol: "UDP"
					  name: serflan-udp
					- containerPort: 8302
					  name: serfwan
					- containerPort: 8300
					  {{- if .Values.server.exposeGossipAndRPCPorts }}
					  hostPort: 8300
					  {{- end }}
					  name: server
					- containerPort: 8600
					  name: dns-tcp
					  protocol: "TCP"
					- containerPort: 8600
					  name: dns-udp
					  protocol: "UDP"
				  readinessProbe:
					# NOTE(mitchellh): when our HTTP status endpoints support the
					# proper status codes, we should switch to that. This is temporary.
					exec:
					  command:
						- "/bin/sh"
						- "-ec"
						- |
						  {{- if .Values.global.tls.enabled }}
						  curl \
							--cacert /consul/tls/ca/tls.crt \
							https://127.0.0.1:8501/v1/status/leader \
						  {{- else }}
						  curl http://127.0.0.1:8500/v1/status/leader \
						  {{- end }}
						  2>/dev/null | grep -E '".+"'
					failureThreshold: 2
					initialDelaySeconds: 5
					periodSeconds: 3
					successThreshold: 1
					timeoutSeconds: 5
				  {{- if .Values.server.resources }}
				  resources:
					{{- if eq (typeOf .Values.server.resources) "string" }}
					{{ tpl .Values.server.resources . | nindent 12 | trim }}
					{{- else }}
					{{- toYaml .Values.server.resources | nindent 12 }}
					{{- end }}
				  {{- end }}
			  {{- if .Values.server.nodeSelector }}
			  nodeSelector:
				{{ tpl .Values.server.nodeSelector . | indent 8 | trim }}
			  {{- end }}
		  volumeClaimTemplates:
			- metadata:
				name: data-{{ .Release.Namespace }}
			  spec:
				accessModes:
				  - ReadWriteOnce
				resources:
				  requests:
					storage: {{ .Values.server.storage }}
				{{- if .Values.server.storageClass }}
				storageClassName: {{ .Values.server.storageClass }}
				{{- end }}
		{{- end }}
`
)

type Acp struct {
	ApusicControlPlane *v1.ApusicControlPlane
}

func (acp *Acp) UIPvc(pvcName string) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: pvcName,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources:        acp.ApusicControlPlane.Spec.Resources,
			StorageClassName: acp.ApusicControlPlane.Spec.StorageClassName,
		},
	}
	return pvc
}

func (acp *Acp) UIService(deploySelector map[string]string) *corev1.Service {
	svcName := acp.ApusicControlPlane.Name + "-uisvc"

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: acp.ApusicControlPlane.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: deploySelector,
			Type:     acp.ApusicControlPlane.Spec.UiServiceType,
			Ports: []corev1.ServicePort{
				{
					Port:       8500,
					TargetPort: intstr.FromInt(8500),
				},
			},
		},
	}
	return svc
}

func (acp *Acp) Deployment(svcName string) (deploy *appsv1.Deployment, pvcName string, selector map[string]string) {
	pvcName = pvcNameGen("apc", "deploy")
	selector = labelsForApusicControlPlane(acp.ApusicControlPlane.Name, "deploy")
	deployName := acp.ApusicControlPlane.Name + "-uideploy"
	replicas := &acp.ApusicControlPlane.Spec.Replicas
	statefulName := acp.ApusicControlPlane.Name + "-stateful"
	retryJoins := make([]string, int(*replicas))
	for i := int32(0); i < *replicas; i++ {
		retryJoins[i] = fmt.Sprintf("-retry-join=%s-%d.%s.$(NAMESPACE).svc.cluster.local", statefulName, i, svcName)
	}
	uiDeployRelicas := int32(1)
	args := []string{"agent",
		"-bind=0.0.0.0",
		"-datacenter=dc1",
		"-data-dir=/data",
		"-disable-host-node-id=true",
		"-domain=cluster.local",
		"-client=0.0.0.0",
		"-ui",
	}
	args = append(args, retryJoins...)
	deploy = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployName,
			Namespace: acp.ApusicControlPlane.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &uiDeployRelicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: selector,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: selector,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            "apusic-consul",
						Image:           defaultConsulImage,
						ImagePullPolicy: defaultConsulImagePolicy,
						Args:            args,
						Ports: []corev1.ContainerPort{{
							ContainerPort: 8500,
							Name:          "http",
							HostPort:      8500,
						}, {
							ContainerPort: 8600,
							Name:          "dns",
							HostPort:      8600,
						}, {
							ContainerPort: 8300,
							Name:          "rpc",
							HostPort:      8300,
						}, {
							ContainerPort: 8302,
							Name:          "wan",
							HostPort:      8302,
						}, {
							ContainerPort: 8301,
							Name:          "lan",
							HostPort:      8301,
						},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "consul-data",
								MountPath: "/data",
							},
						},
					}},
					Volumes: []corev1.Volume{
						{
							Name: "consul-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: pvcName,
								},
							},
						},
					},
				},
			},
		},
	}
	return
}

func (acp *Acp) StatusfulSet(svcName string) (desired *appsv1.StatefulSet) {
	label := labelsForApusicControlPlane(acp.ApusicControlPlane.Name, "statusfulset")
	replicas := &acp.ApusicControlPlane.Spec.Replicas
	statefulName := acp.ApusicControlPlane.Name + "-stateful"
	retryJoins := make([]string, int(*replicas))
	for i := int32(0); i < *replicas; i++ {
		retryJoins[i] = fmt.Sprintf("-retry-join=%s-%d.%s.$(NAMESPACE).svc.cluster.local", statefulName, i, svcName)
	}
	args := []string{"agent",
		"-server",
		"-advertise=$(POD_IP)",
		"-bind=0.0.0.0",
		"-datacenter=dc1",
		"-data-dir=/data",
		"-disable-host-node-id=true",
		"-domain=cluster.local",
		"-client=0.0.0.0",
		fmt.Sprintf("-bootstrap-expect=%d", *replicas),
		"-ui",
	}
	args = append(args, retryJoins...)
	desired = &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      statefulName,
			Namespace: acp.ApusicControlPlane.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: label,
			},
			ServiceName: svcName,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: label,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            "apusic-consul",
						Image:           defaultConsulImage,
						ImagePullPolicy: defaultConsulImagePolicy,
						Env: []corev1.EnvVar{
							{
								Name: "POD_IP",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "status.podIP",
									},
								},
							},
							{
								Name: "NAMESPACE",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "metadata.namespace",
									},
								},
							},
						},
						Args: args,
						Ports: []corev1.ContainerPort{{
							ContainerPort: 8500,
							Name:          "http",
							HostPort:      8500,
						}, {
							ContainerPort: 8600,
							Name:          "dns",
							HostPort:      8600,
						}, {
							ContainerPort: 8300,
							Name:          "rpc",
							HostPort:      8300,
						}, {
							ContainerPort: 8302,
							Name:          "wan",
							HostPort:      8302,
						}, {
							ContainerPort: 8301,
							Name:          "lan",
							HostPort:      8301,
						},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "consul-data",
								MountPath: "/data",
							},
						},
					}},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "consul-data",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						StorageClassName: acp.ApusicControlPlane.Spec.StorageClassName,
						Resources:        acp.ApusicControlPlane.Spec.Resources,
					},
				},
			},
		},
	}
	return
}

func (acp *Acp) HeadlessService() *corev1.Service {
	svcName := acp.ApusicControlPlane.Name + "-svc"
	label := labelsForApusicControlPlane(acp.ApusicControlPlane.Name, "service")
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: acp.ApusicControlPlane.Namespace,
			Labels:    label,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Ports: []corev1.ServicePort{
				{
					Port: 8500,
					Name: "http",
				},
			},
			Selector: label,
		},
	}
	return svc
}

func labelsForApusicControlPlane(name, instance string) map[string]string {
	return map[string]string{"app": "apusiccontrolplane", "apusicas_cr": name, "acp_instance": instance}
}

func pvcNameGen(crName, instanceName string) string {
	return fmt.Sprintf("pvc-%s-%s", crName, instanceName)
}
