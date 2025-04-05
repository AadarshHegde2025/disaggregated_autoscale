package rpcstructs

// These two structs below are kinda misplaced, they are needed here because of ServerUsage rpc struct, but also these values are only
// used but server.go, so may be something to change at a later point
type JobTiming struct {
	JobStartTime     int64
	JobEndTime       int64
	JobExecStartTime int64
	JobExecEndTime   int64
}

type Pair struct {
	J_ID int
	T_ID int
}

type Args struct { // This struct sends job details to the server
	JobId                   int
	PlanCPUResourceUsage    float64
	PlanMemoryResourceUsage float64
	TimeStart               int
	TimeEnd                 int
	TaskId                  int
	ServerIp                string
	RealMaxCPU              float64
	RealMaxMemory           float64
}

type ServerUsage struct { // This struct sends server stats to the autoscaler
	ServerIp     string
	ComputeUsage float64
	MemoryUsage  float64
	JobToTiming  map[Pair]JobTiming
}

type ServerDetails struct { // This struct is used for autoscaler to add server to load balancer
	ServerIp   string
	NodeNumber int
}
