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
	Server_Type             string
}

type ServerUsage struct { // This struct used for server stats to send to and maintain on the autoscaler
	ServerIp         string
	ComputeRemaining float64
	MemoryRemaining  float64
	JobToTiming      map[Pair]JobTiming
	QueueLength      int
	Status           bool
	Time             int64
	Server_Type      string
}

type JobType int

const (
	COMPUTE_HEAVY JobType = iota

	MEMORY_HEAVY
)

type Snapshot struct { // This struct is used for the autoscaler to understand the chronology of jobs completed

	ServerIp string

	JobType JobType

	CpuUtilization float64

	MemoryUtilization float64

	ExecutionTime int64

	TotalTime int64

	Timestamp int64
}

type ServerDetails struct { // This struct is used for autoscaler to add/remove server to load balancer
	ServerIp   string
	NodeNumber int
	ServerType string
}
