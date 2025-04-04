package main

import (
	"bufio"
	rpcstructs "disaggregated_autoscale/rpc_structs"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

type AutoScaler struct{}

var server_to_status = make(map[string]bool)

var mu sync.Mutex

func (t *AutoScaler) RequestedStats(args *rpcstructs.ServerUsage, reply *[]float32) error {
	mu.Lock()
	fmt.Println("Received server stats:", args)
	server_to_status[args.ServerIp] = true // mark the server as online
	mu.Unlock()

	return nil
}

func autoscale() {
	// TODO: Write actual algorithm for autoscaling here

	// autoscaler has to be aware of which servers are online and offline so it knows what can be turned off or on

	// autoscaler has to be pre-trained on the trace

}

func main() {
	// let all the servers start up and establish themselves as online

	config_file, _ := os.Open("config.txt")
	scanner := bufio.NewScanner(config_file)
	var line string

	for scanner.Scan() {
		line = scanner.Text()
		words := strings.Fields(line)
		server_to_status[words[1]] = false // everything starts offline until they identify themselves
	}

	time.Sleep(6 * time.Second)
	for {

	}
}
