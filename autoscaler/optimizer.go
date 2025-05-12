package main

import (
	"container/heap"
	"fmt"
	// "time"
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

func geneticHillclimbingOptimizer(startX, startY, N, MAXITER int, bounds [2][2]int, objectiveFunction func(int, int) float64) (int, int, float64, int) {
	currX := startX
	currY := startY
	currVal := objectiveFunction(currX, currY)
	evaluations := 1
	// stepSize := bounds[0][1] - bounds[0][0]

	// Add items to priority queue and explore areas with higher utility values
	queue := make(PriorityQueue, 0)
    heap.Init(&queue)

	heap.Push(&queue, &Item{value: [4]int{currX, currY, 0, 0}, priority: currVal})

	stepSizeX := (bounds[0][1] - bounds[0][0]) / 2  // Initialize to be half of space
	stepSizeY := (bounds[1][1] - bounds[1][0]) / 2  // Initialize to be half of space

	for i := 0; i < MAXITER; i++{
		if(queue.Len() == 0) {
			break
		}
 
		item := heap.Pop(&queue).(*Item)
		currX = item.value[0]
		currY = item.value[1]
		currVal = item.priority
		xIndex := item.value[2]
		yIndex := item.value[3]

		// fmt.Printf("Current Iteration: (%d, %d) with value %f and index (%d, %d)\n", currX, currY, currVal, xIndex, yIndex)

		improved := false
		// Locally search with radius N
		for _, neighbor := range getNeighbors(currX, currY, N) {
			neighborX, neighborY := neighbor[0], neighbor[1]
			// If our neighbor is out of bounds skip it
			if neighborX < bounds[0][0] || neighborX > bounds[0][1] || neighborY < bounds[1][0] || neighborY > bounds[1][1] {
				continue
			}
			val := objectiveFunction(neighborX, neighborY)
			evaluations++
			// fmt.Printf("neighbor (%d, %d) had value %f\n", neighborX, neighborY, val)
			// As soon as we find a neighbor that's better, take it
			// This only makes sense for N = 1, tbh

			// Prune neighbors that don't make sense to visit (does this violate correctness?)
			if currVal > val {
				continue
			}
			// Attach the utility function of the immediate direction to the new point
			if neighborX < currX {
				step := max(min(currX - bounds[0][0], stepSizeX / (1 << min(xIndex, 50))),1)
				heap.Push(&queue, &Item{
					value: [4]int{currX - step, currY, xIndex + 1, yIndex}, 
					priority: objectiveFunction(currX - step, currY),
				})

			} else if neighborX > currX {
				step := max(min(bounds[0][1] - currX, stepSizeX / (1 << min(xIndex, 50))), 1)
				heap.Push(&queue, &Item{
					value: [4]int{currX + step, currY, xIndex + 1, yIndex}, 
					priority: objectiveFunction(currX + step, currY),
				})
			}

			if neighborY < currY {
				step := max(min(currY - bounds[1][0], stepSizeY / (1 << min(yIndex, 50))), 1)
				heap.Push(&queue, &Item{
					value: [4]int{currX, currY - step, xIndex, yIndex + 1},
					priority: objectiveFunction(currX, currY - step),
				})

			} else if neighborY > currY {
				step := max(min(bounds[1][1] - currY, stepSizeY / (1 << min(yIndex, 50))), 1)
				heap.Push(&queue, &Item{
					value: [4]int{currX, currY + step, xIndex, yIndex + 1}, 
					priority: objectiveFunction(currX, currY + step),
				})
			}
			
			evaluations++

			if val > currVal {
				improved = true				
			}
		}	
		// locally cannot improve, we have reached a local minima
		if(!improved) {
			break
		}


	}
	// fmt.Println()

	return currX, currY, currVal, evaluations

}