// Copyright (c) 2025, ENFLAME INC.  All rights reserved.

package resource

import (
	"os"
	"reflect"
	"testing"

	. "github.com/agiledragon/gomonkey"
	. "github.com/smartystreets/goconvey/convey"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	coretypev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"gcushare-device-plugin/pkg/config"
	"gcushare-device-plugin/pkg/consts"
	"gcushare-device-plugin/pkg/kube"
	"gcushare-device-plugin/pkg/tests"
)

func TestGetCandidatePods(t *testing.T) {
	Convey("test func resource/pod.go", t, func() {
		patcheKube := ApplyFunc(kube.GetKubeClient, func() (*kubernetes.Clientset, error) {
			return &kubernetes.Clientset{}, nil
		})
		defer patcheKube.Reset()
		patcheEnv := ApplyFunc(os.Getenv, func(_ string) string {
			return "node-1"
		})
		defer patcheEnv.Reset()
		config := &config.Config{RegisterResource: []string{"enflame.com/gcu"}}
		clusterResource, err := NewClusterResource(config)
		So(err, ShouldEqual, nil)
		podIns := NewPodResource(clusterResource)
		Convey("1. test func GetCandidatePods query from kubelet success", func() {
			kubeClient := &kube.KubeletClient{}
			queryFromKubelet := true
			pod1 := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-1",
					Namespace: "kube-system",
					UID:       "1993",
					Annotations: map[string]string{
						consts.PodAssignedGCUTime: "1664506283",
						consts.PodHasAssignedGCU:  "false",
					},
				},
				Spec: corev1.PodSpec{
					NodeName: "node-1",
					Containers: []corev1.Container{
						{Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								consts.ResourceName: *resource.NewQuantity(8, ""),
							},
						}},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			}
			pod2 := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-2",
					Namespace: "kube-system",
					UID:       "1994",
					Annotations: map[string]string{
						consts.PodAssignedGCUTime: "1664506290",
						consts.PodHasAssignedGCU:  "false",
					},
				},
				Spec: corev1.PodSpec{
					NodeName: "node-1",
					Containers: []corev1.Container{
						{Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								consts.ResourceName: *resource.NewQuantity(8, ""),
							},
						}},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			}
			patcheGetPodFromKubelet := ApplyMethod(reflect.TypeOf(kubeClient), "GetNodePodsList",
				func(_ *kube.KubeletClient) (*corev1.PodList, error) {
					return &corev1.PodList{Items: []corev1.Pod{pod1, pod2}}, nil
				})
			defer patcheGetPodFromKubelet.Reset()

			rv, err := podIns.GetCandidatePods(queryFromKubelet, kubeClient)
			So(err, ShouldEqual, nil)
			So(len(rv), ShouldEqual, 2)
		})
	})
}

