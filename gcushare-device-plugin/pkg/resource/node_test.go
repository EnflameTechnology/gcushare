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
		config := &config.Config{RegisterResource: []string{"enflame.com/gcu"}}
		resourceName := config.ResourceName(false)
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-1",
			},
			Status: corev1.NodeStatus{
				Capacity: corev1.ResourceList{
					consts.CountName:                  *k8sres.NewQuantity(0, ""),
					corev1.ResourceName(resourceName): *k8sres.NewQuantity(0, ""),
				},
				Allocatable: corev1.ResourceList{
					consts.CountName:                  *k8sres.NewQuantity(0, ""),
					corev1.ResourceName(resourceName): *k8sres.NewQuantity(0, ""),
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
		clusterResource, err := NewClusterResource(config, resourceName)
		So(err, ShouldEqual, nil)
		nodeIns := NewNodeResource(clusterResource, false)
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
	})
}
