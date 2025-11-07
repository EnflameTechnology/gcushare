// Copyright 2022 Enflame. All Rights Reserved.
package resource

import (
	"context"
	"encoding/json"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"

	"gcushare-device-plugin/pkg/consts"
	"gcushare-device-plugin/pkg/device"
	"gcushare-device-plugin/pkg/drs"
	"gcushare-device-plugin/pkg/logs"
	"gcushare-device-plugin/pkg/smi"
	"gcushare-device-plugin/pkg/structs"
	"gcushare-device-plugin/pkg/utils"
)

type NodeResource struct {
	ClusterResource *ClusterResource
	DRSProfiles     map[smi.ProfileName]smi.ProfileID
}

type deviceSpec struct {
	Index    string `json:"index"`
	Minor    string `json:"minor"`
	Capacity int    `json:"capacity"`
}

type DRSGCUCapacity struct {
	Devices  []deviceSpec                      `json:"devices"`
	Profiles map[smi.ProfileName]smi.ProfileID `json:"profiles"`
}

type DRSOwner struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	UUID      string `json:"uuid"`
}

func NewNodeResource(clusterResource *ClusterResource, drsEnabled bool) *NodeResource {
	if drsEnabled {
		profiles, err := getDRSProfiles(clusterResource.NodeName, clusterResource.ClientSet)
		if err != nil {
			logs.Error(err, "get drs profiles failed")
			return nil
		}
		return &NodeResource{
			ClusterResource: clusterResource,
			DRSProfiles:     profiles,
		}
	}
	return &NodeResource{
		ClusterResource: clusterResource,
	}
}

