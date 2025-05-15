// Copyright 2022 Enflame. All Rights Reserved.
package resource

import (
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"

	"gcushare-device-plugin/pkg/config"
	"gcushare-device-plugin/pkg/kube"
	"gcushare-device-plugin/pkg/logs"
)

type ClusterResource struct {
	ClientSet    *kubernetes.Clientset
	NodeName     string
	Config       *config.Config
	ResourceName string
}

func NewClusterResource(config *config.Config, resourceName string) (*ClusterResource, error) {
	clientSet, err := kube.GetKubeClient()
	if err != nil {
		return nil, err
	}
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		err := fmt.Errorf("please set env NODE_NAME")
		logs.Error(err)
		return nil, err
	}
	return &ClusterResource{
		ClientSet:    clientSet,
		NodeName:     nodeName,
		Config:       config,
		ResourceName: resourceName,
	}, nil
}
