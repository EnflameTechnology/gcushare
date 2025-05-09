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
)

type NodeResource BaseResource

func NewNodeResource(baseResource *BaseResource) *NodeResource {
	return (*NodeResource)(baseResource)
}

// Is the Node for GCU sharing
func (res *NodeResource) IsGCUSharingNode(node *v1.Node) bool {
	_, ok := node.Labels[consts.GCUShareLabel]
	if !ok {
		return false
	}
	value, ok := node.Status.Allocatable[v1.ResourceName(res.ResourceName)]
	if !ok {
		return false
	}
	return value.Value() > 0
}

func (res *NodeResource) GetNodeAvailableGCUs(nodeInfo *framework.NodeInfo) (map[string]int, error) {
	gcuTotalMemoryMap, err := res.GetGCUDeviceMemoryMap(nodeInfo.Node())
	if err != nil {
		return nil, err
	}
	availableGCUs := map[string]int{}
	nodeUsedMemory, err := NewPodResource((*BaseResource)(res)).GetUsedGCUs(nodeInfo.Pods)
	if err != nil {
		return nil, err
	}
	for deviceID, totalMemory := range gcuTotalMemoryMap {
		availableGCUs[deviceID] = totalMemory - nodeUsedMemory[deviceID]
		if availableGCUs[deviceID] < 0 {
			err := fmt.Errorf("device: %s on node: %s total size: %d, but used: %d, gcushare slice count may be modified, which is not allowed",
				deviceID, nodeInfo.Node().Name, totalMemory, nodeUsedMemory[deviceID])
			logs.Error(err)
			return nil, err
		}
	}
	return availableGCUs, nil
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
			value, ok := gcushareNode.Status.Allocatable[v1.ResourceName(config.ResourceName())]
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
