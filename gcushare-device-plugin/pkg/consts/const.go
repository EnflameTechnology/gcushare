// Copyright (c) 2022 Enflame. All Rights Reserved.

package consts

import (
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

type DeviceState uint32

const (
	/* for log */
	LOGPATH    = "/var/log/enflame/gcushare/gcushare-device-plugin.log"
	LOGINFO    = "INFO"
	LOGERROR   = "ERROR"
	LOGWARN    = "WARNING"
	LOGDEBUG   = "DEBUG"
	TimeFormat = "2006-01-02 15:04:05"

	/* for component info*/
	COMPONENT_NAME = "gcushare-device-plugin"

	/* for resource */
	DRSResourceName    = "enflame.com/drs-gcu"
	SharedResourceName = "enflame.com/shared-gcu"
	CountName          = "enflame.com/gcu-count"
	ServerSock         = pluginapi.DevicePluginPath + "enflamegcushare.sock"

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
	TOPS_VISIBLE_DEVICES                = "TOPS_VISIBLE_DEVICES"
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
	GCUDRSCapacity        = "enflame.com/gcu-drs-capacity"
	PodAssignedContainers = "assigned-containers"
	PatchCount            = GCUSharedCapacity + "-patch-count"
	DRSAssignedDevice     = "drs-assigned-device"

	// other
	RecommendedKubeConfigPathEnv = "KUBECONFIG"
	MaxRetryTimes                = 30

	// drs
	DRSSchedulerName  = "gcushare-scheduler-drs"
	ProfileNameRegExp = `\b\d+g\.\d+gb\b`
	SliceCountDRS     = 6
	Plugin            = "plugin"
	DRSPlugin         = "drsPlugin"

	// drs configmap
	ConfigMapNode      = "node-name"
	ConfigMapOwner     = "owner"
	SchedulerRecord    = "schedulerRecord"
	StateSuccess       = "Success"
	StateError         = "Error"
	StateUnschedulable = "Unschedulable"
	StateSkip          = "Skip"
)
