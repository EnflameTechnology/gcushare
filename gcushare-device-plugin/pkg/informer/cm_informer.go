package informer

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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"gcushare-device-plugin/pkg/config"
	"gcushare-device-plugin/pkg/consts"
	"gcushare-device-plugin/pkg/device"
	"gcushare-device-plugin/pkg/drs"
	"gcushare-device-plugin/pkg/logs"
	"gcushare-device-plugin/pkg/smi"
	"gcushare-device-plugin/pkg/status"
	"gcushare-device-plugin/pkg/structs"
	"gcushare-device-plugin/pkg/utils"
)

type ConfigMapInformer struct {
	sync.RWMutex
	Config     *config.Config
	NodeName   string
	SliceCount int
	Clientset  *kubernetes.Clientset
}

func NewConfigMapInformer(nodeName string, device *device.Device, clientset *kubernetes.Clientset) *ConfigMapInformer {
	return &ConfigMapInformer{
		Config:     device.Config,
		NodeName:   nodeName,
		SliceCount: device.SliceCount,
		Clientset:  clientset,
	}
}

func (rer *ConfigMapInformer) Start(ctx context.Context) error {
	factory := informers.NewSharedInformerFactoryWithOptions(
		rer.Clientset,
		time.Minute*5,
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.LabelSelector = fmt.Sprintf("%s=%s,%s=%s",
				consts.ConfigMapNode, rer.NodeName,
				consts.ConfigMapOwner, consts.DRSSchedulerName)
		}),
	)

	configMapInformer := factory.Core().V1().ConfigMaps().Informer()

	configMapInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			cm, ok := obj.(*v1.ConfigMap)
			if !ok {
				logs.Error("%v is not the configmap object", obj)
				return
			}
			rer.cmAddHandler(cm)
		},
		UpdateFunc: func(oldObj, newObj any) {
			cm, ok := newObj.(*v1.ConfigMap)
			if !ok {
				logs.Error("%v is not the configmap object", newObj)
				return
			}
			rer.cmUpdateHandler(cm)
		},
		DeleteFunc: func(obj any) {
			cm, ok := obj.(*v1.ConfigMap)
			if !ok {
				logs.Error("%v is not the configmap object", obj)
				return
			}
			rer.cmDeleteHandler(cm)
		},
	})

	go configMapInformer.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), configMapInformer.HasSynced) {
		err := fmt.Errorf("timed out waiting for configmap cache sync")
		logs.Error(err)
		return err
	}
	logs.Info("gcushare-device-plugin configmap informer work start...")
	return nil
}

func (rer *ConfigMapInformer) needExecuteHandler(handler, configmapName, status string) (bool, error) {
	if len(strings.Split(configmapName, ".")) != 5 || status != "" {
		return false, nil
	}
	podName := strings.Split(configmapName, ".")[0]
	podNamespace := strings.Split(configmapName, ".")[1]
	podUid := strings.Split(configmapName, ".")[2]
	pod, err := rer.Clientset.CoreV1().Pods(podNamespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		if k8serr.IsNotFound(err) {
			logs.Error(err, "pod(name: %s, uuid: %s) has been delete from cluster, skip this %s sync for configmap %s",
				podName, podUid, handler, configmapName)
		} else {
			logs.Error(err, "get pod(name: %s, uuid: %s) from cluster failed, skip this %s sync for configmap %s",
				podName, podUid, handler, configmapName)
		}
		return false, err
	}
	if pod.DeletionTimestamp != nil {
		err := fmt.Errorf("pod(name: %s, uuid: %s) is deleting in cluster, skip this %s sync for configmap %s",
			podName, podUid, handler, configmapName)
		logs.Error(err)
		return false, err
	}
	return true, nil
}

