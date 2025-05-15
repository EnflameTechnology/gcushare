package structs

import (
	k8sTypes "k8s.io/apimachinery/pkg/types"
)

type SchedulerRecord struct {
	Filter  *FilterSpec  `json:"filter,omitempty"`
	PreBind *PreBindSpec `json:"preBind,omitempty"`
}

type FilterSpec struct {
	GCUSharePods []GCUSharePod             `json:"gcuSharePods,omitempty"`
	Containers   map[string]AllocateRecord `json:"containers,omitempty"`
	Status       string                    `json:"status,omitempty"`
	Message      string                    `json:"message,omitempty"`
	Device       DeviceSpec                `json:"device,omitempty"`
}

type AllocateRecord struct {
	KubeletAllocated *bool   `json:"allocated,omitempty"`
	Request          *int    `json:"request,omitempty"`
	ProfileID        *string `json:"profileID,omitempty"`
	ProfileName      *string `json:"profileName,omitempty"`
	InstanceID       *string `json:"instanceID,omitempty"`
}

type DeviceSpec struct {
	Index    string `json:"index,omitempty"`
	Minor    string `json:"minor,omitempty"`
	PCIBusID string `json:"pciBusID,omitempty"`
}

type GCUSharePod struct {
	Name       string       `json:"name,omitempty"`
	Namespace  string       `json:"namespace,omitempty"`
	Uuid       k8sTypes.UID `json:"uuid,omitempty"`
	AssignedID string       `json:"assignedID,omitempty"`
}

type PreBindSpec struct {
	Status     string                    `json:"status,omitempty"`
	Message    string                    `json:"message,omitempty"`
	Containers map[string]AllocateRecord `json:"containers,omitempty"`
}
