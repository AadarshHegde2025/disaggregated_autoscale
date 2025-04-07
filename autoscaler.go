package main

import (
	"bufio"
	rpcstructs "disaggregated_autoscale/rpc_structs"
	"fmt"
	"image/color"
	"net"
	"net/rpc"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"math"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

var LOAD_BALANCER_IP string = "sp25-cs525-0919.cs.illinois.edu" // Change this
var port int = 9000

// everything working

// TODO: aggregate the server stats over here. metrics: job latency, efficiency, graph of server usage



// Define the enum type
type Status int

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

// This comes from the server
type ServerState struct {
	Status           Status		// True= Online, False = Offline
	Server_Type      ServerType 	// Compute or memory heavy
	ComputeRemaining float64	// 
	MemoryRemaining  float64	//
}

type AutoScaler struct{}

var server_to_status = make(map[string]ServerState)
var job_completion_times = make(map[string][]int64) // how much time between when the job was added to the server and when it was completed, for each server
var job_execution_times = make(map[string][]int64)  // how much time the job took to execute, for each server

var mu sync.Mutex

var snapshotMutex sync.Mutex

type SnapshotListNode struct {
	next *SnapshotListNode
	prev *SnapshotListNode
	data rpcstructs.Snapshot
}

type SnapshotList struct {
	head *SnapshotListNode
	tail *SnapshotListNode
}

var snapshotList SnapshotList = SnapshotList{}

// Here, x denotes the number of compute heavy VMs that are currently online  
var x int = 0

// Here, y denotes the number of compute heavy VMs that are currently online  
var y int = 0

func captureMetrics() {
	if len(job_completion_times) != len(job_execution_times) {
		panic("Server sets must match in size")
	}

	fmt.Println("Generating per-server graphs...")
	for server, completionList := range job_completion_times {
		executionList, ok := job_execution_times[server]
		if !ok || len(executionList) != len(completionList) {
			fmt.Printf("Skipping %s: mismatched execution data\n", server)
			continue
		}

		pointsCompletion := make(plotter.XYs, len(completionList))
		pointsExecution := make(plotter.XYs, len(executionList))

		for i := range completionList {
			pointsCompletion[i].X = float64(i)
			pointsCompletion[i].Y = float64(completionList[i])
			pointsExecution[i].X = float64(i)
			pointsExecution[i].Y = float64(executionList[i])
		}

		p := plot.New()
		p.Title.Text = fmt.Sprintf("Job Times - %s", server)
		p.X.Label.Text = "Job Index"
		p.Y.Label.Text = "Duration (ms)"

		line1, _ := plotter.NewLine(pointsCompletion)
		line1.Color = color.RGBA{R: 255, A: 100}

		line2, _ := plotter.NewLine(pointsExecution)
		line2.Color = color.RGBA{G: 200, A: 255}
		line2.Width = vg.Points(2)

		p.Add(line1, line2)
		p.Legend.Add("Total Time (Wait + Run)", line1)
		p.Legend.Add("Execution Time", line2)

		filename := fmt.Sprintf("server_%s.png", server)
		if err := p.Save(10*vg.Inch, 5*vg.Inch, filename); err != nil {
			fmt.Printf("Failed to save %s: %v\n", filename, err)
		}
	}

	fmt.Println("Generating aggregate graph...")

	// === Aggregate
	maxJobs := 0
	for _, list := range job_completion_times {
		if len(list) > maxJobs {
			maxJobs = len(list)
		}
	}

	avgCompletion := make([]float64, maxJobs)
	avgExecution := make([]float64, maxJobs)
	counts := make([]int, maxJobs)

	for server, completionList := range job_completion_times {
		execList := job_execution_times[server]
		for i := 0; i < len(completionList); i++ {
			avgCompletion[i] += float64(completionList[i])
			avgExecution[i] += float64(execList[i])
			counts[i]++
		}
	}

	for i := 0; i < maxJobs; i++ {
		if counts[i] > 0 {
			avgCompletion[i] /= float64(counts[i])
			avgExecution[i] /= float64(counts[i])
		}
	}

	pointsAvgCompletion := make(plotter.XYs, maxJobs)
	pointsAvgExecution := make(plotter.XYs, maxJobs)
	for i := 0; i < maxJobs; i++ {
		pointsAvgCompletion[i].X = float64(i)
		pointsAvgCompletion[i].Y = avgCompletion[i]
		pointsAvgExecution[i].X = float64(i)
		pointsAvgExecution[i].Y = avgExecution[i]
	}

	p := plot.New()
	p.Title.Text = "Aggregate Job Times (All Servers)"
	p.X.Label.Text = "Job Index"
	p.Y.Label.Text = "Avg Duration (ms)"

	line1, _ := plotter.NewLine(pointsAvgCompletion)
	line1.Color = color.RGBA{R: 255, A: 100}

	line2, _ := plotter.NewLine(pointsAvgExecution)
	line2.Color = color.RGBA{G: 200, A: 255}
	line2.Width = vg.Points(2)

	p.Add(line1, line2)
	p.Legend.Add("Avg Total Time", line1)
	p.Legend.Add("Avg Execution Time", line2)

	if err := p.Save(10*vg.Inch, 5*vg.Inch, "aggregate_job_times.png"); err != nil {
		fmt.Printf("Failed to save aggregate plot: %v\n", err)
	}
}

// RPC called by Server to update the statistics on the autoscaler side
func (t *AutoScaler) RequestedStats(args *rpcstructs.ServerUsage, reply *string) error {
	mu.Lock()
	fmt.Println("Received server stats:", args.ServerIp, args.ComputeUsage, args.MemoryUsage, args.JobCompletionTime)
	status := server_to_status[args.ServerIp] // mark the server as online
	status.Status = ONLINE
	status.ComputeRemaining = args.ComputeUsage
	status.MemoryRemaining = args.MemoryUsage
	server_to_status[args.ServerIp] = status

	completion_array := job_completion_times[args.ServerIp]
	execution_array := job_execution_times[args.ServerIp]

	completion_array = append(completion_array, args.JobCompletionTime)
	execution_array = append(execution_array, args.JobTraceExecutionTime)

	job_completion_times[args.ServerIp] = completion_array
	job_execution_times[args.ServerIp] = execution_array
	// TODO : Add logic to store the stats in some data structure so that we can do predictive autoscaling

	mu.Unlock()
	*reply = "Stats received"
	return nil
}

// RPC to add a snapshot to the (doubly) linked list
func (t *AutoScaler) AddSnapshotToList(args *rpcstructs.Snapshot, reply *string) error {
	snapshotMutex.Lock()
	defer snapshotMutex.Unlock()

	snapshot := rpcstructs.Snapshot{JobType: args.JobType, CpuUtilization: args.CpuUtilization, MemoryUtilization: args.MemoryUtilization}
	// There is no node in our list, add the new node as the head and tail
	if snapshotList.head == nil {
		snapshotNode := &SnapshotListNode{nil, nil, snapshot}
		snapshotList.head = snapshotNode
		snapshotList.tail = snapshotNode
	
	// There is a node in our list already, add the new node as the tail
	} else{
		snapshotNode := &SnapshotListNode{nil, snapshotList.tail, snapshot}
		snapshotList.tail.next = snapshotNode
		snapshotList.tail = snapshotNode
	}

	*reply = "Snapshot Added"
	return nil
}

// Truncates all of the nodes in the list BEFORE node (i.e. node 
// becomes the new head of our linked list)
func truncateHistory(list *SnapshotList, node *SnapshotListNode){
	node.prev = nil
	list.head = node
}

func findOptimalConfiguration(num_compute_heavy_available int, num_memory_heavy_available int) (int, int){
	var x_prime int
	var y_prime int
	
	var maxVal float64 = math.Inf(-1)


	for i := 0; i < num_compute_heavy_available; i++{
		for j := 0; j < num_memory_heavy_available; j++{
			utility := calculateUtilityFunction(i, j)
			if utility > maxVal {
				maxVal = utility
				x_prime = i
				y_prime = j

			// In case of tiebreak choose configuration that requires least change of servers
			} else if utility == maxVal && 
				(math.Abs(float64(x_prime - x)) + math.Abs(float64(y_prime - y)) > math.Abs(float64(i - x)) + math.Abs(float64(j - y))) {
				maxVal = utility
				x_prime = i
				y_prime = j
			}
		}
	}
	return x_prime, y_prime
}

// calculates the objective function given an x_prime and y_prime (x, y) are implied 
func calculateUtilityFunction(x_prime int, y_prime int) float64{
	var alpha float64 = 1.0
	var lambda float64 = 1.0
	var gamma float64 = 1.0

	// First term
	var A float64 = 0
	var B float64 = 0
	var C float64 = 0
	var D float64 = 0

	A,B = calculateExpectedLatencyAndUtilization(x_prime, y_prime) 

	C = float64(x_prime + y_prime)
	D = float64(max(x_prime - x,0) + max(y_prime - y, 0))

	return alpha * A + lambda * B + C + gamma * D
}

func calculateExpectedLatencyAndUtilization(x_prime int, y_prime int) (float64, float64){
	snapshotMutex.Lock()
	defer snapshotMutex.Unlock()

	const N = 10
	// Look through linked list backwards until we find a timestamp that was N seconds before now
	now := time.Now().Unix()
	current := snapshotList.tail
	for {
		if(current == nil) { break }

		if(current.data.Timestamp < now - N){
			// Remove all data from the linked list that is before our threshold (N)
			truncateHistory(&snapshotList, current.next)
			break
		}
		current = current.prev
	}

	// TODO: Implement simulation of queues logic
	// Initialize queues
	compute_queues := make([][]rpcstructs.Snapshot, x_prime)
	for i := range compute_queues {
		compute_queues[i] = make([]rpcstructs.Snapshot, 0)
	}

	memory_queues := make([][]rpcstructs.Snapshot, y_prime)
	for i := range memory_queues {
		memory_queues[i] = make([]rpcstructs.Snapshot, 0)
	}

	// Assign jobs from the linked list to the queue
	current = snapshotList.head
	compute_queue_index := 0
	memory_queue_index := 0
	for {
		if current == nil {
			break
		}

		// Add job to a queue depending on what type it is
		jobType := current.data.JobType
		switch jobType{
		case rpcstructs.COMPUTE_HEAVY:
			compute_queues[compute_queue_index] = append(compute_queues[compute_queue_index], current.data)
			compute_queue_index = (compute_queue_index + 1) % len(compute_queues)
		case rpcstructs.MEMORY_HEAVY:
			memory_queues[memory_queue_index] = append(memory_queues[memory_queue_index], current.data)
			memory_queue_index = (memory_queue_index + 1) % len(memory_queues) 
		}
		current = current.next
	}

	// for each compute heavy queue
	// t
	var total_latency int64 = 0
	total_jobs := 0
	ComputeRunDuration := 0
	var total_cpu_utilization float64 = 0
	for q_idx := range compute_queues {
		// figure out utilization over the course of the dataset (both memory and cpu)
		// If the compute_queue is empty, do no calculation
		if len(compute_queues[q_idx]) == 0 { continue }
		job_start_time := compute_queues[q_idx][0].Timestamp

		for j_idx := range compute_queues[q_idx]{
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
		if len(memory_queues[q_idx]) == 0 { continue }
		job_start_time := memory_queues[q_idx][0].Timestamp

		for j_idx := range memory_queues[q_idx]{
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

	average_utilization := total_memory_utilization / float64(MemoryRunDuration) + total_cpu_utilization / float64(ComputeRunDuration)
	return float64(total_latency)/float64(total_jobs) , average_utilization
}

func autoscale(num_compute_heavy_available int, num_memory_heavy_available int) {


	// TODO: Write actual algorithm for autoscaling here
	// Use LP approach here
	
	// Let x be the number of compute heavy servers with specs: c1 CPU, m1 Memory, d1 Disk
	// Let y be the number of memory heavy servers with specs: c2 CPU, m2 Memory, d2 Disk
 
	// Constraints:
	//  x + y <= NUM SERVERS - 2      	# (1 for load balancer, 1 for autoscaler) (This is redundant)
	//  x + y >= 1     				  	# At least one server must be online
	//  x <= Number of compute heavy servers available
	//  y <= Number of memory heavy servers available 
	
	// Objective Function:
	//  1/ Latency + lambda * utilization - (Fixed cost for every online server * unit time) - (startup cost for new servers)
	// Fixed cost per online server	= lambda * (x + y)				# 
	// Startup cost for a server 		# May be helpful to prevent thrashing
	// Goal: We look to find a (x, y) pair that maximizes the utility function

	for {
		x_prime, y_prime := findOptimalConfiguration(num_compute_heavy_available, num_memory_heavy_available)
		// The optimal configuration has changed scale up or down servers 
		if x_prime != x || y_prime != y {
			if (x_prime > x){
				// Add (x_prime - x) compute heavy server
			}
			if (y_prime > y){
				// Add (y_prime - y) memory heavy servers
			}
			if(x_prime < x){
				// Sort compute heavy servers by queue size, remove (x - x_prime) servers with the shortest queues
			}
			if(y_prime < y){
				// Sort memory heavy servers by queue size, remove (y - y_prime) servers with the shortest queues 
			}
	
		}
		// attempt autoscale every 5 seconds (?)
		time.Sleep(5 * time.Second)
	}


	// autoscaler has to be aware of which servers are online and offline so it knows what can be turned off or on

	// autoscaler has to be pre-trained on the trace

	// autoscaler also has to let load balancer know when it adds or removes a server

	// basic testing that autoscaler can interact with load balancer
	time.Sleep(20 * time.Second) // TODO: CHANGE THIS wait for load balancer to start
	fmt.Println("Autoscaler is starting to send stats to load balancer")
	load_balancer, err := rpc.Dial("tcp", LOAD_BALANCER_IP+":"+strconv.Itoa(port))
	if err != nil {
		fmt.Println("Error connecting to load balancer:", err)
		return
	}
	args := rpcstructs.ServerDetails{"sp25-cs525-0906.cs.illinois.edu", 5} // TODO: This is just a test

	var reply int
	load_balancer.Call("AddingServer.AddServer", &args, &reply)
	if err != nil {
		fmt.Println("RPC call failed:", err) // Check if this prints
	}

}

func startAutoscaler() { // server listener
	stat_handler := new(AutoScaler)
	rpc.Register(stat_handler)

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


	scanner.Scan()
	num_servers := scanner.Text()
	scanner.Scan()
	num_initially_online := scanner.Text()
	fmt.Println("Number of servers: ", num_servers)
	fmt.Println("Number of initially online servers: ", num_initially_online)

	num_compute_heavy_available := 0
	num_compute_heavy_active := 0

	num_memory_heavy_available := 0
	num_memory_heavy_active := 0
	// Read in the config line by line
	var line string
	for scanner.Scan() {
		line = scanner.Text()
		words := strings.Fields(line)
		var server_type ServerType
		switch words[3] {
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

	fmt.Printf("num_compute_heavy: %v\n", num_compute_heavy_available)
	fmt.Printf("num_compute_heavy_active: %v\n", num_compute_heavy_active)
	
	fmt.Printf("num_memory_heavy: %v\n", num_memory_heavy_available)
	fmt.Printf("num_memory_heavy_active: %v\n", num_memory_heavy_active)

	go startAutoscaler() // handler to receive stats from servers
	go autoscale(num_compute_heavy_available, num_memory_heavy_available)       // actual autoscaling logic

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	captureMetrics()
}

// TODO:
// Figure out how to tell the autoscaler which IP it is
