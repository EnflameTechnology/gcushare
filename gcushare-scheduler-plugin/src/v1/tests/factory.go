// Copyright (c) 2025, ENFLAME INC.  All rights reserved.
package tests

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sTypes "k8s.io/apimachinery/pkg/types"

	"gcushare-scheduler-plugin/pkg/consts"
)

func NewPod(podName, podUID, deviceID, request string, limits ...int) *v1.Pod {
	containers := []v1.Container{}
	for _, limit := range limits {
		containers = append(containers, v1.Container{
			Resources: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					consts.ResourceName: *resource.NewQuantity(int64(limit), ""),
				},
			},
		})
	}
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        podName,
			UID:         k8sTypes.UID(podUID),
			Annotations: map[string]string{},
		},
		Spec: v1.PodSpec{
			SchedulerName: consts.SchedulerName,
			Containers:    containers,
		},
	}
	if deviceID != "" {
		pod.Annotations[consts.PodAssignedGCUID] = deviceID
	}
	if request != "" {
		pod.Annotations[consts.PodRequestGCUSize] = request
	}
	return pod
}

func NewNode(nodeName string, isGCUShareNode bool, deviceMemeryMap string, allocatable int) *v1.Node {
	node := &v1.Node{}
	node.SetName(nodeName)
	if isGCUShareNode {
		node.SetLabels(map[string]string{consts.GCUShareLabel: "true"})
	}
	if deviceMemeryMap != "" {
		node.SetAnnotations(map[string]string{consts.GCUSharedCapacity: deviceMemeryMap})
	}
	node.Status.Allocatable = v1.ResourceList{consts.ResourceName: *resource.NewQuantity(int64(allocatable), "")}
	return node
}

func IntPointer(x int) *int {
	return &x
}
