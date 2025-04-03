package main

import (
	"bufio"
	rpcstructs "disaggregated_autoscale/rpc_structs"
	"encoding/csv"
	"fmt"
	"net/rpc"
	"os"
	"strconv"
	"strings"
	"time"
)

type ServerState int

const (
	ONLINE ServerState = iota
	OFFLINE
)

func main() {
	/* General Architecture:
	- When a client sends some request for work, it first goes to the load balancer to determine which server the job goes to
	- The load balancer will read the trace csv, and route the job id to the best available server
	- Each server needs to maintain "resource counters" - essentially statistics on how much of its resources are being used.
	  Load balancer can send an RPC to any server to get its resource utilization
	- The autoscaler's job is to monitor resource utilization on servers, and based on some algorithm, dynamically add or remove
	  vms from the system to match the workload.
	*/

	// TODO: process config file here, set up connections, choose an initial subset of servers to be "online"

	config_file, _ := os.Open("config.txt")
	scanner := bufio.NewScanner(config_file)

	total_servers := 0
	number_of_online_servers := 0
	var connected_servers map[int]string = make(map[int]string)
	var port int = 9000

	i := 0
	for scanner.Scan() {
		line := scanner.Text()
		if i == 0 {
			num, _ := strconv.Atoi(line)
			total_servers = num - 2
		} else if i == 1 {
			num, _ := strconv.Atoi(line)
			number_of_online_servers = num
		} else {
			// normal servers
			words := strings.Fields(line)
			connected_servers[i-2] = strings.TrimSpace(words[1])
		}

		if i-2 == number_of_online_servers {
			break
		}
		i += 1
	}

	print(total_servers)
	// TODO: process trace here, route the job ids to best matching server
	file, _ := os.Open("./data/cleaned_merged_output.csv")
	reader := csv.NewReader(file)
	i = 0
	for {
		record, err := reader.Read()
		if err != nil {
			break // EOF or error
		}
		fmt.Println("sending to: ", connected_servers[i%number_of_online_servers])
		client, _ := rpc.Dial("tcp", connected_servers[i%number_of_online_servers]+":"+strconv.Itoa(port))

		job_id, _ := strconv.Atoi(record[2])
		task_id, _ := strconv.Atoi(record[3])
		plan_cpu, _ := strconv.Atoi(record[6])
		plan_mem, _ := strconv.Atoi(record[7])
		start_time, _ := strconv.Atoi(record[8])
		end_time, _ := strconv.Atoi(record[9])
		args := rpcstructs.Args{job_id, plan_cpu, plan_mem, start_time, end_time, task_id} // TODO: fill in with actual values from the trace
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
