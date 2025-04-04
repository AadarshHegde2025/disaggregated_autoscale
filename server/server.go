package main

import (
	"bufio"
	rpcstructs "disaggregated_autoscale/rpc_structs"
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

// TODO: Need Locking on Global Variables to ensure consistency

const CPU_AVAILABLE = 2    // number of cores
const MEMORY_AVAILABLE = 4 // in GB

var compute_remaining float32 = CPU_AVAILABLE
var memory_remaining float32 = MEMORY_AVAILABLE

var job_queue []rpcstructs.Args

type Pair struct {
	j_id int
	t_id int
}

var job_to_cpu_resource_usage = make(map[Pair]float32)
var job_to_mem_resource_usage = make(map[Pair]float32)

var mu sync.Mutex // Mutex to ensure thread-safe access to shared resources

var port int = 9000
var my_ip string

type HandleJob struct{}

func sendAutoscalerStatistics() {
	time.Sleep(5 * time.Second) // Wait for a while before sending stats
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

	for {
		time.Sleep(5 * time.Second) // Send stats every 5 seconds
		mu.Lock()
		server_stats := rpcstructs.ServerUsage{my_ip, compute_remaining, memory_remaining}
		var reply string
		err = autoscaler.Call("AutoScaler.RequestedStats", &server_stats, &reply)
		if err != nil {
			fmt.Printf("Error making RPC call to autoscaler: %v\n", err)
			mu.Unlock()
			continue // Skip this iteration and try again
		}
		mu.Unlock()
	}
}

func deallocateResources(jobId int, taskId int) {
	key := Pair{j_id: jobId, t_id: taskId}
	mu.Lock()
	defer mu.Unlock()
	compute_remaining += job_to_cpu_resource_usage[key]
	memory_remaining += job_to_mem_resource_usage[key]
	fmt.Print("Server: Resources deallocated, cpu remaining: ", compute_remaining, " mem remaining: ", memory_remaining, "\n")
}

func processJobQueue() {
	for {
		mu.Lock()
		if len(job_queue) > 0 {
			job := job_queue[0]
			if compute_remaining >= float32(job.CPUResourceUsage)/100 && memory_remaining >= float32(job.MemoryResourceUsage*MEMORY_AVAILABLE) {
				// Remove job from queue
				job_queue = job_queue[1:]
				key := Pair{j_id: job.JobId, t_id: job.TaskId}

				// Allocate resources
				compute_remaining -= job_to_cpu_resource_usage[key]
				memory_remaining -= job_to_mem_resource_usage[key]
				fmt.Print("Server: Processing queued job ", job.JobId)
				fmt.Print("Server: Resources allocated, cpu remaining: ", compute_remaining, " mem remaining: ", memory_remaining, "\n")

				// Schedule resource deallocation
				time.AfterFunc((time.Duration(job.TimeEnd-job.TimeStart) * time.Second), func() { deallocateResources(job.JobId, job.TaskId) })
			}
		}
		mu.Unlock()
		time.Sleep(1 * time.Second) // Check the queue periodically
	}
}

func (t *HandleJob) AddJobs(args *rpcstructs.Args, reply *int) error {
	mu.Lock()
	// if the server can handle the job, immediatelty process, else will have to put in a queue
	key := Pair{j_id: args.JobId, t_id: args.TaskId}
	job_to_cpu_resource_usage[key] = float32(args.CPUResourceUsage) / 100
	job_to_mem_resource_usage[key] = float32(args.MemoryResourceUsage * MEMORY_AVAILABLE)
	my_ip = args.ServerIp
	if compute_remaining < float32(args.CPUResourceUsage)/100 || memory_remaining < float32(args.MemoryResourceUsage*MEMORY_AVAILABLE) {
		fmt.Print("Server: Not enough resources, adding job to queue\n")
		job_queue = append(job_queue, *args)
	} else {
		compute_remaining -= job_to_cpu_resource_usage[key]
		memory_remaining -= job_to_mem_resource_usage[key]
		fmt.Print("Server: Added job ", args.JobId)
		fmt.Println("Server: Resources allocated, cpu remaining: ", compute_remaining, " mem remaining: ", memory_remaining)

		time.AfterFunc((time.Duration(args.TimeEnd-args.TimeStart) * time.Second), func() { deallocateResources(args.JobId, args.TaskId) })
	}
	mu.Unlock()

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
	go processJobQueue()          // Start the job queue processor in a separate goroutine
	go sendAutoscalerStatistics() // Start the autoscaler statistics sender in a separate goroutine
	startServer()
}
