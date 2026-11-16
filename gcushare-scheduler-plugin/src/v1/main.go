// Copyright (c) 2025, ENFLAME INC.  All rights reserved.
package main

import (
	"fmt"
	"os"

	"k8s.io/kubernetes/cmd/kube-scheduler/app"

	"gcushare-scheduler-plugin/pkg/config"
	"gcushare-scheduler-plugin/pkg/consts"
	"gcushare-scheduler-plugin/pkg/kube"
	"gcushare-scheduler-plugin/pkg/logs"
	"gcushare-scheduler-plugin/pkg/resources"
	"gcushare-scheduler-plugin/pkg/utils"
	"gcushare-scheduler-plugin/pkg/watcher"
	"gcushare-scheduler-plugin/plugin"
	"gcushare-scheduler-plugin/routers"
)

var Version string

func main() {
	if utils.HasValue(os.Args[1:], consts.VersionFlag) {
		fmt.Printf("gcushare scheduler plugin version: %v\n", Version)
		return
	}
	logs.Info("Gcushare scheduler plugin work starting...")

	config, err := config.GetConfig()
	if err != nil {
		logs.Error(err, "get topscloud config failed")
		os.Exit(1)
	}
	go watcher.WatchConfig()

	clientset, err := kube.GetKubeClient()
	if err != nil {
		os.Exit(1)
	}
	resources.WaitDevicePluginRunning(clientset, config)
	go routers.Listen(config, clientset, Version)

	gcushareSchedulerPlugin, err := plugin.NewGCUShareSchedulerPlugin(config, clientset)
	if err != nil {
		os.Exit(1)
	}
	command := app.NewSchedulerCommand(
		app.WithPlugin(consts.SchedulerPluginName, gcushareSchedulerPlugin),
	)
	if err := command.Execute(); err != nil {
		logs.Error(err, "running gcushare scheduler plugin failed")
		os.Exit(1)
	}
}
