// Copyright (c) 2025, ENFLAME INC.  All rights reserved.
package resources

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sTypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/scheduler/framework"

	"gcushare-scheduler-plugin/pkg/consts"
	"gcushare-scheduler-plugin/pkg/logs"
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

func (res *PodResource) GetUsedSharedGCU(podInfoList []*framework.PodInfo,
	getReserveCache func(podUID k8sTypes.UID) (map[string]string, bool)) (map[string]int, map[string]struct{}, error) {
	usedSharedGCU := map[string]int{}
	deviceUsedByDRS := map[string]struct{}{}
	for _, podInfo := range podInfoList {
		isDrsPod := res.IsDrsGCUsharingPod(podInfo.Pod)
		isGCUSharingPod := res.IsGCUsharingPod(podInfo.Pod)
		if !isDrsPod && !isGCUSharingPod {
			continue
		}
		assignedMinor, skip, err := res.GetAssignedMinorWithRetry(podInfo.Pod, getReserveCache)
		if err != nil {
			return nil, nil, err
		}
		if skip {
			continue
		}
		if isDrsPod {
			logs.Warn("device: %s is using by drs pod(name: %s, uuid: %s)", assignedMinor, podInfo.Pod.Name, podInfo.Pod.UID)
			if _, ok := deviceUsedByDRS[assignedMinor]; !ok {
				deviceUsedByDRS[assignedMinor] = struct{}{}
			}
		} else {
			request := res.GetRequestFromPodResource(podInfo.Pod)
			logs.Info("device: %s is using by gcushare pod(name: %s, uuid: %s, used: %d)", assignedMinor, podInfo.Pod.Name,
				podInfo.Pod.UID, request)
			usedSharedGCU[assignedMinor] += request
		}
	}
	return usedSharedGCU, deviceUsedByDRS, nil
}

func (res *PodResource) GetUsedDRSGCU(podInfoList []*framework.PodInfo,
	getReserveCache func(podUID k8sTypes.UID) (map[string]string, bool)) (
	map[string]int, map[string]struct{}, error) {
	usedDRSGCU := map[string]int{}
	deviceUsedBySharedGCU := map[string]struct{}{}
	for _, podInfo := range podInfoList {
		isDrsPod := res.IsDrsGCUsharingPod(podInfo.Pod)
		isGCUSharingPod := res.IsGCUsharingPod(podInfo.Pod)
		if !isDrsPod && !isGCUSharingPod {
			continue
		}
		assignedMinor, skip, err := res.GetAssignedMinorWithRetry(podInfo.Pod, getReserveCache)
		if err != nil {
			return nil, nil, err
		}
		if skip {
			continue
		}
		if isDrsPod {
			request := res.GetRequestDrsPodResource(podInfo.Pod)
			logs.Info("device: %s is using by drs pod(name: %s, uuid: %s, used: %d)", assignedMinor, podInfo.Pod.Name,
				podInfo.Pod.UID, request)
			usedDRSGCU[assignedMinor] += request
		} else {
			logs.Warn("device: %s is using by gcushare pod(name: %s, uuid: %s)", assignedMinor, podInfo.Pod.Name, podInfo.Pod.UID)
			if _, ok := deviceUsedBySharedGCU[assignedMinor]; !ok {
				deviceUsedBySharedGCU[assignedMinor] = struct{}{}
			}
		}
	}
	return usedDRSGCU, deviceUsedBySharedGCU, nil
}

func (res *PodResource) GetAssignedMinorWithRetry(pod *v1.Pod, getReserveCache func(podUID k8sTypes.UID) (map[string]string, bool)) (
	string, bool, error) {
	assignedMinor, ok := pod.Annotations[res.Config.ReplaceResource(consts.PodAssignedGCUMinor)]
	if ok {
		return assignedMinor, false, nil
	}
	logs.Warn("assignedMinor not found in pod(name: %s, uuid: %s) annotations: %v. "+
		"This might be caused by the untimely update of the pod informer cache. "+
		"An attempt will be made to obtain the pod annotation from the reserve cache",
		pod.Name, pod.UID, pod.Annotations)
	reserveCache, ok := getReserveCache(pod.UID)
	if ok {
		assignedMinor, ok = reserveCache[res.Config.ReplaceResource(consts.PodAssignedGCUMinor)]
		if !ok {
			err := fmt.Errorf("internal error! reserve cache found but assignedMinor not found for "+
				"pod(name: %s, uuid: %s), reserve cache: %v	", pod.Name, pod.UID, reserveCache)
			logs.Error(err)
			return "", false, err
		}
		logs.Info("assignedMinor found in reserve cache for pod(name: %s, uuid: %s), assignedMinor: %s",
			pod.Name, pod.UID, assignedMinor)
		return assignedMinor, false, nil
	}

	logs.Warn("reserve cache not found for pod(name: %s, uuid: %s), try get it from api-server", pod.Name, pod.UID)
	for i := 0; i < 10; i++ {
		newPod, err := res.Clientset.CoreV1().Pods(pod.Namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
		if err != nil {
			logs.Error(err, "get pod(name: %s, uuid: %s) from cluster failed, retry after 0.1s", pod.Name, pod.UID)
			continue
		}
		if newPod.UID != pod.UID {
			logs.Warn("pod(name: %s, uuid: %s) has been deleted, skip it", pod.Name, pod.UID)
			return "", true, nil
		}
		assignedMinor, ok = newPod.Annotations[res.Config.ReplaceResource(consts.PodAssignedGCUMinor)]
		if ok {
			logs.Info("assignedMinor found in pod(name: %s, uuid: %s), assignedMinor: %s",
				pod.Name, pod.UID, assignedMinor)
			return assignedMinor, false, nil
		}
		logs.Warn("assignedMinor not found in pod(name: %s, uuid: %s) annotations: %v, retry after 0.1s",
			pod.Name, pod.UID, newPod.Annotations)
		time.Sleep(100 * time.Millisecond)
	}
	err := fmt.Errorf("assignedMinor not found in pod(name: %s, uuid: %s) annotations: %v in 10 retries",
		pod.Name, pod.UID, pod.Annotations)
	logs.Error(err)
	return "", false, err
}
