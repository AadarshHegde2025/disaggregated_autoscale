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

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

var LOAD_BALANCER_IP string = "sp25-cs525-0919.cs.illinois.edu" // Change this
var port int = 9000

// everything working

// TODO: aggregate the server stats over here. metrics: job latency, efficiency, graph of server usage

type ServerStatus struct {
	Status           bool
	Server_Type      string // computer or memory heavy
	ComputeRemaining float64
	MemoryRemaining  float64
	JobToTiming      map[rpcstructs.Pair]rpcstructs.JobTiming
}

type AutoScaler struct{}

var server_to_status_overtime = make(map[string][]ServerStatus)

var server_to_status = make(map[string]ServerStatus)
var job_completion_times = make(map[string][]int64) // how much time between when the job was added to the server and when it was completed, for each server
var job_execution_times = make(map[string][]int64)  // how much time the job took to execute, for each server

var mu sync.Mutex

func plotLineGraph(title, xlabel, ylabel string, data plotter.XYs, filename string, lineColor color.RGBA) error {
	p := plot.New()
	p.Title.Text = title
	p.X.Label.Text = xlabel
	p.Y.Label.Text = ylabel

	line, err := plotter.NewLine(data)
	if err != nil {
		return err
	}
	line.Color = lineColor
	line.Width = vg.Points(2)

	p.Add(line)
	p.Legend.Add(title, line)

	return p.Save(10*vg.Inch, 5*vg.Inch, filename)
}

func captureMetrics(servers map[string][]ServerStatus) {
	fmt.Println("Generating resource usage graphs...")

	for serverID, snapshots := range servers {
		var computePoints plotter.XYs
		var memoryPoints plotter.XYs

		for i, snapshot := range snapshots {
			t := float64(i * 5) // time in seconds (5-second interval)
			computePoints = append(computePoints, plotter.XY{X: t, Y: snapshot.ComputeRemaining})
			memoryPoints = append(memoryPoints, plotter.XY{X: t, Y: snapshot.MemoryRemaining})
		}

		// --- Plot Compute Remaining ---
		if err := plotLineGraph(
			fmt.Sprintf("Compute Remaining - %s", serverID),
			"Time (s)", "Compute Remaining (%)", computePoints,
			fmt.Sprintf("compute_remaining_%s.png", serverID),
			color.RGBA{R: 100, G: 200, B: 255, A: 255}); err != nil {
			fmt.Println("Failed to plot compute:", err)
		}

		// --- Plot Memory Remaining ---
		if err := plotLineGraph(
			fmt.Sprintf("Memory Remaining - %s", serverID),
			"Time (s)", "Memory Remaining (%)", memoryPoints,
			fmt.Sprintf("memory_remaining_%s.png", serverID),
			color.RGBA{R: 150, G: 255, B: 150, A: 255}); err != nil {
			fmt.Println("Failed to plot memory:", err)
		}
	}

	fmt.Println("Done plotting usage graphs.")
}

func (t *AutoScaler) RequestedStats(args *rpcstructs.ServerUsage, reply *string) error {
	mu.Lock()
	fmt.Println("Received server stats:", args.ServerIp, args.ComputeUsage, args.MemoryUsage)
	status := server_to_status[args.ServerIp] // mark the server as online
	status.Status = true
	status.ComputeRemaining = args.ComputeUsage
	status.MemoryRemaining = args.MemoryUsage
	status.JobToTiming = args.JobToTiming
	server_to_status[args.ServerIp] = status

	status2 := server_to_status_overtime[args.ServerIp]
	status2 = append(status2, status)
	server_to_status_overtime[args.ServerIp] = status2
	// TODO : Add logic to store the stats in some data structure so that we can do predictive autoscaling

	mu.Unlock()
	*reply = "Stats received"
	return nil
}

func autoscale() {
	// TODO: Write actual algorithm for autoscaling here

	// autoscaler has to be aware of which servers are online and offline so it knows what can be turned off or on

	// autoscaler has to be pre-trained on the trace

	// autoscaler also has to let load balancer know when it adds or removes a server

	// basic testing that autoscaler can interact with load balancer
	time.Sleep(70 * time.Second) // TODO: CHANGE THIS wait for load balancer to start
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

	args = rpcstructs.ServerDetails{"sp25-cs525-0907.cs.illinois.edu", 6} // TODO: This is just a test

	load_balancer.Call("AddingServer.AddServer", &args, &reply)
	if err != nil {
		fmt.Println("RPC call failed:", err) // Check if this prints
	}
	args = rpcstructs.ServerDetails{"sp25-cs525-0908.cs.illinois.edu", 7} // TODO: This is just a test

	load_balancer.Call("AddingServer.AddServer", &args, &reply)
	if err != nil {
		fmt.Println("RPC call failed:", err) // Check if this prints
	}

	args = rpcstructs.ServerDetails{"sp25-cs525-0909.cs.illinois.edu", 8} // TODO: This is just a test

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
		server_to_status[words[1]] = ServerStatus{Status: false, Server_Type: words[3], ComputeRemaining: -1, MemoryRemaining: -1} // everything starts offline until they identify themselves, -1 for resource util until known
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
