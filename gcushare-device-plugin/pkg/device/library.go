// Copyright (c) 2025, ENFLAME INC.  All rights reserved.

package device

import (
	"encoding/json"

	"gcushare-device-plugin/pkg/efpci"
	"gcushare-device-plugin/pkg/logs"
	"gcushare-device-plugin/pkg/smi"
)

type DeviceFullInfo struct {
	smi.SmiDeviceInfo
	efpci.PciDeviceInfo
}

func GetDeviceInfoFromSmiAndPci() ([]DeviceFullInfo, error) {
	smiInfos, err := smi.GetDeviceInfoFromSmi()
	if err != nil {
		return nil, err
	}
	devices := []DeviceFullInfo{}
	for _, smiInfo := range smiInfos {
		pciInfo, err := efpci.GetGCUInfoByBusID(smiInfo.SmiBusID)
		if err != nil {
			return nil, err
		}
		devices = append(devices, DeviceFullInfo{
			SmiDeviceInfo: smiInfo,
			PciDeviceInfo: *pciInfo,
		})
	}
	jsonData, err := json.MarshalIndent(devices, "", "  ")
	if err != nil {
		logs.Error(err, "json marshal failed, content: %v", devices)
		return nil, err
	}
	logs.Info("get devices information from efsmi:\n%s", jsonData)
	return devices, nil
}
