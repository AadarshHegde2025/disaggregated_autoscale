package main

/*
	Description: This file is the code that runs on each server that actually processes a job

	Server's need to each monitor their own resource usage based on the jobs assigned to it.
	This info will be sent to autoscaler via RPCs
*/

// TODO: RPC handler to send resource utilization info

const CPU_AVAILABLE = 100
const MEMORY_AVAILABLE = 100

var compute_remaining float32
var memory_remaining float32

type handle_job struct{}
type Args struct {
	job_id                int
	cpu_resource_usage    int
	memory_resource_usage int
	time_start            int
	time_end              int
}

func (t *handle_job) add_job(args *Args, reply *int) error {

	// if the server can handle the job, it will add it to its queue
	// otherwise, it will return an error
	print("Server: Adding job %d\n", args.job_id)
	*reply = 0
	return nil
}
