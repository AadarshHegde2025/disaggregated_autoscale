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
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

var LOAD_BALANCER_IP string = "sp25-cs525-0919.cs.illinois.edu" // Change this
var port int = 9001

// everything working

// TODO: aggregate the server stats over here. metrics: job latency, efficiency, graph of server usage
type AutoScaler struct{}

var server_to_status_overtime = make(map[string][]rpcstructs.ServerUsage)

var server_to_status = make(map[string]rpcstructs.ServerUsage) // server_ip -> ServerStatus
var job_completion_times = make(map[string][]int64)            // how much time between when the job was added to the server and when it was completed, for each server
var job_execution_times = make(map[string][]int64)             // how much time the job took to execute, for each server

var mu sync.Mutex

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
	status.Status = args.Status
	if status.Status == false {
		fmt.Println("Server , ", args.ServerIp, " is now offline")
	}
	status.ComputeRemaining = args.ComputeRemaining
	status.MemoryRemaining = args.MemoryRemaining
	status.JobToTiming = args.JobToTiming
	status.QueueLength = args.QueueLength
	status.Time = args.Time
	server_to_status[args.ServerIp] = status

	status2 := server_to_status_overtime[args.ServerIp]
	status2 = append(status2, status)
	server_to_status_overtime[args.ServerIp] = status2
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

func autoscale() {
	// TODO: Write actual algorithm for autoscaling here

	// autoscaler has to be aware of which servers are online and offline so it knows what can be turned off or on

	// autoscaler has to be pre-trained on the trace

	// autoscaler also has to let load balancer know when it adds or removes a server

	// basic testing that autoscaler can interact with load balancer
	time.Sleep(30 * time.Second) // wait for load balancer to start
	fmt.Println("Autoscaler is starting to send stats to load balancer")

	load_balancer, err := rpc.Dial("tcp", LOAD_BALANCER_IP+":"+strconv.Itoa(port))
	if err != nil {
		fmt.Println("Error connecting to load balancer:", err)
		return
	}

	var reply int

	// Add servers from sp25-cs525-0906 to sp25-cs525-0918 (inclusive)
	for i := 6; i <= 18; i++ {
		hostname := fmt.Sprintf("sp25-cs525-09%02d.cs.illinois.edu", i)
		nodeNumber := i - 1 // Or however you want to map this
		args := rpcstructs.ServerDetails{hostname, nodeNumber, "C"}

		err := load_balancer.Call("ServerChange.AddServer", &args, &reply)
		if err != nil {
			fmt.Printf("RPC AddServer failed for %s: %v\n", hostname, err)
		}
	}

	time.Sleep(20 * time.Second) // simulate some time passing before removals

	// Remove the same servers
	for i := 6; i <= 18; i++ {
		hostname := fmt.Sprintf("sp25-cs525-09%02d.cs.illinois.edu", i)
		nodeNumber := i - 1
		args := rpcstructs.ServerDetails{hostname, nodeNumber, "C"}

		err := load_balancer.Call("ServerChange.RemoveServer", &args, &reply)
		if err != nil {
			fmt.Printf("RPC RemoveServer failed for %s: %v\n", hostname, err)
		}
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
	var line string
	scanner.Scan()
	num_servers := scanner.Text()
	scanner.Scan()
	num_initially_online := scanner.Text()
	fmt.Println("Number of servers: ", num_servers)
	fmt.Println("Number of initially online servers: ", num_initially_online)
	for scanner.Scan() {
		line = scanner.Text()
		words := strings.Fields(line)
		server_to_status[words[1]] = rpcstructs.ServerUsage{Status: false, Server_Type: words[4], ComputeRemaining: -1, MemoryRemaining: -1} // everything starts offline until they identify themselves, -1 for resource util until known
	}

	go startAutoscaler() // handler to receive stats from servers
	go autoscale()       // actual autoscaling logic

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	captureMetrics(server_to_status_overtime)
}

// TODO:
// Figure out how to tell the autoscaler which IP it is
