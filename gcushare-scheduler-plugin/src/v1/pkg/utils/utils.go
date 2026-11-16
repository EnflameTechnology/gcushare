// Copyright (c) 2022 Enflame. All Rights Reserved.
package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/ghodss/yaml"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"gcushare-scheduler-plugin/pkg/logs"
)

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

func RemovePodManagedFields(pod *v1.Pod) *v1.Pod {
	pod.ManagedFields = []metav1.ManagedFieldsEntry{}
	return pod
}

func ReadYamlToJson(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		logs.Error(err, "read file: %s failed", filePath)
		return "", err
	}
	jsonContent, err := yaml.YAMLToJSON(content)
	if err != nil {
		logs.Error(err, "convert file: %s yaml to json failed", filePath)
		return "", err
	}
	return string(jsonContent), nil
}

func CopyFile(sourceFile, targetFile string) error {
	sourceFileContent, err := os.Open(sourceFile)
	if err != nil {
		logs.Error(err, "open source file: %s failed", sourceFile)
		return err
	}
	defer sourceFileContent.Close()

	targetFileContent, err := os.Create(targetFile)
	if err != nil {
		logs.Error(err, "open or create target file: %s failed", targetFile)
		return err
	}
	defer targetFileContent.Close()

	_, err = io.Copy(targetFileContent, sourceFileContent)
	if err != nil {
		logs.Error(err, "copy %s to %s failed", sourceFile, targetFile)
		return err
	}

	logs.Info("copy %s to %s success", sourceFile, targetFile)
	return nil
}

func FileIsExist(file string) bool {
	_, err := os.Stat(file)
	return err == nil || os.IsExist(err)
}

func HasValue(list []string, value string) bool {
	for _, each := range list {
		if value == each {
			return true
		}
	}
	return false
}
