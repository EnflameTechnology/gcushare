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
	"gcushare-scheduler-plugin/pkg/utils"
)

type filterResult struct {
	isDRSPod     bool
	resourceName string
	configmaps   map[resources.NodeName]*v1.ConfigMap
	allAvailable map[resources.NodeName]map[resources.DeviceID]int
}

type GCUShareSchedulerPlugin struct {
	mu                 *sync.Mutex
	sharedResourceName string
	drsResourceName    string
	clientset          *kubernetes.Clientset
	config             *config.Config
	filterCache        map[k8sTypes.UID]filterResult

	podResource  *resources.PodResource
	nodeResource *resources.NodeResource
}

var _ framework.FilterPlugin = &GCUShareSchedulerPlugin{}
var _ framework.PreBindPlugin = &GCUShareSchedulerPlugin{}
var _ framework.BindPlugin = &GCUShareSchedulerPlugin{}

func (gsp *GCUShareSchedulerPlugin) Name() string {
	return consts.SchedulerPluginName
}

func (fr filterResult) addFilterResult(nodeName string, pod *v1.Pod, nodeAvailableGCUs map[resources.DeviceID]int, cm *v1.ConfigMap) {
	if fr.isDRSPod {
		logs.Info("add drs pod(name: %s, uuid: %s) filter result to filter cache", pod.Name, pod.UID)
		fr.configmaps[resources.NodeName(nodeName)] = cm
	} else {
		logs.Info("add gcushare pod(name: %s, uuid: %s) filter result of node: %s to filter cache", pod.Name, pod.UID, nodeName)
		fr.allAvailable[resources.NodeName(nodeName)] = nodeAvailableGCUs
	}
}

func (gsp *GCUShareSchedulerPlugin) addFilterCache(status *framework.Status, pod *v1.Pod,
	nodeName string, nodeAvailableGCUs *map[resources.DeviceID]int, cm *v1.ConfigMap) {
	gsp.mu.Lock()
	defer gsp.mu.Unlock()
	if status.Code() != framework.Success {
		logs.Warn("pod(name: %s, uuid: %s) filter failed, need not to add to filter cache", pod.Name, pod.UID)
		return
	}
	isDRSPod := gsp.podResource.IsDrsGCUsharingPod(pod)
	if isDRSPod {
		gsp.filterCache[pod.UID] = filterResult{
			isDRSPod:     isDRSPod,
			resourceName: gsp.drsResourceName,
			configmaps:   map[resources.NodeName]*v1.ConfigMap{},
		}
	} else {
		gsp.filterCache[pod.UID] = filterResult{
			isDRSPod:     isDRSPod,
			resourceName: gsp.sharedResourceName,
			allAvailable: map[resources.NodeName]map[resources.DeviceID]int{},
		}
	}
	gsp.filterCache[pod.UID].addFilterResult(nodeName, pod, *nodeAvailableGCUs, cm)
}

