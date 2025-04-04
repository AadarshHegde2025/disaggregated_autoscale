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

type AddingServer struct{}

var mu sync.Mutex
var mu2 sync.Mutex

var connected_servers map[int]string = make(map[int]string)
var port int = 9000
var number_of_online_servers int = 0

func retrieve_corresponding_real_resource_util(job_id int, task_id int) (float64, float64, int, int) {
	fmt.Println("Retrieving real resource utilization for job_id:", job_id, "task_id:", task_id)
	db, err := sql.Open("sqlite3", "./batch_data.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	query := `
		SELECT real_cpu_max, real_mem_max, start_timestamp, end_timestamp
		FROM instances
		WHERE job_id = ? AND task_id = ?
		AND real_cpu_max IS NOT NULL
  		AND real_mem_max IS NOT NULL
		AND status = 'TERMINATED'
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

func (t *AddingServer) AddServer(args *rpcstructs.ServerDetails, reply *int) error {
	mu.Lock()
	fmt.Println("Adding server:", args.ServerIp, "with node number:", args.NodeNumber)
	mu2.Lock()
	number_of_online_servers += 1
	mu2.Unlock()
	connected_servers[args.NodeNumber] = args.ServerIp
	mu.Unlock()
	*reply = 0
	return nil
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
		fmt.Println("sending to: ", connected_servers[i%number_of_online_servers])
		client, _ := rpc.Dial("tcp", connected_servers[i%number_of_online_servers]+":"+strconv.Itoa(port))
		mu2.Unlock()
		mu.Unlock()
		// fmt.Println(record[6])
		var job_id int
		var task_id int
		var plan_cpu float64
		var plan_mem float64

		rows.Scan(&job_id, &task_id, &plan_cpu, &plan_mem)

		fmt.Println("data: ", job_id, " ", task_id, " ", plan_cpu, " ", plan_mem)
		mu.Lock()
		mu2.Lock()
		real_cpu, real_mem, start_time, end_time := retrieve_corresponding_real_resource_util(job_id, task_id)
		args := rpcstructs.Args{job_id, plan_cpu, plan_mem, start_time, end_time, task_id, connected_servers[i%number_of_online_servers], real_cpu, real_mem} // TODO: fill in with actual values from the trace
		mu2.Unlock()
		mu.Unlock()
		var reply int
		client.Call("HandleJob.AddJobs", &args, &reply)
		i += 1
		time.Sleep(1000 * time.Millisecond)
	}

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
		if i == 0 {
			// num, _ := strconv.Atoi(line)
			// total_servers = num - 2
		} else if i == 1 {
			num, _ := strconv.Atoi(line)
			mu2.Lock()
			number_of_online_servers = num
			mu2.Unlock()
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
	round_robin_loadbalancer()
}

/* My notes:

- Use different type of load balancers that we can test with command line arguments
- Need really accurate metrics to see how well this is all working, so there should be some server side logic for that
- For each job given to a server, the server has to track when it ends in order to "deallocate" the simulated resource
- Do we want to artificially generate timestamps to handle short term workloads?


*/
