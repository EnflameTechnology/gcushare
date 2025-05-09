// Copyright (c) 2025, ENFLAME INC.  All rights reserved.
package plugin

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"testing"
	"time"

	. "github.com/agiledragon/gomonkey"
	. "github.com/smartystreets/goconvey/convey"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sTypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/kubernetes/pkg/scheduler/framework"

	"gcushare-scheduler-plugin/pkg/config"
	"gcushare-scheduler-plugin/pkg/consts"
	"gcushare-scheduler-plugin/pkg/resources"
	"gcushare-scheduler-plugin/tests"
)

func TestSchedulerPlugin(t *testing.T) {
	// init scheduler plugin
	clientset := &kubernetes.Clientset{}
	config := &config.Config{RegisterResource: []string{"enflame.com/gcu"}}
	baseResource := resources.NewBaseResource(config, clientset)
	schedulerPlugin := &GCUShareSchedulerPlugin{
		resourceName: config.ResourceName(),
		mu:           new(sync.Mutex),
		clientset:    clientset,
		config:       config,
		filterCache:  make(map[k8sTypes.UID]filterResult),
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

	patcheSleep := ApplyFunc(time.Sleep, func(_ time.Duration) {})
	defer patcheSleep.Reset()
	Convey("1. test filter and bind extension points with single node", t, func() {
		Convey("1. test schedule pod success", func() {
			Convey("1. test schedule pod no schedulered gcushare pods", func() {
				pod := tests.NewPod("pod", "pod-uid", "", "", 16)
				nodeInfo := framework.NodeInfo{}
				nodeInfo.SetNode(tests.NewNode("node1", true, `{"0": 32, "1": 32}`, 64))
				filterStatus := schedulerPlugin.Filter(context.TODO(), &framework.CycleState{}, pod, &nodeInfo)
				So(filterStatus.Code(), ShouldEqual, framework.Success)
				filterCacheExpected := map[k8sTypes.UID]filterResult{pod.UID: {
					result:       map[string]map[string]int{nodeInfo.Node().Name: {"0": 32, "1": 32}},
					resourceName: consts.ResourceName,
				}}
				So(reflect.DeepEqual(schedulerPlugin.filterCache, filterCacheExpected), ShouldEqual, true)

				bindStatus := schedulerPlugin.Bind(context.TODO(), &framework.CycleState{}, pod, nodeInfo.Node().Name)
				So(bindStatus.Code(), ShouldEqual, framework.Success)
				So(len(schedulerPlugin.filterCache), ShouldEqual, 0)
			})
			Convey("2. test schedule pod with schedulered gcushare pods", func() {
				pod := tests.NewPod("pod", "pod-uid", "", "", 16)
				pod1 := tests.NewPod("pod1", "pod1-uid", "0", "16", 16)
				pod2 := tests.NewPod("pod2", "pod2-uid", "", "")                 // not gcushare pod
				pod3 := tests.NewPod("pod3", "pod3-uid", "", "", 16)             // deleted
				pod4 := tests.NewPod("pod4", "pod4-uid", "", "", 16)             // internal error
				pod4Recreate := tests.NewPod("pod4", "pod4-new-uid", "", "", 16) // pod4 recreate, skip it
				pod5 := tests.NewPod("pod5", "pod5-uid", "", "", 16)             // pod5 not assign
				pod5Update := tests.NewPod("pod5", "pod5-uid", "0", "16", 16)    // pod5 assigned
				nodeInfo := framework.NodeInfo{Requested: &framework.Resource{}, NonZeroRequested: &framework.Resource{}}
				nodeInfo.SetNode(tests.NewNode("node1", true, `{"0": 32, "1": 32}`, 64))
				nodeInfo.AddPod(pod1)
				nodeInfo.AddPod(pod2)
				nodeInfo.AddPod(pod3)
				nodeInfo.AddPod(pod4)
				nodeInfo.AddPod(pod5)
				outputs := []OutputCell{
					{Values: Params{nil, tests.NewAPIStatus(metav1.StatusReasonNotFound, http.StatusNotFound)}},
					{Values: Params{nil, tests.NewAPIStatus(metav1.StatusReasonInternalError, http.StatusInternalServerError)}},
					{Values: Params{pod4Recreate, nil}},
					{Values: Params{pod5Update, nil}},
				}
				patcheAssume := ApplyMethodSeq(reflect.TypeOf(new(tests.MockPods)), "Get", outputs)
				defer patcheAssume.Reset()
				status := schedulerPlugin.Filter(context.TODO(), &framework.CycleState{}, pod, &nodeInfo)
				So(status.Code(), ShouldEqual, framework.Success)
				filterCacheExpected := map[k8sTypes.UID]filterResult{pod.UID: {
					result:       map[string]map[string]int{nodeInfo.Node().Name: {"0": 0, "1": 32}},
					resourceName: consts.ResourceName,
				}}
				So(reflect.DeepEqual(schedulerPlugin.filterCache, filterCacheExpected), ShouldEqual, true)

				bindStatus := schedulerPlugin.Bind(context.TODO(), &framework.CycleState{}, pod, nodeInfo.Node().Name)
				So(bindStatus.Code(), ShouldEqual, framework.Success)
				So(len(schedulerPlugin.filterCache), ShouldEqual, 0)
			})
		})
		Convey("2. test filter pod error", func() {
			Convey("1. test pod is not gcushare pod", func() {
				pod := tests.NewPod("pod", "pod-uid", "", "")
				nodeInfo := framework.NodeInfo{}
				nodeInfo.SetNode(tests.NewNode("node1", true, `{"0": 32, "1": 32}`, 64))
				status := schedulerPlugin.Filter(context.TODO(), &framework.CycleState{}, pod, &nodeInfo)
				So(status.Code(), ShouldEqual, framework.Error)
				So(len(schedulerPlugin.filterCache), ShouldEqual, 0)
			})
			Convey("2. test node no annotation device memery map", func() {
				pod := tests.NewPod("pod", "pod-uid", "", "", 16)
				nodeInfo := framework.NodeInfo{}
				nodeInfo.SetNode(tests.NewNode("node1", true, "", 64))
				status := schedulerPlugin.Filter(context.TODO(), &framework.CycleState{}, pod, &nodeInfo)
				So(status.Code(), ShouldEqual, framework.Error)
				So(len(schedulerPlugin.filterCache), ShouldEqual, 0)
			})
			Convey("3. test node device memery map is empty", func() {
				pod := tests.NewPod("pod", "pod-uid", "", "", 16)
				nodeInfo := framework.NodeInfo{}
				nodeInfo.SetNode(tests.NewNode("node1", true, "{}", 64))
				status := schedulerPlugin.Filter(context.TODO(), &framework.CycleState{}, pod, &nodeInfo)
				So(status.Code(), ShouldEqual, framework.Unschedulable)
				So(len(schedulerPlugin.filterCache), ShouldEqual, 0)
			})
		})
		Convey("3. test filter pod unschedulable", func() {
			Convey("1. test node is not gcushare node", func() {
				pod := tests.NewPod("pod", "pod-uid", "", "", 16)
				nodeInfo := framework.NodeInfo{}
				nodeInfo.SetNode(tests.NewNode("node1", false, "", 0))
				status := schedulerPlugin.Filter(context.TODO(), &framework.CycleState{}, pod, &nodeInfo)
				So(status.Code(), ShouldEqual, framework.Unschedulable)
				So(len(schedulerPlugin.filterCache), ShouldEqual, 0)
			})
			Convey("2. test node resource is insufficient", func() {
				pod := tests.NewPod("pod", "pod-uid", "", "", 33)
				nodeInfo := framework.NodeInfo{}
				nodeInfo.SetNode(tests.NewNode("node1", true, `{"0": 32, "1": 32}`, 64))
				status := schedulerPlugin.Filter(context.TODO(), &framework.CycleState{}, pod, &nodeInfo)
				So(status.Code(), ShouldEqual, framework.Unschedulable)
				So(len(schedulerPlugin.filterCache), ShouldEqual, 0)
			})
		})
	})

	Convey("2. test filter and bind extension points with multi node", t, func() {
		pod := tests.NewPod("pod", "pod-uid", "", "", 16)
		// node1 is not gcushare node
		nodeInfo1 := framework.NodeInfo{}
		nodeInfo1.SetNode(tests.NewNode("node1", false, "", 0))
		// node2 available
		pod1 := tests.NewPod("pod1", "pod1-uid", "0", "20", 10, 10)
		nodeInfo2 := framework.NodeInfo{Requested: &framework.Resource{}, NonZeroRequested: &framework.Resource{}}
		nodeInfo2.SetNode(tests.NewNode("node2", true, `{"0": 32, "1": 32}`, 64))
		nodeInfo2.AddPod(pod1)
		// node3 resource insufficient
		pod2 := tests.NewPod("pod2", "pod2-uid", "1", "32", 16, 16)
		nodeInfo3 := framework.NodeInfo{Requested: &framework.Resource{}, NonZeroRequested: &framework.Resource{}}
		nodeInfo3.SetNode(tests.NewNode("node3", true, `{"0": 32, "1": 32}`, 64))
		nodeInfo3.AddPod(pod1)
		nodeInfo3.AddPod(pod2)
		// node4 available
		pod3 := tests.NewPod("pod3", "pod3-uid", "1", "16", 16)
		nodeInfo4 := framework.NodeInfo{Requested: &framework.Resource{}, NonZeroRequested: &framework.Resource{}}
		nodeInfo4.SetNode(tests.NewNode("node4", true, `{"0": 32, "1": 32}`, 64))
		nodeInfo4.AddPod(pod1)
		nodeInfo4.AddPod(pod3)
		Convey("1. test filter pod success", func() {
			var status1, status2, status3, status4 *framework.Status
			wg := &sync.WaitGroup{}
			wg.Add(4)
			go func() {
				defer wg.Done()
				status1 = schedulerPlugin.Filter(context.TODO(), &framework.CycleState{}, pod, &nodeInfo1)
			}()
			go func() {
				defer wg.Done()
				status2 = schedulerPlugin.Filter(context.TODO(), &framework.CycleState{}, pod, &nodeInfo2)
			}()
			go func() {
				defer wg.Done()
				status3 = schedulerPlugin.Filter(context.TODO(), &framework.CycleState{}, pod, &nodeInfo3)
			}()
			go func() {
				defer wg.Done()
				status4 = schedulerPlugin.Filter(context.TODO(), &framework.CycleState{}, pod, &nodeInfo4)
			}()
			wg.Wait()
			So(status1.Code(), ShouldEqual, framework.Unschedulable)
			So(status2.Code(), ShouldEqual, framework.Success)
			So(status3.Code(), ShouldEqual, framework.Unschedulable)
			So(status4.Code(), ShouldEqual, framework.Success)
			filterCacheExpected := map[k8sTypes.UID]filterResult{pod.UID: {
				result: map[string]map[string]int{
					nodeInfo2.Node().Name: {"0": 12, "1": 32},
					nodeInfo4.Node().Name: {"0": 12, "1": 16}},
				resourceName: consts.ResourceName,
			}}
			So(reflect.DeepEqual(schedulerPlugin.filterCache, filterCacheExpected), ShouldEqual, true)
		})
		Convey("2. test bind pod patch pod failed", func() {
			err := fmt.Errorf("patch pod failed")
			patchePodsPatch := ApplyMethod(reflect.TypeOf(new(tests.MockPods)), "Patch", func(_ *tests.MockPods,
				_ context.Context, _ string, _ k8sTypes.PatchType, _ []byte, _ metav1.PatchOptions,
				_ ...string) (*v1.Pod, error) {
				return nil, err
			})
			defer patchePodsPatch.Reset()
			bindStatus := schedulerPlugin.Bind(context.TODO(), &framework.CycleState{}, pod, nodeInfo2.Node().Name)
			So(bindStatus.Code(), ShouldEqual, framework.Error)
			So(bindStatus.Reasons(), ShouldEqual, []string{err.Error()})
			filterCacheExpected := map[k8sTypes.UID]filterResult{pod.UID: {
				result: map[string]map[string]int{
					nodeInfo2.Node().Name: {"0": 12, "1": 32},
					nodeInfo4.Node().Name: {"0": 12, "1": 16}},
				resourceName: consts.ResourceName,
			}}
			So(reflect.DeepEqual(schedulerPlugin.filterCache, filterCacheExpected), ShouldEqual, true)
		})
		Convey("3. test bind pod bind pod failed", func() {
			err := fmt.Errorf("bind pod failed")
			patchePodsBind := ApplyMethod(reflect.TypeOf(new(tests.MockPods)), "Bind", func(_ *tests.MockPods,
				_ context.Context, _ *v1.Binding, _ metav1.CreateOptions) error {
				return err
			})
			defer patchePodsBind.Reset()
			bindStatus := schedulerPlugin.Bind(context.TODO(), &framework.CycleState{}, pod, nodeInfo2.Node().Name)
			So(bindStatus.Code(), ShouldEqual, framework.Error)
			So(bindStatus.Reasons(), ShouldEqual, []string{err.Error()})
			filterCacheExpected := map[k8sTypes.UID]filterResult{pod.UID: {
				result: map[string]map[string]int{
					nodeInfo2.Node().Name: {"0": 12, "1": 32},
					nodeInfo4.Node().Name: {"0": 12, "1": 16}},
				resourceName: consts.ResourceName,
			}}
			So(reflect.DeepEqual(schedulerPlugin.filterCache, filterCacheExpected), ShouldEqual, true)
		})
		Convey("4. test bind pod success", func() {
			bindStatus := schedulerPlugin.Bind(context.TODO(), &framework.CycleState{}, pod, nodeInfo4.Node().Name)
			So(bindStatus.Code(), ShouldEqual, framework.Success)
			So(len(schedulerPlugin.filterCache), ShouldEqual, 0)
		})
	})
}
