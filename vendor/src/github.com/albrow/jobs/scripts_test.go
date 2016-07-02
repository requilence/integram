// Copyright 2015 Alex Browne.  All rights reserved.
// Use of this source code is governed by the MIT
// license, which can be found in the LICENSE file.

package jobs

import (
	"reflect"
	"testing"
	"time"

	"github.com/garyburd/redigo/redis"
)

func TestPopNextJobsScript(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	// Set up some time parameters
	pastTime := time.Now().Add(-10 * time.Millisecond).UTC().UnixNano()

	// Set up the database
	tx0 := newTransaction()
	// One set will mimic the ready and sorted jobs
	tx0.command("ZADD", redis.Args{Keys.JobsTimeIndex, pastTime, "two", pastTime, "four"}, nil)
	// One set will mimic the queued set
	tx0.command("ZADD", redis.Args{StatusQueued.Key(), 1, "one", 2, "two", 3, "three", 4, "four"}, nil)
	// One set will mimic the executing set
	tx0.command("ZADD", redis.Args{StatusExecuting.Key(), 5, "five"}, nil)
	if err := tx0.exec(); err != nil {
		t.Errorf("Unexpected error executing transaction: %s", err.Error())
	}

	// Start a new transaction and execute the script
	tx1 := newTransaction()
	gotJobs := []*Job{}
	testPoolId := "testPool"
	tx1.popNextJobs(2, testPoolId, newScanJobsHandler(&gotJobs))
	if err := tx1.exec(); err != nil {
		t.Errorf("Unexpected error executing transaction: %s", err.Error())
	}

	gotIds := []string{}
	for _, job := range gotJobs {
		gotIds = append(gotIds, job.id)
	}

	// Check the results
	expectedIds := []string{"four", "two"}
	if !reflect.DeepEqual(expectedIds, gotIds) {
		t.Errorf("Ids returned by script were incorrect.\n\tExpected: %v\n\tBut got:  %v", expectedIds, gotIds)
	}
	conn := redisPool.Get()
	defer conn.Close()
	expectedExecuting := []string{"five", "four", "two"}
	gotExecuting, err := redis.Strings(conn.Do("ZREVRANGE", StatusExecuting.Key(), 0, -1))
	if err != nil {
		t.Errorf("Unexpected error in ZREVRANGE: %s", err.Error())
	}
	if !reflect.DeepEqual(expectedExecuting, gotExecuting) {
		t.Errorf("Ids in the executing set were incorrect.\n\tExpected: %v\n\tBut got:  %v", expectedExecuting, gotExecuting)
	}
	expectedQueued := []string{"three", "one"}
	gotQueued, err := redis.Strings(conn.Do("ZREVRANGE", StatusQueued.Key(), 0, -1))
	if err != nil {
		t.Errorf("Unexpected error in ZREVRANGE: %s", err.Error())
	}
	if !reflect.DeepEqual(expectedQueued, gotQueued) {
		t.Errorf("Ids in the queued set were incorrect.\n\tExpected: %v\n\tBut got:  %v", expectedQueued, gotQueued)
	}
	expectKeyNotExists(t, Keys.JobsTemp)
}

func TestRetryOrFailJobScript(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	testJob, err := RegisterType("testJob", 0, noOpHandler)
	if err != nil {
		t.Fatalf("Unexpected error registering job type: %s", err.Error())
	}

	// We'll use table-driven tests here
	testCases := []struct {
		job             *Job
		expectedReturn  bool
		expectedRetries int
	}{
		{
			// One job will start with 2 retries remaining
			job:             &Job{typ: testJob, id: "retriesRemainingJob", retries: 2, status: StatusExecuting},
			expectedReturn:  true,
			expectedRetries: 1,
		},
		{
			// One job will start with 0 retries remaining
			job:             &Job{typ: testJob, id: "noRetriesJob", retries: 0, status: StatusExecuting},
			expectedReturn:  false,
			expectedRetries: 0,
		},
	}

	// We can test all of the cases in a single transaction
	tx := newTransaction()
	gotReturns := make([]bool, len(testCases))
	gotRetries := make([]int, len(testCases))
	for i, tc := range testCases {
		// Save the job
		tx.saveJob(tc.job)
		// Run the script and save the return value in a slice
		tx.retryOrFailJob(tc.job, newScanBoolHandler(&(gotReturns[i])))
		// Get the new number of retries from the database and save the value in a slice
		tx.command("HGET", redis.Args{tc.job.Key(), "retries"}, newScanIntHandler(&(gotRetries[i])))
	}
	// Execute the transaction
	if err := tx.exec(); err != nil {
		t.Errorf("Unexpected error executing transaction: %s", err.Error())
	}

	// Iterate through test cases again and check the results
	for i, tc := range testCases {
		if gotRetries[i] != tc.expectedRetries {
			t.Errorf("Number of retries after executing script was incorrect for test case %d (job:%s). Expected %v but got %v", i, tc.job.id, tc.expectedRetries, gotRetries[i])
		}
		if gotReturns[i] != tc.expectedReturn {
			t.Errorf("Return value from script was incorrect for test case %d (job:%s). Expected %v but got %v", i, tc.job.id, tc.expectedReturn, gotReturns[i])
		}
		// Make sure the job was removed from the executing set and placed in the correct set
		if err := tc.job.Refresh(); err != nil {
			t.Errorf("Unexpected error in job.Refresh(): %s", err.Error())
		}
		if tc.expectedReturn == false {
			// We expect the job to be in the failed set because it had no retries left
			expectStatusEquals(t, tc.job, StatusFailed)
		} else {
			// We expect the job to be in the queued set because it was queued for retry
			expectStatusEquals(t, tc.job, StatusQueued)
		}
	}
}

