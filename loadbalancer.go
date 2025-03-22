package main

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

	// TODO: process trace here, route the job ids to best matching server
}
