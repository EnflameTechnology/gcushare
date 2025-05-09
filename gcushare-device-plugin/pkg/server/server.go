// Copyright 2022 Enflame. All Rights Reserved.
package server

import (
	"context"
	"fmt"
	"math"
	"net"
	"os"
	"path"
	"reflect"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	v1 "k8s.io/api/core/v1"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"

	"gcushare-device-plugin/pkg/config"
	"gcushare-device-plugin/pkg/consts"
	"gcushare-device-plugin/pkg/device"
	"gcushare-device-plugin/pkg/kube"
	"gcushare-device-plugin/pkg/logs"
	"gcushare-device-plugin/pkg/resource"
	"gcushare-device-plugin/pkg/utils"
)

// 4MB is default grpc max recevive message size
// 32MB maximum can meet 16*64GB GCU
var grpcMaxReceiveMessageSize = 32 * 1024 * 1024

// GCUDevicePluginServe implements the Kubernetes device plugin API
type GCUDevicePluginServe struct {
	resourceName      string
	fakeDevices       []*pluginapi.Device
	device            *device.Device
	socket            string
	healthCheck       bool
	resourceIsolation bool
	stop              chan struct{}
	health            chan []*pluginapi.Device
	queryKubelet      bool
	kubeletClient     *kube.KubeletClient
	config            *config.Config
	podResource       *resource.PodResource
	nodeResource      *resource.NodeResource
	deviceCapacityMap map[string]int
	allLocked         chan map[string]struct{}
	sendDevices       map[string]string

	server *grpc.Server
	sync.RWMutex
}

// NewGCUDevicePluginServe returns an initialized EnflameDevicePlugin
func NewGCUDevicePluginServe(healthCheck, queryKubelet bool, client *kube.KubeletClient, device *device.Device) (
	*GCUDevicePluginServe, error) {
	clusterResource, err := resource.NewClusterResource(device.Config)
	if err != nil {
		return nil, err
	}
	nodeResource := resource.NewNodeResource(clusterResource)
	podResource := resource.NewPodResource(clusterResource)
	if err := podResource.Informer.Start(context.TODO()); err != nil {
		return nil, err
	}
	fakeDevices, deviceMap := device.GetFakeDevices()

	// get devices count of node, and patch device info to node, Status.Capacity, Status.Allocatable
	err = nodeResource.PatchGCUCountToNode(device.Count)
	if err != nil {
		return nil, err
	}
	if err := nodeResource.PatchGCUMemoryMapToNode(utils.GetDeviceCapacityMap(fakeDevices)); err != nil {
		return nil, err
	}
	resourceIsolation, err := nodeResource.CheckResourceIsolation(device.ResourceIsolation)
	if err != nil {
		return nil, err
	}
	if resourceIsolation {
		logs.Info("gcushare resource isolation mode is enabled")
		if err := validSliceCount(device); err != nil {
			return nil, err
		}
	} else {
		logs.Warn("gcushare resource isolation mode is disabled")
	}
	return &GCUDevicePluginServe{
		resourceName:      device.Config.ResourceName(),
		fakeDevices:       fakeDevices,
		device:            device,
		socket:            consts.ServerSock,
		healthCheck:       healthCheck,
		resourceIsolation: resourceIsolation,
		stop:              make(chan struct{}),
		health:            make(chan []*pluginapi.Device),
		queryKubelet:      queryKubelet,
		kubeletClient:     client,
		config:            device.Config,
		podResource:       podResource,
		nodeResource:      nodeResource,
		allLocked:         make(chan map[string]struct{}, 1),
		sendDevices:       deviceMap,
	}, nil
}

func validSliceCount(device *device.Device) error {
	for _, devInfo := range device.Info {
		if float64(device.SliceCount) > math.Min(float64(devInfo.Memory), float64(devInfo.Sip)) {
			err := fmt.Errorf("sliceCount: %d should not be greater than device sip: %d and memory: %d",
				device.SliceCount, devInfo.Sip, devInfo.Memory)
			logs.Error(err)
			return err
		}
	}
	if device.SliceCount > consts.RecommendedMaxSliceCount {
		logs.Warn("found sliceCount: %d, but it should not be greater than %d is recommended, otherwise more device fragments may be generated",
			device.SliceCount, consts.RecommendedMaxSliceCount)
	}
	return nil
}

