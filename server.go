package main

import "net"

/*
	Description: This file is the code that runs on each server that actually processes a job

	Server's need to each monitor their own resource usage based on the jobs assigned to it.
	This info will be sent to autoscaler via RPCs
*/

// TODO: RPC handler to send resource utilization info

func handleConnection(connection net.Conn) {

}
