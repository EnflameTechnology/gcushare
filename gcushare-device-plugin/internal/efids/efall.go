// Copyright (c) 2025 Enflame. All Rights Reserved.

package efids

import (
	"fmt"
	"os"
)

type DeviceFullInfo struct {
	DeviceInfo
	GCUDevice
}

func GetDeviceFullInfos() ([]DeviceFullInfo, error) {
	pciPath := "/sys/bus/pci/devices"
	devices, err := os.ReadDir(pciPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read the PCI device directory: %v", err)
	}

	var gcuList []DeviceFullInfo
	for _, device := range devices {
		gcuDevice, err := getGCUDevice(pciPath, device)
		if err != nil {
			return nil, err
		}
		if gcuDevice == nil {
			continue
		}
		deviceInfo, err := getDeviceInfo(pciPath, device)
		if err != nil {
			return nil, err
		}
		if deviceInfo == nil {
			continue
		}

		gcuList = append(gcuList, DeviceFullInfo{
			DeviceInfo: *deviceInfo,
			GCUDevice:  *gcuDevice,
		})
	}
	return gcuList, nil
}
