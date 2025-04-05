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
}

type AutoScaler struct{}

var server_to_status = make(map[string]ServerStatus)
var job_completion_times = make(map[string][]int64) // how much time between when the job was added to the server and when it was completed, for each server
var job_execution_times = make(map[string][]int64)  // how much time the job took to execute, for each server

var mu sync.Mutex

func captureMetrics() {
	fmt.Println("Generating per-server wait time graphs...")

	for server, completionList := range job_completion_times {
		executionList, ok := job_execution_times[server]
		if !ok || len(executionList) != len(completionList) {
			fmt.Printf("Skipping %s: mismatched execution data\n", server)
			continue
		}

		pointsWait := make(plotter.XYs, len(completionList))
		for i := range completionList {
			waitTime := float64(completionList[i] - executionList[i])
			if waitTime < 0 {
				waitTime = 0 // safety check for any inconsistent timestamps
			}
			pointsWait[i].X = float64(i)
			pointsWait[i].Y = waitTime
		}

		p := plot.New()
		p.Title.Text = fmt.Sprintf("Wait Time - %s", server)
		p.X.Label.Text = "Job Index"
		p.Y.Label.Text = "Wait Time (ms)"

		line, err := plotter.NewLine(pointsWait)
		if err != nil {
			fmt.Printf("Error creating line for %s: %v\n", server, err)
			continue
		}
		line.Color = color.RGBA{B: 255, A: 200} // Blue with some transparency
		line.Width = vg.Points(2)

		p.Add(line)
		p.Legend.Add("Wait Time", line)

		filename := fmt.Sprintf("wait_time_%s.png", server)
		if err := p.Save(10*vg.Inch, 5*vg.Inch, filename); err != nil {
			fmt.Printf("Failed to save %s: %v\n", filename, err)
		}
	}

	fmt.Println("Done.")

}

func (t *AutoScaler) RequestedStats(args *rpcstructs.ServerUsage, reply *string) error {
	mu.Lock()
	fmt.Println("Received server stats:", args.ServerIp, args.ComputeUsage, args.MemoryUsage, args.JobCompletionTime)
	status := server_to_status[args.ServerIp] // mark the server as online
	status.Status = true
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

func autoscale() {
	// TODO: Write actual algorithm for autoscaling here

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
	captureMetrics()
}

// TODO:
// Figure out how to tell the autoscaler which IP it is
