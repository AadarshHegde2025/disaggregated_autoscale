package main

import (
	rpcstructs "disaggregated_autoscale/rpc_structs"
	"testing"
)

// Test adding a snapshot to an empty list
func TestSnapshotAddEmptyList(t *testing.T) {
	autoscaler := AutoScaler{}

	snapshot := rpcstructs.Snapshot{
		ServerIp: "1.1.1.1:8000", 
		JobType: rpcstructs.MEMORY_HEAVY, 
		CpuUtilization: 1.0,
		MemoryUtilization: 1.0, 
		ExecutionTime: 1,
		TotalTime: 1, 
		Timestamp: 1,
	}


	autoscaler.AddSnapshotToList(&snapshot)

	head := autoscaler.snapshotList.head 
	expectedServerIp := "1.1.1.1:8000"

	if (head.data.ServerIp != expectedServerIp) {
		t.Errorf("Result was incorrect, got: %s, want: %s.", head.data.ServerIp, expectedServerIp)
	}
	if (head.next != nil){
		t.Error("Head should not have 'next' element in snapshot list")
	}
	
	if (head.prev != nil){
		t.Error("Head should not have 'prev' element in snapshot list")
	}
}

// Test adding multiple elements to a list
func TestSnapshotAddMultiple(t *testing.T) {
	autoscaler := AutoScaler{}

	snapshot := rpcstructs.Snapshot{
		ServerIp: "1.1.1.1:8000", 
		JobType: rpcstructs.MEMORY_HEAVY, 
		CpuUtilization: 1.0,
		MemoryUtilization: 1.0, 
		ExecutionTime: 1,
		TotalTime: 1, 
		Timestamp: 1,
	}

	snapshot_two := rpcstructs.Snapshot{
		ServerIp: "1.2.3.4:5000", 
		JobType: rpcstructs.COMPUTE_HEAVY, 
		CpuUtilization: 6.0,
		MemoryUtilization: 5.0, 
		ExecutionTime: 4,
		TotalTime: 3,
		Timestamp: 2,
	}

	autoscaler.AddSnapshotToList(&snapshot)
	autoscaler.AddSnapshotToList(&snapshot_two)

	head := autoscaler.snapshotList.head 
	expected := snapshot

	next := autoscaler.snapshotList.head.next
	expected_next := snapshot_two

	if (head.data != expected) {
		t.Errorf("Result was incorrect, got: %v, want: %v.", head.data, expected)
	}
	if (head.next == nil || head.next != next){
		t.Error("Head should have 'next' element in snapshot list")
	}
	if (head.prev != nil){
		t.Error("Head should not have 'prev' element in snapshot list")
	}

	if (next.data != expected_next) {
		t.Errorf("Result was incorrect, got: %v, want: %v.", next.data, expected_next)
	}
	if (next.next != nil){
		t.Error("Second element should not have 'next' element in snapshot list")
	}
	if (next.prev != head){
		t.Error("Second element should have 'prev' element in snapshot list")
	}
}

