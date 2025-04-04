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

var LOAD_BALANCER_IP string = "192.168.4.248" // Change this
var port int = 9000

type AutoScaler struct{}

var server_to_status = make(map[string]bool)

var mu sync.Mutex

func (t *AutoScaler) RequestedStats(args *rpcstructs.ServerUsage, reply *string) error {
	mu.Lock()
	fmt.Println("Received server stats:", args.ServerIp, args.ComputeUsage, args.MemoryUsage)
	server_to_status[args.ServerIp] = true // mark the server as online
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
	load_balancer, _ := rpc.Dial("tcp", LOAD_BALANCER_IP+":"+strconv.Itoa(port))
	args := rpcstructs.ServerDetails{"sp25-cs525-0906.cs.illinois.edu", 5} // TODO: fill in with actual values from the trace

	var reply int
	load_balancer.Call("AddingServer.AddServer", &args, &reply)

}

func startAutoscaler() {
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
		server_to_status[words[1]] = false // everything starts offline until they identify themselves
	}

	time.Sleep(6 * time.Second)
	go startAutoscaler() // handler to receive stats from servers
	go autoscale()       // actual autoscaling logic

	// Keep the main function running
	select {} // Blocks forever
}

// TODO:
// Figure out how to tell the autoscaler which IP it is
