// Copyright (c) 2025, ENFLAME INC.  All rights reserved.
package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sTypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/scheduler/framework"

	"gcushare-scheduler-plugin/pkg/config"
	"gcushare-scheduler-plugin/pkg/consts"
	"gcushare-scheduler-plugin/pkg/logs"
	"gcushare-scheduler-plugin/pkg/resources"
	"gcushare-scheduler-plugin/pkg/structs"
)

type filterResult struct {
	allAvailable map[resources.NodeName]map[structs.Minor]int
	drsCapacity  map[resources.NodeName]structs.DRSGCUCapacity
}

type GCUShareSchedulerPlugin struct {
	mu                 *sync.Mutex
	sharedResourceName string
	drsResourceName    string
	clientset          *kubernetes.Clientset
	config             *config.Config
	filterCache        map[k8sTypes.UID]filterResult
	reserveCache       map[k8sTypes.UID]map[string]string

	podResource  *resources.PodResource
	nodeResource *resources.NodeResource
}

var _ framework.FilterPlugin = &GCUShareSchedulerPlugin{}
var _ framework.ReservePlugin = &GCUShareSchedulerPlugin{}

func (gsp *GCUShareSchedulerPlugin) Name() string {
	return consts.SchedulerPluginName
}

func (fr filterResult) addFilterResult(nodeName string, pod *v1.Pod, nodeAvailableGCUs map[structs.Minor]int,
	nodeDeviceSpec structs.DRSGCUCapacity) {
	logs.Info("add pod(name: %s, uuid: %s) filter result of node: %s to filter cache", pod.Name, pod.UID, nodeName)
	fr.allAvailable[resources.NodeName(nodeName)] = nodeAvailableGCUs
	if nodeDeviceSpec.Devices != nil {
		fr.drsCapacity[resources.NodeName(nodeName)] = nodeDeviceSpec
	}
}

func (gsp *GCUShareSchedulerPlugin) addFilterCache(status *framework.Status, pod *v1.Pod,
	nodeName string, nodeAvailableGCUs *map[structs.Minor]int, nodeDeviceSpec *structs.DRSGCUCapacity) {
	if status.Code() != framework.Success {
		logs.Warn("pod(name: %s, uuid: %s) filter failed, need not to add to filter cache", pod.Name, pod.UID)
		return
	}
	gsp.mu.Lock()
	defer gsp.mu.Unlock()
	if _, exist := gsp.filterCache[pod.UID]; !exist {
		logs.Info("init filterCache for pod(name: %s, uuid: %s)", pod.Name, pod.UID)
		gsp.filterCache[pod.UID] = filterResult{
			allAvailable: map[resources.NodeName]map[structs.Minor]int{},
			drsCapacity:  map[resources.NodeName]structs.DRSGCUCapacity{},
		}
	}
	gsp.filterCache[pod.UID].addFilterResult(nodeName, pod, *nodeAvailableGCUs, *nodeDeviceSpec)
}

func (gsp *GCUShareSchedulerPlugin) clearFilterCache(pod *v1.Pod) {
	gsp.mu.Lock()
	defer gsp.mu.Unlock()
	delete(gsp.filterCache, pod.UID)
}

func (gsp *GCUShareSchedulerPlugin) Filter(ctx context.Context, state *framework.CycleState, pod *v1.Pod,
	nodeInfo *framework.NodeInfo) (status *framework.Status) {
	var nodeAvailableGCUs map[structs.Minor]int
	var nodeDeviceSpec structs.DRSGCUCapacity
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("Internal error! Recovered from panic: %v", r)
			logs.Error(errMsg)
			status = framework.NewStatus(framework.Error, errMsg)
		}
		gsp.addFilterCache(status, pod, nodeInfo.Node().Name, &nodeAvailableGCUs, &nodeDeviceSpec)
	}()

	podName := pod.Name
	nodeName := nodeInfo.Node().Name
	logs.Info("----------filter node: %s for pod(name: %s, uuid: %s)----------", nodeName, podName, pod.UID)
	isDRSPod := gsp.podResource.IsDrsGCUsharingPod(pod)
	if !gsp.podResource.IsGCUsharingPod(pod) && !isDRSPod {
		err := fmt.Errorf("pod(name: %s, uuid: %s) does not request %v, please do not specify the scheduler name: %s",
			podName, pod.UID, []string{gsp.sharedResourceName, gsp.drsResourceName}, consts.SchedulerName)
		logs.Error(err)
		return framework.NewStatus(framework.Unschedulable, err.Error())
	}
	if gsp.podResource.IsGCUsharingPod(pod) && isDRSPod {
		err := fmt.Errorf("it is not allowed for pod(name: %s, uuid: %s) request %s and %s at the same time",
			podName, pod.UID, gsp.sharedResourceName, gsp.drsResourceName)
		logs.Error(err)
		return framework.NewStatus(framework.Unschedulable, err.Error())
	}
	if !gsp.nodeResource.IsGCUSharingNode(isDRSPod, nodeInfo.Node()) {
		msg := fmt.Sprintf("node: %s is not gcushare node, skip it", nodeInfo.Node().Name)
		logs.Warn(msg)
		return framework.NewStatus(framework.Unschedulable, msg)
	}
	if isDRSPod {
		return gsp.filterNodeForDRS(pod, nodeInfo, &nodeAvailableGCUs, &nodeDeviceSpec)
	}
	return gsp.filterNode(pod, nodeInfo, &nodeAvailableGCUs)
}

