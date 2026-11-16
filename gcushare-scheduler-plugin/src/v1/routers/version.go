// Copyright (c) 2025, ENFLAME INC.  All rights reserved.
package routers

import (
	"fmt"
	"gcushare-scheduler-plugin/pkg/consts"
	"gcushare-scheduler-plugin/pkg/logs"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

type pluginVersion string

func (ver pluginVersion) GetVersion(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	logs.Info("listen access url: %s, method: %s, request body:%v", consts.PluginVersionRoute, "GET", r)
	fmt.Fprint(w, fmt.Sprint(ver+"\n"))
	logs.Info("listen access url: %s, method: %s, response:%v", consts.PluginVersionRoute, "GET", w)
}
