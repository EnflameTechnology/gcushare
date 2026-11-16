// Copyright (c) 2025 Enflame. All Rights Reserved.

package efids

import (
	"fmt"
	"os"
)

type GCUDevice struct {
	Vendor       string `json:"Vendor"`
	AsicName     string `json:"AsicName"`
	ProductModel string `json:"ProductModel"`
	SipNum       uint32 `json:"SipNum"`
	L3Size       uint32 `json:"L3Size(GB)"` //GB
	L2Size       uint32 `json:"L2Size(MB)"` //MB
}

// Vendor Production Device ID Definitions
var VendorIDS = map[string]map[string]GCUDevice{
	"0x1e36": {
		"0x0001": {"Enflame", "Leo", "T10", 0, 0, 0},
		"0x0002": {"Enflame", "Leo", "T11", 0, 0, 0},
		"0x0003": {"Enflame", "Leo", "T10(QSFP-DD)", 0, 0, 0},
		"0x8011": {"Enflame", "Vela", "i10", 0, 0, 0},
		"0x8012": {"Enflame", "Vela", "i10L", 0, 0, 0},
		"0x0021": {"Enflame", "Pavo", "T20", 0, 0, 0},
		"0x0022": {"Enflame", "Pavo", "T20-M64", 0, 0, 0},
		"0x0023": {"Enflame", "Pavo", "T21", 0, 0, 0},
		"0x0024": {"Enflame", "Pavo", "T21-M64", 0, 0, 0},
		"0x8001": {"Enflame", "Dorado", "i20", 0, 0, 0},
		"0x8002": {"Enflame", "Dorado", "i21", 0, 0, 0},
		"0x8031": {"Enflame", "Scorpio X1", "S6-M16", 0, 0, 0},
		"0x8032": {"Enflame", "Scorpio X1", "S6", 0, 0, 0},
		"0xc031": {"Enflame", "Scorpio X2", "S60-M24", 0, 0, 0},
		"0xc032": {"Enflame", "Scorpio X2S", "S90", 0, 0, 0},
		"0xc033": {"Enflame", "Scorpio X2", "S60G", 24, 48, 64},
		"0xc034": {"Enflame", "Scorpio X2S", "S90-M24", 0, 0, 0},
		"0xc035": {"Enflame", "Scorpio X2", "S60", 24, 48, 64},
		"0xc041": {"Enflame", "Libra", "L300", 0, 64, 0},
		"0xc042": {"Enflame", "Libra", "L600", 0, 96, 0},
		"0xc043": {"Enflame", "Libra", "L900", 0, 144, 0},
	},
	"0x1ea0": {
		"0x2a16": {"Cape", "X",    "C100", 0, 0, 0},
		"0x2a22": {"Cape", "C200", "C200", 24, 48, 64},
	},
}

func GetGCUInfoByVendorIDS() ([]GCUDevice, error) {
	pciPath := "/sys/bus/pci/devices"
	devices, err := os.ReadDir(pciPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read the PCI device directory: %v", err)
	}

	var gcuList []GCUDevice
	for _, device := range devices {
		gcuDevice, err := getGCUDevice(pciPath, device)
		if err != nil {
			return nil, err
		}
		if gcuDevice == nil {
			continue
		}
		gcuList = append(gcuList, *gcuDevice)
	}
	return gcuList, nil
}