// Tests the truncate feature of the snapshot list
func TestSnapshotTruncateToNonEmptyList(t *testing.T){
	autoscaler := AutoScaler{}

	snapshots := []rpcstructs.Snapshot{
        {ServerIp: "1.2.3.4:5000", JobType: rpcstructs.COMPUTE_HEAVY, CpuUtilization: 6.0, MemoryUtilization: 5.0, ExecutionTime: 4, TotalTime: 3, Timestamp: 2,},
		{ServerIp: "1.2.3.4:5000", JobType: rpcstructs.MEMORY_HEAVY, CpuUtilization: 6.0, MemoryUtilization: 5.0, ExecutionTime: 4, TotalTime: 3, Timestamp: 30,},
		{ServerIp: "1.2.3.4:5000", JobType: rpcstructs.COMPUTE_HEAVY, CpuUtilization: 6.0, MemoryUtilization: 5.0, ExecutionTime: 4, TotalTime: 3, Timestamp: 50,},
		{ServerIp: "1.2.3.4:5000", JobType: rpcstructs.MEMORY_HEAVY, CpuUtilization: 6.0, MemoryUtilization: 5.0, ExecutionTime: 4, TotalTime: 3, Timestamp: 92,},
		{ServerIp: "1.2.3.4:5000", JobType: rpcstructs.COMPUTE_HEAVY, CpuUtilization: 6.0, MemoryUtilization: 5.0, ExecutionTime: 4, TotalTime: 3, Timestamp: 200,},
	}

	
	for _, snapshot := range snapshots{
		autoscaler.AddSnapshotToList(&snapshot)
	}

	current := autoscaler.snapshotList.head
	count := 0
	for {
		if current == nil{
			break;
		}
		count++
		current = current.next
	}

	if count != len(snapshots) {
		t.Errorf("Add Snapshot functionality unsuccessful, truncate history cannot be tested. Expected list of length %d, found %d", len(snapshots), count)
	}
	
	//Truncates all snapshots from list that have timestamp < 50 seconds
	autoscaler.truncateHistory(50)
	
	expected := []rpcstructs.Snapshot{
		{ServerIp: "1.2.3.4:5000", JobType: rpcstructs.COMPUTE_HEAVY, CpuUtilization: 6.0, MemoryUtilization: 5.0, ExecutionTime: 4, TotalTime: 3, Timestamp: 50,},
		{ServerIp: "1.2.3.4:5000", JobType: rpcstructs.MEMORY_HEAVY, CpuUtilization: 6.0, MemoryUtilization: 5.0, ExecutionTime: 4, TotalTime: 3, Timestamp: 92,},
		{ServerIp: "1.2.3.4:5000", JobType: rpcstructs.COMPUTE_HEAVY, CpuUtilization: 6.0, MemoryUtilization: 5.0, ExecutionTime: 4, TotalTime: 3, Timestamp: 200,},
	}

	count = 0
	current = autoscaler.snapshotList.head
	for {
		if current == nil{
			break;
		}
		count++
		current = current.next
	}

	// Preliminary length check
	if count != len(expected) {
		t.Errorf("Incorrect size of snapshot list after truncation. Expected list of length %d, found %d", len(expected), count)
	}	

	// Validate our list is correct
	current = autoscaler.snapshotList.head
	for i := range(len(expected)){
		if current.data != expected[i] {
			t.Errorf("Incorrect data at position %d in list: expected %v, actual %v", i, current.data, expected[i])
		}
		current = current.next
	}
}

func TestSnapshotTruncateToEmptyList(t *testing.T){
	autoscaler := AutoScaler{}

	snapshots := []rpcstructs.Snapshot{
        {ServerIp: "1.2.3.4:5000", JobType: rpcstructs.COMPUTE_HEAVY, CpuUtilization: 6.0, MemoryUtilization: 5.0, ExecutionTime: 4, TotalTime: 3, Timestamp: 2,},
		{ServerIp: "1.2.3.4:5000", JobType: rpcstructs.MEMORY_HEAVY, CpuUtilization: 6.0, MemoryUtilization: 5.0, ExecutionTime: 4, TotalTime: 3, Timestamp: 30,},
		{ServerIp: "1.2.3.4:5000", JobType: rpcstructs.COMPUTE_HEAVY, CpuUtilization: 6.0, MemoryUtilization: 5.0, ExecutionTime: 4, TotalTime: 3, Timestamp: 50,},
		{ServerIp: "1.2.3.4:5000", JobType: rpcstructs.MEMORY_HEAVY, CpuUtilization: 6.0, MemoryUtilization: 5.0, ExecutionTime: 4, TotalTime: 3, Timestamp: 92,},
		{ServerIp: "1.2.3.4:5000", JobType: rpcstructs.COMPUTE_HEAVY, CpuUtilization: 6.0, MemoryUtilization: 5.0, ExecutionTime: 4, TotalTime: 3, Timestamp: 200,},
	}

	
	for _, snapshot := range snapshots{
		autoscaler.AddSnapshotToList(&snapshot)
	}

	current := autoscaler.snapshotList.head
	count := 0
	for {
		if current == nil{
			break;
		}
		count++
		current = current.next
	}

	if count != len(snapshots) {
		t.Errorf("Add Snapshot functionality unsuccessful, truncate history cannot be tested. Expected list of length %d, found %d", len(snapshots), count)
	}
	
	//Truncates all snapshots from list that have timestamp < 50 seconds
	autoscaler.truncateHistory(250)
	
	expected := []rpcstructs.Snapshot{
	}

	count = 0
	current = autoscaler.snapshotList.head
	for {
		if current == nil{
			break;
		}
		count++
		current = current.next
	}

	// Preliminary length check
	if count != len(expected) {
		t.Errorf("Incorrect size of snapshot list after truncation. Expected list of length %d, found %d", len(expected), count)
	}	

	// Validate our list is correct
	if autoscaler.snapshotList.head != nil{
		t.Errorf("Expected truncation to an empty list")
	}
}
