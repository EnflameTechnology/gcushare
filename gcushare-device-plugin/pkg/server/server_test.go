// Copyright (c) 2025, ENFLAME INC.  All rights reserved.

package server

import (
	"context"
	"fmt"
	"go-efids/efids"
	"io/fs"
	"net"
	"os"
	"reflect"
	"testing"
	"time"

	. "github.com/agiledragon/gomonkey"
	. "github.com/smartystreets/goconvey/convey"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	k8sres "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	coretypev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"

	"gcushare-device-plugin/pkg/config"
	"gcushare-device-plugin/pkg/consts"
	"gcushare-device-plugin/pkg/device"
	"gcushare-device-plugin/pkg/kube"
	"gcushare-device-plugin/pkg/resource"
	"gcushare-device-plugin/pkg/tests"
)

func TestNewGCUDevicePluginServe(t *testing.T) {
	Convey("test func server/server.go", t, func() {
		clientset := &kubernetes.Clientset{}
		patcheCoreV1 := ApplyMethod(reflect.TypeOf(clientset), "CoreV1",
			func(_ *kubernetes.Clientset) coretypev1.CoreV1Interface {
				return &coretypev1.CoreV1Client{}
			})
		defer patcheCoreV1.Reset()

		patchePods := ApplyMethod(reflect.TypeOf(new(coretypev1.CoreV1Client)), "Pods",
			func(_ *coretypev1.CoreV1Client, _ string) coretypev1.PodInterface {
				return &tests.MockPods{}
			})
		defer patchePods.Reset()

		patcheNodes := ApplyMethod(reflect.TypeOf(new(coretypev1.CoreV1Client)), "Nodes",
			func(_ *coretypev1.CoreV1Client) coretypev1.NodeInterface {
				return &tests.MockNodes{}
			})
		defer patcheNodes.Reset()

		patcheInformerStart := ApplyMethod(reflect.TypeOf(new(resource.PodInformer)), "Start",
			func(_ *resource.PodInformer, _ context.Context) error {
				return nil
			})
		defer patcheInformerStart.Reset()

		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "node-1",
				Annotations: map[string]string{},
				Labels:      map[string]string{},
			},
			Status: corev1.NodeStatus{
				Capacity:    corev1.ResourceList{consts.CountName: *k8sres.NewQuantity(0, "")},
				Allocatable: corev1.ResourceList{consts.CountName: *k8sres.NewQuantity(0, "")},
			},
		}
		patcheGetDeviceFullInfos := ApplyFunc(efids.GetDeviceFullInfos, func() ([]efids.DeviceFullInfo, error) {
			return []efids.DeviceFullInfo{
				{
					DeviceInfo: efids.DeviceInfo{
						VendorID: "0x1e36",
						DeviceID: "0xc033",
						BusID:    "0000:01:00.0",
						UUID:     "T6R231060602",
						Major:    238,
						Minor:    0,
					},
					GCUDevice: efids.GCUDevice{
						Vendor:       "Enflame",
						AsicName:     "Scorpio X2",
						ProductModel: "S60G",
						SipNum:       24,
						L3Size:       48,
						L2Size:       64,
					},
				},
				{
					DeviceInfo: efids.DeviceInfo{
						VendorID: "0x1e36",
						DeviceID: "0xc033",
						BusID:    "0000:09:00.0",
						UUID:     "T6R231010702",
						Major:    238,
						Minor:    1,
					},
					GCUDevice: efids.GCUDevice{
						Vendor:       "Enflame",
						AsicName:     "Scorpio X2",
						ProductModel: "S60G",
						SipNum:       24,
						L3Size:       48,
						L2Size:       64,
					},
				},
			}, nil
		})
		defer patcheGetDeviceFullInfos.Reset()
		plugin := &GCUDevicePluginServe{}
		// Convey("1. test func NewGCUDevicePluginServe success", func() {
		patcheKube := ApplyFunc(kube.GetKubeClient, func() (*kubernetes.Clientset, error) {
			return clientset, nil
		})
		defer patcheKube.Reset()
		os.Setenv("NODE_NAME", "node-1")
		patcheOSStat := ApplyFunc(os.Stat, func(_ string) (fs.FileInfo, error) {
			return nil, nil
		})
		defer patcheOSStat.Reset()

		devices, err := device.NewDevice(4, &config.Config{RegisterResource: []string{"enflame.com/gcu"}}, true)
		So(err, ShouldEqual, nil)

		node1 := node.DeepCopy()
		node1.Status.Capacity[consts.CountName] = *k8sres.NewQuantity(128, "")
		node1.Status.Allocatable[consts.CountName] = *k8sres.NewQuantity(128, "")
		patcheNodesPatch := ApplyMethod(reflect.TypeOf(new(tests.MockNodes)), "PatchStatus",
			func(_ *tests.MockNodes, _ context.Context, _ string, _ []byte) (*corev1.Node, error) {
				return node1, nil
			})
		defer patcheNodesPatch.Reset()

		node2 := node1.DeepCopy()
		node2.Annotations[consts.GCUSharedCapacity] = `{"0": 4, "1": 4}`
		outputs := []OutputCell{
			{Values: Params{node, nil}},
			{Values: Params{node1, nil}},
			{Values: Params{node2, nil}},
		}
		patcheNodeGet := ApplyMethodSeq(reflect.TypeOf(new(tests.MockNodes)), "Get", outputs)
		defer patcheNodeGet.Reset()

		plugin, err = NewGCUDevicePluginServe(true, false, &kube.KubeletClient{}, devices)
		So(err, ShouldEqual, nil)
		So(len(plugin.fakeDevices), ShouldEqual, 8)
		So(plugin.resourceIsolation, ShouldEqual, true)
		// })

		Convey("2. test func cleanup success", func() {
			patcheRemove := ApplyFunc(os.Remove, func(_ string) error {
				return os.ErrNotExist
			})
			defer patcheRemove.Reset()
			err := plugin.cleanup()
			So(err, ShouldEqual, nil)
		})

		Convey("3. test func serve success", func() {
			patcheRemove := ApplyFunc(os.Remove, func(_ string) error {
				return os.ErrNotExist
			})
			defer patcheRemove.Reset()

			patcheListen := ApplyFunc(net.Listen, func(_, _ string) (net.Listener, error) {
				return nil, nil
			})
			defer patcheListen.Reset()

			patcheNewServer := ApplyFunc(grpc.NewServer, func(_ ...grpc.ServerOption) *grpc.Server {
				return &grpc.Server{}
			})
			defer patcheNewServer.Reset()

			patcheRegisterDevicePluginServer := ApplyFunc(pluginapi.RegisterDevicePluginServer,
				func(_ *grpc.Server, _ pluginapi.DevicePluginServer) {
				})
			defer patcheRegisterDevicePluginServer.Reset()

			patcheServe := ApplyMethod(reflect.TypeOf(new(grpc.Server)), "Serve", func(_ *grpc.Server, _ net.Listener) error {
				return nil
			})
			defer patcheServe.Reset()

			patcheDial := ApplyFunc(grpc.Dial, func(_ string, _ ...grpc.DialOption) (*grpc.ClientConn, error) {
				return &grpc.ClientConn{}, nil
			})
			defer patcheDial.Reset()

			patcheClose := ApplyMethod(reflect.TypeOf(new(grpc.ClientConn)), "Close", func(_ *grpc.ClientConn) error {
				return nil
			})
			defer patcheClose.Reset()

			patcheWatchHealth := ApplyFunc(device.WatchHealth, func(_ <-chan struct{}, _ []*pluginapi.Device,
				_ chan<- []*pluginapi.Device, _ map[string]device.DeviceInfo, _ chan map[string]struct{}) {
			})
			defer patcheWatchHealth.Reset()

			patcheNewRegistrationClient := ApplyFunc(pluginapi.NewRegistrationClient, func(_ *grpc.ClientConn) pluginapi.RegistrationClient {
				return &tests.MockRegistrationClient{}
			})
			defer patcheNewRegistrationClient.Reset()

			err := plugin.Serve()
			time.Sleep(1 * time.Second)
			So(err, ShouldEqual, nil)
		})

		Convey("4. test Allocate one card success", func() {
			reqs := &pluginapi.AllocateRequest{
				ContainerRequests: []*pluginapi.ContainerAllocateRequest{
					{
						DevicesIDs: []string{"0-1", "2-6", "2-7", "7-2"},
					},
				},
			}

			// pod1 has been assigned, skip it
			pod1 := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-1",
					Namespace: "kube-system",
					UID:       "pod-1-uid",
					Annotations: map[string]string{
						consts.PodAssignedGCUTime:    "1664506283",
						consts.PodHasAssignedGCU:     "false",
						consts.PodAssignedContainers: `{"pod-1-c1": "true", "pod-1-c2": "true"}`,
					},
				},
				Spec: corev1.PodSpec{
					NodeName: "node-1",
					Containers: []corev1.Container{
						{
							Name: "pod-1-c1",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									consts.ResourceName: *k8sres.NewQuantity(4, ""),
								},
							},
						},
						{
							Name: "pod-1-c2",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									consts.ResourceName: *k8sres.NewQuantity(4, ""),
								},
							},
						},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			}

			// pod2's node name is not node-1, skip it
			pod2 := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-2",
					Namespace: "kube-system",
					UID:       "pod-2-uid",
					Annotations: map[string]string{
						consts.PodAssignedGCUTime: "1664506290",
						consts.PodHasAssignedGCU:  "false",
					},
				},
				Spec: corev1.PodSpec{
					NodeName: "node-2",
					Containers: []corev1.Container{
						{
							Name: "pod-2-c1",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									consts.ResourceName: *k8sres.NewQuantity(4, ""),
								},
							},
						},
						{
							Name: "pod-2-c2",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									consts.ResourceName: *k8sres.NewQuantity(4, ""),
								},
							},
						},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			}

			// pod3 shouled be seleted
			pod3 := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-3",
					Namespace: "kube-system",
					UID:       "pod-3-uid",
					Annotations: map[string]string{
						consts.PodAssignedGCUTime:    "1664506296",
						consts.PodHasAssignedGCU:     "false",
						consts.PodAssignedGCUMinor:   "1",
						consts.PodAssignedContainers: `{"pod-3-c1": "true"}`,
					},
				},
				Spec: corev1.PodSpec{
					NodeName: "node-1",
					Containers: []corev1.Container{
						{
							Name: "pod-3-c1",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									consts.ResourceName: *k8sres.NewQuantity(4, ""),
								},
							},
						},
						{
							Name: "pod-3-c2",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									consts.ResourceName: *k8sres.NewQuantity(2, ""),
								},
							},
						},
						{
							Name: "pod-3-c3",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									consts.ResourceName: *k8sres.NewQuantity(4, ""),
								},
							},
						},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			}

			// pod4 has no container request 4 gcu memory
			pod4 := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-4",
					Namespace: "kube-system",
					UID:       "pod-4-uid",
					Annotations: map[string]string{
						consts.PodAssignedGCUTime:  "1664506100",
						consts.PodHasAssignedGCU:   "false",
						consts.PodAssignedGCUMinor: "3",
					},
				},
				Spec: corev1.PodSpec{
					NodeName: "node-1",
					Containers: []corev1.Container{
						{
							Name: "pod-4-c1",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									consts.ResourceName: *k8sres.NewQuantity(3, ""),
								},
							},
						},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			}

			outputs := []OutputCell{
				{Values: Params{nil, fmt.Errorf("apiserver list pods failed")}},
				{Values: Params{&corev1.PodList{Items: []corev1.Pod{pod1, pod2, pod3, pod4}}, nil}},
			}
			patchePodsList := ApplyMethodSeq(reflect.TypeOf(new(tests.MockPods)), "List", outputs)
			defer patchePodsList.Reset()

			res, err := plugin.Allocate(context.TODO(), reqs)
			So(err, ShouldEqual, nil)
			So(len(res.ContainerResponses), ShouldEqual, 1)
			So(res.ContainerResponses[0].Envs[consts.ENFLAME_VISIBLE_DEVICES], ShouldEqual, "1")
			So(res.ContainerResponses[0].Envs[consts.ENFLAME_CONTAINER_SUB_CARD], ShouldEqual, "false")
			So(res.ContainerResponses[0].Envs[consts.ENFLAME_CONTAINER_USABLE_PROCESSOR], ShouldEqual, "24")
			So(res.ContainerResponses[0].Envs[consts.ENFLAME_CONTAINER_USABLE_GLOBAL_MEM], ShouldEqual, "48")
		})

		Convey("5. test Allocate 1/2 card success", func() {
			reqs := &pluginapi.AllocateRequest{
				ContainerRequests: []*pluginapi.ContainerAllocateRequest{
					{
						DevicesIDs: []string{"0-1", "2-6"},
					},
				},
			}

			pod := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-3",
					Namespace: "kube-system",
					UID:       "pod-3-uid",
					Annotations: map[string]string{
						consts.PodAssignedGCUTime:  "1664506296",
						consts.PodHasAssignedGCU:   "false",
						consts.PodAssignedGCUMinor: "1",
					},
				},
				Spec: corev1.PodSpec{
					NodeName: "node-1",
					Containers: []corev1.Container{
						{
							Name: "pod-3-c1",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									consts.ResourceName: *k8sres.NewQuantity(2, ""),
								},
							},
						},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			}

			outputs := []OutputCell{
				{Values: Params{nil, fmt.Errorf("apiserver list pods failed")}},
				{Values: Params{&corev1.PodList{Items: []corev1.Pod{pod}}, nil}},
			}
			patchePodsList := ApplyMethodSeq(reflect.TypeOf(new(tests.MockPods)), "List", outputs)
			defer patchePodsList.Reset()

			res, err := plugin.Allocate(context.TODO(), reqs)
			So(err, ShouldEqual, nil)
			So(len(res.ContainerResponses), ShouldEqual, 1)
			So(res.ContainerResponses[0].Envs[consts.ENFLAME_VISIBLE_DEVICES], ShouldEqual, "1")
			So(res.ContainerResponses[0].Envs[consts.ENFLAME_CONTAINER_SUB_CARD], ShouldEqual, "true")
			So(res.ContainerResponses[0].Envs[consts.ENFLAME_CONTAINER_USABLE_PROCESSOR], ShouldEqual, "12")
			So(res.ContainerResponses[0].Envs[consts.ENFLAME_CONTAINER_USABLE_GLOBAL_MEM], ShouldEqual, "24")
		})
	})
}