func (gsp *GCUShareSchedulerPlugin) filterNode(pod *v1.Pod, nodeInfo *framework.NodeInfo,
	nodeAvailableGCUs *map[structs.Minor]int) *framework.Status {
	var err error
	*nodeAvailableGCUs, err = gsp.nodeResource.GetNodeAvailableGCUs(nodeInfo, gsp.getReserveCache)
	if err != nil {
		return framework.NewStatus(framework.Error, err.Error())
	}
	logs.Info("node: %s available %s: %v", nodeInfo.Node().Name, gsp.sharedResourceName, *nodeAvailableGCUs)
	podRequest := gsp.podResource.GetRequestFromPodResource(pod)
	for _, available := range *nodeAvailableGCUs {
		if available >= podRequest {
			logs.Info("pod(name: %s, uuid: %s) request %s: %d, and node: %s is available",
				pod.Name, pod.UID, gsp.sharedResourceName, podRequest, nodeInfo.Node().Name)
			return framework.NewStatus(framework.Success)
		}
	}
	msg := fmt.Sprintf("pod(name: %s, uuid: %s) request %s: %d, but it is insufficient of node: %s",
		pod.Name, pod.UID, gsp.sharedResourceName, podRequest, nodeInfo.Node().Name)
	logs.Warn(msg)
	return framework.NewStatus(framework.Unschedulable, msg)
}

func (gsp *GCUShareSchedulerPlugin) filterNodeForDRS(pod *v1.Pod, nodeInfo *framework.NodeInfo,
	nodeAvailableDRS *map[structs.Minor]int, nodeDeviceSpec *structs.DRSGCUCapacity) (status *framework.Status) {
	nodeDevice, ok := nodeInfo.Node().Annotations[consts.GCUDRSCapacity]
	if !ok {
		err := fmt.Errorf("node: %s missing drs gcu capacity annotations: %v", nodeInfo.Node().Name, nodeInfo.Node().Annotations)
		logs.Error(err)
		return framework.NewStatus(framework.Unschedulable, err.Error())
	}
	deviceCapacity := &structs.DRSGCUCapacity{}
	if err := json.Unmarshal([]byte(nodeDevice), deviceCapacity); err != nil {
		logs.Error(err, "json unmarshal failed, content: %s", nodeDevice)
		return framework.NewStatus(framework.Error, err.Error())
	}
	if err := gsp.allExpectedProfileExist(deviceCapacity.Profiles, pod); err != nil {
		return framework.NewStatus(framework.Unschedulable, err.Error())
	}
	*nodeDeviceSpec = *deviceCapacity
	var err error
	*nodeAvailableDRS, err = gsp.nodeResource.GetNodeAvailableDRS(nodeInfo, deviceCapacity.Devices, gsp.getReserveCache)
	if err != nil {
		return framework.NewStatus(framework.Error, err.Error())
	}
	logs.Info("node: %s available %s: %v", nodeInfo.Node().Name, gsp.drsResourceName, *nodeAvailableDRS)
	podRequest := gsp.podResource.GetRequestDrsPodResource(pod)
	for _, available := range *nodeAvailableDRS {
		if available >= podRequest {
			logs.Info("pod(name: %s, uuid: %s) request %s: %d, and node: %s is available",
				pod.Name, pod.UID, gsp.drsResourceName, podRequest, nodeInfo.Node().Name)
			return framework.NewStatus(framework.Success)
		}
	}
	msg := fmt.Sprintf("pod(name: %s, uuid: %s) request %s: %d, but it is insufficient of node: %s",
		pod.Name, pod.UID, gsp.drsResourceName, podRequest, nodeInfo.Node().Name)
	logs.Warn(msg)
	return framework.NewStatus(framework.Unschedulable, msg)
}

