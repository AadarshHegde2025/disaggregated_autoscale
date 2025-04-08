package main

import (
	"bufio"
	"database/sql"
	rpcstructs "disaggregated_autoscale/rpc_structs"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// TODO: Update config file to determine which servers are compute heavy and which are memory heavy

type ServerChange struct{}

var mu sync.Mutex
var mu2 sync.Mutex

var connected_servers map[int]string = make(map[int]string) // node number -> server ip
var server_to_type = make(map[string]string)                // server ip -> server type
var port int = 9001
var number_of_online_servers int = 0

var compute_online_servers = []string{} // list of available compute servers
var memory_online_servers = []string{}  // list of available memory servers

func retrieve_corresponding_real_resource_util(job_id int, task_id int) (float64, float64, int, int) {
	db, err := sql.Open("sqlite3", "./batch_data.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	query := `
		SELECT real_cpu_max, real_mem_max, start_timestamp, end_timestamp
		FROM instances
		WHERE job_id = ? AND task_id = ?
		LIMIT 1
	`

	var cpuAvg, memAvg float64
	var start_time, end_time int
	err = db.QueryRow(query, job_id, task_id).Scan(&cpuAvg, &memAvg, &start_time, &end_time)

	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("No matching record found.")
			return 0, 0, 0, 0
		}
		log.Fatal(err)
	}

	return cpuAvg, memAvg, start_time, end_time

}

func removeFromSlice(slice []string, item string) []string {
	newSlice := make([]string, 0, len(slice))
	for _, v := range slice {
		if v != item {
			newSlice = append(newSlice, v)
		}
	}
	return newSlice
}

func (t *ServerChange) AddServer(args *rpcstructs.ServerDetails, reply *int) error { // what type of server are we adding?
	mu.Lock()
	fmt.Println("Adding server:", args.ServerIp, "with node number:", args.NodeNumber)
	mu2.Lock()
	number_of_online_servers += 1
	mu2.Unlock()
	connected_servers[args.NodeNumber] = args.ServerIp
	if args.ServerType == "C" {
		compute_online_servers = append(compute_online_servers, args.ServerIp)
	} else {
		memory_online_servers = append(memory_online_servers, args.ServerIp)
	}

	mu.Unlock()
	*reply = 0
	return nil
}

func (t *ServerChange) RemoveServer(args *rpcstructs.ServerDetails, reply *int) error { // what type of server are we removing?
	mu.Lock()
	fmt.Println("Removing server server:", args.ServerIp, "with node number:", args.NodeNumber)
	mu2.Lock()
	number_of_online_servers -= 1
	mu2.Unlock()
	client, _ := rpc.Dial("tcp", connected_servers[args.NodeNumber]+":"+strconv.Itoa(port))
	client.Call("HandleJob.ShutDownServer", &args, &reply) // args field doesn't really matter, is not considered by server
	delete(connected_servers, args.NodeNumber)
	if args.ServerType == "C" {
		compute_online_servers = removeFromSlice(compute_online_servers, args.ServerIp)
	} else {
		memory_online_servers = removeFromSlice(memory_online_servers, args.ServerIp)
	}
	mu.Unlock()
	*reply = 0
	return nil
}

func resource_awareness_loadbalancer() {
	db, _ := sql.Open("sqlite3", "./batch_data.db")
	rows, _ := db.Query(`
		SELECT job_id, task_id, plan_cpu, plan_mem
		FROM tasks
	`)

	compute_i := 0
	memory_i := 0
	i := 0
	for rows.Next() {
		// fmt.Println(record[6])
		var job_id int
		var task_id int
		var plan_cpu float64
		var plan_mem float64

		rows.Scan(&job_id, &task_id, &plan_cpu, &plan_mem)
		var main_client *rpc.Client
		var server_ip string

		var values []string
		for _, v := range connected_servers {
			values = append(values, v)
		}

		if (plan_cpu/(100*64)) > plan_mem && len(compute_online_servers) > 0 { // TODO: this is a very naive way of determining if the job is compute or memory heavy, need to be more sophisticated
			server_ip = compute_online_servers[compute_i%len(compute_online_servers)]
			fmt.Println("Sending to compute server: ", server_ip)
			client, err := rpc.Dial("tcp", server_ip+":"+strconv.Itoa(port))
			if err != nil {
				fmt.Println("Error connecting to server:", err)
				continue
			}
			main_client = client
			compute_i += 1

		} else if len(memory_online_servers) > 0 {
			server_ip = memory_online_servers[memory_i%len(memory_online_servers)]
			fmt.Println("Sending to memory server: ", server_ip)
			client, err := rpc.Dial("tcp", server_ip+":"+strconv.Itoa(port))
			if err != nil {
				fmt.Println("Error connecting to server:", err)
				continue
			}
			main_client = client
			memory_i += 1
		} else {
			fmt.Println("No match: ", server_ip)
			server_ip = values[i%number_of_online_servers]
			client, err := rpc.Dial("tcp", server_ip+":"+strconv.Itoa(port))
			if err != nil {
				fmt.Println("Error connecting to server:", err)
				continue
			}
			main_client = client
		}

		mu.Lock()
		mu2.Lock()
		real_cpu, real_mem, start_time, end_time := retrieve_corresponding_real_resource_util(job_id, task_id)
		args := rpcstructs.Args{job_id, plan_cpu, plan_mem, start_time, end_time, task_id, server_ip, real_cpu, real_mem, server_to_type[server_ip]} // TODO: fill in with actual values from the trace
		// fmt.Println("data: ", job_id, " ", task_id, " ", plan_cpu, " ", plan_mem, " ", real_cpu, " ", real_mem)
		mu2.Unlock()
		mu.Unlock()
		var reply int
		err := main_client.Call("HandleJob.AddJobs", &args, &reply)
		if err != nil {
			fmt.Println("Error calling AddJobs:", err)
			continue
		}
		time.Sleep(1000 * time.Millisecond)
		i += 1
	}
}

func round_robin_loadbalancer() {
	db, _ := sql.Open("sqlite3", "./batch_data.db")
	rows, _ := db.Query(`
		SELECT job_id, task_id, plan_cpu, plan_mem
		FROM tasks
	`)

	i := 0
	for rows.Next() {

		mu.Lock()
		mu2.Lock()

		var values []string
		for _, v := range connected_servers {
			values = append(values, v)
		}

		// fmt.Println("sending to: ", connected_servers[i%number_of_online_servers])
		client, _ := rpc.Dial("tcp", values[i%number_of_online_servers]+":"+strconv.Itoa(port))
		mu2.Unlock()
		mu.Unlock()
		// fmt.Println(record[6])
		var job_id int
		var task_id int
		var plan_cpu float64
		var plan_mem float64

		rows.Scan(&job_id, &task_id, &plan_cpu, &plan_mem)

		mu.Lock()
		mu2.Lock()
		real_cpu, real_mem, start_time, end_time := retrieve_corresponding_real_resource_util(job_id, task_id)
		args := rpcstructs.Args{job_id, plan_cpu, plan_mem, start_time, end_time, task_id, values[i%number_of_online_servers], real_cpu, real_mem, server_to_type[values[i%number_of_online_servers]]} // TODO: fill in with actual values from the trace
		// fmt.Println("data: ", job_id, " ", task_id, " ", plan_cpu, " ", plan_mem, " ", real_cpu, " ", real_mem)
		mu2.Unlock()
		mu.Unlock()
		var reply int
		client.Call("HandleJob.AddJobs", &args, &reply)
		i += 1
		time.Sleep(10 * time.Millisecond)
	}

}

func ListenForAutoscalerUpdates() {
	server_adder := new(ServerChange)
	rpc.Register(server_adder)

	listener, err := net.Listen("tcp", ":9001")
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
		fmt.Println("Connection accepted from:", conn.RemoteAddr())
		go rpc.ServeConn(conn)
	}
}

func processConfigFile() {
	config_file, _ := os.Open("config.txt")
	scanner := bufio.NewScanner(config_file)

	i := 0
	for scanner.Scan() {
		line := scanner.Text()
		words := strings.Fields(line)
		if words[4] == "C" {
			server_to_type[words[1]] = "C"
		} else if words[4] == "M" {
			server_to_type[words[1]] = "M"
		}

		if words[3] == "O" {
			number_of_online_servers += 1
			connected_servers[i] = strings.TrimSpace(words[1])

			if server_to_type[words[1]] == "C" {
				compute_online_servers = append(compute_online_servers, strings.TrimSpace(words[1]))
			} else if server_to_type[words[1]] == "M" {
				memory_online_servers = append(memory_online_servers, strings.TrimSpace(words[1]))
			}

		}
		i += 1
	}
	fmt.Println("post processing: ", connected_servers)
}

func main() {
	// Redirect output to debug file if needed
	debugFile, err := os.OpenFile("loadbalancer_debug.txt", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		fmt.Println("Error opening debug file:", err)
		return
	}
	defer debugFile.Close()
	// os.Stdout = debugFile

	// Processing Config File
	processConfigFile()

	// Listen for server updates
	go ListenForAutoscalerUpdates()

	// // Process Jobs:
	resource_awareness_loadbalancer()
	// round_robin_loadbalancer()
}

/* My notes:

- Use different type of load balancers that we can test with command line arguments
- Need really accurate metrics to see how well this is all working, so there should be some server side logic for that
- For each job given to a server, the server has to track when it ends in order to "deallocate" the simulated resource
- Do we want to artificially generate timestamps to handle short term workloads?


*/
