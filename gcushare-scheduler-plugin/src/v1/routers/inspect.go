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
)

type Inspect struct {
	resourceName string
	clientset    *kubernetes.Clientset
	podResource  *resources.PodResource
	nodeResource *resources.NodeResource
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
	TotalGCU     int    `json:"totalGCU,omitempty"`
	UsedGCU      *int   `json:"usedGCU,omitempty"`
	AvailableGCU *int   `json:"availableGCU,omitempty"`
	Pods         []*Pod `json:"pods"`
}

type Pod struct {
	metav1.ObjectMeta `json:",inline"`
	UsedGCU           int         `json:"usedGCU"`
	Phase             v1.PodPhase `json:"phase"`
}

func (ins *Inspect) InspectRoute(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	nodeName := ps.ByName("nodename")
	if nodeName == "" {
		logs.Info("listen access url: %s, method: %s, request body:%v", consts.InspectNodeListRoute, "GET", r)
	} else {
		logs.Info("listen access url: %s, method: %s, request body:%v", consts.InspectNodeDetailRoute, "GET", r)
	}
	result := ins.inspect(nodeName)

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

func (ins *Inspect) inspect(nodeName string) (result interface{}) {
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
		if !ins.nodeResource.IsGCUSharingNode(nodeDetail) {
			err := fmt.Errorf("node: %s is not the gcushare node", nodeName)
			logs.Error(err)
			return NodeInfo{Name: nodeName, Error: err.Error()}
		}
		return ins.buildNodeInfo(nodeDetail)
	}
	nodeList, err := ins.clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logs.Error(err, "list cluster node list failed")
		return map[string]string{"error": err.Error()}
	}
	res := []NodeInfo{}
	for _, node := range nodeList.Items {
		if !ins.nodeResource.IsGCUSharingNode(&node) {
			continue
		}
		res = append(res, ins.buildNodeInfo(&node))
	}
	if len(res) == 0 {
		msg := "the gcushare nodes not found"
		logs.Error(msg)
		return map[string]string{"error": msg}
	}
	return res
}

func (ins *Inspect) buildNodeInfo(node *v1.Node) NodeInfo {
	pods, err := ins.clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + node.Name,
	})
	if err != nil {
		logs.Error(err, "list pods on node: %s failed", node.Name)
		return NodeInfo{Name: node.Name, Error: err.Error()}
	}
	nodeUsedGCU := 0
	devices, nodeTotalGCU, err := ins.initDevices(node)
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
	for _, pod := range pods.Items {
		podRequest := ins.podResource.GetGCUMemoryFromPodResource(&pod)
		if podRequest <= 0 {
			continue
		}
		value := ins.podResource.GetPodAssignedDeviceID(&pod)
		if value == "" {
			logs.Warn("pod(name: %s, uuid: %s) request %s: %d, but has not been assigned device",
				pod.Name, pod.UID, ins.resourceName, podRequest)
			continue
		}
		if _, ok := devices[value]; !ok {
			msg := fmt.Sprintf("pod(name: %s, uuid: %s) has been assigned device id: %s, but device is not found",
				pod.Name, pod.UID, value)
			logs.Warn(msg)
			warnMsg = append(warnMsg, msg)
			continue
		}
		nodeUsedGCU += podRequest
		*devices[value].UsedGCU += podRequest
		*devices[value].AvailableGCU -= podRequest
		if *devices[value].AvailableGCU < 0 {
			err := fmt.Errorf("device: %s on node: %s total size: %d, but at least used: %d, gcushare slice count may be modified, which is not allowed",
				value, node.Name, devices[value].TotalGCU, *devices[value].UsedGCU)
			logs.Error(err)
			return NodeInfo{Name: node.Name, Error: err.Error()}
		}
		devices[value].Pods = append(devices[value].Pods, &Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:              pod.Name,
				Namespace:         pod.Namespace,
				UID:               pod.UID,
				CreationTimestamp: pod.CreationTimestamp,
			},
			UsedGCU: podRequest,
			Phase:   pod.Status.Phase,
		})
	}
	nodeInfo.UsedGCU = &nodeUsedGCU
	*nodeInfo.AvailableGCU = nodeInfo.TotalGCU - nodeUsedGCU
	nodeInfo.Devices = devices
	nodeInfo.Warn = warnMsg
	return nodeInfo
}

func (ins *Inspect) initDevices(node *v1.Node) (map[string]*Device, int, error) {
	deviceMap, err := ins.nodeResource.GetGCUDeviceMemoryMap(node)
	if err != nil {
		return nil, -1, err
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
