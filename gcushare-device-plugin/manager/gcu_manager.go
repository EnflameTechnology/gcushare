//  Copyright 2022 Enflame. All Rights Reserved.

package manager

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"

	"gcushare-device-plugin/pkg/config"
	"gcushare-device-plugin/pkg/consts"
	"gcushare-device-plugin/pkg/device"
	"gcushare-device-plugin/pkg/kube"
	"gcushare-device-plugin/pkg/logs"
	"gcushare-device-plugin/pkg/server"
	"gcushare-device-plugin/pkg/utils"
	"gcushare-device-plugin/pkg/watcher"
)

type ShareGCUManager struct {
	deviceType        string
	resourceIsolation bool
	healthCheck       bool
	queryKubelet      bool
	sliceCount        int
	kubeletClient     *kube.KubeletClient
	config            *config.Config
}

func NewShareGCUManager(resourceIsolation, healthCheck, queryKubelet bool, sliceCount int, client *kube.KubeletClient,
	config *config.Config) *ShareGCUManager {
	return &ShareGCUManager{
		deviceType:        config.DeviceType(),
		resourceIsolation: resourceIsolation,
		healthCheck:       healthCheck,
		queryKubelet:      queryKubelet,
		sliceCount:        sliceCount,
		kubeletClient:     client,
		config:            config,
	}
}

func (manager *ShareGCUManager) Run() error {
	logs.Info("Fetching %s devices", manager.deviceType)
	devices, err := device.NewDevice(manager.sliceCount, manager.config, manager.resourceIsolation)
	if err != nil {
		return err
	}
	if devices.Count == 0 {
		return fmt.Errorf("no %s devices found in cluster", manager.deviceType)
	}
	logs.Info("Starting FS watcher")
	FsWatcher, err := watcher.NewFSWatcher(pluginapi.DevicePluginPath, config.TopscloudPath)
	if err != nil {
		logs.Error(err, "Failed to created FS watcher")
		return err
	}
	defer FsWatcher.Close()
	logs.Info("Starting OS watcher")
	sigs := watcher.NewOSWatcher(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	restart := true
	var devicePlugin *server.GCUDevicePluginServe

Loop:
	for {
		if restart {
			if devicePlugin != nil {
				devicePlugin.Stop()
			}
			logs.Info("start new gcushare device plugin")
			devicePlugin, err = server.NewGCUDevicePluginServe(manager.healthCheck, manager.queryKubelet,
				manager.kubeletClient, devices)
			if err != nil {
				logs.Error(err, "Failed to get gcushare device plugin")
				return err
			}
			if err = devicePlugin.Serve(); err != nil {
				logs.Error(err, "Failed to start gcushare device plugin serve")
				return err
			}
			restart = false
		}

		select {
		case event := <-FsWatcher.Events:
			if event.Name == pluginapi.KubeletSocket && event.Op&fsnotify.Create == fsnotify.Create {
				logs.Info("inotify: %s created, restarting", pluginapi.KubeletSocket)
				restart = true
			}
			// If config file is detected to be modified, the gcushare-device-plugin will be automatically restarted
			if event.Name == config.TopscloudPath+config.ConfigFileName &&
				(event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Write == fsnotify.Write) {
				logs.Warn("fsnotify: config file: %s has been %v, need restart %s", event.Name, event.Op, consts.COMPONENT_NAME)
				time.Sleep(1 * time.Second)
				os.Exit(0)
			}

		case err := <-FsWatcher.Errors:
			logs.Warn("inotify: %s", err.Error())

		case signal := <-sigs:
			switch signal {
			case syscall.SIGHUP:
				logs.Info("Received SIGHUP, restarting")
				restart = true
			case syscall.SIGQUIT:
				t := time.Now()
				timestamp := fmt.Sprint(t.Format("20060102150405"))
				logs.Info("generate core dump")
				utils.Coredump("/etc/kubernetes/go_" + timestamp + ".txt")
			default:
				logs.Info("Received signal %v, shutting down", signal)
				devicePlugin.Stop()
				break Loop
			}
		}
	}
	return nil
}