func TestPodExistContainerCanBind(t *testing.T) {
	Convey("test func resource/pod.go", t, func() {
		patcheKube := ApplyFunc(kube.GetKubeClient, func() (*kubernetes.Clientset, error) {
			return &kubernetes.Clientset{}, nil
		})
		defer patcheKube.Reset()
		patcheEnv := ApplyFunc(os.Getenv, func(_ string) string {
			return "node-1"
		})
		defer patcheEnv.Reset()

		config := &config.Config{RegisterResource: []string{"enflame.com/gcu"}}
		clusterResource, err := NewClusterResource(config)
		So(err, ShouldEqual, nil)
		podIns := NewPodResource(clusterResource)
		Convey("1. test func PodExistContainerCanBind unmarshal assigned containers failed", func() {
			pod1 := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-1",
					Namespace: "kube-system",
					UID:       "1993",
					Annotations: map[string]string{
						consts.PodHasAssignedGCU:     "false",
						consts.PodAssignedContainers: "container1",
					},
				},
				Spec: corev1.PodSpec{
					NodeName: "node-1",
					Containers: []corev1.Container{
						{Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								consts.ResourceName: *resource.NewQuantity(8, ""),
							},
						}},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			}
			containerName, err := podIns.PodExistContainerCanBind(1, &pod1)
			So(containerName, ShouldEqual, "")
			So(err, ShouldNotEqual, nil)
		})

		Convey("2. test func PodExistContainerCanBind true", func() {
			pod1 := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-1",
					Namespace: "kube-system",
					UID:       "1993",
					Annotations: map[string]string{
						consts.PodHasAssignedGCU:     "false",
						consts.PodAssignedContainers: `{"container1": "true"}`,
					},
				},
				Spec: corev1.PodSpec{
					NodeName: "node-1",
					Containers: []corev1.Container{
						{
							Name: "container1",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{consts.ResourceName: *resource.NewQuantity(8, "")},
							},
						},
						{
							Name: "container2",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{consts.ResourceName: *resource.NewQuantity(5, "")},
							},
						},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			}
			containerName, err := podIns.PodExistContainerCanBind(5, &pod1)
			So(err, ShouldEqual, nil)
			So(containerName, ShouldEqual, "container2")
		})

		Convey("3. test func PodExistContainerCanBind false", func() {
			pod1 := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-1",
					Namespace: "kube-system",
					UID:       "1993",
					Annotations: map[string]string{
						consts.PodHasAssignedGCU:     "false",
						consts.PodAssignedContainers: `{"container1": "true"}`,
					},
				},
				Spec: corev1.PodSpec{
					NodeName: "node-1",
					Containers: []corev1.Container{
						{
							Name: "container1",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{consts.ResourceName: *resource.NewQuantity(8, "")},
							},
						},
						{
							Name: "container2",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{consts.ResourceName: *resource.NewQuantity(5, "")},
							},
						},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			}
			containerName, err := podIns.PodExistContainerCanBind(6, &pod1)
			So(err, ShouldEqual, nil)
			So(containerName, ShouldEqual, "")
		})
	})
}

func TestCheckPodAllContainersBind(t *testing.T) {
	Convey("test func resource/pod.go", t, func() {
		clientSet := &kubernetes.Clientset{}
		patcheKube := ApplyFunc(kube.GetKubeClient, func() (*kubernetes.Clientset, error) {
			return clientSet, nil
		})
		defer patcheKube.Reset()

		patcheEnv := ApplyFunc(os.Getenv, func(_ string) string {
			return "node-1"
		})
		defer patcheEnv.Reset()

		config := &config.Config{RegisterResource: []string{"enflame.com/gcu"}}
		clusterResource, err := NewClusterResource(config)
		So(err, ShouldEqual, nil)
		podIns := NewPodResource(clusterResource)

		coreviClient := &coretypev1.CoreV1Client{}
		patcheCoreV1 := ApplyMethod(reflect.TypeOf(clientSet), "CoreV1",
			func(_ *kubernetes.Clientset) coretypev1.CoreV1Interface {
				return coreviClient
			})
		defer patcheCoreV1.Reset()

		patcheCoreV1Pods := ApplyMethod(reflect.TypeOf(coreviClient), "Pods",
			func(_ *coretypev1.CoreV1Client, _ string) coretypev1.PodInterface {
				return &tests.MockPods{}
			})
		defer patcheCoreV1Pods.Reset()
		Convey("1. test func PodExistContainerCanBind unmarshal assigned containers failed", func() {
			pod1 := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-1",
					Namespace: "kube-system",
					UID:       "1993",
					Annotations: map[string]string{
						consts.PodHasAssignedGCU:     "false",
						consts.PodAssignedContainers: `{"container1": "true"}`,
					},
				},
				Spec: corev1.PodSpec{
					NodeName: "node-1",
					Containers: []corev1.Container{
						{
							Name: "container1",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{consts.ResourceName: *resource.NewQuantity(8, "")},
							},
						},
						{
							Name: "container2",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{consts.ResourceName: *resource.NewQuantity(5, "")},
							},
						},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			}
			err := podIns.CheckPodAllContainersBind("container2", &pod1)
			So(err, ShouldEqual, nil)
		})
	})
}
