// Copyright (c) 2025, ENFLAME INC.  All rights reserved.
package routers

import (
	"net/http"
	"os"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"k8s.io/client-go/kubernetes"

	"gcushare-scheduler-plugin/pkg/config"
	"gcushare-scheduler-plugin/pkg/consts"
	"gcushare-scheduler-plugin/pkg/logs"
	"gcushare-scheduler-plugin/pkg/resources"
)

func Listen(config *config.Config, clientset *kubernetes.Clientset, version string) {
	baseResource := resources.NewBaseResource(config, clientset)
	inspect := &Inspect{
		sharedResourceName: config.ResourceName(false),
		drsResourceName:    config.ResourceName(true),
		clientset:          clientset,
		podResource:        resources.NewPodResource(baseResource),
		nodeResource:       resources.NewNodeResource(baseResource),
	}
	router := httprouter.New()
	router.GET(consts.PluginVersionRoute, pluginVersion(version).GetVersion)
	router.GET(consts.InspectNodeListRoute, inspect.InspectRoute)
	router.GET(consts.InspectNodeDetailRoute, inspect.InspectRoute)
	listenPort := os.Getenv("PORT")
	if _, err := strconv.Atoi(listenPort); err != nil {
		logs.Error("the value: %s of listen port is not int type, will use default port: %s", listenPort, consts.PORT)
		listenPort = consts.PORT
	}
	logs.Info("gcushare scheduler plugin inspect starting on the port: %s", listenPort)
	if err := http.ListenAndServe(":"+listenPort, router); err != nil {
		logs.Error(err, "start gcushare scheduler extender server on port: %s failed", listenPort)
		os.Exit(1)
	}
}
