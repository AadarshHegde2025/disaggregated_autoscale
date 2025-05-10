package main

import "fmt"

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
func greedyHillclimbingOptimizer(startX, startY, N, MAXITER int, bounds [2][2]int, objectiveFunction func(int, int) float64) (int, int, float64) {
	currX := startX
	currY := startY
	currVal := objectiveFunction(currX, currY)

	for i := 0; i < MAXITER; i++{
		fmt.Print(currX, currY)
		improved := false
		// Locally search with radius N
		for _, neighbor := range getNeighbors(currX, currY, N) {
			neighborX, neighborY := neighbor[0], neighbor[1]
			if neighborX < bounds[0][0] || neighborX > bounds[0][1] || neighborY < bounds[1][0] || neighborY > bounds[1][1] {
				continue
			}
			val := objectiveFunction(neighborX, neighborY)
			// As soon as we find a neighbor that's better, take it
			if val > currVal {
				currX = neighborX
				currY = neighborY 
				currVal = val
				improved = true
				break 			// This matters a lot, use break to use first neighbor that we find best
								// Use continue to use best neighbor we find
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