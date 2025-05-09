// Copyright (c) 2025, ENFLAME INC.  All rights reserved.
package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sTypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/scheduler/framework"

	"gcushare-scheduler-plugin/pkg/config"
	"gcushare-scheduler-plugin/pkg/consts"
	"gcushare-scheduler-plugin/pkg/logs"
	"gcushare-scheduler-plugin/pkg/resources"
)

type filterResult struct {
	resourceName string
	result       map[string]map[string]int
}

func (fr filterResult) setNode(pod *v1.Pod, status *framework.Status, nodeName string, nodeAvailableGCUs map[string]int) {
	if status.Code() == framework.Success {
		logs.Debug("filter cache for pod(name: %s, uuid: %s) update node: %s available %s from %v to %v",
			pod.Name, pod.UID, nodeName, fr.resourceName, fr.result[nodeName], nodeAvailableGCUs)
		fr.result[nodeName] = nodeAvailableGCUs
		logs.Info("filter cache for pod(name: %s, uuid: %s) set node: %s available %s: %v", pod.Name, pod.UID,
			nodeName, fr.resourceName, nodeAvailableGCUs)
	} else {
		if fr.result[nodeName] != nil {
			logs.Debug("filter cache for pod(name: %s, uuid: %s) update node: %s available %s from %v to %v",
				pod.Name, pod.UID, nodeName, fr.resourceName, fr.result[nodeName], nil)
			fr.result[nodeName] = nil
			logs.Info("filter cache for pod(name: %s, uuid: %s) clear node: %s available %s with filter status: %v",
				pod.Name, pod.UID, nodeName, fr.resourceName, status)
			return
		}
	}
}

type GCUShareSchedulerPlugin struct {
	mu           *sync.Mutex
	resourceName string
	clientset    *kubernetes.Clientset
	config       *config.Config
	filterCache  map[k8sTypes.UID]filterResult
	podResource  *resources.PodResource
	nodeResource *resources.NodeResource
}

var _ framework.BindPlugin = &GCUShareSchedulerPlugin{}
var _ framework.FilterPlugin = &GCUShareSchedulerPlugin{}

func (gsp *GCUShareSchedulerPlugin) Name() string {
	return consts.SchedulerPluginName
}

func (gsp *GCUShareSchedulerPlugin) updateFilterCache(status *framework.Status, pod *v1.Pod,
	nodeName string, nodeAvailableGCUs *map[string]int) {
	gsp.mu.Lock()
	defer gsp.mu.Unlock()
	logs.Info("pod(name: %s, uuid: %s) filter node: %s status: %s", pod.Name, pod.UID, nodeName,
		status.Code().String())
	if gsp.filterCache[pod.UID].result == nil {
		if status.Code() != framework.Success {
			logs.Warn("pod(name: %s, uuid: %s) can not add to filter cache", pod.Name, pod.UID)
			return
		}
		logs.Info("init filter cache for pod(name: %s, uuid: %s)", pod.Name, pod.UID)
		gsp.filterCache[pod.UID] = filterResult{resourceName: gsp.resourceName, result: make(map[string]map[string]int)}
	}
	gsp.filterCache[pod.UID].setNode(pod, status, nodeName, *nodeAvailableGCUs)
}

func (gsp *GCUShareSchedulerPlugin) clearFilterCache(pod *v1.Pod, status *framework.Status, nodeName string) {
	gsp.mu.Lock()
	defer gsp.mu.Unlock()
	if status.Code() == framework.Success {
		logs.Info("clear filter cache for pod(name: %s, uuid: %s) with bind node: %s success ", pod.Name, pod.UID, nodeName)
		delete(gsp.filterCache, pod.UID)
	} else {
		logs.Warn("filter cache for pod(name: %s, uuid: %s) will not be cleard with bind node: %s unsuccessful, status: %v, filter cache: %v",
			pod.Name, pod.UID, nodeName, status, gsp.filterCache[pod.UID])
	}
}

func (gsp *GCUShareSchedulerPlugin) Filter(ctx context.Context, state *framework.CycleState, pod *v1.Pod,
	nodeInfo *framework.NodeInfo) (status *framework.Status) {
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("Internal error! Recovered from panic: %v", r)
			logs.Error(errMsg)
			status = framework.NewStatus(framework.Error, errMsg)
		}
	}()
	var nodeAvailableGCUs map[string]int
	defer func() {
		gsp.updateFilterCache(status, pod, nodeInfo.Node().Name, &nodeAvailableGCUs)
	}()
	podName := pod.Name
	nodeName := nodeInfo.Node().Name
	logs.Info("----------filter node: %s for pod(name: %s, uuid: %s)----------", nodeName, podName, pod.UID)
	if !gsp.podResource.IsGCUsharingPod(pod) {
		err := fmt.Errorf("pod(name: %s, uuid: %s) does not request %s, please do not specify the scheduler name: %s",
			podName, pod.UID, gsp.resourceName, consts.SchedulerName)
		logs.Error(err)
		return framework.NewStatus(framework.Error, err.Error())
	}
	if !gsp.nodeResource.IsGCUSharingNode(nodeInfo.Node()) {
		msg := fmt.Sprintf("node: %s is not gcushare node, skip it", nodeInfo.Node().Name)
		logs.Warn(msg)
		return framework.NewStatus(framework.Unschedulable, msg)
	}
	return gsp.filterNode(pod, nodeInfo, &nodeAvailableGCUs)
}

