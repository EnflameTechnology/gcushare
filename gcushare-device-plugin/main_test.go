// Copyright (c) 2025, ENFLAME INC.  All rights reserved.

package main

import (
	"os"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey"
	. "github.com/smartystreets/goconvey/convey"

	"gcushare-device-plugin/manager"
	"gcushare-device-plugin/pkg/config"
	"gcushare-device-plugin/pkg/kube"
)

func TestMain(t *testing.T) {
	Convey("test func main.go/main", t, func() {
		Convey("1. test start gcushare device plugin success", func() {
			patcheReadFile := gomonkey.ApplyFunc(os.ReadFile, func(_ string) ([]byte, error) {
				return []byte("mock test"), nil
			})
			defer patcheReadFile.Reset()

			patcheNewKube := gomonkey.ApplyFunc(kube.NewKubeletClient, func(_ *kube.KubeletClientConfig) (*kube.KubeletClient, error) {
				return &kube.KubeletClient{}, nil
			})
			defer patcheNewKube.Reset()

			patcheConfig := gomonkey.ApplyFunc(config.GetConfig, func() (*config.Config, error) {
				return &config.Config{RegisterResource: []string{"enflame.com/gcu"}}, nil
			})
			defer patcheConfig.Reset()

			patcheRun := gomonkey.ApplyMethod(reflect.TypeOf(new(manager.ShareGCUManager)), "Run", func(_ *manager.ShareGCUManager) error {
				return nil
			})
			defer patcheRun.Reset()

			main()
		})
	})
}