func (gsp *GCUShareSchedulerPlugin) allExpectedProfileExist(profileMap map[structs.ProfileName]structs.ProfileID, pod *v1.Pod) error {
	profileNamePrefixMap := gsp.getProfileNamePrefixMap(profileMap)
	podExpectedProfile := map[string]int{}
	for _, container := range pod.Spec.Containers {
		if val, ok := container.Resources.Limits[v1.ResourceName(gsp.drsResourceName)]; ok {
			podExpectedProfile[fmt.Sprintf("%dg", int(val.Value()))] += 1
		}
	}
	for profileNamePrefix := range podExpectedProfile {
		if _, ok := profileNamePrefixMap[profileNamePrefix]; !ok {
			err := fmt.Errorf("pod(name: %s, uuid: %s) expected profile name prefix: %s not found in profile map: %v",
				pod.Name, pod.UID, profileNamePrefix, profileMap)
			logs.Error(err)
			return err
		}
	}
	return nil
}

func (gsp *GCUShareSchedulerPlugin) getProfileNamePrefixMap(
	profileMap map[structs.ProfileName]structs.ProfileID) map[string]structs.ProfileName {
	profileNamePrefix := make(map[string]structs.ProfileName, len(profileMap))
	for profileName := range profileMap {
		profileNamePrefix[strings.Split(string(profileName), ".")[0]] = profileName
	}
	logs.Info("profile name prefix map: %v", profileNamePrefix)
	return profileNamePrefix
}

func (gsp *GCUShareSchedulerPlugin) Reserve(ctx context.Context, state *framework.CycleState, pod *v1.Pod,
	nodeName string) (status *framework.Status) {
	logs.Info("----------reserve pod(name: %s, uuid: %s) on node: %s----------", pod.Name, pod.UID, nodeName)
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("Internal error! Recovered from panic: %v", r)
			logs.Error(errMsg)
			status = framework.NewStatus(framework.Error, errMsg)
		}
		if status.Code() == framework.Success {
			logs.Info("Reserve will clear filter cache for pod(name: %s, uuid: %s) with status: %s",
				pod.Name, pod.UID, status.Code())
			gsp.clearFilterCache(pod)
		}
	}()

	selectedID, err := gsp.allocatedDeviceID(nodeName, pod)
	if err != nil {
		return framework.NewStatus(framework.Error, err.Error())
	}
	if err := gsp.patchAssignedResult(pod, selectedID, nodeName); err != nil {
		return framework.NewStatus(framework.Error, err.Error())
	}
	logs.Info("reserve pod(name: %s, uuid: %s) on node: %s success", pod.Name, pod.UID, nodeName)
	return framework.NewStatus(framework.Success)
}

func (gsp *GCUShareSchedulerPlugin) Unreserve(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) {
	logs.Info("----------unreserve pod(name: %s, uuid: %s) on node: %s----------", pod.Name, pod.UID, nodeName)
	logs.Warn("The pod Reserve failed to initiate this method, but it does not need to be handled for the time being")
}

func (gsp *GCUShareSchedulerPlugin) allocatedDeviceID(nodeName string, pod *v1.Pod) (string, error) {
	isDrs := gsp.podResource.IsDrsGCUsharingPod(pod)
	resourceName := gsp.sharedResourceName
	podRequest := gsp.podResource.GetRequestFromPodResource(pod)
	if isDrs {
		resourceName = gsp.drsResourceName
		podRequest = gsp.podResource.GetRequestDrsPodResource(pod)
	}
	gsp.mu.Lock()
	nodeAvailableGCUs, ok := gsp.filterCache[pod.UID].allAvailable[resources.NodeName(nodeName)]
	gsp.mu.Unlock()
	if !ok || len(nodeAvailableGCUs) == 0 {
		err := fmt.Errorf("internal error: node: %s has no available %s", nodeName, resourceName)
		logs.Error(err)
		return "", err
	}
	logs.Info("pod(name: %s, uuid: %s) request %s: %d, available %s: %v", pod.Name, pod.UID, resourceName,
		podRequest, resourceName, nodeAvailableGCUs)

	selectedID := ""
	selectedIDMemory := 0
	for deviceID, available := range nodeAvailableGCUs {
		if available >= podRequest {
			if selectedID == "" || selectedIDMemory > available {
				selectedID = string(deviceID)
				selectedIDMemory = available
			}
		}
	}
	if selectedID == "" {
		err := fmt.Errorf("internal error! node: %s could not bind pod(name: %s, uuid: %s) with available %s: %v",
			nodeName, pod.Name, pod.UID, gsp.sharedResourceName, nodeAvailableGCUs)
		logs.Error(err)
		return "", err
	}
	logs.Info("gcushare scheduler plugin assigned device '%s' to pod(name: %s, uuid: %s)",
		selectedID, pod.Name, pod.UID)
	return selectedID, nil
}

