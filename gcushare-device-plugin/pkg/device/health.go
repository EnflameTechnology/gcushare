// Copyright (c) 2025, ENFLAME INC.  All rights reserved.

package device

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"

	"gcushare-device-plugin/pkg/consts"
	"gcushare-device-plugin/pkg/logs"
	"gcushare-device-plugin/pkg/utils"
)

func WatchHealth(stop <-chan struct{}, devices []*pluginapi.Device, unhealthyChanList chan<- []*pluginapi.Device,
	devInfos map[string]DeviceInfo, allLocked chan map[string]struct{}) {
	var enableHealthCheck = true

	disableHealthChecks := strings.ToLower(os.Getenv(consts.EnvDisableHealthChecks))
	if strings.Contains(disableHealthChecks, consts.AllHealthChecks) {
		logs.Warn("gcushare device need not health check with env name: %s, values:%s",
			consts.EnvDisableHealthChecks, disableHealthChecks)
		enableHealthCheck = false
		return
	}

	allLockedMap := map[string]struct{}{}
	for {
		select {
		case <-stop:
			return
		case allLockedMap = <-allLocked:
			logs.Warn("health check found that some device resources have been used up and are locked")
		default:
		}

		if enableHealthCheck {
			checkedList := map[string]consts.DeviceState{}
			unhealthyList := []*pluginapi.Device{}
			for _, device := range devices {
				realID := strings.Split(device.ID, "-")[0]
				if _, ok := checkedList[realID]; !ok {
					checkedList[realID] = deviceHealthCheck(devInfos[realID].BusID)
				}
				health := checkedList[realID]
				if health != consts.HEALTH_PASS && device.Health == pluginapi.Healthy {
					logs.Info("WatchHealth: Health check error on Device=%s, the device will go unhealthy", device.ID)
					unhealthyList = append(unhealthyList, device)
				} else if health == consts.HEALTH_PASS {
					if device.Health == pluginapi.Unhealthy {
						logs.Info("WatchHealth: Health check recover on Device=%s, the device will go healthy", device.ID)
						unhealthyList = append(unhealthyList, device)
					} else if _, ok := allLockedMap[realID]; ok {
						device.Health = pluginapi.Unhealthy
						logs.Info("WatchHealth: Health check recover on Device=%s, the device will go healthy with shared device all locked",
							device.ID)
						unhealthyList = append(unhealthyList, device)
					}
				}
			}
			allLockedMap = map[string]struct{}{}
			if len(unhealthyList) > 0 {
				unhealthyChanList <- unhealthyList
			}
		}

		//Sleep 5 second
		time.Sleep(5 * time.Second)
	}
}

func deviceHealthCheck(busID string) consts.DeviceState {
	checkPath := filepath.Join(consts.PCIDevicePath, busID)
	if !utils.FileIsExist(checkPath) {
		logs.Warn("health check for device pci path: %s not exist", checkPath)
		return consts.HEALTH_FAIL
	}
	return consts.HEALTH_PASS
}
