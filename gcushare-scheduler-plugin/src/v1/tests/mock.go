// Copyright (c) 2025, ENFLAME INC.  All rights reserved.
package tests

import (
	"context"

	v1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	corev1 "k8s.io/client-go/applyconfigurations/core/v1"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
)

type MockPods struct{}
type MockNodes struct{}
type MockK8sErr struct {
	Reason metav1.StatusReason
	Code   int32
}

func NewAPIStatus(reason metav1.StatusReason, code int32) *MockK8sErr {
	return &MockK8sErr{
		Reason: reason,
		Code:   code,
	}
}

var _ typedcorev1.PodInterface = &MockPods{}
var _ typedcorev1.NodeInterface = &MockNodes{}
var _ k8serr.APIStatus = &MockK8sErr{}
var _ error = &MockK8sErr{}

func (rer *MockPods) Create(ctx context.Context, pod *v1.Pod, opts metav1.CreateOptions) (*v1.Pod, error) {
	return nil, nil
}

func (rer *MockPods) Update(ctx context.Context, pod *v1.Pod, opts metav1.UpdateOptions) (*v1.Pod, error) {
	return nil, nil
}

func (rer *MockPods) UpdateStatus(ctx context.Context, pod *v1.Pod, opts metav1.UpdateOptions) (*v1.Pod, error) {
	return nil, nil
}

func (rer *MockPods) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return nil
}

func (rer *MockPods) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions,
	listOpts metav1.ListOptions) error {
	return nil
}

func (rer *MockPods) Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.Pod, error) {
	return nil, nil
}

func (rer *MockPods) List(ctx context.Context, opts metav1.ListOptions) (*v1.PodList, error) {
	return nil, nil
}

func (rer *MockPods) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return nil, nil
}

func (rer *MockPods) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions,
	subresources ...string) (result *v1.Pod, err error) {
	return nil, nil
}

func (rer *MockPods) Apply(ctx context.Context, pod *corev1.PodApplyConfiguration, opts metav1.ApplyOptions) (
	result *v1.Pod, err error) {
	return nil, nil
}
func (rer *MockPods) ApplyStatus(ctx context.Context, pod *corev1.PodApplyConfiguration, opts metav1.ApplyOptions) (
	result *v1.Pod, err error) {
	return nil, nil
}

func (rer *MockPods) UpdateEphemeralContainers(ctx context.Context, podName string, pod *v1.Pod,
	opts metav1.UpdateOptions) (*v1.Pod, error) {
	return nil, nil
}

func (rer *MockPods) Bind(ctx context.Context, binding *v1.Binding, opts metav1.CreateOptions) error {
	return nil
}

func (rer *MockPods) Evict(ctx context.Context, eviction *policyv1beta1.Eviction) error {
	return nil
}

func (rer *MockPods) EvictV1(ctx context.Context, eviction *policyv1.Eviction) error {
	return nil
}

func (rer *MockPods) EvictV1beta1(ctx context.Context, eviction *policyv1beta1.Eviction) error {
	return nil
}
func (rer *MockPods) GetLogs(name string, opts *v1.PodLogOptions) *restclient.Request {
	return nil
}
func (rer *MockPods) ProxyGet(scheme, name, port, path string, params map[string]string) restclient.ResponseWrapper {
	return nil
}

func (rer *MockK8sErr) Status() metav1.Status {
	return metav1.Status{
		Reason: rer.Reason,
		Code:   rer.Code,
	}
}

func (rer *MockK8sErr) Error() string {
	return string(rer.Reason)
}

func (rer *MockNodes) Create(ctx context.Context, node *v1.Node, opts metav1.CreateOptions) (*v1.Node, error) {
	return nil, nil
}
func (rer *MockNodes) Update(ctx context.Context, node *v1.Node, opts metav1.UpdateOptions) (*v1.Node, error) {
	return nil, nil
}
func (rer *MockNodes) UpdateStatus(ctx context.Context, node *v1.Node, opts metav1.UpdateOptions) (*v1.Node, error) {
	return nil, nil
}
func (rer *MockNodes) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return nil
}
func (rer *MockNodes) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return nil
}
func (rer *MockNodes) Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.Node, error) {
	return nil, nil
}
func (rer *MockNodes) List(ctx context.Context, opts metav1.ListOptions) (*v1.NodeList, error) {
	return nil, nil
}
func (rer *MockNodes) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return nil, nil
}
func (rer *MockNodes) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions,
	subresources ...string) (result *v1.Node, err error) {
	return nil, nil
}
func (rer *MockNodes) Apply(ctx context.Context, node *corev1.NodeApplyConfiguration, opts metav1.ApplyOptions) (
	result *v1.Node, err error) {
	return nil, nil
}
func (rer *MockNodes) ApplyStatus(ctx context.Context, node *corev1.NodeApplyConfiguration, opts metav1.ApplyOptions) (
	result *v1.Node, err error) {
	return nil, nil
}
func (rer *MockNodes) PatchStatus(ctx context.Context, nodeName string, data []byte) (*v1.Node, error) {
	return nil, nil
}
