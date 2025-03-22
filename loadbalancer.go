package main

func main() {
	/* General Architecture:
	- When a client sends some request for work, it first goes to a load balancer to determine which server the job goes to
	- We will assume load balancer has already routed client requests to the appropriate servers. We can partition the alibaba
	  or google cluster trace to simulate this.
	- Each server needs to maintain "resource counters" - essentially statistics on how much of its resources are being used and
	  will periodically send this to the autoscaler server
	- The autoscaler will then figure out the resource allocation details and send that info back to the servers?



	*/
}