// Stop stops the gRPC server
func (plugin *GCUDevicePluginServe) Stop() error {
	if plugin.server == nil {
		return nil
	}
	plugin.server.Stop()
	plugin.server = nil
	close(plugin.stop)
	return plugin.cleanup()
}

func (plugin *GCUDevicePluginServe) cleanup() error {
	if err := os.Remove(plugin.socket); err != nil && !os.IsNotExist(err) {
		logs.Error(err, "remove gcu device plugin socket failed")
		return err
	}
	return nil
}

// Serve starts the gRPC server and register the device plugin to Kubelet
func (plugin *GCUDevicePluginServe) Serve() error {
	if err := plugin.Start(); err != nil {
		return err
	}
	logs.Info("Starting gcushare plugin to serve on %s", plugin.socket)

	if err := plugin.Register(pluginapi.KubeletSocket); err != nil {
		plugin.Stop()
		return err
	}
	logs.Info("Register gcushare device plugin with Kubelet success!")
	return nil
}

// Start starts the gRPC server of the device plugin
func (plugin *GCUDevicePluginServe) Start() error {
	err := plugin.cleanup()
	if err != nil {
		return err
	}

	sock, err := net.Listen("unix", plugin.socket)
	if err != nil {
		logs.Error(err, "listen plugin socket %s failed", plugin.socket)
		return err
	}
	logs.Info("listen plugin socket %s starting", plugin.socket)
	serverOption := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(grpcMaxReceiveMessageSize),
		grpc.MaxSendMsgSize(grpcMaxReceiveMessageSize),
	}
	plugin.server = grpc.NewServer(serverOption...)
	pluginapi.RegisterDevicePluginServer(plugin.server, plugin)

	go plugin.server.Serve(sock)

	// Wait for server to start by launching a blocking connexion
	grpcConnection, err := dial(plugin.socket, 5*time.Second)
	if err != nil {
		return err
	}
	grpcConnection.Close()

	go device.WatchHealth(plugin.stop, plugin.fakeDevices, plugin.health, plugin.device.Info, plugin.allLocked)

	//lastAllocateTime = time.Now()
	return nil
}

// Register registers the device plugin for the given resourceName with Kubelet.
func (plugin *GCUDevicePluginServe) Register(kubeletEndpoint string) error {
	conn, err := dial(kubeletEndpoint, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pluginapi.NewRegistrationClient(conn)
	reqt := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     path.Base(plugin.socket),
		ResourceName: plugin.resourceName,
	}

	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		logs.Error(err, "register device with request %s failed", utils.ConvertToString(reqt))
		return err
	}
	return nil
}

func (plugin *GCUDevicePluginServe) PreStartContainer(context.Context, *pluginapi.PreStartContainerRequest) (
	*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}

// ListAndWatch lists devices and update that list according to the health status
func (plugin *GCUDevicePluginServe) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
	if err := s.Send(&pluginapi.ListAndWatchResponse{Devices: plugin.fakeDevices}); err != nil {
		logs.Error(err, "list watch send gcu device health list to kubelet failed")
		return err
	}

	for {
		select {
		case <-plugin.stop:
			return nil
		case deviceList := <-plugin.health:
			for _, d := range deviceList {
				if d.Health == pluginapi.Healthy {
					d.Health = pluginapi.Unhealthy
				} else {
					d.Health = pluginapi.Healthy
				}
			}
			if err := plugin.updateDeviceCapacityMap(); err != nil {
				return err
			}

			if err := plugin.checkPodIsUsingDisabledCard(deviceList); err != nil {
				return err
			}

			if !plugin.needSend(deviceList) {
				continue
			}
			if err := s.Send(&pluginapi.ListAndWatchResponse{Devices: plugin.fakeDevices}); err != nil {
				logs.Error(err, "list watch send gcu device health list to kubelet failed")
				return err
			}
		}
	}
}

