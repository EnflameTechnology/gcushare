// Copyright 2022 Enflame. All Rights Reserved.
package resource

import (
	"context"
	"encoding/json"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"

	"gcushare-device-plugin/pkg/consts"
	"gcushare-device-plugin/pkg/logs"
	"gcushare-device-plugin/pkg/utils"
)

type NodeResource ClusterResource

func NewNodeResource(clusterResource *ClusterResource) *NodeResource {
	return (*NodeResource)(clusterResource)
}

func (rer *NodeResource) PatchGCUCountToNode(gcuCount int) error {
	node, err := rer.GetNode()
	if err != nil {
		logs.Error(err, "get node %s info from cluster failed", rer.NodeName)
		return err
	}

	resourceCount := v1.ResourceName(rer.Config.ReplaceResource(consts.CountName))
	if val, ok := node.Status.Capacity[resourceCount]; ok {
		if val.Value() == int64(gcuCount) {
			logs.Info("node %s need not to update Capacity %s", rer.NodeName, resourceCount)
			return nil
		}
	}

	newNode := node.DeepCopy()
	newNode.Status.Capacity[resourceCount] = *resource.NewQuantity(int64(gcuCount), resource.DecimalSI)
	newNode.Status.Allocatable[resourceCount] = *resource.NewQuantity(int64(gcuCount), resource.DecimalSI)
	patchNode, err := rer.PatchNodeStatus(node, newNode)
	if err != nil {
		logs.Error(err, "patch node status to cluster failed, node detail:%s",
			utils.ConvertToString(newNode))
		return err
	}
	logs.Info("node %s capacity: %s, allocatable: %s", rer.NodeName, utils.ConvertToString(patchNode.Status.Capacity),
		utils.ConvertToString(patchNode.Status.Allocatable))
	return nil
}

// PatchNodeStatus patches node status.
func (rer *NodeResource) PatchNodeStatus(oldNode *v1.Node, newNode *v1.Node) (*v1.Node, error) {
	patchBytes, err := preparePatchBytesforNodeStatus(rer.NodeName, oldNode, newNode)
	if err != nil {
		return nil, err
	}
	// updatedNode, err := rer.ClientSet.CoreV1().Nodes().Patch(context.TODO(), rer.NodeName,
	// 	k8sType.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	updatedNode, err := rer.PatchNode(patchBytes)
	if err != nil {
		logs.Error(err, "failed to patch status %s for node %s", string(patchBytes), rer.NodeName)
		return nil, fmt.Errorf("failed to patch status %q for node %q: %v", patchBytes, rer.NodeName, err)
	}
	logs.Info("patch node %s status success, patch content: %s", rer.NodeName, string(patchBytes))
	return updatedNode, nil
}

func preparePatchBytesforNodeStatus(nodeName string, oldNode *v1.Node, newNode *v1.Node) ([]byte, error) {
	oldData, err := json.Marshal(oldNode)
	if err != nil {
		logs.Error(err, "failed to Marshal oldData for node %s", nodeName)
		return nil, fmt.Errorf("failed to Marshal oldData for node %q: %v", nodeName, err)
	}

	// Reset spec to make sure only patch for Status or ObjectMeta is generated.
	// Note that we don't reset ObjectMeta here, because:
	// 1. This aligns with Nodes().UpdateStatus().
	// 2. Some component does use this to update node annotations.
	newNode.Spec = oldNode.Spec
	newData, err := json.Marshal(newNode)
	if err != nil {
		logs.Error(err, "failed to Marshal oldData for node %s", nodeName)
		return nil, fmt.Errorf("failed to Marshal newData for node %q: %v", nodeName, err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, v1.Node{})
	if err != nil {
		logs.Error(err, "failed to CreateTwoWayMergePatch for node %s", nodeName)
		return nil, fmt.Errorf("failed to CreateTwoWayMergePatch for node %q: %v", nodeName, err)
	}
	return patchBytes, nil
}

func (rer *NodeResource) CheckResourceIsolation(resourceIsolation bool) (bool, error) {
	node, err := rer.GetNode()
	if err != nil {
		logs.Error(err, "get node: %s failed", rer.NodeName)
		return false, err
	}
	value, ok := node.Labels[consts.ResourceIsolationLabel]
	if !ok {
		return resourceIsolation, nil
	}
	logs.Info("find label: %s=%s exist on node: %s", consts.ResourceIsolationLabel, value, rer.NodeName)
	if value == "true" {
		return true, nil
	} else if value == "false" {
		return false, nil
	}
	err = fmt.Errorf("the value of node label: %s must be bool type, but is: %v", consts.ResourceIsolationLabel, value)
	logs.Error(err)
	return false, err
}

func (rer *NodeResource) PatchGCUMemoryMapToNode(deviceCapacityMap map[string]int) error {
	content, err := json.Marshal(deviceCapacityMap)
	if err != nil {
		logs.Error(err, "json marshal device memory map: %v failed", deviceCapacityMap)
		return err
	}
	node, err := rer.GetNode()
	if err != nil {
		logs.Error(err, "get node: %s from cluster failed", rer.NodeName)
		return err
	}
	node.Annotations[rer.Config.ReplaceResource(consts.GCUSharedCapacity)] = string(content)
	// patchCount := node.Annotations[rer.Config.ReplaceDomain(consts.PatchCount)]
	// if patchCount == "" {
	// 	logs.Info("init device memory map patch count: 0")
	// 	patchCount = "0"
	// }
	// count, err := strconv.Atoi(patchCount)
	// if err != nil {
	// 	logs.Error(err, "patch count: %s must be int type", patchCount)
	// 	return err
	// }
	// count = count + 1
	// node.Annotations[rer.Config.ReplaceDomain(consts.PatchCount)] = fmt.Sprintf("%d", count)
	_, err = rer.ClientSet.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
	if err != nil {
		logs.Error(err, "patch node: %s annotations to cluster failed, content: %v", rer.NodeName, node.Annotations)
		return err
	}
	logs.Info("patch node: %s annotations success, content: %v", rer.NodeName, node.Annotations)
	return nil
}

func (rer *NodeResource) GetNode() (*v1.Node, error) {
	return rer.ClientSet.CoreV1().Nodes().Get(context.TODO(), rer.NodeName, metav1.GetOptions{})
}

func (rer *NodeResource) PatchNode(patchBytes []byte) (*v1.Node, error) {
	return rer.ClientSet.CoreV1().Nodes().PatchStatus(context.TODO(), rer.NodeName, patchBytes)
}