func (rer *ConfigMapInformer) cmAddHandler(configmap *v1.ConfigMap) (state *status.SchedulingStatus) {
	rer.Lock()
	defer rer.Unlock()
	logs.Info("start add handler for configmap: %s", configmap.Name)
	schedulerRecord := new(structs.SchedulerRecord)
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("Internal error! Recovered from panic: %v", r)
			logs.Error(errMsg)
			return
		}
		rer.patchConfigMap("cmAddHandler", configmap, state, schedulerRecord)
		logs.Info("add handler finish for configmap: %s", configmap.Name)
	}()

	result := configmap.Data[consts.SchedulerRecord]
	if err := json.Unmarshal([]byte(result), schedulerRecord); err != nil {
		logs.Error(err, "json unmarshal failed, detail: %s", result)
		return status.NewStatus(consts.StateError, err.Error())
	}
	ok, err := rer.needExecuteHandler("cmAddHandler", configmap.Name, schedulerRecord.Filter.Status)
	if err != nil {
		return status.NewStatus(consts.StateError, err.Error())
	}
	if !ok {
		return status.NewStatus(consts.StateSkip, "")
	}

	deviceInfo, err := device.GetDeviceInfoFromSmiAndPci()
	if err != nil {
		return status.NewStatus(consts.StateError, err.Error())
	}

	selectedIndex := ""
	selectedAvailable := 0
	selectedMinor := ""
	selectedBusID := ""

	podRequest := 0
	podExpectedProfile := map[string]int{}
	for _, alloc := range schedulerRecord.Filter.Containers {
		podExpectedProfile[fmt.Sprintf("%dg", *alloc.Request)] += 1
		podRequest += *alloc.Request
	}

	deviceInfo, availableProfiles, state := drs.GetAvailableProfiles(deviceInfo, configmap)
	if state != nil {
		return state
	}
	var allAvailable int
	var availableInstance map[string]drs.AvailableInstance
	for _, device := range deviceInfo {
		if device.GCUVirt != "DRS" && device.GCUVirt != "Disable" {
			logs.Warn("device index: %s virt is: %s, skip it", device.Index, device.GCUVirt)
			continue
		}
		allAvailable, availableInstance, err = drs.GetAvailableInstance(device.Index, rer.SliceCount, availableProfiles)
		if err != nil {
			return status.NewStatus(consts.StateError, err.Error())
		}
		if podRequest > allAvailable {
			logs.Info("device index: %s available: %d, but pod request: %d, skip it", device.Index, allAvailable, podRequest)
			continue
		}
		pass, err := rer.checkDeviceCanAllocate(podExpectedProfile, availableInstance, configmap)
		if err != nil {
			return status.NewStatus(consts.StateError, err.Error())
		}
		if !pass {
			logs.Warn("device index: %s instances count is insufficient. skip it", device.Index)
			continue
		}
		if selectedIndex == "" || selectedAvailable > allAvailable {
			selectedIndex = device.Index
			selectedAvailable = allAvailable
			selectedMinor = device.Minor
			selectedBusID = device.PCIBusID
		}
	}

	if selectedIndex != "" {
		schedulerRecord.Filter.Device = structs.DeviceSpec{Index: selectedIndex, Minor: selectedMinor, PCIBusID: selectedBusID}
		rer.setAssignedContainers(schedulerRecord, availableInstance)
		logs.Info("device plugin select device(index: %s, minor: %s, busID: %s) to allocate drs for: %s, detail: %s",
			selectedIndex, selectedMinor, selectedBusID, configmap.Name, utils.ConvertToString(schedulerRecord.Filter.Containers))
	} else {
		msg := fmt.Sprintf("device plugin not found device can allocate drs for: %s", configmap.Name)
		logs.Warn(msg)
		return status.NewStatus(consts.StateUnschedulable, msg)
	}
	return status.NewStatus(consts.StateSuccess, "")
}

func (rer *ConfigMapInformer) setAssignedContainers(schedulerRecord *structs.SchedulerRecord,
	availableInstance map[string]drs.AvailableInstance) {
	for containerName, alloc := range schedulerRecord.Filter.Containers {
		selectedProfileName := availableInstance[fmt.Sprintf("%dg", *alloc.Request)].ProfileName
		selectedProfileID := availableInstance[fmt.Sprintf("%dg", *alloc.Request)].ProfileID
		alloc.ProfileName = &selectedProfileName
		alloc.ProfileID = &selectedProfileID
		schedulerRecord.Filter.Containers[containerName] = alloc
	}
}

func (rer *ConfigMapInformer) checkDeviceCanAllocate(podExpectedProfile map[string]int,
	availableInstance map[string]drs.AvailableInstance, configmap *v1.ConfigMap) (bool, error) {
	for profileNamePrefix, request := range podExpectedProfile {
		available, ok := availableInstance[profileNamePrefix]
		if !ok {
			msg := fmt.Sprintf("configmap: %s pod exit container request: %s, but the profile associated with it does not exist",
				configmap.Name, profileNamePrefix)
			logs.Error(msg)
			return false, fmt.Errorf("%s", msg)
		}
		if request > available.AvailableInstanceCount {
			logs.Warn("configmap: %s request profile: %s instance: %d, but available instance count: %d",
				configmap.Name, available.ProfileName, request, available.AvailableInstanceCount)
			return false, nil
		}
	}
	return true, nil
}

func (rer *ConfigMapInformer) cmUpdateHandler(configmap *v1.ConfigMap) (state *status.SchedulingStatus) {
	rer.Lock()
	defer rer.Unlock()
	logs.Info("start update handler for configmap: %s", configmap.Name)
	schedulerRecord := new(structs.SchedulerRecord)
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("Internal error! Recovered from panic: %v", r)
			logs.Error(errMsg)
			return
		}
		rer.patchConfigMap("cmUpdateHandler", configmap, state, schedulerRecord)
		logs.Info("update handler finish for configmap: %s", configmap.Name)
	}()

	result := configmap.Data[consts.SchedulerRecord]
	if err := json.Unmarshal([]byte(result), schedulerRecord); err != nil {
		logs.Error(err, "json unmarshal failed, detail: %s", result)
		return status.NewStatus(consts.StateError, err.Error())
	}
	if schedulerRecord.PreBind == nil {
		logs.Warn("preBind record not found, skip this sync for configmap: %s", configmap.Name)
		return status.NewStatus(consts.StateSkip, "")
	}

	ok, err := rer.needExecuteHandler("cmUpdateHandler", configmap.Name, schedulerRecord.PreBind.Status)
	if err != nil {
		return status.NewStatus(consts.StateError, err.Error())
	}
	if !ok {
		return status.NewStatus(consts.StateSkip, "")
	}

	for containerName, allocated := range schedulerRecord.PreBind.Containers {
		instanceID, err := drs.CreateDRSInstance(schedulerRecord.Filter.Device.Index, *allocated.ProfileName,
			*allocated.ProfileID)
		if err != nil {
			return status.NewStatus(consts.StateError, err.Error())
		}
		allocated.InstanceID = &instanceID
		schedulerRecord.PreBind.Containers[containerName] = allocated
	}
	return status.NewStatus(consts.StateSuccess, "")
}

