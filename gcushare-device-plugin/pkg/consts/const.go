// Copyright (c) 2022 Enflame. All Rights Reserved.

package consts

import (
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

type DeviceState uint32

const (
	/* for log */
	LOGPATH  = "/var/log/enflame/gcushare/gcushare-device-plugin.log"
	LOGINFO  = "INFO"
	LOGERROR = "ERROR"
	LOGWARN  = "WARNING"

	/* for component info*/
	COMPONENT_NAME = "gcushare-device-plugin"

	/* for resource */
	ResourceName = "enflame.com/shared-gcu"
	CountName    = "enflame.com/gcu-count"
	ServerSock   = pluginapi.DevicePluginPath + "enflamegcushare.sock"

	// for device health check
	PCIDevicePath                      = "/sys/bus/pci/devices"
	DEVICE_UNAVAILABLE                 = "0"
	EnvDisableHealthChecks             = "DP_DISABLE_HEALTHCHECKS"
	AllHealthChecks                    = "all"
	HEALTH_PASS            DeviceState = 0 // Success
	HEALTH_WARN            DeviceState = 1 // WARN
	HEALTH_DEBUG           DeviceState = 2 // DEBUG
	HEALTH_FAIL            DeviceState = 3 // FAIL

	DEVICE_COUNT_MAX   = 128
	DEVICE_PATH_PREFIX = "/dev"
	DEVICE_TYPE        = "gcu"

	// response env
	ENFLAME_VISIBLE_DEVICES             = "ENFLAME_VISIBLE_DEVICES"
	ENFLAME_CONTAINER_SUB_CARD          = "ENFLAME_CONTAINER_SUB_CARD"
	ENFLAME_CONTAINER_USABLE_PROCESSOR  = "ENFLAME_CONTAINER_USABLE_PROCESSOR"
	ENFLAME_CONTAINER_USABLE_SHARED_MEM = "ENFLAME_CONTAINER_USABLE_SHARED_MEM"
	ENFLAME_CONTAINER_USABLE_GLOBAL_MEM = "ENFLAME_CONTAINER_USABLE_GLOBAL_MEM"

	// annotations
	PodRequestGCUSize     = "enflame.com/gcu-request-size"
	PodAssignedGCUID      = "enflame.com/gcu-assigned-id"
	PodHasAssignedGCU     = "enflame.com/gcu-assigned"
	PodAssignedGCUTime    = "enflame.com/gcu-assigned-time"
	GCUSharedCapacity     = "enflame.com/gcu-shared-capacity"
	PodAssignedContainers = "assigned-containers"
	PatchCount            = GCUSharedCapacity + "-patch-count"

	// other
	RecommendedMaxSliceCount     = 6
	RecommendedKubeConfigPathEnv = "KUBECONFIG"
	ResourceIsolationLabel       = "enflame.com/gcushare-resource-isolation"
	MaxRetryTimes                = 30
)
