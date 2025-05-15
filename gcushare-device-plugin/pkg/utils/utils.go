// Copyright 2022 Enflame. All Rights Reserved.
package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"

	"gcushare-device-plugin/pkg/logs"
)

func StackTrace(all bool) string {
	buf := make([]byte, 10240)

	for {
		size := runtime.Stack(buf, all)

		if size == len(buf) {
			buf = make([]byte, len(buf)<<1)
			continue
		}
		break

	}

	return string(buf)
}

func Coredump(fileName string) {
	logs.Info("Dump stacktrace to %s", fileName)
	os.WriteFile(fileName, []byte(StackTrace(true)), 0644)
}

func ConvertToString(value interface{}) string {
	byteVal, err := json.Marshal(value)
	if err != nil {
		logs.Error(err, "marshal value:%v to string failed", value)
		return fmt.Sprintf("%v", value)
	}
	return string(byteVal)
}

func JsonMarshalIndent(value interface{}) string {
	byteVal, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		logs.Error(err, "marshal value:%v to string failed", value)
		return fmt.Sprintf("%v", value)
	}
	return string(byteVal)
}

func FileIsExist(file string) bool {
	_, err := os.Stat(file)
	return err == nil || os.IsExist(err)
}

func GetDeviceCapacityMap(devices []*pluginapi.Device) map[string]int {
	deviceCapacityMap := map[string]int{}
	for _, fakeDev := range devices {
		realID := strings.Split(fakeDev.ID, "-")[0]
		deviceCapacityMap[realID] = deviceCapacityMap[realID] + 1
	}
	return deviceCapacityMap
}

func ExecCommand(name string, args ...string) (string, error) {
	maxRetryTimes := 5
	for i := 0; i < maxRetryTimes; i++ {
		var stdoutBuf, stderrBuf bytes.Buffer
		cmd := exec.Command(name, args...)
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf
		if err := cmd.Run(); err != nil {
			logs.Error(err, "execute command: '%s %s' failed, retry times: %d, detail: %s", name,
				strings.Join(args, " "), i, stderrBuf.String())
			time.Sleep(1 * time.Second)
			continue
		}
		return stdoutBuf.String(), nil
	}
	err := fmt.Errorf("execute command: '%s %s' failed with max retry times: %d",
		name, strings.Join(args, " "), maxRetryTimes)
	logs.Error(err)
	return "", err
}
