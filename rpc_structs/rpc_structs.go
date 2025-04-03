package rpcstructs

type Args struct {
	JobId               int
	CPUResourceUsage    float64
	MemoryResourceUsage float64
	TimeStart           int
	TimeEnd             int
	TaskId              int
}
