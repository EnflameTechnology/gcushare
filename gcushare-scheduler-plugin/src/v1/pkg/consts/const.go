// Copyright (c) 2025, ENFLAME INC.  All rights reserved.

package consts

const (

	/* for log */
	LOGPATH    = "/var/log/enflame/gcushare/gcushare-scheduler-plugin.log"
	LOGINFO    = "INFO"
	LOGERROR   = "ERROR"
	LOGWARN    = "WARNING"
	LOGDEBUG   = "DEBUG"
	TimeFormat = "2006-01-02 15:04:05"

	/* for component info*/
	PORT           = "12345"
	COMPONENT_NAME = "gcushare-scheduler-plugin"
	VersionFlag    = "--version"

	/* for resource */
	SchedulerName       = "gcushare-scheduler"
	SchedulerPluginName = "GCUShareSchedulerPlugin"
	SharedResourceName  = "enflame.com/shared-gcu"
	DRSResourceName     = "enflame.com/drs-gcu"

	// pod label or annotations
	PodRequestGCUSize     = "enflame.com/gcu-request-size"
	PodAssignedGCUID      = "enflame.com/gcu-assigned-id"
	PodHasAssignedGCU     = "enflame.com/gcu-assigned"
	PodAssignedGCUTime    = "enflame.com/gcu-assigned-time"
	GCUSharedCapacity     = "enflame.com/gcu-shared-capacity"
	GCUDRSCapacity        = "enflame.com/gcu-drs-capacity"
	PodAssignedContainers = "assigned-containers"
	DRSAssignedDevice     = "drs-assigned-device"

	// drs
	DRSSchedulerName = "gcushare-scheduler-drs"

	// drs configmap
	ConfigMapNode      = "node-name"
	ConfigMapOwner     = "owner"
	SchedulerRecord    = "schedulerRecord"
	StateSuccess       = "Success"
	StateError         = "Error"
	StateUnschedulable = "Unschedulable"

	/* for routers */
	ApiPrefix              = "/gcushare-scheduler"
	PluginVersionRoute     = "/version"
	InspectNodeDetailRoute = ApiPrefix + "/inspect/:nodename"
	InspectNodeListRoute   = ApiPrefix + "/inspect"
	VirtShared             = "Shared"
	VirtDRS                = "DRS"

	// gcushare node label
	GCUShareLabel = "enflame.com/gcushare"

	// others
	MaxRetryTimes = 60
)
