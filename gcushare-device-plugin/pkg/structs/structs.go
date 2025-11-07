package structs

type AllocateRecord struct {
	KubeletAllocated *bool   `json:"allocated,omitempty"`
	Request          *int    `json:"request,omitempty"`
	ProfileID        *string `json:"profileID,omitempty"`
	ProfileName      *string `json:"profileName,omitempty"`
	InstanceID       *string `json:"instanceID,omitempty"`
	InstanceUUID     *string `json:"instanceUUID,omitempty"`
}
