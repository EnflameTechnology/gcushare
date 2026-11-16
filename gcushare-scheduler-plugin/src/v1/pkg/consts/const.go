// Copyright (c) 2025, ENFLAME INC.  All rights reserved.

package consts

const (

	/* for log */
	LOGPATH    = "/var/log/enflame/gcushare/gcushare-scheduler-plugin.log"
	LOGINFO    = "INFO"
	LOGERROR   = "ERROR"
	LOGWARN    = "WARNING"
	LOGDEBUG   = "DEBUG"
	TimeFormat = "2006-01-02 15:04:05.00"

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
	PodAssignedGCUMinor   = "enflame.com/gcu-assigned-minor"
	PodAssignedGCUIndex   = "enflame.com/gcu-assigned-index"
	PodHasAssignedGCU     = "enflame.com/gcu-assigned"
	PodAssignedGCUTime    = "enflame.com/gcu-assigned-time"
	GCUSharedCapacity     = "enflame.com/gcu-shared-capacity"
	GCUDRSCapacity        = "enflame.com/gcu-drs-capacity"
	PodAssignedContainers = "assigned-containers"

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
