// Copyright (c) 2025, ENFLAME INC.  All rights reserved.

package plugin

import (
	"context"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	k8sTypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/scheduler/framework"

	"gcushare-scheduler-plugin/pkg/config"
	"gcushare-scheduler-plugin/pkg/resources"
)

func NewGCUShareSchedulerPlugin(config *config.Config, clientset *kubernetes.Clientset) (func(ctx context.Context,
	configuration runtime.Object, f framework.Handle) (framework.Plugin, error), error) {
	return func(ctx context.Context, configuration runtime.Object, f framework.Handle) (framework.Plugin, error) {
		// if configuration != nil {
		// 	codec := runtimejson.NewSerializerWithOptions(runtimejson.DefaultMetaFactory, nil, nil, runtimejson.SerializerOptions{})
		// 	buf, err := runtime.Encode(codec, configuration)
		// 	if err != nil {
		// 		return nil, fmt.Errorf("failed to encode configuration: %v", err)
		// 	}
		// }
		baseResource := resources.NewBaseResource(config, clientset)
		return &GCUShareSchedulerPlugin{
			mu:           new(sync.Mutex),
			resourceName: config.ResourceName(),
			clientset:    clientset,
			config:       config,
			filterCache:  make(map[k8sTypes.UID]filterResult),
			podResource:  resources.NewPodResource(baseResource),
			nodeResource: resources.NewNodeResource(baseResource),
		}, nil
	}, nil
}
