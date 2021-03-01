package resources

import (
	"fmt"
	v1 "github.com/jstage/apusic-aas-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type ResTypeFunc func(ctrl string) string

type ResName int

const (
	SVCNAME ResName = iota
	STATEFULNAME
	PVCNAME
	DEPLOYNAME
	HEADLESS

	defaultConsulImage = "consul:v1.8.5"

	defaultConsulImagePolicy = corev1.PullIfNotPresent
)

type Acp struct {
	ApusicControlPlane *v1.ApusicControlPlane
	ResTypeFuncs       []ResTypeFunc
}

func NewAcp(ctrl *v1.ApusicControlPlane) *Acp {
	temp := &Acp{
		ApusicControlPlane: ctrl,
	}
	temp.inits()
	return temp
}

func (acp *Acp) inits() {
	acp.ResTypeFuncs = make([]ResTypeFunc, 5)
	acp.ResTypeFuncs[SVCNAME] = func(ctrl string) string {
		return ctrl + "-" + "svc"
	}
	acp.ResTypeFuncs[STATEFULNAME] = func(ctrl string) string {
		return ctrl + "-" + "stateful"
	}
	acp.ResTypeFuncs[PVCNAME] = func(ctrl string) string {
		return ctrl + "-" + "pvc"
	}
	acp.ResTypeFuncs[DEPLOYNAME] = func(ctrl string) string {
		return ctrl + "-" + "deploy"
	}
	acp.ResTypeFuncs[HEADLESS] = func(ctrl string) string {
		return ctrl + "-" + "headless"
	}
}

func (acp *Acp) UIPvc(pvcName string) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: acp.ApusicControlPlane.Namespace,
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
	svcNameFunc := acp.ResTypeFuncs[SVCNAME]
	svcName := svcNameFunc(acp.ApusicControlPlane.Name)
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
	pvcNameFunc := acp.ResTypeFuncs[PVCNAME]
	pvcName = pvcNameFunc(acp.ApusicControlPlane.Name)
	selector = labelsForApusicControlPlane(acp.ApusicControlPlane.Name, "deploy")
	deployNameFunc := acp.ResTypeFuncs[DEPLOYNAME]
	deployName := deployNameFunc(acp.ApusicControlPlane.Name)
	replicas := &acp.ApusicControlPlane.Spec.Replicas
	statefulNameFunc := acp.ResTypeFuncs[STATEFULNAME]
	statefulName := statefulNameFunc(acp.ApusicControlPlane.Name)
	retryJoins := make([]string, int(*replicas))
	for i := int32(0); i < *replicas; i++ {
		retryJoins[i] = fmt.Sprintf("-retry-join=%s-%d.%s.%s.svc.cluster.local", statefulName, i, acp.ApusicControlPlane.Namespace, svcName)
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
	statefulNameFunc := acp.ResTypeFuncs[STATEFULNAME]
	statefulName := statefulNameFunc(acp.ApusicControlPlane.Name)
	retryJoins := make([]string, int(*replicas))
	for i := int32(0); i < *replicas; i++ {
		retryJoins[i] = fmt.Sprintf("-retry-join=%s-%d.%s.%s.svc.cluster.local", statefulName, i, svcName, acp.ApusicControlPlane.Namespace)
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
	svcNameFunc := acp.ResTypeFuncs[HEADLESS]
	label := labelsForApusicControlPlane(acp.ApusicControlPlane.Name, "service")
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcNameFunc(acp.ApusicControlPlane.Name),
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
