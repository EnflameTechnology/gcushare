// Copyright (c) 2025, ENFLAME INC.  All rights reserved.

package resource

import (
	"os"
	"reflect"
	"testing"

	. "github.com/agiledragon/gomonkey"
	. "github.com/smartystreets/goconvey/convey"
	corev1 "k8s.io/api/core/v1"
	k8sres "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"gcushare-device-plugin/pkg/config"
	"gcushare-device-plugin/pkg/consts"
	"gcushare-device-plugin/pkg/kube"
)

func TestPatchGCUCountToNode(t *testing.T) {
	Convey("test func resource/node.go", t, func() {
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-1",
			},
			Status: corev1.NodeStatus{
				Capacity: corev1.ResourceList{
					consts.CountName:    *k8sres.NewQuantity(0, ""),
					consts.ResourceName: *k8sres.NewQuantity(0, ""),
				},
				Allocatable: corev1.ResourceList{
					consts.CountName:    *k8sres.NewQuantity(0, ""),
					consts.ResourceName: *k8sres.NewQuantity(0, ""),
				},
			},
		}

		patcheKube := ApplyFunc(kube.GetKubeClient, func() (*kubernetes.Clientset, error) {
			return &kubernetes.Clientset{}, nil
		})
		defer patcheKube.Reset()

		patcheEnv := ApplyFunc(os.Getenv, func(_ string) string {
			return "node-1"
		})
		defer patcheEnv.Reset()
		config := &config.Config{RegisterResource: []string{"enflame.com/gcu"}}
		clusterResource, err := NewClusterResource(config)
		So(err, ShouldEqual, nil)
		nodeIns := NewNodeResource(clusterResource)
		Convey("1. test func PatchGCUCountToNode success", func() {
			patcheGetNode := ApplyMethod(reflect.TypeOf(nodeIns), "GetNode", func(_ *NodeResource) (*corev1.Node, error) {
				return node, nil
			})
			defer patcheGetNode.Reset()

			patchePatchNode := ApplyMethod(reflect.TypeOf(nodeIns), "PatchNode", func(_ *NodeResource, _ []byte) (*corev1.Node, error) {
				return node, nil
			})
			defer patchePatchNode.Reset()

			rv := nodeIns.PatchGCUCountToNode(2)
			So(rv, ShouldEqual, nil)
		})

		Convey("2. test func DisableCGCUIsolationOrNot success", func() {
			node.Labels = map[string]string{consts.ResourceIsolationLabel: "true"}
			patcheGetNode := ApplyMethod(reflect.TypeOf(nodeIns), "GetNode", func(_ *NodeResource) (*corev1.Node, error) {
				return node, nil
			})
			defer patcheGetNode.Reset()

			rv, err := nodeIns.CheckResourceIsolation(false)
			So(err, ShouldEqual, nil)
			So(rv, ShouldEqual, true)
		})
	})
}
