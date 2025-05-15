package drs

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"

	"gcushare-device-plugin/pkg/consts"
	"gcushare-device-plugin/pkg/device"
	"gcushare-device-plugin/pkg/logs"
	"gcushare-device-plugin/pkg/smi"
	"gcushare-device-plugin/pkg/status"
	"gcushare-device-plugin/pkg/structs"
	"gcushare-device-plugin/pkg/utils"
)

type AvailableInstance struct {
	ProfileName            string `json:"profileName"`
	ProfileID              string `json:"profileID"`
	InstanceCount          int    `json:"instanceCount"`
	AvailableInstanceCount int    `json:"availableInstanceCount"`
}

func GetAvailableDrs(index string, sliceCount int) (int, error) {
	created := 0
	instanceList, err := smi.ListInstance(index)
	if err != nil {
		return -1, err
	}
	logs.Info("list device index: %s instance list: %s", index, utils.ConvertToString(instanceList))
	for _, instance := range instanceList {
		used := strings.Split(instance.ProfileName, "g.")[1]
		usedInt, err := strconv.Atoi(used)
		if err != nil {
			return -1, err
		}
		created += usedInt
	}

	return sliceCount - created, nil
}

func CreateDRSInstance(index, profileName, profileID string) (string, error) {
	if err := smi.OpenDRS(index); err != nil {
		return "", err
	}
	logs.Info("device index: %s open DRS success", index)

	instanceIdsBefore := map[string]struct{}{}
	instanceList, err := smi.ListInstance(index)
	if err != nil {
		return "", err
	}
	logs.Info("list device index: %s instance list before create: %s", index, utils.ConvertToString(instanceList))
	for _, instance := range instanceList {
		if instance.ProfileName == profileName {
			instanceIdsBefore[instance.InstanceID] = struct{}{}
		}
	}

	if err := smi.CreateInstance(index, profileID); err != nil {
		return "", err
	}
	logs.Info("create device index: %s drs instance use profile: %s success", index, profileName)

	instanceIdsAfter := map[string]struct{}{}
	instanceList, err = smi.ListInstance(index)
	if err != nil {
		return "", err
	}
	logs.Info("list device index: %s instance list after create: %s", index, utils.ConvertToString(instanceList))
	for _, instance := range instanceList {
		if instance.ProfileName == profileName {
			instanceIdsAfter[instance.InstanceID] = struct{}{}
		}
	}

	for newInstanceID := range instanceIdsAfter {
		if _, ok := instanceIdsBefore[newInstanceID]; !ok {
			logs.Info("allocate drs instance(device index: %s, profileName: %s, instanceID: %s) to container success",
				index, profileName, newInstanceID)
			return newInstanceID, nil
		}
	}
	err = fmt.Errorf("not found new created instance id of device index: %s, profile name: %s", index, profileName)
	logs.Error(err)
	return "", err
}

func GetAvailableInstance(index string, sliceCount int, availableProfiles map[string]AvailableInstance) (
	int, map[string]AvailableInstance, error) {
	allAvailable := sliceCount
	availableInstanceMap := map[string]AvailableInstance{}
	for key, value := range availableProfiles {
		availableInstance := AvailableInstance{
			ProfileName:            value.ProfileName,
			ProfileID:              value.ProfileID,
			InstanceCount:          value.InstanceCount,
			AvailableInstanceCount: value.InstanceCount,
		}
		availableInstanceMap[key] = availableInstance
	}
	instanceList, err := smi.ListInstance(index)
	if err != nil {
		logs.Error(err, "list device index: %s drs instance failed", index)
		return -1, nil, err
	}
	for _, instance := range instanceList {
		availableInstance := availableInstanceMap[strings.Split(instance.ProfileName, ".")[0]]
		logs.Info("find device index: %s drs instance(profileName: %s, profileID: %s) exist",
			index, instance.ProfileName, instance.ProfileID)
		availableInstance.AvailableInstanceCount -= 1
		availableInstanceMap[strings.Split(instance.ProfileName, ".")[0]] = availableInstance

		prefix := strings.Split(instance.ProfileName, "g.")[0]
		instaceUsed, err := strconv.Atoi(prefix)
		if err != nil {
			logs.Error(err, "device index: %s profile name prefix must be int, but is: %s", index, prefix)
			return -1, nil, err
		}
		allAvailable -= instaceUsed
	}
	logs.Info("device index: %s available instance: %s", index, utils.JsonMarshalIndent(availableInstanceMap))
	return allAvailable, availableInstanceMap, nil
}

