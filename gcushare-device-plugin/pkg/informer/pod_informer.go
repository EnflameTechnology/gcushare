package informer

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
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
	"gcushare-device-plugin/pkg/consts"
	"gcushare-device-plugin/pkg/device"
	"gcushare-device-plugin/pkg/logs"
	"gcushare-device-plugin/pkg/smi"
	"gcushare-device-plugin/pkg/structs"
	"gcushare-device-plugin/pkg/utils"
)

type PodInformer struct {
	sync.RWMutex
	ResourceName  string
	Config        *config.Config
	GCUSharedPods map[types.UID]*v1.Pod
	NodeName      string
	Clientset     *kubernetes.Clientset
	DeviceInfo    map[string]device.DeviceInfo
}

func NewPodInformer(nodeName, resourceName string, config *config.Config, clientset *kubernetes.Clientset,
	deviceInfo map[string]device.DeviceInfo) *PodInformer {
	return &PodInformer{
		ResourceName:  resourceName,
		Config:        config,
		GCUSharedPods: make(map[types.UID]*v1.Pod),
		NodeName:      nodeName,
		Clientset:     clientset,
		DeviceInfo:    deviceInfo,
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
	logs.Info("gcushare pod informer for resource: %s work start...", rer.ResourceName)
	return nil
}

func (rer *PodInformer) addPod(pod *v1.Pod) {
	rer.Lock()
	defer rer.Unlock()
	request := 0
	for _, container := range pod.Spec.Containers {
		if val, ok := container.Resources.Limits[v1.ResourceName(rer.ResourceName)]; ok {
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
	if _, ok := rer.GCUSharedPods[pod.UID]; !ok {
		return
	}
	delete(rer.GCUSharedPods, pod.UID)
	logs.Info("remove pod(name: %s, uuid: %s) from gcushared pods local cache", pod.Name, pod.UID)
	if rer.ResourceName == rer.Config.ResourceName(true) {
		rer.clearResource(pod)
	}
}
func (rer *PodInformer) clearResource(pod *v1.Pod) {
	pciBusID := rer.DeviceInfo[pod.Annotations[consts.PodAssignedGCUMinor]].BusID
	if !utils.FileIsExist(filepath.Join(consts.PCIDevicePath, pciBusID)) {
		logs.Warn("device pciBusID: %s not exist, need not clear drs instance", pciBusID)
		return
	}

	index := pod.Annotations[consts.PodAssignedGCUIndex]
	assignedContainers := pod.Annotations[consts.PodAssignedContainers]
	assignedMap := map[string]structs.AllocateRecord{}
	if err := json.Unmarshal([]byte(assignedContainers), &assignedMap); err != nil {
		logs.Error(err, "unmarshal assigned containers to map of pod(name: %s, uuid: %s) failed, detail: %s",
			pod.Name, pod.UID, assignedContainers)
		return
	}
	for _, alloc := range assignedMap {
		if err := smi.DeleteInstance(index, *alloc.InstanceID); err != nil {
			logs.Error(err, "delete device index: %s drs instance(profileName: %s, instanceID: %s) failed",
				index, *alloc.ProfileName, *alloc.InstanceID)
		} else {
			logs.Info("delete device index: %s drs instance(profileName: %s, instanceID: %s) success",
				index, *alloc.ProfileName, *alloc.InstanceID)
		}
	}

	instanceList, err := smi.ListInstance(index)
	if err != nil {
		logs.Error(err, "list device index: %s drs instance failed", index)
		return
	}
	if len(instanceList) == 0 {
		logs.Info("device index: %s drs instance not found, need close drs", index)
		if err := smi.CloseDRS(index); err != nil {
			logs.Error(err, "close device index: %s drs failed", index)
		} else {
			logs.Info("close device index: %s drs success", index)
		}
	}
}
