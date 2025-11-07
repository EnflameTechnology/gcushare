package structs

type (
	Minor       string
	ProfileName string
	ProfileID   string
)

type DeviceSpec struct {
	Index    string `json:"index"`
	Minor    string `json:"minor"`
	Capacity int    `json:"capacity"`
}

type DRSGCUCapacity struct {
	Devices  []DeviceSpec              `json:"devices"`
	Profiles map[ProfileName]ProfileID `json:"profiles"`
}

type AllocateRecord struct {
	KubeletAllocated *bool   `json:"allocated,omitempty"`
	Request          *int    `json:"request,omitempty"`
	ProfileID        *string `json:"profileID,omitempty"`
	ProfileName      *string `json:"profileName,omitempty"`
	InstanceID       *string `json:"instanceID,omitempty"`
	InstanceUUID     *string `json:"instanceUUID,omitempty"`
}
