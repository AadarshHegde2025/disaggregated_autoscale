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

func main() {
	// Redirect output to debug file
	debugFile, err := os.OpenFile("loadbalancer_debug.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("Error opening debug file:", err)
		return
	}
	defer debugFile.Close()
	os.Stdout = debugFile

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

	number_of_online_servers := 0
	var connected_servers map[int]string = make(map[int]string)
	var port int = 9000

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
			connected_servers[i-2] = strings.TrimSpace(words[1])
		}

		if i-2 == number_of_online_servers {
			break
		}
		i += 1
	}

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
		fmt.Println("sending to: ", connected_servers[i%number_of_online_servers])
		client, _ := rpc.Dial("tcp", connected_servers[i%number_of_online_servers]+":"+strconv.Itoa(port))

		// fmt.Println(record[6])
		job_id, _ := strconv.Atoi(record[2])
		task_id, _ := strconv.Atoi(record[3])
		plan_cpu, _ := strconv.ParseFloat(record[6], 64)
		plan_mem, _ := strconv.ParseFloat(record[7], 64)
		start_time, _ := strconv.Atoi(record[8])
		end_time, _ := strconv.Atoi(record[9])

		fmt.Println("data: ", job_id, " ", task_id, " ", plan_cpu, " ", plan_mem, " ", start_time, " ", end_time)

		args := rpcstructs.Args{job_id, plan_cpu, plan_mem, start_time, end_time, task_id, connected_servers[i%number_of_online_servers]} // TODO: fill in with actual values from the trace
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
