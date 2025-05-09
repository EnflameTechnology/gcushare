// Copyright (c) 2022 Enflame. All Rights Reserved.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"k8s.io/client-go/rest"

	"gcushare-device-plugin/manager"
	"gcushare-device-plugin/pkg/config"
	"gcushare-device-plugin/pkg/consts"
	"gcushare-device-plugin/pkg/kube"
	"gcushare-device-plugin/pkg/logs"
)

var Version string

var (
	resourceIsolation = flag.Bool("resource-isolation", true, "Enable or disable resource isolation")
	healthCheck       = flag.Bool("health-check", false, "Enable or disable Health check")
	sliceCount        = flag.Int("slice-count", 6, "Set slice count of GCU device")
	queryFromKubelet  = flag.Bool("query-kubelet", false, "Query pending pods from kubelet instead of kube-apiserver")
	kubeletAddress    = flag.String("kubelet-address", "0.0.0.0", "Kubelet IP Address")
	kubeletPort       = flag.Uint("kubelet-port", 10250, "Kubelet listened Port")
	clientCert        = flag.String("client-cert", "", "Kubelet TLS client certificate")
	clientKey         = flag.String("client-key", "", "Kubelet TLS client key")
	token             = flag.String("token", "", "Kubelet client bearer token")
	timeout           = flag.Int("timeout", 10, "Kubelet client http timeout duration")
	versionflag       = flag.Bool("version", false, "enable version output")
)

func main() {
	flag.Parse()
	if *versionflag {
		fmt.Printf("gcushare device plugin version: %v\n", Version)
		return
	}
	logs.Info("Starting gcushare device plugin...")
	kubeletClient, err := BuildKubeletClient()
	if err != nil {
		os.Exit(1)
	}
	config, err := config.GetConfig()
	if err != nil {
		logs.Error(err, "get topscloud config failed")
		os.Exit(1)
	}
	gcuManager := manager.NewShareGCUManager(*resourceIsolation, *healthCheck, *queryFromKubelet, *sliceCount,
		kubeletClient, config)
	if err = gcuManager.Run(); err != nil {
		logs.Error(err, "run gcushare device plugin failed, will restart")
		os.Exit(1)
	}
}

func BuildKubeletClient() (*kube.KubeletClient, error) {
	if *clientCert == "" && *clientKey == "" && *token == "" {
		tokenByte, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
		if err != nil {
			logs.Error(err, "read cluster token failed")
			return nil, err
		}
		tokenStr := string(tokenByte)
		token = &tokenStr
	}
	kubeletClient, err := kube.NewKubeletClient(&kube.KubeletClientConfig{
		Address: *kubeletAddress,
		Port:    *kubeletPort,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure:   true,
			ServerName: consts.COMPONENT_NAME,
			CertFile:   *clientCert,
			KeyFile:    *clientKey,
		},
		BearerToken: *token,
		HTTPTimeout: time.Duration(*timeout) * time.Second,
	})
	if err != nil {
		logs.Error(err, "get kube client failed")
		return nil, err
	}
	logs.Info("build kubelet client success")
	return kubeletClient, nil
}
