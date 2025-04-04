package rpcstructs

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
}

type ServerDetails struct { // This struct is used for autoscaler to add server to load balancer
	ServerIp   string
	NodeNumber int
}