func (gsp *GCUShareSchedulerPlugin) clearFilterCache(caller string, pod *v1.Pod) {
	gsp.mu.Lock()
	defer gsp.mu.Unlock()
	if gsp.filterCache[pod.UID].isDRSPod {
		for _, configMap := range gsp.filterCache[pod.UID].configmaps {
			gsp.clearConfigMap(configMap.Name, configMap.Namespace)
		}
	}
	delete(gsp.filterCache, pod.UID)
	logs.Info("%s clear filter cache for pod(name: %s, uuid: %s)", caller, pod.Name, pod.UID)
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
	var nodeAvailableGCUs map[resources.DeviceID]int
	var configMap v1.ConfigMap
	defer func() {
		gsp.addFilterCache(status, pod, nodeInfo.Node().Name, &nodeAvailableGCUs, &configMap)
	}()
	podName := pod.Name
	nodeName := nodeInfo.Node().Name
	logs.Info("----------filter node: %s for pod(name: %s, uuid: %s)----------", nodeName, podName, pod.UID)
	isDRSPod := gsp.podResource.IsDrsGCUsharingPod(pod)
	if !gsp.podResource.IsGCUsharingPod(pod) && !isDRSPod {
		err := fmt.Errorf("pod(name: %s, uuid: %s) does not request %v, please do not specify the scheduler name: %s",
			podName, pod.UID, []string{gsp.sharedResourceName, gsp.drsResourceName}, consts.SchedulerName)
		logs.Error(err)
		return framework.NewStatus(framework.Error, err.Error())
	}
	if gsp.podResource.IsGCUsharingPod(pod) && isDRSPod {
		err := fmt.Errorf("it is not allowed for pod(name: %s, uuid: %s) request %s and %s at the same time",
			podName, pod.UID, gsp.sharedResourceName, gsp.drsResourceName)
		logs.Error(err)
		return framework.NewStatus(framework.Error, err.Error())
	}
	if !gsp.nodeResource.IsGCUSharingNode(isDRSPod, nodeInfo.Node()) {
		msg := fmt.Sprintf("node: %s is not gcushare node, skip it", nodeInfo.Node().Name)
		logs.Warn(msg)
		return framework.NewStatus(framework.Unschedulable, msg)
	}
	if !gsp.previousPodHasBind(pod) {
		return framework.NewStatus(framework.Error, "wait previous pods bind finish")
	}
	if isDRSPod {
		return gsp.filterNodeForDRS(pod, nodeInfo, &configMap)
	}
	return gsp.filterNode(pod, nodeInfo, &nodeAvailableGCUs)
}

func (gsp *GCUShareSchedulerPlugin) previousPodHasBind(pod *v1.Pod) bool {
	maxRetryTimes := 5
	for i := 0; i < maxRetryTimes; i++ {
		_, isSelf := gsp.filterCache[pod.UID]
		if len(gsp.filterCache) == 0 || (len(gsp.filterCache) == 1 && isSelf) {
			return true
		}
		logs.Warn("pod(name: %s, uuid: %s) in-place wait previous pods bind finish, retry times: %d, detail: %v",
			pod.Name, pod.UID, i, gsp.filterCache)
		time.Sleep(1 * time.Second)
	}
	msg := fmt.Sprintf("previous pods bind not finish in max retry time: %d, ", maxRetryTimes)
	msg += fmt.Sprintf("pod(name: %s, uuid: %s) cannot be scheduled and will be re-joined to the scheduling queue",
		pod.Name, pod.UID)
	logs.Error(msg)
	return false
}

func (gsp *GCUShareSchedulerPlugin) filterNode(pod *v1.Pod, nodeInfo *framework.NodeInfo,
	nodeAvailableGCUs *map[resources.DeviceID]int) *framework.Status {
	var err error
	*nodeAvailableGCUs, err = gsp.nodeResource.GetNodeAvailableGCUs(nodeInfo)
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
	cmForCache *v1.ConfigMap) (status *framework.Status) {
	configmapName := fmt.Sprintf("%s.%s.%s.%s.configmap", pod.Name, pod.Namespace,
		strings.Split(string(pod.UID), "-")[0], nodeInfo.Node().Name)
	configmapNamespace := pod.Namespace
	defer func() {
		if status.Code() != framework.Success {
			gsp.clearConfigMap(configmapName, configmapNamespace)
		}
	}()
	gcusharePods := []structs.GCUSharePod{}
	for _, podInfo := range nodeInfo.Pods {
		// Need to record the list of pods that use the shared GCU as non-drs
		if !gsp.podResource.IsGCUsharingPod(podInfo.Pod) {
			continue
		}
		podDeleted, assignedID := gsp.podResource.GetPodAssignedDeviceIDWithRetry(podInfo)
		if podDeleted {
			continue
		}
		if assignedID == "" {
			err := fmt.Errorf("assignedID not found in pod(name: %s, uuid: %s) annotations with maxRetryTimes: %d",
				podInfo.Pod.Name, podInfo.Pod.UID, consts.MaxRetryTimes)
			logs.Error(err)
			return framework.NewStatus(framework.Error, err.Error())
		}
		gcusharePods = append(gcusharePods, structs.GCUSharePod{
			Name:       podInfo.Pod.Name,
			Namespace:  podInfo.Pod.Namespace,
			Uuid:       podInfo.Pod.UID,
			AssignedID: assignedID,
		})
	}
	if err := gsp.createConfigMap(nodeInfo.Node().Name, configmapName, pod, gcusharePods); err != nil {
		return framework.NewStatus(framework.Error, err.Error())
	}
	logs.Info("wait for device-plugin to confirm whether the node: %s has an available drs device", nodeInfo.Node().Name)
	cm, status := gsp.waitConfigmapAssignDRSDevice(nodeInfo.Node().Name, configmapName, pod)
	if status.Code() != framework.Success {
		return status
	}
	*cmForCache = *cm
	return framework.NewStatus(framework.Success)
}

