package smi

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"gcushare-device-plugin/pkg/consts"
	"gcushare-device-plugin/pkg/logs"
	"gcushare-device-plugin/pkg/utils"
)

type SmiDeviceInfo struct {
	Index    string `json:"index"`
	Product  string `json:"product"`
	SmiBusID string `json:"smiBusID"`
	GCUVirt  string `json:"gcuVirt"`
}

type Profile struct {
	Index         string `json:"index"`
	ProfileName   string `json:"profileName"`
	ProfileID     string `json:"profileID"`
	InstanceCount string `json:"instanceCount"`
	Memory        string `json:"memory"`
	Sip           string `json:"sip"`
}

type Instance struct {
	Index       string `json:"index"`
	ProfileName string `json:"profileName"`
	ProfileID   string `json:"profileID"`
	InstanceID  string `json:"instanceID"`
	Placement   string `json:"placement"`
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
			line = strings.Trim(line, "|")
			partOne := strings.Fields(strings.Split(line, "|")[0])
			// partTwo := strings.Fields(strings.Split(line, "|")[1])
			partThr := strings.Fields(strings.Split(line, "|")[2])
			product := strings.Join(partOne[1:], "")

			nextLine := lines[i+1]
			nextLine = strings.ReplaceAll(nextLine, "|", "")
			nextLine = strings.ReplaceAll(nextLine, "/", "")
			nextLineFields := strings.Fields(nextLine)
			devices = append(devices, SmiDeviceInfo{
				Index:    partOne[0],
				Product:  product,
				SmiBusID: partThr[0],
				GCUVirt:  nextLineFields[5],
			})
			i++
		}
	}
	return devices, nil
}

func ListProfile(index string) ([]Profile, error) {
	cmd := "efsmi --drs --list-profile"
	if index != "" {
		cmd = fmt.Sprintf("efsmi -i %s --drs --list-profile", index)
	}
	output, err := utils.ExecCommand("sh", "-c", cmd)
	if err != nil {
		return nil, err
	}
	profilePattern := regexp.MustCompile(consts.ProfileNameRegExp)
	scanner := bufio.NewScanner(bytes.NewReader([]byte(output)))
	profileList := []Profile{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !profilePattern.MatchString(line) {
			continue
		}
		line = strings.Trim(line, "|")
		fieldsList := strings.Fields(line)
		profileList = append(profileList, Profile{
			Index:         fieldsList[0],
			ProfileName:   fieldsList[1],
			ProfileID:     fieldsList[2],
			InstanceCount: fieldsList[3],
			Memory:        fieldsList[4],
			Sip:           fieldsList[5],
		})
	}
	return profileList, nil
}

func ListInstance(index string) ([]Instance, error) {
	cmd := "efsmi --drs --list-instance"
	if index != "" {
		cmd = fmt.Sprintf("efsmi -i %s --drs --list-instance", index)
	}
	output, err := utils.ExecCommand("sh", "-c", cmd)
	if err != nil {
		return nil, err
	}
	profilePattern := regexp.MustCompile(consts.ProfileNameRegExp)
	scanner := bufio.NewScanner(bytes.NewReader([]byte(output)))
	instanceList := []Instance{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !profilePattern.MatchString(line) {
			continue
		}
		line = strings.Trim(line, "|")
		fieldsList := strings.Fields(line)
		instanceList = append(instanceList, Instance{
			Index:       fieldsList[0],
			ProfileName: fieldsList[1],
			ProfileID:   fieldsList[2],
			InstanceID:  fieldsList[3],
			Placement:   fieldsList[4],
		})
	}
	return instanceList, nil
}

func OpenDRS(index string) error {
	cmd := "efsmi --drs on"
	if index != "" {
		cmd = fmt.Sprintf("efsmi -i %s --drs on", index)
	}
	_, err := utils.ExecCommand("sh", "-c", cmd)
	if err != nil {
		return err
	}
	return nil
}

func CloseDRS(index string) error {
	cmd := "efsmi --drs off"
	if index != "" {
		cmd = fmt.Sprintf("efsmi -i %s --drs off", index)
	}
	_, err := utils.ExecCommand("sh", "-c", cmd)
	if err != nil {
		return err
	}
	return nil
}

func CreateInstance(index, profileID string) error {
	cmd := fmt.Sprintf("efsmi --drs --create-instance %s", profileID)
	if index != "" {
		cmd = fmt.Sprintf("efsmi -i %s --drs --create-instance %s", index, profileID)
	}
	_, err := utils.ExecCommand("sh", "-c", cmd)
	if err != nil {
		return err
	}
	return nil
}

func DeleteInstance(index, instanceID string) error {
	exist, err := InstanceExist(index, instanceID)
	if err != nil {
		return err
	}
	if !exist {
		logs.Warn("device index: %s instance: %s not found, need not to delete", index, instanceID)
		return nil
	}
	cmd := fmt.Sprintf("efsmi --drs --destroy-instance %s", instanceID)
	if index != "" {
		cmd = fmt.Sprintf("efsmi -i %s --drs --destroy-instance %s", index, instanceID)
	}
	_, err = utils.ExecCommand("sh", "-c", cmd)
	if err != nil {
		return err
	}
	return nil
}

func InstanceExist(index, instanceID string) (bool, error) {
	instanceList, err := ListInstance(index)
	if err != nil {
		return false, err
	}
	for _, instance := range instanceList {
		if instance.InstanceID == instanceID {
			return true, nil
		}
	}
	return false, nil
}
