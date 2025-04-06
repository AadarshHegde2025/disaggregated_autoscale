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
	ServerIp              string
	ComputeUsage          float64
	MemoryUsage           float64
	JobCompletionTime     int64 // the turn around time between when the job was added to the server and when it was completed
	JobTraceExecutionTime int64 // the actual time taken to execute the job (time end - time start from the trace)
}

type ServerDetails struct { // This struct is used for autoscaler to add server to load balancer
	ServerIp   string
	NodeNumber int
}

type JobType int

const (
	COMPUTE_HEAVY JobType = iota
	MEMORY_HEAVY
)

type Snapshot struct { // This struct is used for the autoscaler to understand the chronology of jobs completed
	ServerIp			string
	JobType 			JobType
	CpuUtilization 		float64
	MemoryUtilization 	float64
	ExecutionTime		int64
	TotalTime			int64
	Timestamp			int64
}