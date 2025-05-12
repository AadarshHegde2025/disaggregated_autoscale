package main

import (
	// rpcstructs "disaggregated_autoscale/rpc_structs"
	"fmt"
	"math"
	"testing"
	"os"
	"math/rand"
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

	currX, currY, currVal, _, _ := greedyHillclimbingOptimizer(startX, startY, N, MAXITER, bounds, objectiveFunction)

	if currX != 250 || currY != 250 || currVal != 500 {
		t.Errorf("Incorrect Optimal value calculation. Expected X = %d, Y = %d, Value = %f (actual X = %d, Y = %d, Value = %f", 
		250, 250, float64(500), currX, currY, currVal)
	}

}

func TestGreedyOptimizerComplexObjectiveFunction(t *testing.T) {
	
	bounds := [2][2]int{
		{1, 250},
		{1, 250},
	}

	// monotonically increasing function 
	objectiveFunction := func(x, y int) float64 {

		flx := float64(x)
		fly := float64(y) 
		return float64(-1.0 * ((flx - 100) * (flx - 100) + (fly - 100) * (fly - 100)))
	}
	startX := 80
	startY := 70
	N := 1
	MAXITER := 1000

	currX, currY, currVal, _, _ := greedyHillclimbingOptimizer(startX, startY, N, MAXITER, bounds, objectiveFunction)

	if currX != 100 || currY != 100 || math.Abs(currVal) >= 0.1 {
		t.Errorf("Incorrect Optimal value calculation. Expected X = %d, Y = %d, Value = %f (actual X = %d, Y = %d, Value = %f", 
		250, 250, float64(500), currX, currY, currVal)
	}

}

func TestGeneticOptimizerSimpleObjectiveFunction(t *testing.T) {
	
	bounds := [2][2]int{
		{1, 250},
		{1, 250},
	}

	// monotonically increasing function 
	objectiveFunction := func(x, y int) float64 {
		return float64(x + y)
	}
	startX := 2
	startY := 3
	N := 1
	MAXITER := 1000

	currX, currY, currVal, _, _ := geneticHillclimbingOptimizer(startX, startY, N, MAXITER, bounds, objectiveFunction)

	if currX != 250 || currY != 250 || currVal != 500 {
		t.Errorf("Incorrect Optimal value calculation. Expected X = %d, Y = %d, Value = %f (actual X = %d, Y = %d, Value = %f", 
		250, 250, float64(500), currX, currY, currVal)
	}

}

func TestGeneticOptimizerComplexObjectiveFunction(t *testing.T) {
	
	bounds := [2][2]int{
		{1, 250},
		{1, 250},
	}

	// monotonically increasing function 
	objectiveFunction := func(x, y int) float64 {
		// return float64(x + y)
		res :=float64(-1.0 * ((100 - x) * (100 -x) + (100 - y) * (100 - y)))
		return res
	}
	startX := 80
	startY := 70
	N := 1
	MAXITER := 100000

	currX, currY, currVal, _, _ := geneticHillclimbingOptimizer(startX, startY, N, MAXITER, bounds, objectiveFunction)

	if currX != 100 || currY != 100 || math.Abs(currVal) >= 0.1 {
		t.Errorf("Incorrect Optimal value calculation. Expected X = %d, Y = %d, Value = %f (actual X = %d, Y = %d, Value = %f", 
		250, 250, float64(500), currX, currY, currVal)
	}
}

func TestBruteForceOptimizerSimpleObjectiveFunction(t *testing.T) {
	
	bounds := [2][2]int{
		{1, 250},
		{1, 250},
	}

	// monotonically increasing function 
	objectiveFunction := func(x, y int) float64 {
		return float64(x + y)
	}
	startX := 2
	startY := 3
	N := 1
	MAXITER := 1000

	currX, currY, currVal, _, _ := bruteForceOptimizer(startX, startY, N, MAXITER, bounds, objectiveFunction)

	if currX != 250 || currY != 250 || currVal != 500 {
		t.Errorf("Incorrect Optimal value calculation. Expected X = %d, Y = %d, Value = %f (actual X = %d, Y = %d, Value = %f", 
		250, 250, float64(500), currX, currY, currVal)
	}

}

func TestBruteForceOptimizerComplexObjectiveFunction(t *testing.T) {
	
	bounds := [2][2]int{
		{1, 250},
		{1, 250},
	}

	// monotonically increasing function 
	objectiveFunction := func(x, y int) float64 {
		// return float64(x + y)
		res :=float64(-1.0 * ((100 - x) * (100 -x) + (100 - y) * (100 - y)))
		return res
	}
	startX := 80
	startY := 70
	N := 1
	MAXITER := 100000

	currX, currY, currVal, _, _ := bruteForceOptimizer(startX, startY, N, MAXITER, bounds, objectiveFunction)

	if currX != 100 || currY != 100 || math.Abs(currVal) >= 0.1 {
		t.Errorf("Incorrect Optimal value calculation. Expected X = %d, Y = %d, Value = %f (actual X = %d, Y = %d, Value = %f", 
		250, 250, float64(500), currX, currY, currVal)
	}
}

func TestOptimizerEvaluationOutput(t *testing.T){
	evaluationf, _ := os.Create("evaluation_test.csv")
	iterationf, err := os.Create("iteration_test.csv")

    if err != nil {
        t.Fatal(err)
    }
    defer evaluationf.Close()
	defer iterationf.Close()
	
	fmt.Fprintln(evaluationf, "Brute Force, Greedy, Genetic")
	fmt.Fprintln(iterationf, "Brute Force, Greedy, Genetic")
	sizes := []int{30, 300, 750, 1500, 3000}
	for _, size := range sizes {
		for i := 0; i < 100; i++ {
			a := rand.Intn(size) 
			b := rand.Intn(size)
			c := rand.Intn(size)
			d := rand.Intn(size)
			for {
				if a != b && c != d{
					break
				}
				b = rand.Intn(size)
				c = rand.Intn(size)
			}

			xl := min(a, b)
			xu := max(a, b)
			yl := min(c, d)
			yu := max(c, d)

			bounds := [2][2]int{
				{xl, xu},
				{yl, yu},
			}

			optx := xl + rand.Intn(xu - xl)
			opty := yl + rand.Intn(yu - yl)

			// monotonically increasing function 
			objectiveFunction := func(x, y int) float64 {
				// return float64(x + y)
				res :=float64(-1.0 * ((optx - x) * (optx -x) + (opty - y) * (opty - y)))
				return res
			}
			startX := xl + rand.Intn(xu - xl)
			startY := yl + rand.Intn(yu - yl)
			N := 1
			MAXITER := 100000



			bX, bY, bVal, bEvals, bi := bruteForceOptimizer(startX, startY, N, MAXITER, bounds, objectiveFunction)
			genX, genY, genVal, genEvals, geni := geneticHillclimbingOptimizer(startX, startY, N, MAXITER, bounds, objectiveFunction)
			greeX, greeY, greeVal, greeEvals, greei := greedyHillclimbingOptimizer(startX, startY, N, MAXITER, bounds, objectiveFunction)

			if(bX != genX || genX != greeX || greeY != genY || genY != bY || bVal != genVal || genVal != greeVal){
				t.Errorf("Optimizers did not converge on the same value") 
			}


			fmt.Fprintf(evaluationf, "%d, %d, %d, %d\n", bEvals, greeEvals, genEvals, size)
			fmt.Fprintf(iterationf, "%d, %d, %d, %d\n", bi, greei, geni, size)
		}
	} 
}