func TestSetStatusScript(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	job, err := createAndSaveTestJob()
	if err != nil {
		t.Fatalf("Unexpected error in createAndSaveTestJob(): %s", err.Error())
	}

	// For all possible statuses, execute the script and check that the job status was set correctly
	for _, status := range possibleStatuses {
		if status == StatusDestroyed {
			continue
		}
		tx := newTransaction()
		tx.setStatus(job, status)
		if err := tx.exec(); err != nil {
			t.Errorf("Unexpected error in tx.exec(): %s", err.Error())
		}
		if err := job.Refresh(); err != nil {
			t.Errorf("Unexpected error in job.Refresh(): %s", err.Error())
		}
		expectStatusEquals(t, job, status)
	}
}

func TestDestroyJobScript(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	job, err := createAndSaveTestJob()
	if err != nil {
		t.Fatalf("Unexpected error in createAndSaveTestJob(): %s", err.Error())
	}

	// Execute the script to destroy the job
	tx := newTransaction()
	tx.destroyJob(job)
	if err := tx.exec(); err != nil {
		t.Errorf("Unexpected err in tx.exec(): %s", err.Error())
	}

	// Make sure the job was destroyed
	job.status = StatusDestroyed
	expectStatusEquals(t, job, StatusDestroyed)
}

func TestPurgeStalePoolScript(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	testType, err := RegisterType("testType", 0, noOpHandler)
	if err != nil {
		t.Fatalf("Unexpected error in RegisterType(): %s", err.Error())
	}

	// Set up the database. We'll put some jobs in the executing set with a stale poolId,
	// and some jobs with an active poolId.
	staleJobs := []*Job{}
	stalePoolId := "stalePool"
	for i := 0; i < 4; i++ {
		job := &Job{typ: testType, status: StatusExecuting, poolId: stalePoolId}
		if err := job.save(); err != nil {
			t.Errorf("Unexpected error in job.save(): %s", err.Error())
		}
		staleJobs = append(staleJobs, job)
	}
	activeJobs := []*Job{}
	activePoolId := "activePool"
	for i := 0; i < 4; i++ {
		job := &Job{typ: testType, status: StatusExecuting, poolId: activePoolId}
		if err := job.save(); err != nil {
			t.Errorf("Unexpected error in job.save(): %s", err.Error())
		}
		activeJobs = append(activeJobs, job)
	}

	// Add both pools to the set of active pools
	conn := redisPool.Get()
	defer conn.Close()
	if _, err := conn.Do("SADD", Keys.ActivePools, stalePoolId, activePoolId); err != nil {
		t.Errorf("Unexpected error adding pools to set: %s", err)
	}

	// Execute the script to purge the stale pool
	tx := newTransaction()
	tx.purgeStalePool(stalePoolId)
	if err := tx.exec(); err != nil {
		t.Errorf("Unexpected err in tx.exec(): %s", err.Error())
	}

	// Check the result
	// The active pools set should contain only the activePoolId
	expectSetDoesNotContain(t, Keys.ActivePools, stalePoolId)
	expectSetContains(t, Keys.ActivePools, activePoolId)
	// All the active jobs should still be executing
	for _, job := range activeJobs {
		if err := job.Refresh(); err != nil {
			t.Errorf("Unexpected error in job.Refresh(): %s", err.Error())
		}
		expectStatusEquals(t, job, StatusExecuting)
	}
	// All the stale jobs should now be queued and have an empty poolId
	for _, job := range staleJobs {
		if err := job.Refresh(); err != nil {
			t.Errorf("Unexpected error in job.Refresh(): %s", err.Error())
		}
		expectStatusEquals(t, job, StatusQueued)
		expectJobFieldEquals(t, job, "poolId", "", stringConverter)
	}
}

