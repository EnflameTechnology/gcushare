// Copyright (c) 2025, ENFLAME INC.  All rights reserved.
package routers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"gcushare-scheduler-plugin/pkg/consts"
	"gcushare-scheduler-plugin/pkg/logs"
	"gcushare-scheduler-plugin/pkg/resources"
	"gcushare-scheduler-plugin/pkg/structs"
)

type Inspect struct {
	sharedResourceName string
	drsResourceName    string
	clientset          *kubernetes.Clientset
	podResource        *resources.PodResource
	nodeResource       *resources.NodeResource
}

type NodeInfo struct {
	Name         string             `json:"name"`
	TotalGCU     int                `json:"totalGCU,omitempty"`
	UsedGCU      *int               `json:"usedGCU,omitempty"`
	AvailableGCU *int               `json:"availableGCU,omitempty"`
	Devices      map[string]*Device `json:"devices,omitempty"`
	Error        string             `json:"error,omitempty"`
	Warn         []string           `json:"warn,omitempty"`
}

type Device struct {
	Virt         *string `json:"virt,omitempty"`
	TotalGCU     int     `json:"totalGCU,omitempty"`
	UsedGCU      *int    `json:"usedGCU,omitempty"`
	AvailableGCU *int    `json:"availableGCU,omitempty"`
	Pods         []*Pod  `json:"pods"`
}

type Pod struct {
	metav1.ObjectMeta `json:",inline"`
	UsedGCU           int                               `json:"usedGCU"`
	Phase             v1.PodPhase                       `json:"phase"`
	Containers        map[string]structs.AllocateRecord `json:"containers,omitempty"`
}

func (ins *Inspect) InspectRoute(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	nodeName := ps.ByName("nodename")
	if nodeName == "" {
		logs.Info("listen access url: %s, method: %s, request body:%v", consts.InspectNodeListRoute, "GET", r)
	} else {
		logs.Info("listen access url: %s, method: %s, request body:%v", consts.InspectNodeDetailRoute, "GET", r)
	}
	drs := false
	if r.URL.Query().Get("drs") == "true" {
		drs = true
	}
	result := ins.inspect(nodeName, drs)

	if resultBody, err := json.MarshalIndent(result, "", "  "); err != nil {
		logs.Error(err, "marshal inspect result:%v failed", result)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		errMsg := fmt.Sprintf("{'error':'%s'}\n", err.Error())
		w.Write([]byte(errMsg))
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resultBody = append(resultBody, '\n')
		w.Write(resultBody)
	}
}

func (ins *Inspect) inspect(nodeName string, drs bool) (result interface{}) {
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("Internal error! Recovered from panic: %v", r)
			logs.Error(errMsg)
			result = map[string]string{"error": errMsg}
		}
	}()
	if nodeName != "" {
		nodeDetail, err := ins.clientset.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil {
			logs.Error(err, "get node: %s from cluster failed", nodeName)
			return NodeInfo{Name: nodeName, Error: err.Error()}
		}
		if !ins.nodeResource.IsGCUSharingNode(drs, nodeDetail) {
			err := fmt.Errorf("node: %s is not the gcushare node", nodeName)
			logs.Error(err)
			return NodeInfo{Name: nodeName, Error: err.Error()}
		}
		return ins.buildNodeInfo(nodeDetail, drs)
	}
	nodeList, err := ins.clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logs.Error(err, "list cluster node list failed")
		return map[string]string{"error": err.Error()}
	}
	res := []NodeInfo{}
	for _, node := range nodeList.Items {
		if !ins.nodeResource.IsGCUSharingNode(drs, &node) {
			continue
		}
		res = append(res, ins.buildNodeInfo(&node, drs))
	}
	if len(res) == 0 {
		msg := "the gcushare nodes not found"
		logs.Error(msg)
		return map[string]string{"error": msg}
	}
	return res
}

