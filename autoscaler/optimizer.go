package main

import (

)

// Function to handle integer absolute value
func AbsVal(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func getNeighbors(x, y, N int) [][2]int {
	deltas := [][2]int{}
	for dx := -N; dx <= N; dx++{
		for dy := AbsVal(dx) - N; dy <= N - AbsVal(dx); dy++{
			if(dx == 0 && dy == 0) {
				continue
			}
			deltas = append(deltas, [2]int{x + dx, y + dy})
		}
	}
	return deltas
}

//
func greedyHillclimbingOptimizer(startX, startY, N, MAXITER int, objectiveFunction func(int, int) float64) (int, int, float64) {
	currX := startX
	currY := startY
	currVal := objectiveFunction(currX, currY)

	for i := 0; i < MAXITER; i++{
		improved := false
		// Locally search with radius N
		for _, neighbor := range getNeighbors(currX, currY, N) {
			neighborX, neighborY := neighbor[0], neighbor[1]

			val := objectiveFunction(neighborX, neighborY)
			// As soon as we find a neighbor that's better, take it
			if val < currVal {
				currX = neighborX
				currY = neighborY 
				currVal = val
				improved = true
			}
		}

		if(!improved) {
			break
		}
	}
	return currX, currY, currVal
}

func geneticHillclimbingOptimizer(startX, startY, N, MAXITER int, objectiveFunction func(int, int) float64) (int, int, float64) {
	currX := startX
	currY := startY
	currVal := objectiveFunction(currX, currY)

	// TODO: Implement genetic algorithm, or simulated annealing
	return currX, currY, currVal
}