func (gsp *GCUShareSchedulerPlugin) createConfigMap(nodeName, configmapName string, pod *v1.Pod,
	gcusharePods []structs.GCUSharePod) error {
	schedulerRecord := structs.SchedulerRecord{
		Filter: &structs.FilterSpec{
			GCUSharePods: gcusharePods,
			Containers:   gsp.podResource.InitPodAssignedContainers(pod),
		},
	}
	schedulerRecordBytes, err := json.Marshal(schedulerRecord)
	if err != nil {
		logs.Error(err, "json marshal failed, content: %v", schedulerRecord)
		return err
	}
	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmapName,
			Namespace: pod.Namespace,
			Labels: map[string]string{
				consts.ConfigMapNode:  nodeName,
				consts.ConfigMapOwner: consts.DRSSchedulerName,
			},
		},
		Data: map[string]string{
			consts.SchedulerRecord: string(schedulerRecordBytes),
		},
	}
	logs.Debug("configmap for pod(name: %s, uuid: %s) will be created, detail: %s",
		pod.Name, pod.UID, utils.JsonMarshalIndent(configMap))
	if _, err := gsp.clientset.CoreV1().ConfigMaps(configMap.Namespace).Create(context.TODO(), configMap,
		metav1.CreateOptions{}); err != nil {
		logs.Error(err, "create configMap: %s failed", configmapName)
		return err
	}
	logs.Info("create configmap: %s for pod(name: %s, uuid: %s) success", configMap.Name, pod.Name, pod.UID)
	return nil
}

func (gsp *GCUShareSchedulerPlugin) waitConfigmapAssignDRSDevice(nodeName, configmapName string, pod *v1.Pod) (
	*v1.ConfigMap, *framework.Status) {
	for retryTime := 0; retryTime < consts.MaxRetryTimes; retryTime++ {
		time.Sleep(1 * time.Second)
		cm, err := gsp.clientset.CoreV1().ConfigMaps(pod.Namespace).Get(context.TODO(), configmapName,
			metav1.GetOptions{})
		if err != nil {
			logs.Error(err, "get configmap: %s failed, retry times: %d", configmapName, retryTime)
			return nil, framework.NewStatus(framework.Error, err.Error())
		}
		result := cm.Data[consts.SchedulerRecord]
		schedulerRecord := &structs.SchedulerRecord{}
		if err := json.Unmarshal([]byte(result), schedulerRecord); err != nil {
			logs.Error(err, "json unmarshal failed, detail: %s", result)
			return nil, framework.NewStatus(framework.Error, err.Error())
		}
		status := schedulerRecord.Filter.Status
		switch status {
		case "":
			logs.Warn("configmap: %s handler for filter not work finish, retry times: %d", configmapName, retryTime)
			continue
		case consts.StateSuccess:
			logs.Info("configmap: %s filter node: %s for pod(name: %s, uuid: %s) success, detail: %s", configmapName,
				nodeName, pod.Name, pod.UID, utils.ConvertToString(*schedulerRecord.Filter))
			return cm, framework.NewStatus(framework.Success)
		case consts.StateError:
			err := fmt.Errorf("configmap: %s filter node: %s for pod(name: %s, uuid: %s) failed: %s", configmapName,
				nodeName, pod.Name, pod.UID, schedulerRecord.Filter.Message)
			logs.Error(err)
			return nil, framework.NewStatus(framework.Error, err.Error())
		case consts.StateUnschedulable:
			msg := fmt.Sprintf("configmap: %s filter node: %s for pod(name: %s, uuid: %s) failed: %s", configmapName,
				nodeName, pod.Name, pod.UID, schedulerRecord.Filter.Message)
			logs.Warn(msg)
			return nil, framework.NewStatus(framework.Unschedulable, msg)
		default:
			err = fmt.Errorf("internal error: configmap: %s filter node: %s for pod(name: %s, uuid: %s) failed: unexpected status: %s",
				configmapName, nodeName, pod.Name, pod.UID, status)
			logs.Error(err)
			return nil, framework.NewStatus(framework.Error, err.Error())
		}
	}
	err := fmt.Errorf("wait for configmap: %s handler for filter of pod(name: %s, uuid: %s) timeout with maxRetryTimes: %d",
		configmapName, pod.Name, pod.UID, consts.MaxRetryTimes)
	logs.Error(err)
	return nil, framework.NewStatus(framework.Error, err.Error())
}

