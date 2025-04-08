package main

import (
	"bufio"
	rpcstructs "disaggregated_autoscale/rpc_structs"
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

/*
	Description: This file is the code that runs on each server that actually processes a job

	Server's need to each monitor their own resource usage based on the jobs assigned to it.
	This info will be sent to autoscaler via RPCs
*/

// TODO: Currently using plan_cpu and plan_mem as the actual resource usage

// TODO:  important for good metrics: Probably a race condition for queue job vs incoming job

// TODO: Optimize Queueing:what if the second element in the queue can be processed before the first element?

// need a hash map + linked list implementation?

// TODO: The following are the real numbers for the server, however can make them different via commandline args
var CPU_AVAILABLE float64    // number of cores
var MEMORY_AVAILABLE float64 // in GB

var compute_remaining float64
var memory_remaining float64

var job_queue []rpcstructs.Args
var job_queue_len int = 0

var job_to_cpu_resource_usage = make(map[rpcstructs.Pair]float64)
var job_to_mem_resource_usage = make(map[rpcstructs.Pair]float64)
var job_to_timing = make(map[rpcstructs.Pair]rpcstructs.JobTiming)

var status bool = false // false is offline

// Queueing Optimization 1 Data Structure: Aims to solve the problem of the initial job in the queue not having enough available resources
var job_to_marked = make(map[rpcstructs.Pair]int)
var spots_to_pushback = 0

var mu sync.Mutex // Mutex to ensure thread-safe access to shared resources

var port int = 9001
var my_ip string
var my_type string

type HandleJob struct{}

func sendAutoscalerStatistics() { // only send when a job with 'key' has completed trade off is higher network usage for sending per completed job
	// inefficient, do not have to loop through every single time, just store in a map or something

	if status == false && len(job_queue) == 0 {
		return
	}

	config_file, _ := os.Open("config.txt")
	scanner := bufio.NewScanner(config_file)
	var line string
	for scanner.Scan() {
		line = scanner.Text()
	}

	words := strings.Fields(line)
	autoscaler, err := rpc.Dial("tcp", words[1]+":"+strconv.Itoa(port))
	if err != nil {
		fmt.Printf("Error connecting to autoscaler at %s:%d: %v\n", words[1], port, err)
		return // Exit the function if the connection fails
	}

	mu.Lock()
	server_stats := rpcstructs.ServerUsage{my_ip, compute_remaining, memory_remaining, job_to_timing, len(job_queue), status, time.Now().Unix(), my_type}
	var reply string
	err = autoscaler.Call("AutoScaler.RequestedStats", &server_stats, &reply)
	if err != nil {
		fmt.Printf("Error making RPC call to autoscaler: %v\n", err)
		mu.Unlock()
		return
	}
	mu.Unlock()
}

func deallocateResources(jobId int, taskId int) {
	key := rpcstructs.Pair{J_ID: jobId, T_ID: taskId}
	mu.Lock()

	compute_remaining += job_to_cpu_resource_usage[key]
	memory_remaining += job_to_mem_resource_usage[key]
	state := job_to_timing[key]
	state.JobEndTime = time.Now().Unix()
	job_to_timing[key] = state
	fmt.Print("Server: Resources deallocated, cpu remaining: ", compute_remaining, " mem remaining: ", memory_remaining, "\n")
	mu.Unlock()
}

func processJobQueue() {
	for {
		mu.Lock()
		if len(job_queue) > 0 {
			key := rpcstructs.Pair{J_ID: job_queue[0].JobId, T_ID: job_queue[0].TaskId}
			if compute_remaining < job_to_cpu_resource_usage[key] || memory_remaining < job_to_mem_resource_usage[key] {
				// TODO: Push the current job back in the queue due to lack of resources
				job_to_marked[key] = 1

				job_to_delay := job_queue[0]

				// remove that job from the queue and add it back in
				job_queue = job_queue[1:]
				if spots_to_pushback >= job_queue_len { // is happening because a job is getting double marked?
					job_queue = append(job_queue, job_to_delay)
				} else {
					job_queue = append(job_queue[:spots_to_pushback], append([]rpcstructs.Args{job_to_delay}, job_queue[spots_to_pushback:]...)...)
					spots_to_pushback += 1 // future jobs should be pushed back behind where we placed this one
				}
			} else {
				compute_remaining -= job_to_cpu_resource_usage[key]
				memory_remaining -= job_to_mem_resource_usage[key]

				jid := job_queue[0].JobId
				tid := job_queue[0].TaskId
				duration := job_queue[0].TimeEnd - job_queue[0].TimeStart
				time.AfterFunc((time.Duration(duration) * time.Second), func() { deallocateResources(jid, tid) })
				job_queue = job_queue[1:] // remove the job from the queue
				job_queue_len -= 1

				if job_to_marked[key] == 1 {
					spots_to_pushback -= 1
				}

			}

		}
		mu.Unlock()
		time.Sleep(300 * time.Millisecond)
	}
}

func (t *HandleJob) AddJobs(args *rpcstructs.Args, reply *int) error {
	mu.Lock()
	job_queue = append(job_queue, *args)
	fmt.Println("Job added to queue, job id: ", args.JobId, " task id: ", args.TaskId, " plan cpu: ", args.PlanCPUResourceUsage, " plan mem: ", args.PlanMemoryResourceUsage)
	job_queue_len += 1
	status = true
	key := rpcstructs.Pair{J_ID: args.JobId, T_ID: args.TaskId}
	job_to_cpu_resource_usage[key] = float64(args.RealMaxCPU) * 100 // number of cores being used
	job_to_mem_resource_usage[key] = float64(args.RealMaxMemory * MEMORY_AVAILABLE)
	job_to_timing[key] = rpcstructs.JobTiming{JobStartTime: time.Now().Unix(), JobEndTime: -1, JobExecStartTime: int64(args.TimeStart), JobExecEndTime: int64(args.TimeEnd)}
	my_ip = args.ServerIp
	mu.Unlock()

	*reply = 0
	return nil
}

func (t *HandleJob) ShutDownServer(args *rpcstructs.Args, reply *int) error {
	mu.Lock()
	fmt.Println("Shutting Down")
	status = false
	mu.Unlock()

	*reply = 0
	return nil
}

func startServer() {
	job_handler := new(HandleJob)
	rpc.Register(job_handler)

	listener, err := net.Listen("tcp", ":9001")
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

func periodicallySendStats() {
	for {
		sendAutoscalerStatistics()
		time.Sleep(5 * time.Second)
	}

}

func main() {

	cpuPtr := flag.Float64("cpu", 2.0, "Total CPU available (cores)")
	memPtr := flag.Float64("mem", 4.0, "Total memory available (GB)")
	flag.Parse()

	CPU_AVAILABLE = *cpuPtr
	MEMORY_AVAILABLE = *memPtr

	compute_remaining = CPU_AVAILABLE
	memory_remaining = MEMORY_AVAILABLE

	go processJobQueue() // Start the job queue processor in a separate goroutine
	go periodicallySendStats()
	startServer()
}
