package rpcstructs

type Args struct {
	JobId               int
	CPUResourceUsage    float64
	MemoryResourceUsage float64
	TimeStart           int
	TimeEnd             int
	TaskId              int
	ServerIp            string
}

type ServerUsage struct {
	ServerIp     string
	ComputeUsage float32
	MemoryUsage  float32
}

type ServerDetails struct {
	ServerIp   string
	NodeNumber int
}