func (rer *ConfigMapInformer) patchConfigMap(caller string, configmap *v1.ConfigMap, state *status.SchedulingStatus,
	schedulerRecord *structs.SchedulerRecord) {
	if state.Status == consts.StateSkip {
		return
	}
	if caller == "cmAddHandler" {
		if schedulerRecord.Filter == nil {
			schedulerRecord.Filter = &structs.FilterSpec{}
		}
		schedulerRecord.Filter.Status = state.Status
		schedulerRecord.Filter.Message = state.Message
	} else {
		if schedulerRecord.PreBind == nil {
			schedulerRecord.PreBind = &structs.PreBindSpec{}
		}
		schedulerRecord.PreBind.Status = state.Status
		schedulerRecord.PreBind.Message = state.Message
	}

	schedulerRecordBytes, err := json.Marshal(*schedulerRecord)
	if err != nil {
		logs.Error(err, "json marshal failed, content: %v", *schedulerRecord)
		return
	}

	patchData := map[string]interface{}{
		"data": map[string]string{
			consts.SchedulerRecord: string(schedulerRecordBytes),
		},
	}
	patchBytes, err := json.Marshal(patchData)
	if err != nil {
		logs.Error(err)
		return
	}
	if _, err := rer.Clientset.CoreV1().ConfigMaps(configmap.Namespace).Patch(
		context.TODO(),
		configmap.Name,
		types.MergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	); err != nil {
		logs.Error(err, "%s patch configmap: %s failed, content: %s", caller, configmap.Name, string(patchBytes))
		return
	}
	logs.Info("%s patch configmap: %s success, content: %s", caller, configmap.Name, string(patchBytes))
}

func (rer *ConfigMapInformer) cmDeleteHandler(configmap *v1.ConfigMap) {
	rer.Lock()
	defer rer.Unlock()
	if len(strings.Split(configmap.Name, ".")) != 5 {
		return
	}
	logs.Info("start delete handler for configmap: %s", configmap.Name)
	schedulerRecord := new(structs.SchedulerRecord)
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("Internal error! Recovered from panic: %v", r)
			logs.Error(errMsg)
			return
		}
		logs.Info("delete handler finish for configmap: %s", configmap.Name)
	}()

	result := configmap.Data[consts.SchedulerRecord]
	if err := json.Unmarshal([]byte(result), schedulerRecord); err != nil {
		logs.Error(err, "json unmarshal failed, detail: %s", result)
		return
	}
	if schedulerRecord.PreBind == nil {
		return
	}
	podName := strings.Split(configmap.Name, ".")[0]
	pod, err := rer.Clientset.CoreV1().Pods(configmap.Namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		if !k8serr.IsNotFound(err) {
			logs.Error("get pod(name: %s) from cluster failed, configmap delete handler do nothing", podName)
			return
		}
		logs.Warn("pod: %s has been deleted, will try to clear drs", podName)
	} else if pod.DeletionTimestamp == nil {
		logs.Info("pod(name: %s, uuid: %s) is not in delete status, skip it", pod.Name, pod.UID)
		return
	} else {
		logs.Warn("pod(name: %s, uuid: %s) is deleting, will try to clear drs", podName, pod.UID)
	}
	index := schedulerRecord.Filter.Device.Index
	for _, alloc := range schedulerRecord.PreBind.Containers {
		if alloc.InstanceID == nil {
			continue
		}
		if err := smi.DeleteInstance(index, *alloc.InstanceID); err != nil {
			logs.Error(err, "delete device index: %s drs instance(profileName: %s, instanceID: %s) failed",
				index, *alloc.ProfileName, *alloc.InstanceID)
		} else {
			logs.Info("delete device index: %s drs instance(profileName: %s, instanceID: %s) success",
				index, *alloc.ProfileName, *alloc.InstanceID)
		}
	}
	instanceList, err := smi.ListInstance(index)
	if err != nil {
		logs.Error(err, "list device index: %s drs instance failed", index)
		return
	}
	if len(instanceList) == 0 {
		logs.Info("device index: %s drs instance not found, need close drs", index)
		if err := smi.CloseDRS(index); err != nil {
			logs.Error(err, "close device index: %s drs failed", index)
		} else {
			logs.Info("close device index: %s drs success", index)
		}
	}
}
