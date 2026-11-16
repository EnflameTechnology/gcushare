package efpci

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gcushare-device-plugin/pkg/consts"
	"gcushare-device-plugin/pkg/logs"
	"gcushare-device-plugin/pkg/utils"
)

type PciDeviceInfo struct {
	Major    string `json:"major"`
	Minor    string `json:"minor"`
	PCIBusID string `json:"pciBusID"`
}

func GetGCUInfoByBusID(busid string) (*PciDeviceInfo, error) {
	devices, err := os.ReadDir(consts.PCIDevicePath)
	if err != nil {
		logs.Error(err, "failed to read the PCI device directory: %s", consts.PCIDevicePath)
		return nil, err
	}

	for _, device := range devices {
		if !strings.HasSuffix(device.Name(), busid) {
			continue
		}
		devicePath := filepath.Join(consts.PCIDevicePath, device.Name(), "enflame")
		if !utils.FileIsExist(devicePath) {
			devicePath = filepath.Join(consts.PCIDevicePath, device.Name(), "zixiao")
			if !utils.FileIsExist(devicePath) {
				err := fmt.Errorf("folder 'enflame' or 'zixiao' not found in path: %s",
					filepath.Join(consts.PCIDevicePath, device.Name()))
				logs.Error(err)
				return nil, err
			}
		}
		gcus, err := os.ReadDir(devicePath)
		if err != nil {
			logs.Error(err, "failed to read the directory: %s", devicePath)
			return nil, err
		}
		for _, gcuDir := range gcus {
			id := strings.Split(gcuDir.Name(), "gcu")[1]
			if _, err := strconv.Atoi(id); err != nil {
				continue
			}
			minorPath := filepath.Join(devicePath, gcuDir.Name(), "dev")
			content, err := os.ReadFile(minorPath)
			if err != nil {
				logs.Error(err, "failed to read the file: %s", minorPath)
				return nil, err
			}
			devParts := strings.Split(strings.TrimSpace(string(content)), ":")
			if len(devParts) != 2 {
				err := fmt.Errorf("invalid device number format: %s", string(content))
				return nil, err
			}
			PciDeviceInfo := &PciDeviceInfo{
				Major:    devParts[0],
				Minor:    devParts[1],
				PCIBusID: device.Name(),
			}
			return PciDeviceInfo, nil
		}
	}
	err = fmt.Errorf("can not found pci device information by busID: %s", busid)
	logs.Error(err)
	return nil, err
}
