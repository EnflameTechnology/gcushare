package smi

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"

	"gcushare-device-plugin/pkg/utils"
)

type SmiDeviceInfo struct {
	Index    string `json:"index"`
	Vendor   string `json:"vendor"`
	Model    string `json:"model"`
	SmiBusID string `json:"smiBusID"`
	L3Memory string `json:"L3Memory"`
	GCUVirt  string `json:"gcuVirt"`
}

func GetDeviceInfoFromSmi() ([]SmiDeviceInfo, error) {
	cmd := "efsmi"
	output, err := utils.ExecCommand("sh", "-c", cmd)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(bytes.NewReader([]byte(output)))
	var lines []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.Contains(line, "-") {
			lines = append(lines, line)
		}
	}
	busPattern := regexp.MustCompile(`[0-9a-fA-F]{2}:[0-9a-fA-F]{2}:[0-9a-fA-F]{2}\.[0-7]`)
	var devices []SmiDeviceInfo

	for i := 0; i < len(lines)-1; i++ {
		line := lines[i]
		if busPattern.MatchString(line) {
			line = strings.ReplaceAll(line, "|", "")
			curLineFields := strings.Fields(line)
			nextLine := lines[i+1]
			nextLine = strings.ReplaceAll(nextLine, "|", "")
			nextLine = strings.ReplaceAll(nextLine, "/", "")
			nextLineFields := strings.Fields(nextLine)
			busid := curLineFields[4]
			devices = append(devices, SmiDeviceInfo{
				Index:    curLineFields[0],
				Vendor:   curLineFields[1],
				Model:    curLineFields[2],
				SmiBusID: busid,
				L3Memory: strings.Split(nextLineFields[4], "MiB")[0],
				GCUVirt:  nextLineFields[5],
			})
			i++
		}
	}
	return devices, nil
}
