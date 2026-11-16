package efids

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

func pathIsExist(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func getEnflameVendorIDAndDeviceID(devicePath string) (string, string) {
	classFile := filepath.Join(devicePath, "class")
	classBytes, err := os.ReadFile(classFile)
	if err != nil {
		fmt.Printf("Failed to read class file: %v", err)
		return "", ""
	}

	// processing accelerators （Class 0x120000)
	// see https://admin.pci-ids.ucw.cz/read/PD
	classCode := strings.TrimSpace(string(classBytes))
	if !strings.HasPrefix(classCode, "0x12") {
		return "", ""
	}

	vendorFile := filepath.Join(devicePath, "vendor")
	vendorBytes, err := os.ReadFile(vendorFile)
	if err != nil {
		fmt.Printf("Failed to read vendor file: %v", err)
		return "", ""
	}
	vendorID := strings.TrimSpace(string(vendorBytes))

	if vendorID != "0x1e36" && vendorID != "0x1ea0" {
		return "", ""
	}
	deviceBytes, err := os.ReadFile(filepath.Join(devicePath, "device"))
	if err != nil {
		fmt.Printf("Failed to read device file: %v", err)
		return "", ""
	}
	deviceID := strings.TrimSpace(string(deviceBytes))
	return vendorID, deviceID
}

func getGCUDevice(pciPath string, device os.DirEntry) (*GCUDevice, error) {
	devicePath := filepath.Join(pciPath, device.Name())
	vendorID, deviceID := getEnflameVendorIDAndDeviceID(devicePath)
	if vendorID == "" {
		return nil, nil
	}
	gcuDevice, exists := VendorIDS[strings.ToLower(vendorID)][strings.ToLower(deviceID)]
	if !exists {
		return nil, fmt.Errorf("internal error! device(vendorID: %s, deviceID: %s) not found in VendorIDS dict",
			strings.ToLower(vendorID), strings.ToLower(deviceID))
	}
	return &gcuDevice, nil
}

func isValidGCUString(s string, gcu string) bool {
	if !strings.HasPrefix(s, gcu) {
		return false
	}
	// get char after "gcu" in "gcuXXXXX"
	suffix := s[3:]
	if len(suffix) == 0 {
		return false
	}
	for _, char := range suffix {
		if !unicode.IsDigit(char) {
			return false
		}
	}

	return true
}

func getDeviceInfo(pciPath string, device os.DirEntry) (*DeviceInfo, error) {
	config, err := ParseConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}
	devicePath := filepath.Join(pciPath, device.Name())
	vendorID, deviceID := getEnflameVendorIDAndDeviceID(devicePath)
	if vendorID == "" {
		return nil, nil
	}

	uuidPath := filepath.Join(devicePath, config.UUIDPath[0])
	pathExist, err := pathIsExist(uuidPath)
	if err != nil || !pathExist {
		fmt.Printf("Failed to read ssm path: %v", err)
		return nil, nil
	}
	uuidBytes, err := os.ReadFile(filepath.Join(uuidPath, config.UUIDFile[0]))
	if err != nil {
		fmt.Printf("Failed to read ssm chipid file: %v", err)
		return nil, nil
	}
	uuid := strings.TrimSpace(string(uuidBytes))

	vendorName := config.Vendor[0]
	if vendorID == "0x1ea0" {
		vendorName = config.Vendor[1]
	}

	// Dynamically find all gcu* directories
	vendorPath := filepath.Join(devicePath, vendorName)
	pathExist, err = pathIsExist(vendorPath)
	if err != nil || !pathExist {
		fmt.Printf("Path not exist: %v", err)
		return nil, nil
	}

	gcuDirs, err := os.ReadDir(vendorPath)
	if err != nil {
		return nil, nil
	}

	var devName string = ""
	var major, minor int = -1, -1
	for _, gcuDir := range gcuDirs {
		if isValidGCUString(gcuDir.Name(), config.IDPrefix[0]) {
			gcuPath := filepath.Join(vendorPath, gcuDir.Name())
			// Read dev file and parse major and minor numbers
			devPath := filepath.Join(gcuPath, config.IDFile[0])
			devBytes, err := os.ReadFile(devPath)
			if err != nil {
				fmt.Printf("Failed to read dev file: %v", err)
				continue
			}

			devParts := strings.Split(strings.TrimSpace(string(devBytes)), ":")
			if len(devParts) != 2 {
				return nil, fmt.Errorf("invalid device number format: %q", devParts)
			}

			major64, err := strconv.ParseInt(devParts[0], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid major number: %v", err)
			}
			major = int(major64)

			minor64, err := strconv.ParseInt(devParts[1], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid minor number: %v", err)
			}
			minor = int(minor64)
			devName = fmt.Sprintf("/dev/%s%d", config.IDPrefix[0], minor)

			// only one gcuN dir used
			return &DeviceInfo{
				VendorID: vendorID,
				DeviceID: deviceID,
				BusID:    device.Name(),
				UUID:     uuid,
				Major:    major,
				Minor:    minor,
				DevName:  devName,
			}, nil
		}
	}
	return nil, nil
}