func GetAvailableProfiles(deviceInfo []device.DeviceFullInfo, configmap *v1.ConfigMap) ([]device.DeviceFullInfo,
	map[string]AvailableInstance, *status.SchedulingStatus) {
	virtDRSIndex := ""
	virtDisableIndex := ""
	filterDeviceInfo := []device.DeviceFullInfo{}
	for _, device := range deviceInfo {
		if deviceUsedByGCUSharePod(device.Minor, configmap) {
			continue
		}
		filterDeviceInfo = append(filterDeviceInfo, device)
		if device.GCUVirt == "DRS" && virtDRSIndex == "" {
			virtDRSIndex = device.Index
		}
		if device.GCUVirt == "Disable" && virtDisableIndex == "" {
			virtDisableIndex = device.Index
		}
	}

	useIndex := virtDRSIndex
	if useIndex == "" {
		useIndex = virtDisableIndex
		if useIndex == "" {
			msg := "not found device virt is 'DRS' or 'Disable'"
			logs.Warn(msg)
			return nil, nil, status.NewStatus(consts.StateUnschedulable, msg)
		}
		if err := smi.OpenDRS(useIndex); err != nil {
			logs.Error(err, "open device index: %s drs failed", useIndex)
			return nil, nil, status.NewStatus(consts.StateError, err.Error())
		}
		logs.Info("open device index: %s drs success", useIndex)
		defer func() {
			if err := smi.CloseDRS(useIndex); err != nil {
				logs.Error(err, "close device index: %s drs failed", useIndex)
			} else {
				logs.Info("close device index: %s drs success", useIndex)
			}
		}()
	}

	profileList, err := smi.ListProfile(useIndex)
	if err != nil {
		logs.Error(err, "list device index: %s profile failed", useIndex)
		return nil, nil, status.NewStatus(consts.StateError, err.Error())
	}
	availableInstanceMap := map[string]AvailableInstance{}
	for _, profile := range profileList {
		count, err := strconv.Atoi(profile.InstanceCount)
		if err != nil {
			return nil, nil, status.NewStatus(consts.StateError, err.Error())
		}
		availableInstance := AvailableInstance{
			ProfileName:            profile.ProfileName,
			ProfileID:              profile.ProfileID,
			InstanceCount:          count,
			AvailableInstanceCount: count,
		}
		availableInstanceMap[strings.Split(profile.ProfileName, ".")[0]] = availableInstance
	}
	logs.Info("device index: %s profile list: %v", useIndex, utils.JsonMarshalIndent(availableInstanceMap))
	logs.Warn("gcushare assumes that the drs profiles of all gcu devices are the same")
	return filterDeviceInfo, availableInstanceMap, nil
}

func deviceUsedByGCUSharePod(minor string, configmap *v1.ConfigMap) bool {
	schedulerRecord := &structs.SchedulerRecord{}
	schedulerRecordString := configmap.Data[consts.SchedulerRecord]
	if err := json.Unmarshal([]byte(schedulerRecordString), schedulerRecord); err != nil {
		logs.Error(err, "json unmarshal failed, skip device minor: %s for configmap: %s, content: %s",
			minor, configmap.Name, schedulerRecordString)
		return true
	}
	for _, pod := range schedulerRecord.Filter.GCUSharePods {
		if pod.AssignedID == minor {
			logs.Warn("device minor: %s is using by gcushare pod(name: %s, uuid: %s), can not allocate drs, skip it",
				minor, pod.Name, pod.Uuid)
			return true
		}
	}
	return false
}

// func CheckDeviceVirt(index, virt string) (bool, error) {
// 	if virt == "Disable" {
// 		if err := smi.OpenDRS(index); err != nil {
// 			logs.Error(err, "open device index: %s drs failed", index)
// 			return false, err
// 		}
// 		logs.Info("open device index: %s drs for check", index)
// 		return true, nil
// 	}
// 	if virt == "DRS" {
// 		logs.Info("device index: %s virt alerady is DRS", index)
// 		return false, nil
// 	}
// 	err := fmt.Errorf("gcu virt: %s is not support by gcushare, only 'Disable' or 'DRS'", virt)
// 	logs.Error(err)
// 	return false, err
// }

// func CloseDrs(patchData map[string]interface{}, needCloses map[string]bool) {
// 	errMsg := ""
// 	for index, close := range needCloses {
// 		if close {
// 			if err := smi.CloseDRS(index); err != nil {
// 				logs.Error(err, "close device index: %s failed", index)
// 				errMsg += fmt.Sprintf("%s,", err.Error())
// 			}
// 		}
// 	}
// 	if errMsg != "" {
// 		patchData["data"] = map[string]string{consts.DRSAssignedDevice: errMsg}
// 	}
// }