func TestGetJobsByIdsScript(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	// Create and save some jobs
	jobs, err := createAndSaveTestJobs(5)
	if err != nil {
		t.Fatalf("Unexpected error in createAndSaveTestJobs: %s", err.Error())
	}

	// Execute the script to get the jobs we just created
	jobsCopy := []*Job{}
	tx := newTransaction()
	tx.getJobsByIds(StatusSaved.Key(), newScanJobsHandler(&jobsCopy))
	if err := tx.exec(); err != nil {
		t.Errorf("Unexpected err in tx.exec(): %s", err.Error())
	}

	// Check the result
	if len(jobsCopy) != len(jobs) {
		t.Errorf("getJobsByIds did not return the right number of jobs. Expected %d but got %d", len(jobs), len(jobsCopy))
	}
	if !reflect.DeepEqual(jobs, jobsCopy) {
		t.Errorf("Result of getJobsByIds was incorrect.\n\tExpected: %v\n\tbut got:  %v", jobs, jobsCopy)
	}
}

func TestSetJobFieldScript(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	// Create a test job
	jobs, err := createAndSaveTestJobs(1)
	if err != nil {
		t.Fatalf("Unexpected error in createAndSaveTestJobs: %s", err.Error())
	}
	job := jobs[0]

	// Set the time to 7 days ago
	tx := newTransaction()
	expectedTime := time.Now().Add(-7 * 24 * time.Hour).UTC().UnixNano()
	tx.setJobField(job, "time", expectedTime)
	if err := tx.exec(); err != nil {
		t.Errorf("Unexpected err in tx.exec(): %s", err.Error())
	}

	// Make sure the time field was set properly
	if err := job.Refresh(); err != nil {
		t.Errorf("Unexpected err in job.Refresh: %s", err.Error())
	}
	if job.time != expectedTime {
		t.Errorf("time field was not set. Expected %d but got %d", job.time, expectedTime)
	}

	// Destroy the job and make sure the script does not set the field
	if err := job.Destroy(); err != nil {
		t.Errorf("Unexpected err in job.Destroy: %s", err.Error())
	}
	tx = newTransaction()
	tx.setJobField(job, "foo", "bar")
	if err := tx.exec(); err != nil {
		t.Errorf("Unexpected err in tx.exec(): %s", err.Error())
	}
	conn := redisPool.Get()
	defer conn.Close()
	exists, err := redis.Bool(conn.Do("EXISTS", job.Key()))
	if err != nil {
		t.Errorf("Unexpected err in EXISTS: %s", err.Error())
	}
	if exists {
		t.Error("Expected job to not exist after being destroyed but it did.")
	}
}

func TestAddJobToSetScript(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	// Create a test job
	jobs, err := createAndSaveTestJobs(1)
	if err != nil {
		t.Fatalf("Unexpected error in createAndSaveTestJobs: %s", err.Error())
	}
	job := jobs[0]

	// Add the job to the time index with a score of 7 days ago
	tx := newTransaction()
	expectedScore := float64(time.Now().Add(-7 * 24 * time.Hour).UTC().UnixNano())
	tx.addJobToSet(job, Keys.JobsTimeIndex, expectedScore)
	if err := tx.exec(); err != nil {
		t.Errorf("Unexpected err in tx.exec(): %s", err.Error())
	}

	// Make sure the job was added to the set properly
	conn := redisPool.Get()
	defer conn.Close()
	score, err := redis.Float64(conn.Do("ZSCORE", Keys.JobsTimeIndex, job.id))
	if err != nil {
		t.Errorf("Unexpected error in ZSCORE: %s", err.Error())
	}
	if score != expectedScore {
		t.Errorf("Score in time index set was incorrect. Expected %f but got %f", expectedScore, score)
	}

	// Destroy the job and make sure the script does not add it to a new set
	if err := job.Destroy(); err != nil {
		t.Errorf("Unexpected err in job.Destroy: %s", err.Error())
	}
	tx = newTransaction()
	tx.addJobToSet(job, "fooSet", 42.0)
	if err := tx.exec(); err != nil {
		t.Errorf("Unexpected err in tx.exec(): %s", err.Error())
	}
	exists, err := redis.Bool(conn.Do("EXISTS", "fooSet"))
	if err != nil {
		t.Errorf("Unexpected err in EXISTS: %s", err.Error())
	}
	if exists {
		t.Error("Expected fooSet to not exist after the job was destroyed but it did.")
	}
}
