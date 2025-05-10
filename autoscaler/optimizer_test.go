package main

import (
	// rpcstructs "disaggregated_autoscale/rpc_structs"
	// "fmt"
	// "math"
	"testing"
)

func TestGreedyOptimizerSimpleObjectiveFunction(t *testing.T) {
	
	bounds := [2][2]int{
		{1, 250},
		{1, 250},
	}

	// monotonically increasing function 
	objectiveFunction := func(x, y int) float64 {
		return float64(x + y)
	}
	startX := 3
	startY := 2
	N := 3
	MAXITER := 1000

	currX, currY, currVal := greedyHillclimbingOptimizer(startX, startY, N, MAXITER, bounds, objectiveFunction)

	if currX != 250 || currY != 250 || currVal != 500 {
		t.Errorf("Incorrect Optimal value calculation. Expected X = %d, Y = %d, Value = %f (actual X = %d, Y = %d, Value = %f", 
		250, 250, float64(500), currX, currY, currVal)
	}

}