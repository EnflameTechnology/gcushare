package status

type SchedulingStatus struct {
	Status  string
	Message string
}

func NewStatus(status, message string) *SchedulingStatus {
	return &SchedulingStatus{Status: status, Message: message}
}
