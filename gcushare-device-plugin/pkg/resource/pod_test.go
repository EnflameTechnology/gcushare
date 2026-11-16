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

	"gcushare-device-plugin/pkg/config"
	"gcushare-device-plugin/pkg/consts"
	"gcushare-device-plugin/pkg/device"
	"gcushare-device-plugin/pkg/kube"
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
		resourceName := config.ResourceName(false)
		clusterResource, err := NewClusterResource(config, resourceName)
		So(err, ShouldEqual, nil)
		podIns := NewPodResource(clusterResource, map[string]device.DeviceInfo{})
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
								consts.SharedResourceName: *resource.NewQuantity(8, ""),
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
								consts.SharedResourceName: *resource.NewQuantity(8, ""),
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
		resourceName := config.ResourceName(false)
		clusterResource, err := NewClusterResource(config, resourceName)
		So(err, ShouldEqual, nil)
		podIns := NewPodResource(clusterResource, map[string]device.DeviceInfo{})
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
								consts.SharedResourceName: *resource.NewQuantity(8, ""),
							},
						}},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			}
			ok, _, err := podIns.PodExistContainerCanBind(1, &pod1)
			So(ok, ShouldBeFalse)
			So(err, ShouldNotEqual, nil)
		})

		Convey("2. test func PodExistContainerCanBind no container match request", func() {
			pod1 := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-1",
					Namespace: "kube-system",
					UID:       "1993",
					Annotations: map[string]string{
						consts.PodHasAssignedGCU:     "false",
						consts.PodAssignedContainers: `{"container1": {"allocated": true}}`,
					},
				},
				Spec: corev1.PodSpec{
					NodeName: "node-1",
					Containers: []corev1.Container{
						{Name: "container1", Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{consts.SharedResourceName: *resource.NewQuantity(8, "")},
						}},
						{Name: "container2", Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{consts.SharedResourceName: *resource.NewQuantity(5, "")},
						}},
					},
				},
				Status: corev1.PodStatus{Phase: corev1.PodPending},
			}

			ok, _, err := podIns.PodExistContainerCanBind(6, &pod1)
			So(err, ShouldEqual, nil)
			So(ok, ShouldBeFalse)
		})
	})
}

func TestGetGCUMemoryFromPodResource(t *testing.T) {
	Convey("test func resource/pod.go GetGCUMemoryFromPodResource", t, func() {
		patcheKube := ApplyFunc(kube.GetKubeClient, func() (*kubernetes.Clientset, error) {
			return &kubernetes.Clientset{}, nil
		})
		defer patcheKube.Reset()
		patcheEnv := ApplyFunc(os.Getenv, func(_ string) string { return "node-1" })
		defer patcheEnv.Reset()
		config := &config.Config{RegisterResource: []string{"enflame.com/gcu"}}
		resourceName := config.ResourceName(false)
		clusterResource, err := NewClusterResource(config, resourceName)
		So(err, ShouldEqual, nil)
		podIns := NewPodResource(clusterResource, map[string]device.DeviceInfo{})
		pod := corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{consts.SharedResourceName: *resource.NewQuantity(4, "")},
					}},
				},
			},
		}
		total := podIns.GetGCUMemoryFromPodResource(&pod)
		So(total, ShouldEqual, 4)
	})
}
