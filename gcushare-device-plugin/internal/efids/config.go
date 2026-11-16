// Copyright (c) 2025 Enflame. All Rights Reserved.

package efids

import (
	// "encoding/json"
	// "fmt"
	// "os"
)

type Config struct {
	Version  string   `json:"version"`
	Vendor   []string `json:"vendor"`
	IDPrefix []string `json:"idPrefix"`
	IDFile   []string `json:"idFile"`
	UUIDPath []string `json:"uuidPath"`
	UUIDFile []string `json:"uuidFile"`
}

func ParseConfig() (*Config, error) {
	// configFile := "/etc/topcloud/topscloud.json"
	// content, err := os.ReadFile(configFile)
	// if err != nil {
	// 	return nil, fmt.Errorf("Failed to read config file: %w", err)
	// }

	// var config Config
	// if err := json.Unmarshal(content, &config); err != nil {
	// 	return nil, fmt.Errorf("Failed to parse config file: %w", err)
	// }

	config := Config{
		Version:  "v1",
		Vendor:   []string{"enflame", "zixiao"},
		IDPrefix: []string{"gcu"},
		IDFile:   []string{"dev"},
		UUIDPath: []string{"ssm"},
		UUIDFile: []string{"chipid"},
	}

	return &config, nil
}
