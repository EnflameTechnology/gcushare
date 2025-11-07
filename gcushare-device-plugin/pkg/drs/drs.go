package drs

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"gcushare-device-plugin/pkg/consts"
	"gcushare-device-plugin/pkg/logs"
	"gcushare-device-plugin/pkg/smi"
	"gcushare-device-plugin/pkg/utils"
)

func CreateDRSInstance(index, profileName, profileID string) (string, string, error) {
	if err := smi.OpenDRS(index); err != nil {
		return "", "", err
	}
	logs.Info("device index: %s open DRS success", index)

	instanceUUIDsBefore := map[string]string{}
	instanceList, err := smi.ListInstance(index)
	if err != nil {
		return "", "", err
	}
	logs.Info("list device index: %s instance list before create: %s", index, utils.ConvertToString(instanceList))
	for _, instance := range instanceList {
		if instance.ProfileName == profileName {
			instanceUUIDsBefore[instance.UUID] = instance.InstanceID
		}
	}

	if err := smi.CreateInstance(index, profileID); err != nil {
		return "", "", err
	}
	logs.Info("create device index: %s drs instance use profile: %s success", index, profileName)

	instanceUUIDsAfter := map[string]string{}
	instanceList, err = smi.ListInstance(index)
	if err != nil {
		return "", "", err
	}
	logs.Info("list device index: %s instance list after create: %s", index, utils.ConvertToString(instanceList))
	for _, instance := range instanceList {
		if instance.ProfileName == profileName {
			instanceUUIDsAfter[instance.UUID] = instance.InstanceID
		}
	}

	for newInstanceUUID := range instanceUUIDsAfter {
		if _, ok := instanceUUIDsBefore[newInstanceUUID]; !ok {
			logs.Info("allocate drs instance(device index: %s, profileName: %s, instanceUUID: %s) to container success",
				index, profileName, newInstanceUUID)
			return instanceUUIDsAfter[newInstanceUUID], newInstanceUUID, nil
		}
	}
	err = fmt.Errorf("not found new created instance id of device index: %s, profile name: %s", index, profileName)
	logs.Error(err)
	return "", "", err
}

func GetProfileTemplate(lines []string) (map[smi.ProfileName]smi.ProfileSpec, error) {
	indexRDS, needCloseDRS := findDeviceEnabledDRS(lines)
	if needCloseDRS {
		defer func() {
			if err := smi.CloseDRS(indexRDS); err != nil {
				logs.Error(err, "close device index: %s drs failed", indexRDS)
			} else {
				logs.Info("close device index: %s drs success", indexRDS)
			}
		}()
	}
	if indexRDS == "" {
		msg := "no device is in DRS status or can enable DRS, please check the device status"
		logs.Error(msg)
		return nil, errors.New(msg)
	}
	profileTemplate, err := smi.ListProfile(indexRDS)
	if err != nil {
		return nil, err
	}

	jsonData, err := json.MarshalIndent(profileTemplate, "", "  ")
	if err != nil {
		return nil, err
	}
	logs.Info("get profile template from smi success, detail: \n%s", string(jsonData))
	logs.Warn("Note: Profile template of all devices on the current node must be the same; otherwise, it won't work properly")
	return profileTemplate, nil
}

func findDeviceEnabledDRS(lines []string) (string, bool) {
	busPattern := regexp.MustCompile(consts.BusIDRegExp)
	for i := 0; i < len(lines)-1; i++ {
		line := lines[i]
		if busPattern.MatchString(line) {
			line = strings.Trim(line, "|")
			partOne := strings.Fields(strings.Split(line, "|")[0])
			deviceID := partOne[0]
			nextLine := lines[i+1]
			nextLine = strings.ReplaceAll(nextLine, "|", "")
			nextLine = strings.ReplaceAll(nextLine, "/", "")
			nextLineFields := strings.Fields(nextLine)
			virtType := nextLineFields[5]
			if virtType == "DRS" {
				logs.Info("find device index: %s in DRS status", deviceID)
				return deviceID, false
			}
		}
	}
	for i := 0; i < len(lines)-1; i++ {
		line := lines[i]
		if busPattern.MatchString(line) {
			line = strings.Trim(line, "|")
			partOne := strings.Fields(strings.Split(line, "|")[0])
			deviceID := partOne[0]
			nextLine := lines[i+1]
			nextLine = strings.ReplaceAll(nextLine, "|", "")
			nextLine = strings.ReplaceAll(nextLine, "/", "")
			nextLineFields := strings.Fields(nextLine)
			virtType := nextLineFields[5]
			if virtType != "Disable" {
				logs.Warn("device index: %s is not in DRS or Disable status, skip it", deviceID)
				continue
			}
			if err := smi.OpenDRS(deviceID); err != nil {
				logs.Warn("open device index: %s drs failed: %s, skip it", deviceID, err.Error())
				continue
			}
			logs.Info("open device index: %s drs success", deviceID)
			return deviceID, true
		}
	}
	return "", false
}
