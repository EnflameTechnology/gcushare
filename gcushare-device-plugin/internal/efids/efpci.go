// Copyright (c) 2025 Enflame. All Rights Reserved.

package efids

import (
	"fmt"
	"os"
)

// DeviceInfo defines the structure to hold PCI device information
type DeviceInfo struct {
	VendorID string `json:"VersionID"`
	DeviceID string `json:"DeviceID"`
	BusID    string `json:"BusID"`
	UUID     string `json:"UUID"`
	Major    int    `json:"Major"`   // major number
	Minor    int    `json:"Minor"`   // minor number
	DevName  string `json:"DevName"` // /dev/gcuN
}

// parse path, example : /sys/bus/pci/devices/0000:01:00.0/enflame/gcu0
func GetGCUInfoByPciBusID() ([]DeviceInfo, error) {
	pciPath := "/sys/bus/pci/devices"
	devices, err := os.ReadDir(pciPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read the PCI device directory: %v", err)
	}

	var gcuList []DeviceInfo
	for _, device := range devices {
		deviceInfo, err := getDeviceInfo(pciPath, device)
		if err != nil {
			return nil, err
		}
		if deviceInfo == nil {
			continue
		}
		gcuList = append(gcuList, *deviceInfo)
	}
	return gcuList, nil
}