func (gsp *GCUShareSchedulerPlugin) clearConfigMap(name, namespace string) {
	for retryTime := 0; retryTime < consts.MaxRetryTimes; retryTime++ {
		if err := gsp.clientset.CoreV1().ConfigMaps(namespace).Delete(context.TODO(), name,
			metav1.DeleteOptions{}); err != nil && !k8serr.IsNotFound(err) {
			logs.Error(err, "delete configmap(name: %s, namespace: %s) failed, retry times: %d",
				name, namespace, retryTime)
			time.Sleep(3 * time.Second)
			continue
		}
		logs.Info("delete configmap(name: %s, namespace: %s) success", name, namespace)
		return
	}
	logs.Error("delete configmap(name: %s, namespace: %s) timeout, retry times: %d", name, namespace, consts.MaxRetryTimes)
}

func (gsp *GCUShareSchedulerPlugin) PreBind(ctx context.Context, state *framework.CycleState, pod *v1.Pod,
	nodeName string) (status *framework.Status) {
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("Internal error! Recovered from panic: %v", r)
			logs.Error(errMsg)
			status = framework.NewStatus(framework.Error, errMsg)
		}
	}()
	defer func() {
		if status.Code() != framework.Success {
			logs.Warn("PreBind will clear filter cache for pod(name: %s, uuid: %s) with status: %s",
				pod.Name, pod.UID, status.Code())
			gsp.clearFilterCache("PreBind", pod)
		}
	}()
	logs.Info("----------preBind pod(name: %s, uuid: %s) to node: %s----------", pod.Name, pod.UID, nodeName)
	if gsp.filterCache[pod.UID].isDRSPod {
		cm := gsp.filterCache[pod.UID].configmaps[resources.NodeName(nodeName)]
		cm, status := gsp.patchConfigmapCreateDRS(cm, pod)
		if status.Code() != framework.Success {
			return status
		}
		if err := gsp.patchAssignedResult(cm, pod); err != nil {
			return framework.NewStatus(framework.Error, err.Error())
		}
	} else {
		if err := gsp.allocatedDeviceID(nodeName, pod); err != nil {
			return framework.NewStatus(framework.Error, err.Error())
		}
	}
	return framework.NewStatus(framework.Success, "")
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
		logs.Warn("Bind will clear filter cache for pod(name: %s, uuid: %s) with status: %s",
			pod.Name, pod.UID, status.Code())
		gsp.clearFilterCache("Bind", pod)
	}()
	logs.Info("----------bind pod(name: %s, uuid: %s) to node: %s----------", pod.Name, pod.UID, nodeName)

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

