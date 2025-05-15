// Copyright 2022 Enflame. All Rights Reserved.
package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"

	"gcushare-device-plugin/pkg/consts"
	"gcushare-device-plugin/pkg/device"
	"gcushare-device-plugin/pkg/informer"
	"gcushare-device-plugin/pkg/kube"
	"gcushare-device-plugin/pkg/logs"
	"gcushare-device-plugin/pkg/structs"
	"gcushare-device-plugin/pkg/utils"
)

type PodResource struct {
	ClusterResource
	Informer *informer.PodInformer
}

var podAssignedGCUTime string

func NewPodResource(clusterResource *ClusterResource) *PodResource {
	podAssignedGCUTime = clusterResource.Config.ReplaceResource(consts.PodAssignedGCUTime)
	return &PodResource{
		ClusterResource: *clusterResource,
		Informer: informer.NewPodInformer(clusterResource.NodeName, clusterResource.ResourceName,
			clusterResource.Config, clusterResource.ClientSet),
	}
}

// pick up the gpushare pod with assigned status is false, and
func (rer *PodResource) GetCandidatePods(queryKubelet bool, client *kube.KubeletClient) ([]*v1.Pod, error) {
	candidatePods := []*v1.Pod{}
	allPods, err := rer.getPendingPodsInNode(queryKubelet, client)
	if err != nil {
		return candidatePods, err
	}
	for i := range allPods {
		if !rer.filterAssumedPod(&allPods[i]) {
			candidatePods = append(candidatePods, &allPods[i])
		}
	}
	return makePodOrderdByAge(candidatePods), nil
}

// determine if the pod is GCU share pod, and is already assumed but not assigned
func (rer *PodResource) filterAssumedPod(pod *v1.Pod) bool {
	// 1. Check if it's for GCU share
	if rer.GetGCUMemoryFromPodResource(pod) <= 0 {
		logs.Warn("pod(name: %s, uuid: %s) is not gcushare pod, skip it", pod.Name, pod.UID)
		return true
	}

	// 2. Check if it already has assume time
	if _, ok := pod.ObjectMeta.Annotations[rer.Config.ReplaceResource(consts.PodAssignedGCUTime)]; !ok {
		logs.Warn("pod(name: %s, uuid: %s) is not scheduled finish by scheduler plugin, skip it", pod.Name, pod.UID)
		return true
	}

	// // 3. Check if it has been assigned already
	// if assigned, ok := pod.ObjectMeta.Annotations[consts.PodHasAssignedGCU]; ok {
	// 	if assigned == "false" {
	// 		logs.Info("Found GCUSharedAssumed assumed pod %s in namespace %s", pod.Name, pod.Namespace)
	// 		assumed = true
	// 	} else {
	// 		logs.Warn("GCU assigned Flag for pod %s exists in namespace %s and its assigned status is %s, so it's not GCUSharedAssumed assumed pod",
	// 			pod.Name, pod.Namespace, assigned)
	// 	}
	// } else {
	// 	logs.Warn("No GCU assigned Flag for pod %s in namespace %s, so it's not GCUSharedAssumed assumed pod",
	// 		pod.Name, pod.Namespace)
	// }
	// return assumed
	return false
}

func (rer *PodResource) getPendingPodsInNode(queryKubelet bool, kubeletClient *kube.KubeletClient) ([]v1.Pod, error) {
	pods := []v1.Pod{}
	podIDMap := map[types.UID]bool{}
	var podList *v1.PodList
	var err error
	if queryKubelet {
		podList, err = rer.getPodListsByQueryKubelet(kubeletClient)
		if err != nil {
			return nil, err
		}
	} else {
		podList, err = rer.getPodListsByListAPIServer()
		if err != nil {
			return nil, err
		}
	}

	for _, pod := range podList.Items {
		if pod.Spec.NodeName != rer.NodeName {
			logs.Warn("pod(name: %s, uuid: %s) is not assigned to node %s as expected, it's placed on node %s, skip it",
				pod.Name, pod.UID, rer.NodeName, pod.Spec.NodeName)
		} else {
			logs.Info("find pod(name: %s, uuid: %s) on node %s and status is %s", pod.Name, pod.UID, rer.NodeName,
				pod.Status.Phase)
			if _, ok := podIDMap[pod.UID]; !ok {
				pods = append(pods, pod)
				podIDMap[pod.UID] = true
			}
		}

	}
	return pods, nil
}

func (rer *PodResource) getPodListsByQueryKubelet(kubeletClient *kube.KubeletClient) (*v1.PodList, error) {
	podList, err := getPendingPodList(kubeletClient)
	for i := 0; i < consts.MaxRetryTimes && err != nil; i++ {
		logs.Warn("failed to get pending pod list, retry time: %d", i+1)
		podList, err = getPendingPodList(kubeletClient)
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		logs.Error(err, "not found from kubelet pods api, start try to list apiserver")
		podList, err = rer.getPodListsByListAPIServer()
		if err != nil {
			return nil, err
		}
	}
	return podList, nil
}

