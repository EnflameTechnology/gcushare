// Copyright (c) 2025, ENFLAME INC.  All rights reserved.
package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/scheduler/framework"

	"gcushare-scheduler-plugin/pkg/config"
	"gcushare-scheduler-plugin/pkg/consts"
	"gcushare-scheduler-plugin/pkg/logs"
	"gcushare-scheduler-plugin/pkg/structs"
)

type NodeResource BaseResource
type NodeName string
type DeviceID string

func NewNodeResource(baseResource *BaseResource) *NodeResource {
	return (*NodeResource)(baseResource)
}

// Is the Node for GCU sharing
func (res *NodeResource) IsGCUSharingNode(isDRSPod bool, node *v1.Node) bool {
	_, ok := node.Labels[consts.GCUShareLabel]
	if !ok {
		return false
	}
	if isDRSPod {
		value, ok := node.Status.Allocatable[v1.ResourceName(res.DRSResourceName)]
		if !ok {
			return false
		}
		if value.Value() <= 0 {
			return false
		}
		return true
	}

	value, ok := node.Status.Allocatable[v1.ResourceName(res.SharedResourceName)]
	if !ok {
		return false
	}
	if value.Value() <= 0 {
		return false
	}
	return true
}

func (res *NodeResource) GetNodeAvailableGCUs(nodeInfo *framework.NodeInfo) (map[DeviceID]int, error) {
	gcuTotalMemoryMap, err := res.GetGCUDeviceMemoryMap(false, nodeInfo.Node())
	if err != nil {
		return nil, err
	}
	availableGCUs := map[DeviceID]int{}
	nodeUsedMemory, err := NewPodResource((*BaseResource)(res)).GetUsedGCUs(nodeInfo.Pods)
	if err != nil {
		return nil, err
	}
	for deviceID, totalMemory := range gcuTotalMemoryMap {
		drsUsed, err := CheckDeviceUsed(deviceID, nodeInfo)
		if err != nil {
			return nil, err
		}
		if drsUsed {
			continue
		}
		availableGCUs[DeviceID(deviceID)] = totalMemory - nodeUsedMemory[deviceID]
		if availableGCUs[DeviceID(deviceID)] < 0 {
			err := fmt.Errorf("device: %s on node: %s total size: %d, but used: %d, gcushare slice count may be modified, which is not allowed",
				deviceID, nodeInfo.Node().Name, totalMemory, nodeUsedMemory[deviceID])
			logs.Error(err)
			return nil, err
		}
	}
	return availableGCUs, nil
}

func (res *NodeResource) GetGCUDeviceMemoryMap(drs bool, node *v1.Node) (map[string]int, error) {
	field := consts.GCUSharedCapacity
	if drs {
		field = consts.GCUDRSCapacity
	}
	annotationMemoryMap, ok := node.Annotations[res.Config.ReplaceDomain(field)]
	if !ok {
		err := fmt.Errorf("annotation field: %s not found in node: %s, node annotations: %v",
			res.Config.ReplaceDomain(field), node.Name, node.Annotations)
		logs.Error(err)
		return nil, err
	}
	memoryMap := map[string]int{}
	if err := json.Unmarshal([]byte(annotationMemoryMap), &memoryMap); err != nil {
		logs.Error(err, "json unmarshal string: %s to map failed", annotationMemoryMap)
		return nil, err
	}
	if len(memoryMap) == 0 {
		err := fmt.Errorf("gcu device not found on node: %s", node.Name)
		logs.Error(err)
	}
	return memoryMap, nil
}

func WaitDevicePluginRunning(clientSet *kubernetes.Clientset, config *config.Config) {
	selector := labels.SelectorFromSet(labels.Set{consts.GCUShareLabel: "true"})
	for {
		gcushareNodes, err := clientSet.CoreV1().Nodes().List(context.TODO(),
			metav1.ListOptions{LabelSelector: selector.String()})
		if err != nil {
			logs.Error(err, "list cluster gcushare nodes list failed")
			time.Sleep(time.Second * 3)
			continue
		}
		if len(gcushareNodes.Items) == 0 {
			logs.Error("GCUShare nodes not found, please check the component GCUShare Device Plugin has deployed?")
			time.Sleep(time.Second * 3)
			continue
		}

		allRunning := true
		for _, gcushareNode := range gcushareNodes.Items {
			value, ok := gcushareNode.Status.Allocatable[v1.ResourceName(config.ResourceName(false))]
			if !ok || value.Value() <= 0 {
				logs.Warn("GCUShare Device Plugin on node: %s is not working normally", gcushareNode.Name)
				allRunning = false
			}
		}
		if allRunning {
			logs.Info("GCUShare Device Plugin of all gcushare nodes is working normally")
			return
		}
		time.Sleep(time.Second * 3)
	}
}

func CheckDeviceUsed(deviceID string, nodeInfo *framework.NodeInfo) (bool, error) {
	for _, pod := range nodeInfo.Pods {
		drsDevice, ok := pod.Pod.Annotations[consts.DRSAssignedDevice]
		if !ok {
			continue
		}
		assignedDevice := &structs.DeviceSpec{}
		if err := json.Unmarshal([]byte(drsDevice), assignedDevice); err != nil {
			logs.Error(err, "json unmarshal failed, content: %s", drsDevice)
			return false, err
		}
		if assignedDevice.Minor == deviceID {
			logs.Warn("device: %s is using by drs pod(name: %s, uuid: %s), skip it", deviceID,
				pod.Pod.Name, pod.Pod.UID)
			return true, nil
		}
	}
	return false, nil
}