func (gsp *GCUShareSchedulerPlugin) patchConfigmapCreateDRS(cm *v1.ConfigMap, pod *v1.Pod) (*v1.ConfigMap, *framework.Status) {
	schedulerRecordString := cm.Data[consts.SchedulerRecord]
	schedulerRecord := &structs.SchedulerRecord{}
	if err := json.Unmarshal([]byte(schedulerRecordString), schedulerRecord); err != nil {
		logs.Error(err, "json unmarshal failed, detail: %s", schedulerRecordString)
		return nil, framework.NewStatus(framework.Error, err.Error())
	}
	schedulerRecord.PreBind = &structs.PreBindSpec{
		Containers: schedulerRecord.Filter.Containers,
	}
	schedulerRecordBytes, err := json.Marshal(schedulerRecord)
	if err != nil {
		logs.Error(err, "json marshal failed, content: %v", *schedulerRecord)
		return nil, framework.NewStatus(framework.Error, err.Error())
	}

	patchData := map[string]interface{}{
		"data": map[string]string{
			consts.SchedulerRecord: string(schedulerRecordBytes),
		},
	}
	patchBytes, err := json.Marshal(patchData)
	if err != nil {
		logs.Error(err, "json marshal failed, content: %v", patchData)
		return nil, framework.NewStatus(framework.Error, err.Error())
	}

	if _, err := gsp.clientset.CoreV1().ConfigMaps(cm.Namespace).Patch(
		context.TODO(),
		cm.Name,
		k8sTypes.MergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	); err != nil {
		logs.Error(err, "patch configmap: %s failed", cm.Name)
		return nil, framework.NewStatus(framework.Error, err.Error())
	}
	logs.Info("preBind exetend points patch configmap: %s success, content: %s", cm.Name, string(patchBytes))
	logs.Info("preBind exetend points wait configmap: %s create drs...", cm.Name)
	return gsp.waitDRSCreateFinish(cm, pod)
}

func (gsp *GCUShareSchedulerPlugin) waitDRSCreateFinish(cm *v1.ConfigMap, pod *v1.Pod) (*v1.ConfigMap, *framework.Status) {
	for retryTime := 0; retryTime < consts.MaxRetryTimes; retryTime++ {
		time.Sleep(1 * time.Second)
		cm, err := gsp.clientset.CoreV1().ConfigMaps(cm.Namespace).Get(context.TODO(), cm.Name, metav1.GetOptions{})
		if err != nil {
			logs.Error(err, "get configmap: %s from cluster failed, retry times: %d", cm.Name, retryTime)
			return nil, framework.NewStatus(framework.Error, err.Error())
		}
		result := cm.Data[consts.SchedulerRecord]
		schedulerRecord := &structs.SchedulerRecord{}
		if err := json.Unmarshal([]byte(result), schedulerRecord); err != nil {
			logs.Error(err, "json unmarshal failed, detail: %s", result)
			return nil, framework.NewStatus(framework.Error, err.Error())
		}
		status := schedulerRecord.PreBind.Status
		switch status {
		case "":
			logs.Warn("configmap: %s handler for bind not work finish, retry times: %d", retryTime)
			continue
		case consts.StateSuccess:
			logs.Info("configmap: %s create drs for pod(name: %s, uuid: %s) success, detail: %s", cm.Name,
				pod.Name, pod.UID, utils.ConvertToString(*schedulerRecord.PreBind))
			return cm, framework.NewStatus(framework.Success)
		case consts.StateError:
			err := fmt.Errorf("configmap: %s create drs for pod(name: %s, uuid: %s) failed: %s", cm.Name,
				pod.Name, pod.UID, schedulerRecord.PreBind.Message)
			logs.Error(err)
			return nil, framework.NewStatus(framework.Error, err.Error())
		case consts.StateUnschedulable:
			msg := fmt.Sprintf("configmap: %s create drs for pod(name: %s, uuid: %s) failed: %s", cm.Name,
				pod.Name, pod.UID, schedulerRecord.Filter.Message)
			logs.Warn(msg)
			return nil, framework.NewStatus(framework.Unschedulable, msg)
		default:
			err = fmt.Errorf("internal error: configmap: %s create drs for pod(name: %s, uuid: %s) failed: unexpected status: %s",
				cm.Name, pod.Name, pod.UID, status)
			logs.Error(err)
			return nil, framework.NewStatus(framework.Error, err.Error())
		}
	}
	err := fmt.Errorf("wait for configmap: %s create drs for pod(name: %s, uuid: %s) timeout with maxRetryTimes: %d",
		cm.Name, pod.Name, pod.UID, consts.MaxRetryTimes)
	logs.Error(err)
	return nil, framework.NewStatus(framework.Error, err.Error())
}