func (rer *PodResource) getPodListsByListAPIServer() (*v1.PodList, error) {
	selector := fields.SelectorFromSet(fields.Set{"spec.nodeName": rer.NodeName, "status.phase": "Pending"})
	podList, err := rer.ClientSet.CoreV1().Pods(v1.NamespaceAll).List(context.TODO(), metav1.ListOptions{
		FieldSelector: selector.String(),
		LabelSelector: labels.Everything().String(),
	})
	for i := 0; i < consts.MaxRetryTimes && err != nil; i++ {
		logs.Error(err, "failed to get pending pod list by apiserver, retry time: %d", i+1)
		podList, err = rer.ClientSet.CoreV1().Pods(v1.NamespaceAll).List(context.TODO(), metav1.ListOptions{
			FieldSelector: selector.String(),
			LabelSelector: labels.Everything().String(),
		})
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		logs.Error(err, "get cluster pod list by apiserver failed in %d seconds", consts.MaxRetryTimes)
		return nil, err
	}
	return podList, nil
}

func getPendingPodList(kubeletClient *kube.KubeletClient) (*v1.PodList, error) {
	podList, err := kubeletClient.GetNodePodsList()
	if err != nil {
		return nil, err
	}

	resultPodList := &v1.PodList{}
	for _, metaPod := range podList.Items {
		if metaPod.Status.Phase == v1.PodPending {
			resultPodList.Items = append(resultPodList.Items, metaPod)
		}
	}

	if len(resultPodList.Items) == 0 {
		err := fmt.Errorf("not found pending pod when execute allocate gcu device")
		logs.Error(err)
		return nil, err
	}
	return resultPodList, nil
}

// Get GCU Memory of the Pod
func (rer *PodResource) GetGCUMemoryFromPodResource(pod *v1.Pod) uint {
	var total uint
	containers := pod.Spec.Containers
	for _, container := range containers {
		if val, ok := container.Resources.Limits[v1.ResourceName(rer.ClusterResource.ResourceName)]; ok {
			total += uint(val.Value())
		}
	}
	return total
}

func (rer *PodResource) PodExistContainerCanBind(containerReq int, pod *v1.Pod) (bool, string, error) {
	assignedMap := map[string]structs.AllocateRecord{}
	assignedContainers, ok := pod.Annotations[consts.PodAssignedContainers]
	if ok {
		if err := json.Unmarshal([]byte(assignedContainers), &assignedMap); err != nil {
			logs.Error(err, "unmarshal assigned containers to map of pod(name: %s, uuid: %s) failed, detail: %s",
				pod.Name, pod.UID, assignedContainers)
			return false, "", err
		}
	}
	selectedContainerName := ""
	requestResourceCount := 0
	kubeletAllocatedCount := 0
	for _, container := range pod.Spec.Containers {
		if val, ok := container.Resources.Limits[v1.ResourceName(rer.ClusterResource.ResourceName)]; ok {
			requestResourceCount += 1
			if assigned, ok := assignedMap[container.Name]; ok && assigned.KubeletAllocated != nil && *assigned.KubeletAllocated {
				kubeletAllocatedCount += 1
				continue
			}
			if int(val.Value()) == containerReq {
				if selectedContainerName == "" {
					logs.Info("container: %s of pod(name: %s, uuid: %s) will allocate shared gcu: %d", container.Name,
						pod.Name, pod.UID, containerReq)
					selectedContainerName = container.Name
				} else {
					logs.Info("container: %s of pod(name: %s, uuid: %s) request %s: %d, but has not assign up to now",
						container.Name, pod.Name, pod.UID, rer.ResourceName, containerReq)
				}
			}
		}
	}
	if selectedContainerName == "" {
		logs.Warn("pod(name: %s, uuid: %s) no container can allocated shared gcu: %d, skip this pod",
			pod.Name, pod.UID, containerReq)
		return false, "", nil
	}

	alloc, ok := assignedMap[selectedContainerName]
	if !ok {
		alloc = structs.AllocateRecord{}
	}
	boolTrue := true
	alloc.KubeletAllocated = &boolTrue
	assignedMap[selectedContainerName] = alloc

	values, err := json.Marshal(assignedMap)
	if err != nil {
		logs.Error(err, "marshal assigned containers to string of pod(name: %s, uuid: %s) failed, detail: %v",
			pod.Name, pod.UID, assignedMap)
		return false, "", err
	}
	allContainersAssigned := pod.Annotations[rer.Config.ReplaceResource(consts.PodHasAssignedGCU)]
	if requestResourceCount-kubeletAllocatedCount == 1 {
		logs.Info("check all containers of pod(name: %s, uuid: %s) assigned %s success", pod.Name, pod.UID, rer.ResourceName)
		allContainersAssigned = "true"
	}
	if err := rer.patchPodAnnotationsAssignedContainers(string(values), allContainersAssigned, pod); err != nil {
		return false, "", err
	}
	if alloc.InstanceID == nil {
		return true, "", nil
	}
	return true, *alloc.InstanceID, nil
}