func (ins *Inspect) buildNodeInfo(node *v1.Node, drs bool) NodeInfo {
	pods, err := ins.clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + node.Name,
	})
	if err != nil {
		logs.Error(err, "list pods on node: %s failed", node.Name)
		return NodeInfo{Name: node.Name, Error: err.Error()}
	}
	nodeUsedGCU := 0
	devices, nodeTotalGCU, err := ins.initDevices(drs, node)
	if err != nil {
		return NodeInfo{Name: node.Name, Error: err.Error()}
	}
	warnMsg := []string{}
	nodeInfo := NodeInfo{
		Name:         node.Name,
		TotalGCU:     nodeTotalGCU,
		UsedGCU:      new(int),
		AvailableGCU: new(int),
	}
	ignoreDevice := map[string]struct{}{}
	for _, pod := range pods.Items {
		gcusharePod := ins.podResource.IsGCUsharingPod(&pod)
		drsPod := ins.podResource.IsDrsGCUsharingPod(&pod)
		if !gcusharePod && !drsPod {
			continue
		}
		minor := pod.Annotations[consts.PodAssignedGCUMinor]
		if minor == "" {
			logs.Warn("found pod(name: %s, uuid: %s), but has not been assigned device", pod.Name, pod.UID)
			continue
		}
		_, ok1 := ignoreDevice[minor]
		if _, ok2 := devices[minor]; !ok1 && !ok2 {
			msg := fmt.Sprintf("pod(name: %s, uuid: %s) has been assigned device id: %s, but device is not found",
				pod.Name, pod.UID, minor)
			logs.Warn(msg)
			warnMsg = append(warnMsg, msg)
			continue
		}
		var podRequest int
		var virt string
		if drs {
			if gcusharePod {
				logs.Warn("device: %s is using by gcushare pod(name: %s, uuid: %s)", minor, pod.Name, pod.UID)
				if device, ok := devices[minor]; ok {
					nodeInfo.TotalGCU -= device.TotalGCU
					delete(devices, minor)
					ignoreDevice[minor] = struct{}{}
				}
				continue
			}
			podRequest = ins.podResource.GetRequestDrsPodResource(&pod)
			virt = consts.VirtDRS
		} else {
			if drsPod {
				logs.Warn("device: %s is using by drs pod(name: %s, uuid: %s)", minor, pod.Name, pod.UID)
				if device, ok := devices[minor]; ok {
					nodeInfo.TotalGCU -= device.TotalGCU
					delete(devices, minor)
					ignoreDevice[minor] = struct{}{}
				}
				continue
			}
			podRequest = ins.podResource.GetRequestFromPodResource(&pod)
			virt = consts.VirtShared
		}
		if devices[minor].Virt == nil {
			devices[minor].Virt = &virt
		}
		nodeUsedGCU += podRequest
		*devices[minor].UsedGCU += podRequest
		*devices[minor].AvailableGCU -= podRequest
		if *devices[minor].AvailableGCU < 0 {
			err := fmt.Errorf("device: %s on node: %s total size: %d, but at least used: %d, gcushare slice count may be modified, which is not allowed",
				minor, node.Name, devices[minor].TotalGCU, *devices[minor].UsedGCU)
			logs.Error(err)
			return NodeInfo{Name: node.Name, Error: err.Error()}
		}
		assignedMap := map[string]structs.AllocateRecord{}
		assignedContainers := pod.Annotations[consts.PodAssignedContainers]
		if err := json.Unmarshal([]byte(assignedContainers), &assignedMap); err != nil {
			logs.Error(err, "unmarshal assigned containers to map of pod(name: %s, uuid: %s) failed, detail: %s",
				pod.Name, pod.UID, assignedContainers)
			return NodeInfo{Name: node.Name, Error: err.Error()}
		}
		devices[minor].Pods = append(devices[minor].Pods, &Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:              pod.Name,
				Namespace:         pod.Namespace,
				UID:               pod.UID,
				CreationTimestamp: pod.CreationTimestamp,
			},
			UsedGCU:    podRequest,
			Phase:      pod.Status.Phase,
			Containers: assignedMap,
		})
	}
	nodeInfo.UsedGCU = &nodeUsedGCU
	*nodeInfo.AvailableGCU = nodeInfo.TotalGCU - nodeUsedGCU
	nodeInfo.Devices = devices
	nodeInfo.Warn = warnMsg
	return nodeInfo
}

func (ins *Inspect) initDevices(drs bool, node *v1.Node) (map[string]*Device, int, error) {
	var deviceMap map[string]int
	var err error
	if drs {
		nodeDevice, ok := node.Annotations[consts.GCUDRSCapacity]
		if !ok {
			err := fmt.Errorf("node: %s missing drs gcu capacity annotations: %v", node.Name, node.Annotations)
			logs.Error(err)
			return nil, -1, err
		}
		deviceCapacity := &structs.DRSGCUCapacity{}
		if err := json.Unmarshal([]byte(nodeDevice), deviceCapacity); err != nil {
			logs.Error(err, "json unmarshal failed, content: %s", nodeDevice)
			return nil, -1, err
		}
		deviceMap = map[string]int{}
		for _, device := range deviceCapacity.Devices {
			deviceMap[device.Minor] = device.Capacity
		}
	} else {
		deviceMap, err = ins.nodeResource.GetGCUDeviceMemoryMap(node)
		if err != nil {
			return nil, -1, err
		}
	}
	devices := map[string]*Device{}
	nodeTotalGCU := 0
	for deviceID, totalSize := range deviceMap {
		nodeTotalGCU += totalSize
		available := totalSize
		devices[deviceID] = &Device{
			TotalGCU:     totalSize,
			UsedGCU:      new(int),
			AvailableGCU: &available,
			Pods:         []*Pod{},
		}
	}
	return devices, nodeTotalGCU, nil
}