func (gsp *GCUShareSchedulerPlugin) patchAssignedResult(cm *v1.ConfigMap, pod *v1.Pod) error {
	result := cm.Data[consts.SchedulerRecord]
	schedulerRecord := &structs.SchedulerRecord{}
	if err := json.Unmarshal([]byte(result), schedulerRecord); err != nil {
		logs.Error(err, "json unmarshal failed, detail: %s", result)
		return err
	}
	podRequest := gsp.podResource.GetRequestFromPodResource(pod)
	drsDevice, err := json.Marshal(schedulerRecord.Filter.Device)
	if err != nil {
		logs.Error(err, "json marshal failed, detail: %v", schedulerRecord.Filter.Device)
		return err
	}
	assignedContainers, err := json.Marshal(schedulerRecord.PreBind.Containers)
	if err != nil {
		logs.Error(err, "json marshal failed, detail: %v", schedulerRecord.PreBind.Containers)
		return err
	}
	patchAnnotations := map[string]interface{}{
		"metadata": map[string]map[string]string{"annotations": {
			consts.DRSAssignedDevice:                              string(drsDevice),
			consts.PodAssignedContainers:                          string(assignedContainers),
			gsp.config.ReplaceResource(consts.PodAssignedGCUID):   schedulerRecord.Filter.Device.Minor,
			gsp.config.ReplaceResource(consts.PodRequestGCUSize):  fmt.Sprintf("%d", podRequest),
			gsp.config.ReplaceResource(consts.PodHasAssignedGCU):  "false",
			gsp.config.ReplaceResource(consts.PodAssignedGCUTime): fmt.Sprintf("%d", time.Now().UnixNano()),
		}}}
	if err := gsp.patchPodAnnotations(pod, patchAnnotations); err != nil {
		return err
	}
	return nil
}

func (gsp *GCUShareSchedulerPlugin) allocatedDeviceID(nodeName string, pod *v1.Pod) error {
	nodeAvailableGCUs, ok := gsp.filterCache[pod.UID].allAvailable[resources.NodeName(nodeName)]
	if !ok || len(nodeAvailableGCUs) == 0 {
		err := fmt.Errorf("internal error: node: %s has no available %s", nodeName, gsp.sharedResourceName)
		logs.Error(err)
		return err
	}
	podRequest := gsp.podResource.GetRequestFromPodResource(pod)
	logs.Info("pod(name: %s, uuid: %s) request %s: %d, available %s: %v", pod.Name, pod.UID, gsp.sharedResourceName,
		podRequest, gsp.sharedResourceName, nodeAvailableGCUs)

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
		return err
	}
	logs.Info("gcushare scheduler plugin assigned device '%s' to pod(name: %s, uuid: %s)",
		selectedID, pod.Name, pod.UID)
	patchAnnotations := map[string]interface{}{
		"metadata": map[string]map[string]string{"annotations": {
			gsp.config.ReplaceResource(consts.PodAssignedGCUID):   selectedID,
			gsp.config.ReplaceResource(consts.PodRequestGCUSize):  fmt.Sprintf("%d", podRequest),
			gsp.config.ReplaceResource(consts.PodHasAssignedGCU):  "false",
			gsp.config.ReplaceResource(consts.PodAssignedGCUTime): fmt.Sprintf("%d", time.Now().UnixNano()),
		}}}
	if err := gsp.patchPodAnnotations(pod, patchAnnotations); err != nil {
		return err
	}
	return nil
}

func (gsp *GCUShareSchedulerPlugin) patchPodAnnotations(pod *v1.Pod, patchAnnotations map[string]interface{}) error {
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
		logs.Info("bind patch pod(name: %s, uuid: %s) annotations: %s success", pod.Name, pod.UID,
			string(patchedAnnotationBytes))
		return nil
	}
	err = fmt.Errorf("bind patch pod(name: %s, uuid: %s) annotations failed with max retry time: %d, content: %s",
		pod.Name, pod.UID, consts.MaxRetryTimes, string(patchedAnnotationBytes))
	logs.Error(err)
	return err
}
