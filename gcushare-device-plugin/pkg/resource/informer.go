package resource

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"gcushare-device-plugin/pkg/config"
	"gcushare-device-plugin/pkg/logs"
)

type PodInformer struct {
	sync.RWMutex
	Config        *config.Config
	GCUSharedPods map[types.UID]*v1.Pod
	NodeName      string
	Clientset     *kubernetes.Clientset
}

func NewPodInformer(nodeName string, config *config.Config, clientset *kubernetes.Clientset) *PodInformer {
	return &PodInformer{
		Config:        config,
		GCUSharedPods: make(map[types.UID]*v1.Pod),
		NodeName:      nodeName,
		Clientset:     clientset,
	}
}

func (rer *PodInformer) Start(ctx context.Context) error {
	factory := informers.NewSharedInformerFactoryWithOptions(
		rer.Clientset,
		time.Minute, // Resync time interval
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = "spec.nodeName=" + rer.NodeName
		}),
	)

	podInformer := factory.Core().V1().Pods().Informer()

	// register event handle funcs
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			if pod, ok := obj.(*v1.Pod); ok {
				rer.addPod(pod)
			}
		},
		UpdateFunc: func(oldObj, newObj any) {
			if oldPod, ok := oldObj.(*v1.Pod); ok {
				if newPod, ok := newObj.(*v1.Pod); ok {
					rer.updatePod(oldPod, newPod)
				}
			}
		},
		DeleteFunc: func(obj any) {
			if pod, ok := obj.(*v1.Pod); ok {
				rer.removePod(pod)
			}
		},
	})

	// start pod informer
	go podInformer.Run(ctx.Done())

	// wait first sync finish
	if !cache.WaitForCacheSync(ctx.Done(), podInformer.HasSynced) {
		err := fmt.Errorf("timed out waiting for gcushare pods local cache sync")
		logs.Error(err)
		return err
	}
	logs.Info("gcushare pod informer work start...")
	return nil
}

func (rer *PodInformer) addPod(pod *v1.Pod) {
	rer.Lock()
	defer rer.Unlock()
	request := 0
	for _, container := range pod.Spec.Containers {
		if val, ok := container.Resources.Limits[v1.ResourceName(rer.Config.ResourceName())]; ok {
			request += int(val.Value())
		}
	}
	if request <= 0 {
		return
	}
	rer.GCUSharedPods[pod.UID] = pod
	logs.Info("add pod(name: %s, uuid: %s) to gcushared pods local cache", pod.Name, pod.UID)
}

func (rer *PodInformer) updatePod(oldPod, newPod *v1.Pod) {
	rer.Lock()
	defer rer.Unlock()
	if _, ok := rer.GCUSharedPods[oldPod.UID]; !ok {
		return
	}
	if reflect.DeepEqual(oldPod.Annotations, newPod.Annotations) {
		return
	}
	rer.GCUSharedPods[oldPod.UID] = newPod
	logs.Info("update pod(name: %s, uuid: %s) to gcushared pods local cache, pod annotations: %v",
		newPod.Name, newPod.UID, newPod.Annotations)
}

func (rer *PodInformer) removePod(pod *v1.Pod) {
	rer.Lock()
	defer rer.Unlock()
	if _, ok := rer.GCUSharedPods[pod.UID]; ok {
		delete(rer.GCUSharedPods, pod.UID)
		logs.Info("remove pod(name: %s, uuid: %s) from gcushared pods local cache", pod.Name, pod.UID)
	}
}
