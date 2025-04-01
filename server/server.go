package main

import (
	rpcstructs "disaggregated_autoscale/rpc_structs"
	"fmt"
	"net"
	"net/rpc"
)

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

type HandleJob struct{}

func (t *HandleJob) add_job(args *rpcstructs.Args, reply *int) error {

	// if the server can handle the job, it will add it to its queue
	// otherwise, it will return an error
	print("Server: Adding job %d\n", args.JobId)
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
