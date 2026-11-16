// Copyright (c) 2024, ENFLAME INC.  All rights reserved.

package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gcushare-scheduler-plugin/pkg/consts"
	"gcushare-scheduler-plugin/pkg/logs"
	"gcushare-scheduler-plugin/pkg/utils"
)

const (
	TopscloudPath     string = "/etc/topscloud/"
	defaultConfigPath string = "/tmp/"
	releaseConfigPath string = "./config/"
	ConfigFileName    string = "topscloud.json"
	devicePath        string = "/dev/"
)

type Config struct {
	Version          string   `json:"version"`
	RegisterResource []string `json:"registerResource"`
	MainDevice       []string `json:"mainDevice"`
	VirtualDevice    []string `json:"virtualDevice"`
	DriverDevice     []string `json:"driverDevice"`
	UtilityBins      []string `json:"utilityBins"`
	UtilityLibs      []string `json:"utilityLibs"`
}

// example: enflame.com
func (conf *Config) Domain() string {
	return strings.Split(conf.RegisterResource[0], "/")[0]
}

// example: gcu
func (conf *Config) DeviceType() string {
	return strings.Split(conf.RegisterResource[0], "/")[1]
}

// example: enflame.com/shared-gcu
func (conf *Config) ResourceName(drsEnabled bool) string {
	if drsEnabled {
		return conf.Domain() + "/drs-" + conf.DeviceType()
	}
	return conf.Domain() + "/shared-" + conf.DeviceType()
}

// example: /dev/gcu
func (conf *Config) DeviceName() string {
	return devicePath + conf.DeviceType()
}

func (conf *Config) DriverDevices() []string {
	return conf.DriverDevice
}

func (conf *Config) ReplaceDomain(value string) string {
	return strings.ReplaceAll(value, "enflame.com", conf.Domain())
}

func (conf *Config) ReplaceDeviceType(value string) string {
	return strings.ReplaceAll(value, "gcu", conf.DeviceType())
}

func (conf *Config) ReplaceResource(value string) string {
	value = conf.ReplaceDomain(value)
	return conf.ReplaceDeviceType(value)
}

// GetConfig read config file from /etc/topscloud/topscloud.json
func GetConfig() (*Config, error) {
	// if the config file does not exist, read from the /tmp directory, and init it to /etc/topscloud
	configFilePath := TopscloudPath + ConfigFileName
	if !utils.FileIsExist(configFilePath) {
		logs.Info("topscloud config file: %s is not exist", configFilePath)
		fileInitPath := defaultConfigPath + ConfigFileName
		if !utils.FileIsExist(fileInitPath) {
			fileInitPath = releaseConfigPath + ConfigFileName
			logs.Warn("init config file: %s not exist, use: %s", defaultConfigPath+ConfigFileName, releaseConfigPath+ConfigFileName)
		}
		if err := copyFile(fileInitPath, configFilePath); err != nil {
			return nil, err
		}
		logs.Info("init config file: %s from %s success", configFilePath, fileInitPath)
	} else {
		logs.Info("found already exist config file: %s", configFilePath)
	}

	content, err := os.ReadFile(configFilePath)
	if err != nil {
		logs.Error(err, "read config file: %s failed", configFilePath)
		return nil, err
	}
	logs.Info("run %s use config:\n%s", consts.COMPONENT_NAME, string(content))
	config := new(Config)
	if err := json.Unmarshal(content, config); err != nil {
		logs.Error(err, "json unmarshal content: %s to struct failed", string(content))
		return nil, err
	}
	// if registerResource in config file is invalid, replace it by default value
	if len(config.RegisterResource) == 0 {
		defaultResource := []string{strings.ReplaceAll(consts.SharedResourceName, "shared-", "")}
		logs.Warn("registerResource: %v in config file is invalid, replace it by: %v",
			config.RegisterResource, defaultResource)
		config.RegisterResource = defaultResource
	}
	if len(strings.Split(config.RegisterResource[0], "/")) != 2 {
		err := fmt.Errorf("registerResource: %v in config file is invalid", config.RegisterResource[0])
		logs.Error(err)
		return nil, err
	}
	return config, nil
}

func copyFile(src, dst string) error {
	// open the source file
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		logs.Error(err, "os stat source file: %s", src)
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("error: %s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		logs.Error(err, "open the source file: %s", src)
		return err
	}
	defer source.Close()

	// special scenario: consider the case where the specified YAML file is not used
	dir, _ := filepath.Split(dst)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logs.Error(err, "create config file parents folders: %s failed", dir)
		return err
	}
	// create the destination file
	destination, err := os.Create(dst)
	if err != nil {
		logs.Error(err, "create the destination file: %s failed", dst)
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	if err != nil {
		logs.Error(err, "copy the source file: %s to destination file: %s failed", src, dst)
		return err
	}
	return nil
}
