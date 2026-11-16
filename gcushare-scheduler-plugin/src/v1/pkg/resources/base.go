// Copyright (c) 2025, ENFLAME INC.  All rights reserved.
package resources

import (
	"k8s.io/client-go/kubernetes"

	"gcushare-scheduler-plugin/pkg/config"
)

type BaseResource struct {
	SharedResourceName string
	DRSResourceName    string
	Config             *config.Config
	Clientset          *kubernetes.Clientset
}

func NewBaseResource(config *config.Config, clientset *kubernetes.Clientset) *BaseResource {
	return &BaseResource{
		SharedResourceName: config.ResourceName(false),
		DRSResourceName:    config.ResourceName(true),
		Config:             config,
		Clientset:          clientset,
	}
}
