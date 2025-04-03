package main

import (
	rpcstructs "disaggregated_autoscale/rpc_structs"
	"fmt"
	"net"
	"net/rpc"
	"time"
)

/*
	Description: This file is the code that runs on each server that actually processes a job

	Server's need to each monitor their own resource usage based on the jobs assigned to it.
	This info will be sent to autoscaler via RPCs
*/

// TODO: RPC handler to send resource utilization info

const CPU_AVAILABLE = 2    // number of cores
const MEMORY_AVAILABLE = 4 // in GB

var compute_remaining float32 = CPU_AVAILABLE
var memory_remaining float32 = MEMORY_AVAILABLE

type Pair struct {
	j_id int
	t_id int
}

var job_to_cpu_resource_usage = make(map[Pair]float32)
var job_to_mem_resource_usage = make(map[Pair]float32)

type HandleJob struct{}

func deallocateResources(jobId int, taskId int) {
	key := Pair{j_id: jobId, t_id: taskId}

	compute_remaining += job_to_cpu_resource_usage[key]
	memory_remaining += job_to_mem_resource_usage[key]
	print("Server: Resources deallocated, cpu remaining: ", compute_remaining, " mem remaining: ", memory_remaining, "\n")
}

func (t *HandleJob) AddJobs(args *rpcstructs.Args, reply *int) error {

	// if the server can handle the job, it will add it to its queue
	// otherwise, it will return an error
	key := Pair{j_id: args.JobId, t_id: args.TaskId}
	job_to_cpu_resource_usage[key] = float32(args.CPUResourceUsage) / 100
	job_to_mem_resource_usage[key] = float32(args.MemoryResourceUsage * MEMORY_AVAILABLE)

	compute_remaining -= float32(args.CPUResourceUsage) / 100
	memory_remaining -= float32(args.MemoryResourceUsage * MEMORY_AVAILABLE)
	print("Server: Added job %d\n", args.JobId)
	print("Server: Resources allocated, cpu remaining: ", compute_remaining, " mem remaining: ", memory_remaining, "\n")

	time.AfterFunc((time.Duration(args.TimeEnd-args.TimeStart) * time.Second), func() { deallocateResources(args.JobId, args.TaskId) })
	*reply = 0
	return nil
}

func startServer() {
	job_handler := new(HandleJob)
	rpc.Register(job_handler)

	listener, err := net.Listen("tcp", ":9000")
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
	fmt.Println("Server listening on port 9000")

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Connection error:", err)
			continue
		}
		go rpc.ServeConn(conn)
	}
}

func main() {
	startServer()
}
