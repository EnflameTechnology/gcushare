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
	k8sTypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/scheduler/framework"

	"gcushare-scheduler-plugin/pkg/config"
	"gcushare-scheduler-plugin/pkg/consts"
	"gcushare-scheduler-plugin/pkg/logs"
	"gcushare-scheduler-plugin/pkg/structs"
)

type NodeResource BaseResource
type NodeName string

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

func (res *NodeResource) GetNodeAvailableGCUs(nodeInfo *framework.NodeInfo,
	getReserveCache func(podUID k8sTypes.UID) (map[string]string, bool)) (map[structs.Minor]int, error) {
	gcuTotalMemoryMap, err := res.GetGCUDeviceMemoryMap(nodeInfo.Node())
	if err != nil {
		return nil, err
	}
	availableGCUs := map[structs.Minor]int{}
	usedSharedGCU, deviceUsedByDRS, err := NewPodResource((*BaseResource)(res)).GetUsedSharedGCU(nodeInfo.Pods, getReserveCache)
	if err != nil {
		return nil, err
	}
	logs.Info("node: %s already used shared gcus: %v", nodeInfo.Node().Name, usedSharedGCU)
	if len(deviceUsedByDRS) > 0 {
		logs.Warn("node: %s following devices are used by drs pod: %v", nodeInfo.Node().Name, deviceUsedByDRS)
	}
	for deviceID, totalMemory := range gcuTotalMemoryMap {
		if _, ok := deviceUsedByDRS[deviceID]; ok {
			continue
		}
		availableGCUs[structs.Minor(deviceID)] = totalMemory - usedSharedGCU[deviceID]
		if availableGCUs[structs.Minor(deviceID)] < 0 {
			err := fmt.Errorf("device: %s on node: %s total size: %d, but used: %d, gcushare slice count may be modified, which is not allowed",
				deviceID, nodeInfo.Node().Name, totalMemory, usedSharedGCU[deviceID])
			logs.Error(err)
			return nil, err
		}
	}
	return availableGCUs, nil
}

func (res *NodeResource) GetNodeAvailableDRS(nodeInfo *framework.NodeInfo, devices []structs.DeviceSpec,
	getReserveCache func(podUID k8sTypes.UID) (map[string]string, bool)) (map[structs.Minor]int, error) {
	availableDRS := map[structs.Minor]int{}
	nodeUsedDRS, deviceUsedBySharedGCU, err := NewPodResource((*BaseResource)(res)).GetUsedDRSGCU(nodeInfo.Pods, getReserveCache)
	if err != nil {
		return nil, err
	}
	logs.Info("node: %s already used drs device: %v", nodeInfo.Node().Name, nodeUsedDRS)
	if len(deviceUsedBySharedGCU) > 0 {
		logs.Warn("node: %s following devices are used by gcushare pod: %v", nodeInfo.Node().Name, deviceUsedBySharedGCU)
	}
	for _, devicespec := range devices {
		if _, ok := deviceUsedBySharedGCU[devicespec.Minor]; ok {
			continue
		}
		availableDRS[structs.Minor(devicespec.Minor)] = devicespec.Capacity - nodeUsedDRS[devicespec.Minor]
	}
	return availableDRS, nil
}

func (res *NodeResource) GetGCUDeviceMemoryMap(node *v1.Node) (map[string]int, error) {
	annotationMemoryMap, ok := node.Annotations[res.Config.ReplaceDomain(consts.GCUSharedCapacity)]
	if !ok {
		err := fmt.Errorf("annotation field: %s not found in node: %s, node annotations: %v",
			res.Config.ReplaceDomain(consts.GCUSharedCapacity), node.Name, node.Annotations)
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