func (plugin *GCUDevicePluginServe) needSend(deviceList []*pluginapi.Device) bool {
	stateChanged := false
	for _, device := range deviceList {
		if device.Health != plugin.sendDevices[device.ID] {
			plugin.sendDevices[device.ID] = device.Health
			stateChanged = true
		}
	}
	if stateChanged {
		logs.Info("need to send GRPC information to kubelet to update device information")
		return true
	}
	logs.Info("need not to send GRPC information to kubelet to update device information")
	return false
}

func (plugin *GCUDevicePluginServe) checkPodIsUsingDisabledCard(deviceList []*pluginapi.Device) error {
	disabledCardInfo, err := plugin.podResource.GetDisabledCardInfo(plugin.device.Info)
	if err != nil {
		return err
	}
	allLockedMap := map[string]struct{}{}
	for disabledRealID, used := range disabledCardInfo {
		lockCount := 0
		for _, device := range deviceList {
			realID := strings.Split(device.ID, "-")[0]
			if disabledRealID == realID {
				if device.Health == pluginapi.Unhealthy {
					device.Health = pluginapi.Healthy
					lockCount += 1
				}
				if lockCount == used {
					logs.Warn("device: %s is disabled, but it used: %d by pods, should not update health state",
						realID, used)
					break
				}
			}
		}
		if lockCount == plugin.device.SliceCount {
			allLockedMap[disabledRealID] = struct{}{}
		}
		if lockCount != used {
			err := fmt.Errorf("internal error! list watch ready to update device list: %v, but device: %s used: %d by pods",
				deviceList, disabledRealID, used)
			logs.Error(err)
			return err
		}
	}
	if len(allLockedMap) > 0 {
		logs.Warn("The health status of the following devices is all locked: %v", allLockedMap)
		plugin.allLocked <- allLockedMap
	}
	return nil
}

func (plugin *GCUDevicePluginServe) updateDeviceCapacityMap() error {
	healthyList := []*pluginapi.Device{}
	for _, device := range plugin.fakeDevices {
		if device.Health == pluginapi.Healthy {
			healthyList = append(healthyList, device)
		}
	}
	deviceCapacityMap := utils.GetDeviceCapacityMap(healthyList)
	if reflect.DeepEqual(deviceCapacityMap, plugin.deviceCapacityMap) {
		logs.Info("device capacity: %v not changed, list watch need not to update node annotation", deviceCapacityMap)
		return nil
	}
	logs.Info("list watch will update device capacity information to node annotations")
	if err := plugin.nodeResource.PatchGCUMemoryMapToNode(deviceCapacityMap); err != nil {
		return err
	}
	plugin.deviceCapacityMap = deviceCapacityMap
	return nil
}

// Allocate which return list of devices.
// Allocate called by kubelet
func (plugin *GCUDevicePluginServe) Allocate(ctx context.Context, reqs *pluginapi.AllocateRequest) (
	*pluginapi.AllocateResponse, error) {
	logs.Info("----Allocate shared gcu starting----")
	if len(reqs.ContainerRequests) != 1 {
		err := fmt.Errorf("internal error! the Allocate() method can only allocate resources for one container at a time currently")
		logs.Error(err)
		return nil, err
	}

	containerRequest := len(reqs.ContainerRequests[0].DevicesIDs)
	logs.Info("container request %s: %d, the fake device ids list: %v", plugin.resourceName,
		containerRequest, reqs.ContainerRequests[0].DevicesIDs)

	plugin.Lock()
	defer plugin.Unlock()
	assumePod, err := plugin.getAssumePod(containerRequest)
	if err != nil {
		return nil, err
	}
	if assumePod == nil {
		msg := fmt.Sprintf("invalid allocation request: unable to find pod request %s: %d", plugin.resourceName, containerRequest)
		logs.Error(fmt.Errorf(msg), "")
		return nil, fmt.Errorf(msg)
	}

	assignedID, err := plugin.podResource.GetGCUIDFromPodAnnotation(assumePod)
	if err != nil {
		return nil, err
	}
	responses, err := plugin.makeContainerResponse(assignedID, containerRequest)
	if err != nil {
		return nil, err
	}
	logs.Info("device plugin allocate pod(name: %s, uuid: %s) device: %s, response: %s", assumePod.Name,
		assumePod.UID, assignedID, utils.ConvertToString(responses))
	logs.Info("----Allocate shared gcu for pod(name: %s, uuid: %s) success----", assumePod.Name, assumePod.UID)
	return responses, nil
}

