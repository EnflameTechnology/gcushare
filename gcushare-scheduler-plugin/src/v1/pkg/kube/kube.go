// Copyright (c) 2025, ENFLAME INC.  All rights reserved.
package kube

import (
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"gcushare-scheduler-plugin/pkg/logs"
)

const RecommendedKubeConfigPathEnv = "KUBECONFIG"

func GetKubeClient() (*kubernetes.Clientset, error) {
	kubeConfig := ""
	if len(os.Getenv(RecommendedKubeConfigPathEnv)) > 0 {
		// use the current context in kubeconfig
		// This is very useful for running locally.
		kubeConfig = os.Getenv(RecommendedKubeConfigPathEnv)
	}

	// Get kubernetes config.
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		logs.Error(err, "build rest config failed")
		return nil, err
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		logs.Error(err, "init kube client set failed")
		return nil, err
	}
	logs.Info("init kube client success")
	return clientset, nil
}
