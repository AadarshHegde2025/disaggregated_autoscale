package rpcstructs

type Args struct {
	JobId               int
	CPUResourceUsage    int
	MemoryResourceUsage int
	TimeStart           int
	TimeEnd             int
	TaskId              int
}
