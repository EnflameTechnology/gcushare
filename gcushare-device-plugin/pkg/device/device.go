// Copyright 2022 Enflame. All Rights Reserved.
package device

import (
	"fmt"
	"strconv"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"

	"gcushare-device-plugin/pkg/config"
	"gcushare-device-plugin/pkg/consts"
	"gcushare-device-plugin/pkg/logs"
)

type Device struct {
	Config            *config.Config
	Count             int
	SliceCount        int
	ResourceIsolation bool
	Info              map[string]DeviceInfo
}

type DeviceInfo struct {
	BusID  string
	Path   string
	Sip    int
	L2     int
	Memory int
}

func NewDevice(sliceCount int, config *config.Config, resourceIsolation bool) (*Device, error) {
	logs.Info("each device will be shared into %d with SliceCount=%d", sliceCount, sliceCount)
	device := &Device{
		Config:            config,
		SliceCount:        sliceCount,
		ResourceIsolation: resourceIsolation,
		Info:              map[string]DeviceInfo{},
	}
	if err := device.buildDeviceInfo(); err != nil {
		return nil, err
	}
	return device, nil
}

func (rer *Device) GetFakeDevices() ([]*pluginapi.Device, map[string]string) {
	fakeDeviceCount := len(rer.Info) * rer.SliceCount
	deviceList := make([]*pluginapi.Device, 0, fakeDeviceCount)
	deviceMap := make(map[string]string, fakeDeviceCount)
	for index := range rer.Info {
		for sliceIndex := 0; sliceIndex < rer.SliceCount; sliceIndex++ {
			fakeID := generateFakeDeviceID(index, sliceIndex)
			if sliceIndex == 0 {
				logs.Info("generate first fake device id: %s for device: %s", fakeID, index)
			}
			if sliceIndex == rer.SliceCount-1 {
				logs.Info("generate last fake device id: %s for device: %s", fakeID, index)
			}
			deviceList = append(deviceList, &pluginapi.Device{
				ID:     fakeID,
				Health: pluginapi.Healthy,
			})
			deviceMap[fakeID] = pluginapi.Healthy
		}
	}
	logs.Info("generate fake device count: %d", len(deviceList))
	return deviceList, deviceMap
}

func (rer *Device) buildDeviceInfo() error {
	devType := rer.Config.DeviceType()
	gcuDevicesInfo, err := GetDeviceInfoFromSmiAndPci()
	if err != nil {
		logs.Error(err, "get gcu devices info by pci failed")
		return err
	}
	logs.Info("found %s device count: %d from efsmi", devType, len(gcuDevicesInfo))
	for _, gcuDevice := range gcuDevicesInfo {
		L3Memory, err := strconv.Atoi(gcuDevice.L3Memory)
		if err != nil {
			logs.Error(err, "device(minor: %s, index: %s) L3 memory must be int type, found: %s", gcuDevice.Minor,
				gcuDevice.Index, gcuDevice.L3Memory)
			return err
		}
		L3Memory /= 1024
		devPath := fmt.Sprintf("%s/%s%s", consts.DEVICE_PATH_PREFIX, devType, gcuDevice.Minor)
		sipNum, L2Memory := 24, 64 // Hardcoded for now, since it cannot be queried from efsmi
		rer.Info[gcuDevice.Minor] = DeviceInfo{
			Path:   devPath,
			BusID:  gcuDevice.PCIBusID,
			Sip:    sipNum,
			L2:     L2Memory,
			Memory: L3Memory,
		}
		logs.Info("found device %s%s(minor: %s, path: %s, type: %s, memory: %dGB, sip: %d, L2: %dMB)", devType,
			gcuDevice.Minor, gcuDevice.Minor, devPath, gcuDevice.Model, L3Memory, sipNum, L2Memory)
	}
	rer.Count = len(rer.Info)
	return nil
}

func generateFakeDeviceID(deviceID string, sliceIndex int) string {
	return fmt.Sprintf("%s-%d", deviceID, sliceIndex)
}
