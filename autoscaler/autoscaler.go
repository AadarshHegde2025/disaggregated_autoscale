package main

import (
	"bufio"
	rpcstructs "disaggregated_autoscale/rpc_structs"
	"fmt"
	"image/color"
	"math"
	"net"
	"net/rpc"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

var LOAD_BALANCER_IP string = "sp25-cs525-0919.cs.illinois.edu" // Change this
var port int = 9000

// everything working

// TODO: aggregate the server stats over here. metrics: job latency, efficiency, graph of server usage
type AutoScaler struct{
	snapshotList SnapshotList
	snapshotMutex sync.Mutex
}

var server_to_status_overtime = make(map[string][]rpcstructs.ServerUsage)

var server_to_status = make(map[string]ServerState) // server_ip -> ServerStatus
var job_completion_times = make(map[string][]int64) // how much time between when the job was added to the server and when it was completed, for each server
var job_execution_times = make(map[string][]int64)  // how much time the job took to execute, for each server

var mu sync.Mutex



type Status int

type ServerState struct {
	Status Status // True= Online, False = Offline

	Server_Type ServerType // Compute or memory heavy

	ComputeRemaining float64 //

	MemoryRemaining float64 //

}

// Define the enum values as constants

const (
	ONLINE Status = iota

	OFFLINE
)

type ServerType int

const (
	COMPUTE_HEAVY ServerType = iota

	MEMORY_HEAVY
)

type SnapshotListNode struct {
	next *SnapshotListNode

	prev *SnapshotListNode

	data rpcstructs.Snapshot
}

type SnapshotList struct {
	head *SnapshotListNode

	tail *SnapshotListNode
}


// Here, x denotes the number of compute heavy VMs that are currently online

var x int = 0

var online_compute_vms = make(map[string]bool)

// Here, y denotes the number of compute heavy VMs that are currently online

var y int = 0

var online_memory_vms = make(map[string]bool)

func plotOverlayedMetric(title, filename, ylabel string, allData map[string]plotter.XYs) {
	p := plot.New()
	p.Title.Text = title
	p.X.Label.Text = "Time (relative, s)"
	p.Y.Label.Text = ylabel

	colorList := []color.RGBA{
		{R: 255, G: 99, B: 132, A: 255},
		{R: 54, G: 162, B: 235, A: 255},
		{R: 255, G: 206, B: 86, A: 255},
		{R: 75, G: 192, B: 192, A: 255},
		{R: 153, G: 102, B: 255, A: 255},
		{R: 255, G: 159, B: 64, A: 255},
		{R: 100, G: 255, B: 100, A: 255},
	}

	i := 0
	for serverID, data := range allData {
		line, err := plotter.NewLine(data)
		if err != nil {
			fmt.Printf("Skipping %s: %v\n", serverID, err)
			continue
		}
		line.Color = colorList[i%len(colorList)]
		line.Width = vg.Points(2)
		p.Add(line)
		p.Legend.Add(serverID, line)
		i++
	}

	if err := p.Save(12*vg.Inch, 6*vg.Inch, filename); err != nil {
		fmt.Printf("Failed to save %s: %v\n", filename, err)
	}
}

func captureMetrics(servers map[string][]rpcstructs.ServerUsage) {
	fmt.Println("Generating multi-server metric overlays...")

	for k, v := range server_to_status_overtime {
		fmt.Println(k, len(v))
	}

	type metricData map[string]plotter.XYs

	queueData := make(metricData)
	computeData := make(metricData)
	memoryData := make(metricData)
	waitData := make(metricData)

	// --- Step 1: Find the earliest timestamp across all servers ---
	var globalStartTime int64 = math.MaxInt64
	for _, snapshots := range servers {
		for _, snap := range snapshots {
			if snap.Time > 0 && snap.Time < globalStartTime {
				globalStartTime = snap.Time
			}
		}
	}

	// --- Step 2: Build relative time series per server ---
	for serverID, snapshots := range servers {
		var computePoints, memoryPoints, waitPoints, queuePoints plotter.XYs

		for _, snap := range snapshots {
			// Align time relative to global first data point
			t := float64(snap.Time - globalStartTime)

			// Skip any 0 or nonsense timestamps
			if t < 0 {
				continue
			}

			computePoints = append(computePoints, plotter.XY{X: t, Y: snap.ComputeRemaining})
			memoryPoints = append(memoryPoints, plotter.XY{X: t, Y: snap.MemoryRemaining})
			queuePoints = append(queuePoints, plotter.XY{X: t, Y: float64(snap.QueueLength)})

			// --- Wait time ---
			var totalWait float64
			var jobCount int
			for _, timing := range snap.JobToTiming {
				if timing.JobStartTime == 0 || timing.JobEndTime == 0 ||
					timing.JobExecStartTime == 0 || timing.JobExecEndTime == 0 {
					continue
				}
				total := timing.JobEndTime - timing.JobStartTime
				exec := timing.JobExecEndTime - timing.JobExecStartTime
				wait := float64(total - exec)
				if wait < 0 {
					wait = 0
				}
				totalWait += wait
				jobCount++
			}

			avgWait := 0.0
			if jobCount > 0 {
				avgWait = totalWait / float64(jobCount)
			}
			waitPoints = append(waitPoints, plotter.XY{X: t, Y: avgWait})
		}

		computeData[serverID] = computePoints
		memoryData[serverID] = memoryPoints
		queueData[serverID] = queuePoints
		waitData[serverID] = waitPoints
	}

	// --- Step 3: Plot each metric across all servers ---
	plotOverlayedMetric("Queue Length", "queue_length_all.png", "Queue Length", queueData)
	plotOverlayedMetric("Compute Remaining (%)", "compute_remaining_all.png", "Compute Remaining (%)", computeData)
	plotOverlayedMetric("Memory Remaining (%)", "memory_remaining_all.png", "Memory Remaining (%)", memoryData)
	plotOverlayedMetric("Average Wait Time (ms)", "avg_wait_time_all.png", "Avg Wait Time (ms)", waitData)

	fmt.Println("Done plotting overlayed metrics by server.")
}

func (t *AutoScaler) RequestedStats(args *rpcstructs.ServerUsage, reply *string) error {
	mu.Lock()
	fmt.Println("Received server stats:", args.ServerIp, args.ComputeRemaining, args.MemoryRemaining)
	status := server_to_status[args.ServerIp] // mark the server as online
	status.Status = ONLINE
	if status.Status == OFFLINE {
		fmt.Println("Server , ", args.ServerIp, " is now offline")
	}
	if status.Status == ONLINE {
		if status.Server_Type == COMPUTE_HEAVY {
			online_compute_vms[args.ServerIp] = true
		} else {
			online_memory_vms[args.ServerIp] = true
		}
	} else {
		if status.Server_Type == COMPUTE_HEAVY {
			online_compute_vms[args.ServerIp] = false

		} else {
			online_memory_vms[args.ServerIp] = false
		}
	}

	compute_count := 0
	mem_count := 0
	for _, exists := range online_compute_vms {
		if exists {
			compute_count++
		}
	}
	for _, exists := range online_memory_vms {
		if exists {
			mem_count++
		}
	}
	x = compute_count
	y = mem_count

	status.ComputeRemaining = args.ComputeRemaining
	status.MemoryRemaining = args.MemoryRemaining
	server_to_status[args.ServerIp] = status

	// TODO : Add logic to store the stats in some data structure so that we can do predictive autoscaling

	mu.Unlock()
	*reply = "Stats received"
	return nil
}

func adjust_server(power_flag bool) { // if true turn on, if false turn off, need to turn on first, wait a little, then tell load balancer to add server
	if power_flag {
		fmt.Println("Turning on server")
		exec.Command("python3", "vm_power.py", "--vm", "5", "--state", "on").Run()
	} else {
		fmt.Println("Turning off server")
		exec.Command("python3", "vm_power.py", "--vm", "5", "--state", "off").Run()
	}
}

// Adds snapshot to our snapshot list
func (t *AutoScaler) AddSnapshotToList(args *rpcstructs.Snapshot) error {

	t.snapshotMutex.Lock()

	defer t.snapshotMutex.Unlock()

	snapshot := rpcstructs.Snapshot{ServerIp: args.ServerIp, JobType: args.JobType, CpuUtilization: args.CpuUtilization, MemoryUtilization: args.MemoryUtilization, ExecutionTime: args.ExecutionTime, TotalTime: args.TotalTime, Timestamp: args.Timestamp}

	// There is no node in our list, add the new node as the head and tail

	if t.snapshotList.head == nil {

		snapshotNode := &SnapshotListNode{nil, nil, snapshot}

		t.snapshotList.head = snapshotNode

		t.snapshotList.tail = snapshotNode

	// There is a node in our list already, add the new node as the tail

	} else {

		snapshotNode := &SnapshotListNode{nil, t.snapshotList.tail, snapshot}

		t.snapshotList.tail.next = snapshotNode

		t.snapshotList.tail = snapshotNode

	}


	return nil

}

// Truncates snapshot list so that the timestamp of all timestamps is at least `minimum timestamp`  
func (t *AutoScaler) truncateHistory(minimum_timestamp int64) {
	t.snapshotMutex.Lock()
	defer t.snapshotMutex.Unlock()

	current := t.snapshotList.tail

	for {

		if current == nil {
			return
		}

		if current.data.Timestamp < minimum_timestamp {

			// Remove all data from the linked list that is before our threshold
			t.snapshotList.head = current.next
			if current.next != nil{
				current.next.prev = nil
			}
			return
		}

		current = current.prev

	}
}

func (t *AutoScaler) findOptimalConfiguration(num_compute_heavy_available int, num_memory_heavy_available int) (int, int) {

	const N = 10

	// Look through linked list backwards until we find a timestamp that was N seconds before now before we try to optimize 

	now := time.Now().Unix()
	timestamp := now - N
	t.truncateHistory(timestamp)

	var x_prime int = 0

	var y_prime int = 0

	var maxVal float64 = math.Inf(-1)

	for i := 0; i <= num_compute_heavy_available; i++ {

		for j := 0; j <= num_memory_heavy_available; j++ {

			if i == 0 && j == 0 {
				continue
			}

			utility := t.calculateUtilityFunction(i, j)

			if utility > maxVal {
				fmt.Println(maxVal)

				maxVal = utility

				x_prime = i

				y_prime = j

				// In case of tiebreak choose configuration that requires least change of servers

			} else if utility == maxVal &&

				(math.Abs(float64(x_prime-x))+math.Abs(float64(y_prime-y)) > math.Abs(float64(i-x))+math.Abs(float64(j-y))) {

				maxVal = utility

				x_prime = i

				y_prime = j

			}

		}

	}

	return x_prime, y_prime

}

func (t *AutoScaler) calculateUtilityFunction(x_prime int, y_prime int) float64 {

	var alpha float64 = 100 // latency

	var lambda float64 = 1.0 // utilization

	var gamma float64 = 1.0 // avoids thrashing

	// First term

	var A float64 = 0

	var B float64 = 0

	var C float64 = 0

	var D float64 = 0

	A, B = t.calculateExpectedLatencyAndUtilization(x_prime, y_prime)

	C = float64(x_prime + y_prime)

	D = float64(max(x_prime-x, 0) + max(y_prime-y, 0))

	fmt.Println(A, B, C, D)

	return alpha*A + lambda*B + C + gamma*D

}

func (t *AutoScaler) calculateExpectedLatencyAndUtilization(x_prime int, y_prime int) (float64, float64) {

	t.snapshotMutex.Lock()

	defer t.snapshotMutex.Unlock()
	
	// Initialize simulation queues
	// Compute heavy queues...
	compute_queues := make([][]rpcstructs.Snapshot, x_prime)
	for i := range compute_queues {
		compute_queues[i] = make([]rpcstructs.Snapshot, 0)
	}

	// ... and memory heavy queues
	memory_queues := make([][]rpcstructs.Snapshot, y_prime)
	for i := range memory_queues {

		memory_queues[i] = make([]rpcstructs.Snapshot, 0)

	}

	// Assign jobs from the linked list to the queue
	current := t.snapshotList.head
	compute_queue_index := 0
	memory_queue_index := 0

	for {
		// No more jobs to assign
		if current == nil {
			break
		}

		// Add job to a queue depending on what type it is

		jobType := current.data.JobType

		switch jobType {

		case rpcstructs.COMPUTE_HEAVY:
			if len(compute_queues) > 0 {
				compute_queues[compute_queue_index] = append(compute_queues[compute_queue_index], current.data)

				compute_queue_index = (compute_queue_index + 1) % len(compute_queues)
			}

		case rpcstructs.MEMORY_HEAVY:
			if len(memory_queues) > 0 {
				memory_queues[memory_queue_index] = append(memory_queues[memory_queue_index], current.data)

				memory_queue_index = (memory_queue_index + 1) % len(memory_queues)
			}
		}

		current = current.next

	}

	// for each compute heavy queue


	var total_latency int64 = 0

	total_jobs := 0

	ComputeRunDuration := 0

	var total_cpu_utilization float64 = 0

	for q_idx := range compute_queues {

		// figure out utilization over the course of the dataset (both memory and cpu)

		// If the compute_queue is empty, do no calculation

		if len(compute_queues[q_idx]) == 0 {
			continue
		}

		job_start_time := compute_queues[q_idx][0].Timestamp

		for j_idx := range compute_queues[q_idx] {

			job := compute_queues[q_idx][j_idx]

			// When the job will finish (the same as when the next job is allowed to start)

			job_true_start := max(job_start_time, job.Timestamp)

			job_finish_time := job_true_start + job.ExecutionTime

			// Latency = time when job finished - time when job came in

			job_latency := job_finish_time - job.Timestamp

			// Weight cpu utilization with time spent processing

			total_cpu_utilization += job.CpuUtilization * float64(job.ExecutionTime)

			job_start_time = job_finish_time

			total_latency += job_latency

		}

		ComputeRunDuration += int(job_start_time)

		total_jobs += len(compute_queues[q_idx])

	}

	var total_memory_utilization float64 = 0

	MemoryRunDuration := 0

	for q_idx := range memory_queues {

		// figure out utilization over the course of the dataset (both memory and cpu)

		// If the compute_queue is empty, do no calculation

		if len(memory_queues[q_idx]) == 0 {
			continue
		}

		job_start_time := memory_queues[q_idx][0].Timestamp

		for j_idx := range memory_queues[q_idx] {

			job := memory_queues[q_idx][j_idx]

			// When the job will finish (the same as when the next job is allowed to start)

			job_true_start := max(job_start_time, job.Timestamp)

			job_finish_time := job_true_start + job.ExecutionTime

			// Latency = time when job finished - time when job came in

			job_latency := job_finish_time - job.Timestamp

			// Weight memory utilization with

			total_memory_utilization += job.MemoryUtilization * float64(job.ExecutionTime)

			job_start_time = job_finish_time

			total_latency += job_latency

		}

		MemoryRunDuration += int(job_start_time)

		total_jobs += len(memory_queues[q_idx])

	}

	average_utilization := total_memory_utilization/float64(MemoryRunDuration) + total_cpu_utilization/float64(ComputeRunDuration)

	return float64(total_latency) / float64(total_jobs), average_utilization

}

func (t *AutoScaler) autoscale(num_compute_heavy_available int, num_memory_heavy_available int) {
	// TODO: Write actual algorithm for autoscaling here

	// autoscaler has to be aware of which servers are online and offline so it knows what can be turned off or on

	// autoscaler has to be pre-trained on the trace

	// autoscaler also has to let load balancer know when it adds or removes a server

	// basic testing that autoscaler can interact with load balancer

	fmt.Println("Autoscaler started")

	for {

		fmt.Printf("x: %v\n", x)
		fmt.Printf("y: %v\n", y)
		x_prime, y_prime := t.findOptimalConfiguration(num_compute_heavy_available, num_memory_heavy_available)
		fmt.Println("Optimal configuration: ", x_prime, y_prime)
		fmt.Println("Current configuration: ", x, y)
		// The optimal configuration has changed scale up or down servers

		if x_prime != x || y_prime != y {

			if x_prime > x {

				// Add (x_prime - x) compute heavy server
				fmt.Println("Adding compute heavy server")

			}

			if y_prime > y {

				// Add (y_prime - y) memory heavy servers
				fmt.Println("Adding memory heavy server")
			}

			if x_prime < x {

				// Sort compute heavy servers by queue size, remove (x - x_prime) servers with the shortest queues
				fmt.Println("Removing compute heavy server")
			}

			if y_prime < y {

				// Sort memory heavy servers by queue size, remove (y - y_prime) servers with the shortest queues
				fmt.Println("Removing memory heavy server")

			}

		}
		// attempt autoscale every 5 seconds (?)
		time.Sleep(5 * time.Second)

	}

}

func (t *AutoScaler) startAutoscaler() { // server listener
	
	rpc.Register(t)

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
	// let all the servers start up and establish themselves as online

	config_file, _ := os.Open("config.txt")
	scanner := bufio.NewScanner(config_file)
	var line string
	scanner.Scan()
	num_servers := scanner.Text()
	scanner.Scan()
	num_initially_online := scanner.Text()
	fmt.Println("Number of servers: ", num_servers)
	fmt.Println("Number of initially online servers: ", num_initially_online)

	var num_memory_heavy_available int = 0
	var num_compute_heavy_available int = 0
	for scanner.Scan() {
		line = scanner.Text()
		words := strings.Fields(line)
		var server_type ServerType

		switch words[4] {

		case "C":

			server_type = COMPUTE_HEAVY

		case "M":

			server_type = MEMORY_HEAVY

		// TODO: Add more robust default here (e.g. unspecified) and error checking

		default:

			server_type = COMPUTE_HEAVY

		}

		server_to_status[words[1]] = ServerState{Status: OFFLINE, Server_Type: server_type, ComputeRemaining: -1, MemoryRemaining: -1} // everything starts offline until they identify themselves, -1 for resource util until known

		// Count whether server is online or offline

		if server_type == MEMORY_HEAVY {

			num_memory_heavy_available += 1

		} else if server_type == COMPUTE_HEAVY {

			num_compute_heavy_available += 1

		}
	}
	fmt.Println(num_compute_heavy_available)
	// fmt.Printf("num_compute_heavy_available: %v\n", num_compute_heavy_available)
	// fmt.Printf("num_memory_heavy_available: %v\n", num_memory_heavy_available)

	stat_handler := new(AutoScaler)
	go stat_handler.startAutoscaler()                                                  // handler to receive stats from servers
	go stat_handler.autoscale(num_compute_heavy_available, num_memory_heavy_available) // actual autoscaling logic

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	captureMetrics(server_to_status_overtime)
}

// TODO:
// Figure out how to tell the autoscaler which IP it is