func (gsp *GCUShareSchedulerPlugin) filterNode(pod *v1.Pod, nodeInfo *framework.NodeInfo,
	nodeAvailableGCUs *map[string]int) *framework.Status {
	var err error
	*nodeAvailableGCUs, err = gsp.nodeResource.GetNodeAvailableGCUs(nodeInfo)
	if err != nil {
		return framework.NewStatus(framework.Error, err.Error())
	}
	logs.Info("node: %s available %s: %v", nodeInfo.Node().Name, gsp.resourceName, *nodeAvailableGCUs)
	podRequest := gsp.podResource.GetGCUMemoryFromPodResource(pod)
	for _, available := range *nodeAvailableGCUs {
		if available >= podRequest {
			logs.Info("pod(name: %s, uuid: %s) request %s: %d, and node: %s is available",
				pod.Name, pod.UID, gsp.resourceName, podRequest, nodeInfo.Node().Name)
			return framework.NewStatus(framework.Success)
		}
	}
	msg := fmt.Sprintf("pod(name: %s, uuid: %s) request %s: %d, but it is insufficient of node: %s",
		pod.Name, pod.UID, gsp.resourceName, podRequest, nodeInfo.Node().Name)
	logs.Warn(msg)
	return framework.NewStatus(framework.Unschedulable, msg)
}

func (gsp *GCUShareSchedulerPlugin) Bind(ctx context.Context, state *framework.CycleState,
	pod *v1.Pod, nodeName string) (status *framework.Status) {
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("Internal error! Recovered from panic: %v", r)
			logs.Error(errMsg)
			status = framework.NewStatus(framework.Error, errMsg)
		}
	}()
	defer func() {
		gsp.clearFilterCache(pod, status, nodeName)
	}()
	logs.Info("----------bind pod(name: %s, uuid: %s) to node: %s----------", pod.Name, pod.UID, nodeName)
	if err := gsp.allocatedDeviceID(nodeName, pod); err != nil {
		return framework.NewStatus(framework.Error, err.Error())
	}
	if err := gsp.clientset.CoreV1().Pods(pod.Namespace).Bind(context.TODO(), &v1.Binding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
		Target: v1.ObjectReference{
			Kind: "Node",
			Name: nodeName,
		},
	}, metav1.CreateOptions{}); err != nil {
		logs.Error(err, "bind pod(name: %s, uuid: %s) to node %s failed", pod.Name, pod.UID, nodeName)
		return framework.NewStatus(framework.Error, err.Error())
	}
	logs.Info("binding pod(name: %s, uuid: %s) to node %s success", pod.Name, pod.UID, nodeName)
	return framework.NewStatus(framework.Success, "")
}

func (gsp *GCUShareSchedulerPlugin) allocatedDeviceID(nodeName string, pod *v1.Pod) error {
	nodeAvailableGCUs, ok := gsp.filterCache[pod.UID].result[nodeName]
	if !ok || len(nodeAvailableGCUs) == 0 {
		err := fmt.Errorf("internal error: node: %s has no available %s", nodeName, gsp.resourceName)
		logs.Error(err)
		return err
	}
	podRequest := gsp.podResource.GetGCUMemoryFromPodResource(pod)
	logs.Info("pod(name: %s, uuid: %s) request %s: %d, available %s: %v", pod.Name, pod.UID, gsp.resourceName,
		podRequest, gsp.resourceName, nodeAvailableGCUs)

	selectedID := ""
	selectedIDMemory := 0
	for deviceID, available := range nodeAvailableGCUs {
		if available >= podRequest {
			if selectedID == "" || selectedIDMemory > available {
				selectedID = deviceID
				selectedIDMemory = available
			}
		}
	}
	if selectedID == "" {
		err := fmt.Errorf("internal error! node: %s could not bind pod(name: %s, uuid: %s) with available %s: %v",
			nodeName, pod.Name, pod.UID, gsp.resourceName, nodeAvailableGCUs)
		logs.Error(err)
		return err
	}
	logs.Info("gcushare scheduler plugin assigned device '%s' to pod(name: %s, uuid: %s)",
		selectedID, pod.Name, pod.UID)
	if err := gsp.patchPodAnnotations(pod, selectedID, podRequest); err != nil {
		return err
	}
	return nil
}

func (gsp *GCUShareSchedulerPlugin) patchPodAnnotations(pod *v1.Pod, selectedID string, podRequest int) error {
	patchAnnotations := map[string]interface{}{
		"metadata": map[string]map[string]string{"annotations": {
			gsp.config.ReplaceResource(consts.PodAssignedGCUID):   selectedID,
			gsp.config.ReplaceResource(consts.PodRequestGCUSize):  fmt.Sprintf("%d", podRequest),
			gsp.config.ReplaceResource(consts.PodHasAssignedGCU):  "false",
			gsp.config.ReplaceResource(consts.PodAssignedGCUTime): fmt.Sprintf("%d", time.Now().UnixNano()),
		}}}
	patchedAnnotationBytes, err := json.Marshal(patchAnnotations)
	if err != nil {
		logs.Error(err, "marshal patch annotations failed")
		return err
	}
	_, err = gsp.clientset.CoreV1().Pods(pod.Namespace).Patch(context.TODO(), pod.Name,
		k8sTypes.StrategicMergePatchType, patchedAnnotationBytes, metav1.PatchOptions{})
	if err != nil {
		logs.Error(err, "patch pod(name: %s, uuid: %s) annotations to cluster failed, content: %s",
			pod.Name, pod.UID, string(patchedAnnotationBytes))
		return err
	}
	logs.Info("patch pod(name: %s, uuid: %s) annotations: %s success", pod.Name, pod.UID,
		string(patchedAnnotationBytes))
	return nil
}
