package main

import (
	"bufio"
	rpcstructs "disaggregated_autoscale/rpc_structs"
	"encoding/csv"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type AddingServer struct{}

var mu sync.Mutex

var connected_servers map[int]string = make(map[int]string)
var port int = 9000

func (t *AddingServer) AddServer(args *rpcstructs.ServerDetails, reply *int) error {
	mu.Lock()
	fmt.Println("Adding server:", args.ServerIp, "with node number:", args.NodeNumber)
	connected_servers[args.NodeNumber] = args.ServerIp
	mu.Unlock()
	*reply = 0
	return nil
}

func ListenForAutoscalerUpdates() {
	server_adder := new(AddingServer)
	rpc.Register(server_adder)

	listener, err := net.Listen("tcp", ":9000")
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
	fmt.Println("Load balancer listening on port 9000")

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
	// Redirect output to debug file
	debugFile, err := os.OpenFile("loadbalancer_debug.txt", os.O_CREATE, 0666)
	if err != nil {
		fmt.Println("Error opening debug file:", err)
		return
	}
	defer debugFile.Close()
	os.Stdout = debugFile

	// Processing Config File

	config_file, _ := os.Open("config.txt")
	scanner := bufio.NewScanner(config_file)

	number_of_online_servers := 0

	i := 0
	for scanner.Scan() {
		line := scanner.Text()
		if i == 0 {
			// num, _ := strconv.Atoi(line)
			// total_servers = num - 2
		} else if i == 1 {
			num, _ := strconv.Atoi(line)
			number_of_online_servers = num
		} else {
			// normal servers
			words := strings.Fields(line)
			mu.Lock()
			connected_servers[i-2] = strings.TrimSpace(words[1])
			mu.Unlock()
		}

		if i-2 == number_of_online_servers {
			break
		}
		i += 1
	}

	// Listen for server updates
	go ListenForAutoscalerUpdates()

	// Process Jobs:

	file, _ := os.Open("./data/cleaned_file.csv")
	reader := csv.NewReader(file)
	i = 0
	for {
		record, _ := reader.Read()
		if i == 0 {
			// Skip the first line (header)
			i += 1
			continue

		}
		fmt.Println(record)
		mu.Lock()
		fmt.Println("sending to: ", connected_servers[i%number_of_online_servers])
		client, _ := rpc.Dial("tcp", connected_servers[i%number_of_online_servers]+":"+strconv.Itoa(port))
		mu.Unlock()
		// fmt.Println(record[6])
		job_id, _ := strconv.Atoi(record[2])
		task_id, _ := strconv.Atoi(record[3])
		plan_cpu, _ := strconv.ParseFloat(record[6], 64)
		plan_mem, _ := strconv.ParseFloat(record[7], 64)
		start_time, _ := strconv.Atoi(record[8])
		end_time, _ := strconv.Atoi(record[9])

		fmt.Println("data: ", job_id, " ", task_id, " ", plan_cpu, " ", plan_mem, " ", start_time, " ", end_time)
		mu.Lock()
		args := rpcstructs.Args{job_id, plan_cpu, plan_mem, start_time, end_time, task_id, connected_servers[i%number_of_online_servers]} // TODO: fill in with actual values from the trace
		mu.Unlock()
		var reply int
		client.Call("HandleJob.AddJobs", &args, &reply)
		i += 1
		time.Sleep(500 * time.Millisecond)
	}

}

/* My notes:

- Use different type of load balancers that we can test with command line arguments
- Need really accurate metrics to see how well this is all working, so there should be some server side logic for that
- For each job given to a server, the server has to track when it ends in order to "deallocate" the simulated resource
- Do we want to artificially generate timestamps to handle short term workloads?


*/
