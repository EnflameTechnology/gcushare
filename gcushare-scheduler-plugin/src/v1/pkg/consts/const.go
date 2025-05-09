// Copyright (c) 2025, ENFLAME INC.  All rights reserved.

package consts

const (

	/* for log */
	LOGPATH  = "/var/log/enflame/gcushare/gcushare-scheduler-plugin.log"
	LOGINFO  = "INFO"
	LOGERROR = "ERROR"
	LOGWARN  = "WARNING"
	LOGDEBUG = "DEBUG"

	/* for component info*/
	PORT           = "12345"
	COMPONENT_NAME = "gcushare-scheduler-plugin"
	VersionFlag    = "--version"

	/* for resource */
	SchedulerName       = "gcushare-scheduler"
	SchedulerPluginName = "GCUShareSchedulerPlugin"
	ResourceName        = "enflame.com/shared-gcu"

	// pod label or annotations
	PodRequestGCUSize  = "enflame.com/gcu-request-size"
	PodAssignedGCUID   = "enflame.com/gcu-assigned-id"
	PodHasAssignedGCU  = "enflame.com/gcu-assigned"
	PodAssignedGCUTime = "enflame.com/gcu-assigned-time"
	GCUSharedCapacity  = "enflame.com/gcu-shared-capacity"

	/* for routers */
	ApiPrefix              = "/gcushare-scheduler"
	PluginVersionRoute     = "/version"
	InspectNodeDetailRoute = ApiPrefix + "/inspect/:nodename"
	InspectNodeListRoute   = ApiPrefix + "/inspect"

	// gcushare node label
	GCUShareLabel = "enflame.com/gcushare"

	// others
	MaxRetryTimes = 60
)
