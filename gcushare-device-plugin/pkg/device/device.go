// Copyright 2022 Enflame. All Rights Reserved.
package device

import (
	"fmt"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"

	"gcushare-device-plugin/pkg/config"
	"gcushare-device-plugin/pkg/consts"
	"gcushare-device-plugin/pkg/logs"
)

type Device struct {
	Config     *config.Config
	Count      int
	SliceCount int
	Info       map[string]DeviceInfo
}

type DeviceInfo struct {
	BusID string
	Path  string
}

func NewDevice(config *config.Config) (*Device, error) {
	device := &Device{
		Config: config,
		Info:   map[string]DeviceInfo{},
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
		devPath := fmt.Sprintf("%s/%s%s", consts.DEVICE_PATH_PREFIX, devType, gcuDevice.Minor)
		rer.Info[gcuDevice.Minor] = DeviceInfo{
			Path:  devPath,
			BusID: gcuDevice.PCIBusID,
		}
		logs.Info("found device %s%s(minor: %s, path: %s, product: %s, busid: %s)", devType, gcuDevice.Minor,
			gcuDevice.Minor, devPath, gcuDevice.Product, gcuDevice.PCIBusID)
	}
	rer.Count = len(rer.Info)
	return nil
}

func generateFakeDeviceID(deviceID string, sliceIndex int) string {
	return fmt.Sprintf("%s-%d", deviceID, sliceIndex)
}
