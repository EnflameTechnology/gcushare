// Copyright (c) 2025, ENFLAME INC.  All rights reserved.
package routers

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	. "github.com/agiledragon/gomonkey"
	. "github.com/smartystreets/goconvey/convey"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"gcushare-scheduler-plugin/pkg/config"
	"gcushare-scheduler-plugin/pkg/resources"
	"gcushare-scheduler-plugin/tests"
)

func TestInspect(t *testing.T) {
	clientset := &kubernetes.Clientset{}
	config := &config.Config{RegisterResource: []string{"enflame.com/gcu"}}
	baseResource := resources.NewBaseResource(config, clientset)
	inspect := &Inspect{
		clientset:    clientset,
		podResource:  resources.NewPodResource(baseResource),
		nodeResource: resources.NewNodeResource(baseResource),
	}
	patcheCoreV1 := ApplyMethod(reflect.TypeOf(clientset), "CoreV1", func(_ *kubernetes.Clientset) corev1.CoreV1Interface {
		return &corev1.CoreV1Client{}
	})
	defer patcheCoreV1.Reset()

	patchePods := ApplyMethod(reflect.TypeOf(new(corev1.CoreV1Client)), "Pods", func(_ *corev1.CoreV1Client, _ string) corev1.PodInterface {
		return &tests.MockPods{}
	})
	defer patchePods.Reset()
	patcheNodes := ApplyMethod(reflect.TypeOf(new(corev1.CoreV1Client)), "Nodes", func(_ *corev1.CoreV1Client) corev1.NodeInterface {
		return &tests.MockNodes{}
	})
	defer patcheNodes.Reset()
	Convey("1. test inspect with node name", t, func() {
		Convey("1. test inspect single node success", func() {
			Convey("1. test inspect single node no pod use gcu", func() {
				node1 := tests.NewNode("node1", true, `{"0": 32, "1": 32}`, 64)
				patcheNodeGet := ApplyMethod(reflect.TypeOf(new(tests.MockNodes)), "Get", func(_ *tests.MockNodes,
					_ context.Context, _ string, _ metav1.GetOptions) (*v1.Node, error) {
					return node1, nil
				})
				defer patcheNodeGet.Reset()
				patchePodList := ApplyMethod(reflect.TypeOf(new(tests.MockPods)), "List", func(_ *tests.MockPods,
					_ context.Context, _ metav1.ListOptions) (*v1.PodList, error) {
					return &v1.PodList{}, nil
				})
				defer patchePodList.Reset()
				result, ok := inspect.inspect("node1").(NodeInfo)
				So(ok, ShouldEqual, true)
				So(result.Name, ShouldEqual, "node1")
				So(result.TotalGCU, ShouldEqual, 64)
				So(*result.UsedGCU, ShouldEqual, 0)
				So(*result.AvailableGCU, ShouldEqual, 64)
				So(result.Error, ShouldEqual, "")
				So(len(result.Warn), ShouldEqual, 0)
				So(len(result.Devices), ShouldEqual, 2)
				So(result.Devices["0"].TotalGCU, ShouldEqual, 32)
				So(*result.Devices["0"].UsedGCU, ShouldEqual, 0)
				So(*result.Devices["0"].AvailableGCU, ShouldEqual, 32)
				So(len(result.Devices["0"].Pods), ShouldEqual, 0)
			})
			Convey("2. test inspect single node has pod use gcu", func() {
				node1 := tests.NewNode("node1", true, `{"0": 32, "1": 32}`, 64)
				patcheNodeGet := ApplyMethod(reflect.TypeOf(new(tests.MockNodes)), "Get", func(_ *tests.MockNodes,
					_ context.Context, _ string, _ metav1.GetOptions) (*v1.Node, error) {
					return node1, nil
				})
				defer patcheNodeGet.Reset()
				pod1 := tests.NewPod("pod1", "pod1-uid", "", "")
				pod2 := tests.NewPod("pod2", "pod2-uid", "", "", 16)
				pod3 := tests.NewPod("pod3", "pod3-uid", "100", "16", 16)
				pod4 := tests.NewPod("pod4", "pod4-uid", "0", "16", 16)
				pod5 := tests.NewPod("pod5", "pod5-uid", "1", "24", 24)
				patchePodList := ApplyMethod(reflect.TypeOf(new(tests.MockPods)), "List", func(_ *tests.MockPods,
					_ context.Context, _ metav1.ListOptions) (*v1.PodList, error) {
					return &v1.PodList{Items: []v1.Pod{*pod1, *pod2, *pod3, *pod4, *pod5}}, nil
				})
				defer patchePodList.Reset()
				result, ok := inspect.inspect("node1").(NodeInfo)
				So(ok, ShouldEqual, true)
				So(result.Name, ShouldEqual, "node1")
				So(result.TotalGCU, ShouldEqual, 64)
				So(*result.UsedGCU, ShouldEqual, 40)
				So(*result.AvailableGCU, ShouldEqual, 24)
				So(result.Error, ShouldEqual, "")
				So(len(result.Warn), ShouldEqual, 1)
				So(strings.Contains(result.Warn[0], "pod3-uid"), ShouldEqual, true)

				So(len(result.Devices), ShouldEqual, 2)
				So(result.Devices["0"].TotalGCU, ShouldEqual, 32)
				So(*result.Devices["0"].UsedGCU, ShouldEqual, 16)
				So(*result.Devices["0"].AvailableGCU, ShouldEqual, 16)
				So(len(result.Devices["0"].Pods), ShouldEqual, 1)
				So(result.Devices["0"].Pods[0].Name, ShouldEqual, "pod4")
				So(string(result.Devices["0"].Pods[0].UID), ShouldEqual, "pod4-uid")
				So(result.Devices["0"].Pods[0].UsedGCU, ShouldEqual, 16)

				So(result.Devices["1"].TotalGCU, ShouldEqual, 32)
				So(*result.Devices["1"].UsedGCU, ShouldEqual, 24)
				So(*result.Devices["1"].AvailableGCU, ShouldEqual, 8)
				So(len(result.Devices["1"].Pods), ShouldEqual, 1)
				So(result.Devices["1"].Pods[0].Name, ShouldEqual, "pod5")
				So(string(result.Devices["1"].Pods[0].UID), ShouldEqual, "pod5-uid")
				So(result.Devices["1"].Pods[0].UsedGCU, ShouldEqual, 24)
			})
		})
		Convey("2. test inspect single node failed", func() {
			Convey("1. test inspect get node failed", func() {
				err := fmt.Errorf("get cluster node failed")
				patcheNodeGet := ApplyMethod(reflect.TypeOf(new(tests.MockNodes)), "Get", func(_ *tests.MockNodes,
					_ context.Context, _ string, _ metav1.GetOptions) (*v1.Node, error) {
					return nil, err
				})
				defer patcheNodeGet.Reset()
				result, ok := inspect.inspect("node1").(NodeInfo)
				So(ok, ShouldEqual, true)
				So(result.Name, ShouldEqual, "node1")
				So(result.Error, ShouldEqual, err.Error())
			})
			Convey("2. test inspect node is not gcushare node", func() {
				node1 := tests.NewNode("node1", false, "", 0)
				patcheNodeGet := ApplyMethod(reflect.TypeOf(new(tests.MockNodes)), "Get", func(_ *tests.MockNodes,
					_ context.Context, _ string, _ metav1.GetOptions) (*v1.Node, error) {
					return node1, nil
				})
				defer patcheNodeGet.Reset()
				result, ok := inspect.inspect("node1").(NodeInfo)
				So(ok, ShouldEqual, true)
				So(result.Name, ShouldEqual, "node1")
				So(result.Error, ShouldEqual, "node: node1 is not the gcushare node")
			})
			Convey("3. test inspect list node pods failed", func() {
				node1 := tests.NewNode("node1", true, `{"0": 32, "1": 32}`, 64)
				patcheNodeGet := ApplyMethod(reflect.TypeOf(new(tests.MockNodes)), "Get", func(_ *tests.MockNodes,
					_ context.Context, _ string, _ metav1.GetOptions) (*v1.Node, error) {
					return node1, nil
				})
				defer patcheNodeGet.Reset()
				err := fmt.Errorf("list node pods failed")
				patchePodList := ApplyMethod(reflect.TypeOf(new(tests.MockPods)), "List", func(_ *tests.MockPods,
					_ context.Context, _ metav1.ListOptions) (*v1.PodList, error) {
					return nil, err
				})
				defer patchePodList.Reset()
				result, ok := inspect.inspect("node1").(NodeInfo)
				So(ok, ShouldEqual, true)
				So(result.Name, ShouldEqual, "node1")
				So(result.Error, ShouldEqual, err.Error())
			})
		})
	})
	Convey("2. test inspect all node", t, func() {
		Convey("1. test inspect all node success", func() {
			node1 := tests.NewNode("node1", true, `{"0": 32, "1": 32}`, 64)
			node2 := tests.NewNode("node2", false, "", 0)
			node3 := tests.NewNode("node3", true, `{"0": 32, "1": 32}`, 64)
			patcheNodeList := ApplyMethod(reflect.TypeOf(new(tests.MockNodes)), "List", func(_ *tests.MockNodes,
				_ context.Context, _ metav1.ListOptions) (*v1.NodeList, error) {
				return &v1.NodeList{Items: []v1.Node{*node1, *node2, *node3}}, nil
			})
			defer patcheNodeList.Reset()

			pod1 := tests.NewPod("pod1", "pod1-uid", "", "")
			pod2 := tests.NewPod("pod2", "pod2-uid", "", "", 16)
			pod3 := tests.NewPod("pod3", "pod3-uid", "100", "16", 16)
			pod4 := tests.NewPod("pod4", "pod4-uid", "0", "16", 16)
			pod5 := tests.NewPod("pod5", "pod5-uid", "1", "24", 24)
			pod6 := tests.NewPod("pod6", "pod6-uid", "1", "32", 32)
			outputs := []OutputCell{
				{Values: Params{&v1.PodList{Items: []v1.Pod{*pod1, *pod2, *pod3, *pod4, *pod5}}, nil}},
				{Values: Params{&v1.PodList{Items: []v1.Pod{*pod6}}, nil}},
			}
			patchePodList := ApplyMethodSeq(reflect.TypeOf(new(tests.MockPods)), "List", outputs)
			defer patchePodList.Reset()
			result, ok := inspect.inspect("").([]NodeInfo)
			So(ok, ShouldEqual, true)
			So(len(result), ShouldEqual, 2)
			So(result[0].Name, ShouldEqual, "node1")
			So(result[0].TotalGCU, ShouldEqual, 64)
			So(*result[0].UsedGCU, ShouldEqual, 40)
			So(*result[0].AvailableGCU, ShouldEqual, 24)
			So(result[0].Error, ShouldEqual, "")
			So(len(result[0].Warn), ShouldEqual, 1)
			So(strings.Contains(result[0].Warn[0], "pod3-uid"), ShouldEqual, true)

			So(len(result[0].Devices), ShouldEqual, 2)
			So(result[0].Devices["0"].TotalGCU, ShouldEqual, 32)
			So(*result[0].Devices["0"].UsedGCU, ShouldEqual, 16)
			So(*result[0].Devices["0"].AvailableGCU, ShouldEqual, 16)
			So(len(result[0].Devices["0"].Pods), ShouldEqual, 1)
			So(result[0].Devices["0"].Pods[0].Name, ShouldEqual, "pod4")
			So(string(result[0].Devices["0"].Pods[0].UID), ShouldEqual, "pod4-uid")
			So(result[0].Devices["0"].Pods[0].UsedGCU, ShouldEqual, 16)

			So(result[0].Devices["1"].TotalGCU, ShouldEqual, 32)
			So(*result[0].Devices["1"].UsedGCU, ShouldEqual, 24)
			So(*result[0].Devices["1"].AvailableGCU, ShouldEqual, 8)
			So(len(result[0].Devices["1"].Pods), ShouldEqual, 1)
			So(result[0].Devices["1"].Pods[0].Name, ShouldEqual, "pod5")
			So(string(result[0].Devices["1"].Pods[0].UID), ShouldEqual, "pod5-uid")
			So(result[0].Devices["1"].Pods[0].UsedGCU, ShouldEqual, 24)

			So(result[1].Name, ShouldEqual, "node3")
			So(result[1].TotalGCU, ShouldEqual, 64)
			So(*result[1].UsedGCU, ShouldEqual, 32)
			So(*result[1].AvailableGCU, ShouldEqual, 32)
			So(result[1].Error, ShouldEqual, "")
			So(len(result[1].Warn), ShouldEqual, 0)

			So(len(result[1].Devices), ShouldEqual, 2)
			So(result[1].Devices["0"].TotalGCU, ShouldEqual, 32)
			So(*result[1].Devices["0"].UsedGCU, ShouldEqual, 0)
			So(*result[1].Devices["0"].AvailableGCU, ShouldEqual, 32)
			So(len(result[1].Devices["0"].Pods), ShouldEqual, 0)

			So(result[1].Devices["1"].TotalGCU, ShouldEqual, 32)
			So(*result[1].Devices["1"].UsedGCU, ShouldEqual, 32)
			So(*result[1].Devices["1"].AvailableGCU, ShouldEqual, 0)
			So(len(result[1].Devices["1"].Pods), ShouldEqual, 1)
			So(result[1].Devices["1"].Pods[0].Name, ShouldEqual, "pod6")
			So(string(result[1].Devices["1"].Pods[0].UID), ShouldEqual, "pod6-uid")
			So(result[1].Devices["1"].Pods[0].UsedGCU, ShouldEqual, 32)
		})
		Convey("2. test inspect some node failed", func() {
			node1 := tests.NewNode("node1", true, `{"0": 32, "1": 32}`, 64)
			node2 := tests.NewNode("node2", false, "", 0)
			node3 := tests.NewNode("node3", true, `{"0": 32, "1": 32}`, 64)
			patcheNodeList := ApplyMethod(reflect.TypeOf(new(tests.MockNodes)), "List", func(_ *tests.MockNodes,
				_ context.Context, _ metav1.ListOptions) (*v1.NodeList, error) {
				return &v1.NodeList{Items: []v1.Node{*node1, *node2, *node3}}, nil
			})
			defer patcheNodeList.Reset()

			pod6 := tests.NewPod("pod6", "pod6-uid", "1", "32", 32)
			err := fmt.Errorf("list node1 pods failed")
			outputs := []OutputCell{
				{Values: Params{nil, err}},
				{Values: Params{&v1.PodList{Items: []v1.Pod{*pod6}}, nil}},
			}
			patchePodList := ApplyMethodSeq(reflect.TypeOf(new(tests.MockPods)), "List", outputs)
			defer patchePodList.Reset()
			result, ok := inspect.inspect("").([]NodeInfo)
			So(ok, ShouldEqual, true)
			So(len(result), ShouldEqual, 2)
			So(result[0].Name, ShouldEqual, "node1")
			So(result[0].Error, ShouldEqual, err.Error())

			So(result[1].Name, ShouldEqual, "node3")
			So(result[1].TotalGCU, ShouldEqual, 64)
			So(*result[1].UsedGCU, ShouldEqual, 32)
			So(*result[1].AvailableGCU, ShouldEqual, 32)
			So(result[1].Error, ShouldEqual, "")
			So(len(result[1].Warn), ShouldEqual, 0)

			So(len(result[1].Devices), ShouldEqual, 2)
			So(result[1].Devices["0"].TotalGCU, ShouldEqual, 32)
			So(*result[1].Devices["0"].UsedGCU, ShouldEqual, 0)
			So(*result[1].Devices["0"].AvailableGCU, ShouldEqual, 32)
			So(len(result[1].Devices["0"].Pods), ShouldEqual, 0)

			So(result[1].Devices["1"].TotalGCU, ShouldEqual, 32)
			So(*result[1].Devices["1"].UsedGCU, ShouldEqual, 32)
			So(*result[1].Devices["1"].AvailableGCU, ShouldEqual, 0)
			So(len(result[1].Devices["1"].Pods), ShouldEqual, 1)
			So(result[1].Devices["1"].Pods[0].Name, ShouldEqual, "pod6")
			So(string(result[1].Devices["1"].Pods[0].UID), ShouldEqual, "pod6-uid")
			So(result[1].Devices["1"].Pods[0].UsedGCU, ShouldEqual, 32)
		})
		Convey("3. test inspect gcushare nodes not found", func() {
			patcheNodeList := ApplyMethod(reflect.TypeOf(new(tests.MockNodes)), "List", func(_ *tests.MockNodes,
				_ context.Context, _ metav1.ListOptions) (*v1.NodeList, error) {
				return &v1.NodeList{}, nil
			})
			defer patcheNodeList.Reset()
			result, ok := inspect.inspect("").(map[string]string)
			So(ok, ShouldEqual, true)
			So(len(result), ShouldEqual, 1)
			So(result["error"], ShouldEqual, "the gcushare nodes not found")
		})
	})
	Convey("3. test inspect internal error", t, func() {
		patcheNodeList := ApplyMethod(reflect.TypeOf(new(tests.MockNodes)), "List", func(_ *tests.MockNodes,
			_ context.Context, _ metav1.ListOptions) (*v1.NodeList, error) {
			return nil, nil
		})
		defer patcheNodeList.Reset()
		result, ok := inspect.inspect("").(map[string]string)
		So(ok, ShouldEqual, true)
		So(len(result), ShouldEqual, 1)
		So(strings.Contains(result["error"], "Internal error! Recovered from panic"), ShouldEqual, true)
	})
}
