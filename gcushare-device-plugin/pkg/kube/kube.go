//  Copyright 2022 Enflame. All Rights Reserved.
package kube

import (
	"encoding/json"
	"fmt"
	"io"

	"net/http"
	"time"

	v1 "k8s.io/api/core/v1"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/transport"

	"gcushare-device-plugin/pkg/logs"
	"gcushare-device-plugin/pkg/utils"
)

type KubeletClient struct {
	defaultPort uint
	host        string
	client      *http.Client
}

func (kube *KubeletClient) GetNodePodsList() (*v1.PodList, error) {
	url := fmt.Sprintf("https://%v:%d/pods/", kube.host, kube.defaultPort)
	resp, err := kube.client.Get(url)
	if err != nil {
		logs.Error(err, "get cluster pod failed by url: %s", url)
		return nil, err
	}

	body, err := ReadAll(resp.Body)
	if err != nil {
		logs.Error(err, "read cluster response of pods failed, response: %s", utils.ConvertToString(resp.Body))
		return nil, err
	}
	podLists := &v1.PodList{}
	if err = json.Unmarshal(body, &podLists); err != nil {
		logs.Error(err, "unmarshal response to v1.podList failed response: %s", string(body))
		return nil, err
	}
	return podLists, nil
}

// KubeletClientConfig defines config parameters for the kubelet client
type KubeletClientConfig struct {
	// Address specifies the kubelet address
	Address string

	// Port specifies the default port - used if no information about Kubelet port can be found in Node.NodeStatus.DaemonEndpoints.
	Port uint

	// TLSClientConfig contains settings to enable transport layer security
	rest.TLSClientConfig

	// Server requires Bearer authentication
	BearerToken string

	// HTTPTimeout is used by the client to timeout http requests to Kubelet.
	HTTPTimeout time.Duration
}

func ReadAll(resp io.Reader) ([]byte, error) {
	respByte := make([]byte, 0, 512)
	for {
		if len(respByte) == cap(respByte) {
			// Add more capacity (let append pick how much).
			respByte = append(respByte, 0)[:len(respByte)]
		}
		n, err := resp.Read(respByte[len(respByte):cap(respByte)])
		respByte = respByte[:len(respByte)+n]
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return respByte, err
		}
	}
}

func NewKubeletClient(config *KubeletClientConfig) (*KubeletClient, error) {
	trans, err := makeTransport(config, true)
	if err != nil {
		return nil, err
	}
	client := &http.Client{
		Transport: trans,
		Timeout:   config.HTTPTimeout,
	}
	return &KubeletClient{
		host:        config.Address,
		defaultPort: config.Port,
		client:      client,
	}, nil
}

// makeTransport creates a RoundTripper for HTTP Transport.
func makeTransport(config *KubeletClientConfig, insecureSkipTLSVerify bool) (http.RoundTripper, error) {
	// do the insecureSkipTLSVerify on the pre-transport *before* we go get a potentially cached connection.
	// transportConfig always produces a new struct pointer.
	preTLSConfig := config.transportConfig()
	if insecureSkipTLSVerify && preTLSConfig != nil {
		preTLSConfig.TLS.Insecure = true
		preTLSConfig.TLS.CAData = nil
		preTLSConfig.TLS.CAFile = ""
	}

	tlsConfig, err := transport.TLSConfigFor(preTLSConfig)
	if err != nil {
		return nil, err
	}

	rt := http.DefaultTransport
	if tlsConfig != nil {
		// If SSH Tunnel is turned on
		rt = utilnet.SetOldTransportDefaults(&http.Transport{
			TLSClientConfig: tlsConfig,
		})
	}

	return transport.HTTPWrappersForConfig(config.transportConfig(), rt)
}

// transportConfig converts a client config to an appropriate transport config.
func (config *KubeletClientConfig) transportConfig() *transport.Config {
	cfg := &transport.Config{
		TLS: transport.TLSConfig{
			CAFile:   config.CAFile,
			CAData:   config.CAData,
			CertFile: config.CertFile,
			CertData: config.CertData,
			KeyFile:  config.KeyFile,
			KeyData:  config.KeyData,
		},
		BearerToken: config.BearerToken,
	}
	if !cfg.HasCA() {
		cfg.TLS.Insecure = true
	}
	return cfg
}

func GetKubeClient() (*kubernetes.Clientset, error) {
	kubeConfig := ""
	// if len(os.Getenv(types.RecommendedKubeConfigPathEnv)) > 0 {
	// 	// use the current context in kubeconfig
	// 	// This is very useful for running locally.
	// 	kubeConfig = os.Getenv(types.RecommendedKubeConfigPathEnv)
	// }

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
	logs.Info("init client-go client success")
	return clientset, nil
}

// // BuildConfigFromFlags is a helper function that builds configs from a master
// // url or a kubeconfig filepath. These are passed in as command line flags for cluster
// // components. Warnings should reflect this usage. If neither masterUrl or kubeconfigPath
// // are passed in we fallback to inClusterConfig. If inClusterConfig fails, we fallback
// // to the default config.
// func BuildConfigFromFlags(masterUrl, kubeconfigPath string) (*rest.Config, error) {
// 	if kubeconfigPath == "" && masterUrl == "" {
// 		logs.Warn("Neither --kubeconfig nor --master was specified.  Using the inClusterConfig.  This might not work.")
// 		kubeconfig, err := InClusterConfig()
// 		if err == nil {
// 			return kubeconfig, nil
// 		}
// 		logs.Warn("error creating inClusterConfig, falling back to default config: %s", err.Error())
// 	}
// 	return nil, fmt.Errorf("ssssssss")
// }

// // InClusterConfig returns a config object which uses the service account
// // kubernetes gives to pods. It's intended for clients that expect to be
// // running inside a pod running on kubernetes. It will return ErrNotInCluster
// // if called from a process not running in a kubernetes environment.
// func InClusterConfig() (*rest.Config, error) {
// 	const (
// 		tokenFile  = "/var/run/secrets/kubernetes.io/serviceaccount/token"
// 		rootCAFile = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
// 	)
// 	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
// 	if len(host) == 0 || len(port) == 0 {
// 		msg := fmt.Sprintf("both env %s and %s can not be empty", "KUBERNETES_SERVICE_HOST", "KUBERNETES_SERVICE_PORT")
// 		logs.Error(fmt.Errorf(msg), "")
// 		return nil, rest.ErrNotInCluster
// 	}

// 	token, err := ioutil.ReadFile(tokenFile)
// 	if err != nil {
// 		logs.Error(err, "read token file from %s failed", tokenFile)
// 		return nil, err
// 	}

// 	tlsClientConfig := rest.TLSClientConfig{}

// 	if _, err := certutil.NewPool(rootCAFile); err != nil {
// 		logs.Error(err, "load root CA config from %s failed", rootCAFile)
// 	} else {
// 		tlsClientConfig.CAFile = rootCAFile
// 	}

// 	return &rest.Config{
// 		// TODO: switch to using cluster DNS.
// 		Host:            "https://" + net.JoinHostPort(host, port),
// 		TLSClientConfig: tlsClientConfig,
// 		BearerToken:     string(token),
// 		BearerTokenFile: tokenFile,
// 	}, nil
// }
