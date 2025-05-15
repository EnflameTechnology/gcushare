// Copyright (c) 2025, ENFLAME INC.  All rights reserved.
package resources

import (
	"context"
	"fmt"
	"strconv"
	"time"

	v1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/scheduler/framework"

	"gcushare-scheduler-plugin/pkg/consts"
	"gcushare-scheduler-plugin/pkg/logs"
	"gcushare-scheduler-plugin/pkg/structs"
)

type PodResource BaseResource

func NewPodResource(baseResource *BaseResource) *PodResource {
	return (*PodResource)(baseResource)
}

func (res *PodResource) IsGCUsharingPod(pod *v1.Pod) bool {
	return res.GetRequestFromPodResource(pod) > 0
}

// GetGCUMemoryFromPodResource gets GCU Memory of the Pod
func (res *PodResource) GetRequestFromPodResource(pod *v1.Pod) int {
	total := 0
	for _, container := range pod.Spec.Containers {
		if val, ok := container.Resources.Limits[v1.ResourceName(res.SharedResourceName)]; ok {
			total += int(val.Value())
		}
	}
	return total
}

func (res *PodResource) IsDrsGCUsharingPod(pod *v1.Pod) bool {
	return res.GetRequestDrsPodResource(pod) > 0
}

// GetGCUMemoryFromPodResource gets GCU Memory of the Pod
func (res *PodResource) GetRequestDrsPodResource(pod *v1.Pod) int {
	total := 0
	for _, container := range pod.Spec.Containers {
		if val, ok := container.Resources.Limits[v1.ResourceName(res.DRSResourceName)]; ok {
			total += int(val.Value())
		}
	}
	return total
}

func (res *PodResource) GetUsedGCUs(podInfoList []*framework.PodInfo) (map[string]int, error) {
	nodeUsedMemory := map[string]int{}
	for _, podInfo := range podInfoList {
		if !res.IsGCUsharingPod(podInfo.Pod) {
			continue
		}
		podDeleted, assignedID := res.GetPodAssignedDeviceIDWithRetry(podInfo)
		if podDeleted {
			continue
		}
		if assignedID == "" {
			err := fmt.Errorf("assignedID not found in pod(name: %s, uuid: %s) annotations with maxRetryTimes: %d",
				podInfo.Pod.Name, podInfo.Pod.UID, consts.MaxRetryTimes)
			logs.Error(err)
			return nil, err
		}

		podRequestGCU, ok := podInfo.Pod.Annotations[res.Config.ReplaceResource(consts.PodRequestGCUSize)]
		if !ok {
			err := fmt.Errorf("internal error: podRequestGCU not found in pod(name: %s, uuid: %s) annotations: %v",
				podInfo.Pod.Name, podInfo.Pod.UID, podInfo.Pod.Annotations)
			logs.Error(err)
			return nil, err
		}
		value, err := strconv.Atoi(podRequestGCU)
		if err != nil {
			logs.Error(err)
			return nil, err
		}
		logs.Info("gcu:%s is using by pod(name: %s, uuid: %s, used: %d)", assignedID, podInfo.Pod.Name,
			podInfo.Pod.UID, value)
		nodeUsedMemory[assignedID] += value
	}
	return nodeUsedMemory, nil
}

func (res *PodResource) GetPodAssignedDeviceIDWithRetry(podInfo *framework.PodInfo) (bool, string) {
	for retryTime := 0; retryTime < consts.MaxRetryTimes; retryTime++ {
		if assignedID := res.GetPodAssignedDeviceID(podInfo.Pod); assignedID != "" {
			return false, assignedID
		}
		time.Sleep(3 * time.Second)
		pod, err := res.Clientset.CoreV1().Pods(podInfo.Pod.Namespace).Get(context.TODO(),
			podInfo.Pod.Name, metav1.GetOptions{})
		if err != nil {
			if k8serr.IsNotFound(err) {
				logs.Warn("pod(name: %s, uuid: %s) has been deleted, skip it", podInfo.Pod.Name, podInfo.Pod.UID)
				return true, ""
			}
			logs.Error(err, "get pod(name: %s, uuid: %s) from cluster failed, retry time: %d",
				podInfo.Pod.Name, podInfo.Pod.UID, retryTime)
			continue
		}
		if podInfo.Pod.UID != pod.UID {
			logs.Warn("pod(name: %s, uuid: %s) has been recreat, skip it", podInfo.Pod.Name, podInfo.Pod.UID)
			return true, ""
		}
		logs.Warn("framework.PodInfo update pod(name: %s, uuid: %s)", podInfo.Pod.Name, podInfo.Pod.UID)
		podInfo.Pod = pod
	}
	return false, ""
}

func (res *PodResource) GetPodAssignedDeviceID(pod *v1.Pod) string {
	value, ok := pod.ObjectMeta.Annotations[res.Config.ReplaceResource(consts.PodAssignedGCUID)]
	if !ok {
		logs.Warn("pod(name: %s, uuid: %s) has not been assigned device", pod.Name, pod.UID)
		return ""
	}
	return value
}

func (res *PodResource) InitPodAssignedContainers(pod *v1.Pod) map[string]structs.AllocateRecord {
	result := make(map[string]structs.AllocateRecord, len(pod.Spec.Containers))
	for _, container := range pod.Spec.Containers {
		if val, ok := container.Resources.Limits[v1.ResourceName(res.DRSResourceName)]; ok && val.Value() > 0 {
			alloc := structs.AllocateRecord{}
			value := int(val.Value())
			alloc.Request = &value
			result[container.Name] = alloc
		}
	}
	return result
}