func (gsp *GCUShareSchedulerPlugin) patchAssignedResult(pod *v1.Pod, selectedID string, nodeName string) error {
	podRequest := gsp.podResource.GetRequestFromPodResource(pod)
	if gsp.podResource.IsDrsGCUsharingPod(pod) {
		podRequest = gsp.podResource.GetRequestDrsPodResource(pod)
	}
	patchAnnotations := map[string]string{
		gsp.config.ReplaceResource(consts.PodAssignedGCUMinor): selectedID,
		gsp.config.ReplaceResource(consts.PodRequestGCUSize):   fmt.Sprintf("%d", podRequest),
		gsp.config.ReplaceResource(consts.PodHasAssignedGCU):   "false",
		gsp.config.ReplaceResource(consts.PodAssignedGCUTime):  fmt.Sprintf("%d", time.Now().UnixNano()),
	}
	if gsp.podResource.IsDrsGCUsharingPod(pod) {
		deviceSpec := gsp.filterCache[pod.UID].drsCapacity[resources.NodeName(nodeName)]
		for _, device := range deviceSpec.Devices {
			if device.Minor == selectedID {
				patchAnnotations[consts.PodAssignedGCUIndex] = device.Index
				break
			}
		}
		profileNamePrefixMap := gsp.getProfileNamePrefixMap(deviceSpec.Profiles)
		assignedContainers := map[string]structs.AllocateRecord{}
		for _, container := range pod.Spec.Containers {
			if val, ok := container.Resources.Limits[v1.ResourceName(gsp.drsResourceName)]; ok {
				profileName := string(profileNamePrefixMap[fmt.Sprintf("%dg", int(val.Value()))])
				assignedContainers[container.Name] = structs.AllocateRecord{
					KubeletAllocated: &[]bool{false}[0],
					Request:          &[]int{int(val.Value())}[0],
					ProfileName:      &profileName,
					ProfileID:        &[]string{string(deviceSpec.Profiles[structs.ProfileName(profileName)])}[0],
				}
			}
		}
		assignedContainersBytes, err := json.Marshal(assignedContainers)
		if err != nil {
			logs.Error(err, "json marshal failed, detail: %v", assignedContainers)
			return err
		}
		patchAnnotations[consts.PodAssignedContainers] = string(assignedContainersBytes)
	}
	if err := gsp.patchPodAnnotations(pod, patchAnnotations); err != nil {
		return err
	}
	gsp.setReserveCache(pod, patchAnnotations)
	return nil
}

/*
This function is used in scenarios where the update of the pod informer cache lags behind,
providing the annotations cache of the previous pod for the next pod entering the filter extension point
*/
func (gsp *GCUShareSchedulerPlugin) setReserveCache(pod *v1.Pod, patchAnnotations map[string]string) {
	gsp.mu.Lock()
	defer gsp.mu.Unlock()
	gsp.reserveCache = make(map[k8sTypes.UID]map[string]string)
	gsp.reserveCache[pod.UID] = patchAnnotations
	logs.Info("clear reserve cache and set reserve cache for pod(name: %s, uuid: %s), annotations: %v",
		pod.Name, pod.UID, patchAnnotations)
}

func (gsp *GCUShareSchedulerPlugin) getReserveCache(podUID k8sTypes.UID) (map[string]string, bool) {
	gsp.mu.Lock()
	defer gsp.mu.Unlock()
	reserveCache, ok := gsp.reserveCache[podUID]
	return reserveCache, ok
}

func (gsp *GCUShareSchedulerPlugin) patchPodAnnotations(pod *v1.Pod, annotations map[string]string) error {
	patchAnnotations := map[string]interface{}{
		"metadata": map[string]map[string]string{"annotations": annotations}}
	patchedAnnotationBytes, err := json.Marshal(patchAnnotations)
	if err != nil {
		logs.Error(err, "marshal patch annotations failed")
		return err
	}
	for retryTime := 0; retryTime < consts.MaxRetryTimes; retryTime++ {
		_, err = gsp.clientset.CoreV1().Pods(pod.Namespace).Patch(context.TODO(), pod.Name,
			k8sTypes.StrategicMergePatchType, patchedAnnotationBytes, metav1.PatchOptions{})
		if err != nil && !k8serr.IsNotFound(err) {
			logs.Error(err, "patch pod(name: %s, uuid: %s) annotations to cluster failed, content: %s",
				pod.Name, pod.UID, string(patchedAnnotationBytes))
			time.Sleep(3 * time.Second)
			continue
		}
		logs.Info("patch pod(name: %s, uuid: %s) annotations: %s success", pod.Name, pod.UID,
			string(patchedAnnotationBytes))
		return nil
	}
	err = fmt.Errorf("patch pod(name: %s, uuid: %s) annotations failed with max retry time: %d, content: %s",
		pod.Name, pod.UID, consts.MaxRetryTimes, string(patchedAnnotationBytes))
	logs.Error(err)
	return err
}
