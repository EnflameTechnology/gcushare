// Copyright (c) 2025, ENFLAME INC.  All rights reserved.

package device

import (
	"go-efids/efids"
	"testing"

	. "github.com/agiledragon/gomonkey"
	. "github.com/smartystreets/goconvey/convey"

	"gcushare-device-plugin/pkg/config"
)

func TestNewDevice(t *testing.T) {
	Convey("test func pkg/device/device.go/NewDevice", t, func() {
		config := &config.Config{RegisterResource: []string{"enflame.com/gcu"}}

		Convey("1. test GetDevices success", func() {
			patcheGetDeviceFullInfos := ApplyFunc(efids.GetDeviceFullInfos, func() ([]efids.DeviceFullInfo, error) {
				return []efids.DeviceFullInfo{
					{
						DeviceInfo: efids.DeviceInfo{
							VendorID: "0x1e36",
							DeviceID: "0xc033",
							BusID:    "0000:01:00.0",
							UUID:     "T6R231060602",
							Major:    238,
							Minor:    0,
						},
						GCUDevice: efids.GCUDevice{
							Vendor:       "Enflame",
							AsicName:     "Scorpio X2",
							ProductModel: "S60G",
							SipNum:       24,
							L3Size:       48,
							L2Size:       64,
						},
					},
					{
						DeviceInfo: efids.DeviceInfo{
							VendorID: "0x1e36",
							DeviceID: "0xc033",
							BusID:    "0000:09:00.0",
							UUID:     "T6R231010702",
							Major:    238,
							Minor:    1,
						},
						GCUDevice: efids.GCUDevice{
							Vendor:       "Enflame",
							AsicName:     "Scorpio X2",
							ProductModel: "S60G",
							SipNum:       24,
							L3Size:       48,
							L2Size:       64,
						},
					},
				}, nil
			})
			defer patcheGetDeviceFullInfos.Reset()

			rv, err := NewDevice(12, config, true)
			So(err, ShouldEqual, nil)
			So(rv.Count, ShouldEqual, 2)
			So(rv.SliceCount, ShouldEqual, 12)
			So(rv.Info["0"].Memory, ShouldEqual, 48)
			So(rv.Info["0"].Sip, ShouldEqual, 24)
			So(rv.Info["0"].Path, ShouldEqual, "/dev/gcu0")
		})
	})
}