func (rer *PodResource) patchPodAnnotationsAssignedContainers(values, allContainersAssigned string, pod *v1.Pod) error {
	patchAnnotations := map[string]interface{}{
		"metadata": map[string]map[string]string{"annotations": {
			rer.Config.ReplaceResource(consts.PodHasAssignedGCU): allContainersAssigned,
			consts.PodAssignedContainers:                         values,
		}}}
	patchedAnnotationBytes, err := json.Marshal(patchAnnotations)
	if err != nil {
		logs.Error(err, "marshal patch annotations failed")
		return err
	}
	_, err = rer.ClientSet.CoreV1().Pods(pod.Namespace).Patch(context.TODO(), pod.Name,
		types.StrategicMergePatchType, patchedAnnotationBytes, metav1.PatchOptions{})
	if err != nil {
		logs.Error(err, "patch pod(name: %s, uuid: %s) annotations to cluster failed %s",
			pod.Name, pod.UID, string(patchedAnnotationBytes))
		return err
	}
	logs.Info("patch pod(name: %s, uuid: %s) annotations: %s success", pod.Name, pod.UID,
		string(patchedAnnotationBytes))
	return nil
}

// make the pod ordered by GCU assumed time
func makePodOrderdByAge(pods []*v1.Pod) []*v1.Pod {
	newPodList := make(orderedPodByAssumeTime, 0, len(pods))
	for _, v := range pods {
		newPodList = append(newPodList, v)
	}
	sort.Sort(newPodList)
	logs.Info("candidatePods list after order by assigned time: %s", utils.ConvertToString([]*v1.Pod(newPodList)))
	return []*v1.Pod(newPodList)
}

type orderedPodByAssumeTime []*v1.Pod

func (rer orderedPodByAssumeTime) Len() int {
	return len(rer)
}

func (rer orderedPodByAssumeTime) Less(i, j int) bool {
	return GetAssumeTimeFromPodAnnotation(rer[i]) <= GetAssumeTimeFromPodAnnotation(rer[j])
}

func (rer orderedPodByAssumeTime) Swap(i, j int) {
	rer[i], rer[j] = rer[j], rer[i]
}

// get assumed timestamp
func GetAssumeTimeFromPodAnnotation(pod *v1.Pod) (assumeTime uint64) {
	if assumeTimeStr, ok := pod.ObjectMeta.Annotations[podAssignedGCUTime]; ok {
		u64, err := strconv.ParseUint(assumeTimeStr, 10, 64)
		if err != nil {
			logs.Error(err, "Failed to parse assume Timestamp %s", assumeTimeStr)
		} else {
			assumeTime = u64
		}
	}
	return assumeTime
}

func (rer *PodResource) GetGCUIDFromPodAnnotation(pod *v1.Pod) (string, error) {
	value, ok := pod.ObjectMeta.Annotations[rer.Config.ReplaceResource(consts.PodAssignedGCUID)]
	if !ok || value == "" {
		err := fmt.Errorf("pod(name: %s, uuid: %s) has not been assigned device", pod.Name, pod.UID)
		logs.Error(err)
		return "", err
	}
	return value, nil
}

func (rer *PodResource) GetDisabledCardInfo(deviceInfo map[string]device.DeviceInfo) (map[string]int, error) {
	disabledCardInfo := map[string]int{}
	for _, pod := range rer.Informer.GCUSharedPods {
		assignedID, ok := pod.Annotations[rer.Config.ReplaceResource(consts.PodAssignedGCUID)]
		if !ok {
			continue
		}
		checkPath := filepath.Join(consts.PCIDevicePath, deviceInfo[assignedID].BusID)
		if utils.FileIsExist(checkPath) {
			continue
		}
		podUsed := rer.GetGCUMemoryFromPodResource(pod)
		logs.Warn("pod(name: %s, uuid: %s) is using disabled card: %s, used: %d", pod.Name, pod.UID, assignedID, podUsed)
		disabledCardInfo[assignedID] += int(podUsed)
	}
	if len(disabledCardInfo) > 0 {
		logs.Warn("found disabled card used info: %v", disabledCardInfo)
	}
	return disabledCardInfo, nil
}