func (rer *NodeResource) PatchGCUCountToNode(gcuCount int) error {
	node, err := rer.GetNode()
	if err != nil {
		logs.Error(err, "get node %s info from cluster failed", rer.ClusterResource.NodeName)
		return err
	}

	resourceCount := v1.ResourceName(rer.ClusterResource.Config.ReplaceResource(consts.CountName))
	if val, ok := node.Status.Capacity[resourceCount]; ok {
		if val.Value() == int64(gcuCount) {
			logs.Info("node %s need not to update Capacity %s", rer.ClusterResource.NodeName, resourceCount)
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
	logs.Info("node %s capacity: %s, allocatable: %s", rer.ClusterResource.NodeName, utils.ConvertToString(patchNode.Status.Capacity),
		utils.ConvertToString(patchNode.Status.Allocatable))
	return nil
}

// PatchNodeStatus patches node status.
func (rer *NodeResource) PatchNodeStatus(oldNode *v1.Node, newNode *v1.Node) (*v1.Node, error) {
	patchBytes, err := preparePatchBytesforNodeStatus(rer.ClusterResource.NodeName, oldNode, newNode)
	if err != nil {
		return nil, err
	}
	// updatedNode, err := rer.ClientSet.CoreV1().Nodes().Patch(context.TODO(), rer.NodeName,
	// 	k8sType.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	updatedNode, err := rer.PatchNode(patchBytes)
	if err != nil {
		logs.Error(err, "failed to patch status %s for node %s", string(patchBytes), rer.ClusterResource.NodeName)
		return nil, fmt.Errorf("failed to patch status %q for node %q: %v", patchBytes, rer.ClusterResource.NodeName, err)
	}
	logs.Info("patch node %s status success, patch content: %s", rer.ClusterResource.NodeName,
		string(patchBytes))
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

func (rer *NodeResource) PatchSharedGCUCapacityToNode(deviceCapacityMap map[string]int) error {
	content, err := json.Marshal(deviceCapacityMap)
	if err != nil {
		logs.Error(err, "json marshal device memory map: %v failed", deviceCapacityMap)
		return err
	}
	annotationKey := rer.ClusterResource.Config.ReplaceResource(consts.GCUSharedCapacity)
	annotations := map[string]string{
		annotationKey: string(content),
	}
	return rer.patchNodeAnnotations(annotations)
}

func (rer *NodeResource) patchNodeAnnotations(annotations map[string]string) error {
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": annotations,
		},
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		logs.Error(err, "json marshal failed, content: %v", patch)
		return err
	}
	if _, err := rer.ClusterResource.ClientSet.CoreV1().Nodes().Patch(
		context.TODO(),
		rer.ClusterResource.NodeName,
		types.MergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	); err != nil {
		logs.Error(err, "patch node: %s annotations to cluster failed, content: %s", rer.ClusterResource.NodeName, string(patchBytes))
		return err
	}
	logs.Info("patch node: %s annotations success, content: %s", rer.ClusterResource.NodeName, string(patchBytes))
	return nil
}

func (rer *NodeResource) PatchDRSGCUCapacityToNode(deviceCapacityMap map[string]int, deviceInfo map[string]device.DeviceInfo) error {
	devices := []deviceSpec{}
	for minor, capacity := range deviceCapacityMap {
		devices = append(devices, deviceSpec{
			Index:    deviceInfo[minor].Index,
			Minor:    minor,
			Capacity: capacity,
		})
	}

	drsGCUCapacity := DRSGCUCapacity{
		Devices:  devices,
		Profiles: rer.DRSProfiles,
	}
	content, err := json.Marshal(drsGCUCapacity)
	if err != nil {
		logs.Error(err, "json marshal drs gcu capacity: %v failed", drsGCUCapacity)
		return err
	}
	return rer.patchNodeAnnotations(map[string]string{
		rer.ClusterResource.Config.ReplaceResource(consts.GCUDRSCapacity): string(content),
	})
}

func getDRSProfiles(nodeName string, clientSet *kubernetes.Clientset) (map[smi.ProfileName]smi.ProfileID, error) {
	instances, err := smi.ListInstance("")
	if err != nil {
		return nil, err
	}
	if len(instances) > 0 {
		unKnownDRSList, err := checkDRSOwner(instances, nodeName, clientSet)
		if err != nil {
			logs.Error(err, "check drs owner failed")
			return nil, err
		}
		if len(unKnownDRSList) > 0 {
			err := fmt.Errorf("drs resource already exist when gcushare device plugin was launched. "+
				"The following DRS instances: %v is not used by the drs pod created by gcushare", unKnownDRSList)
			logs.Error(err)
			return nil, err
		}
		logs.Warn("drs resource already exist when gcushare device plugin was launched. " +
			"This might lead to some unexpected problems")
	}
	lines, err := smi.Smi()
	if err != nil {
		return nil, err
	}
	profileTemplate, err := drs.GetProfileTemplate(lines)
	if err != nil {
		return nil, err
	}
	profiles := make(map[smi.ProfileName]smi.ProfileID)
	for profileName, profileSpec := range profileTemplate {
		profiles[profileName] = smi.ProfileID(profileSpec.ProfileID)
	}
	logs.Info("get drs profiles from smi success, detail: %v", profiles)
	return profiles, nil
}

func checkDRSOwner(instances []smi.Instance, nodeName string, clientSet *kubernetes.Clientset) ([]string, error) {
	unKnownDRSList := []string{}
	podList, err := clientSet.CoreV1().Pods(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	})
	if err != nil {
		logs.Error(err, "get pod list failed")
		return nil, err
	}

	DRSListHasOwner := make(map[string]DRSOwner)
	for _, pod := range podList.Items {
		if pod.Annotations[consts.PodAssignedGCUIndex] == "" {
			continue
		}
		assignedContainers := new(map[string]structs.AllocateRecord)
		if err := json.Unmarshal([]byte(pod.Annotations[consts.PodAssignedContainers]), assignedContainers); err != nil {
			logs.Error(err, "unmarshal pod %s assigned containers failed", pod.Name)
			return nil, err
		}
		for _, container := range *assignedContainers {
			if container.InstanceUUID != nil && *container.InstanceUUID != "" {
				DRSListHasOwner[*container.InstanceUUID] = DRSOwner{
					Name:      pod.Name,
					Namespace: pod.Namespace,
					UUID:      string(pod.UID),
				}
			}
		}
	}

	for _, instance := range instances {
		if owner, ok := DRSListHasOwner[instance.UUID]; ok {
			logs.Info("drs instance %s is owned by pod(name: %s, namespace: %s, uuid: %s)",
				instance.UUID, owner.Name, owner.Namespace, owner.UUID)
		} else {
			unKnownDRSList = append(unKnownDRSList, instance.UUID)
		}
	}
	return unKnownDRSList, nil
}

func (rer *NodeResource) GetNode() (*v1.Node, error) {
	return rer.ClusterResource.ClientSet.CoreV1().Nodes().Get(context.TODO(), rer.ClusterResource.NodeName, metav1.GetOptions{})
}

func (rer *NodeResource) PatchNode(patchBytes []byte) (*v1.Node, error) {
	return rer.ClusterResource.ClientSet.CoreV1().Nodes().PatchStatus(context.TODO(), rer.ClusterResource.NodeName, patchBytes)
}