func (plugin *GCUDevicePluginServe) makeContainerResponse(assignedID string, containerRequest int) (
	*pluginapi.AllocateResponse, error) {
	responses := &pluginapi.AllocateResponse{}
	response := pluginapi.ContainerAllocateResponse{
		Envs: map[string]string{
			consts.ENFLAME_VISIBLE_DEVICES: assignedID,
		},
	}
	plugin.setEnvForIsolation(response, containerRequest, assignedID)
	responses.ContainerResponses = append(responses.ContainerResponses, &response)
	return responses, nil
}

func (plugin *GCUDevicePluginServe) setEnvForIsolation(response pluginapi.ContainerAllocateResponse,
	containerRequest int, assignedID string) {
	if !plugin.resourceIsolation {
		logs.Warn("gcushare resource isolation mode is disabled, which may cause resource conflicts")
		return
	}
	if containerRequest == plugin.device.SliceCount {
		response.Envs[consts.ENFLAME_CONTAINER_SUB_CARD] = "false"
	} else {
		response.Envs[consts.ENFLAME_CONTAINER_SUB_CARD] = "true"
	}
	response.Envs[consts.ENFLAME_CONTAINER_USABLE_PROCESSOR] = fmt.Sprintf("%d",
		plugin.device.Info[assignedID].Sip*containerRequest/plugin.device.SliceCount)
	response.Envs[consts.ENFLAME_CONTAINER_USABLE_SHARED_MEM] = fmt.Sprintf("%d",
		plugin.device.Info[assignedID].L2*containerRequest/plugin.device.SliceCount)
	response.Envs[consts.ENFLAME_CONTAINER_USABLE_GLOBAL_MEM] = fmt.Sprintf("%d",
		plugin.device.Info[assignedID].Memory*containerRequest/plugin.device.SliceCount)
}

func (plugin *GCUDevicePluginServe) getAssumePod(containerRequest int) (*v1.Pod, error) {
	pods, err := plugin.podResource.GetCandidatePods(plugin.queryKubelet, plugin.kubeletClient)
	if err != nil {
		logs.Error(err, "invalid allocation requst: Failed to find candidate pods")
		return nil, err
	}

	var assumePod *v1.Pod
	for _, pod := range pods {
		logs.Info("pod(name: %s, uuid: %s) request %s: %d with timestamp: %v", pod.Name, pod.UID, plugin.resourceName,
			plugin.podResource.GetGCUMemoryFromPodResource(pod), resource.GetAssumeTimeFromPodAnnotation(pod))
		if assumePod == nil {
			containerName, err := plugin.podResource.PodExistContainerCanBind(containerRequest, pod)
			if err != nil {
				return nil, err
			}
			if containerName == "" {
				continue
			}
			if err := plugin.podResource.CheckPodAllContainersBind(containerName, pod); err != nil {
				return assumePod, err
			}
			logs.Info("candidate pod(name: %s, uuid: %s) request %s: %d with timestamp: %v is selected",
				pod.Name, pod.UID, plugin.resourceName, containerRequest, resource.GetAssumeTimeFromPodAnnotation(pod))
			assumePod = pod
		}
	}
	return assumePod, nil
}

func (plugin *GCUDevicePluginServe) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{}, nil
}

// dial establishes the gRPC communication with the registered device plugin.
func dial(unixSocketPath string, timeout time.Duration) (*grpc.ClientConn, error) {
	dialOption := []grpc.DialOption{
		grpc.WithInsecure(),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcMaxReceiveMessageSize)),
		grpc.WithBlock(),
		grpc.WithTimeout(timeout),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),
	}
	grpcConnection, err := grpc.Dial(unixSocketPath, dialOption...)
	if err != nil {
		logs.Error(err, "establishes grpc connection with socket %s failed", unixSocketPath)
		return nil, err
	}
	logs.Info("dial establishes grpc connection with socket %s success", unixSocketPath)
	return grpcConnection, nil
}
