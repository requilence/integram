// Copyright 2015 Alex Browne.  All rights reserved.
// Use of this source code is governed by the MIT
// license, which can be found in the LICENSE file.

package jobs

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/garyburd/redigo/redis"
)

// TestPoolIdSet tests that the pool id is set properly when a pool is started
// and removed when it is closed
func TestPoolIdSet(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	pool, err := NewPool(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Start(); err != nil {
		t.Errorf("Unexpected error in pool.Start(): %s", err.Error())
	}
	expectSetContains(t, Keys.ActivePools, pool.id)
	pool.Close()
	if err := pool.Wait(); err != nil {
		t.Errorf("Unexpected error in pool.Wait(): %s", err.Error())
	}
	expectSetDoesNotContain(t, Keys.ActivePools, pool.id)
}

// TestGetNextJobs tests the getNextJobs function, which queries the database to find
// the next queued jobs, in order of their priority.
func TestGetNextJobs(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	// Create a test job with high priority
	highPriorityJob, err := createTestJob()
	if err != nil {
		t.Fatalf("Unexpected error creating test job: %s", err.Error())
	}
	highPriorityJob.priority = 1000
	highPriorityJob.id = "highPriorityJob"
	if err := highPriorityJob.save(); err != nil {
		t.Fatalf("Unexpected error saving test job: %s", err.Error())
	}
	if err := highPriorityJob.enqueue(); err != nil {
		t.Fatalf("Unexpected error enqueuing test job: %s", err.Error())
	}

	// Create more tests with lower priorities
	for i := 0; i < 10; i++ {
		job, err := createTestJob()
		if err != nil {
			t.Fatalf("Unexpected error creating test job: %s", err.Error())
		}
		job.priority = 100
		job.id = "lowPriorityJob" + strconv.Itoa(i)
		if err := job.save(); err != nil {
			t.Fatalf("Unexpected error saving test job: %s", err.Error())
		}
		if err := job.enqueue(); err != nil {
			t.Fatalf("Unexpected error enqueuing test job: %s", err.Error())
		}
	}

	// Call getNextJobs with n = 1. We expect the one job returned to be the
	// highpriority one, but the status should now be executing
	testPoolId := "testPool"
	jobs, err := getNextJobs(1, testPoolId)
	if err != nil {
		t.Errorf("Unexpected error from getNextJobs: %s", err.Error())
	}
	if len(jobs) != 1 {
		t.Errorf("Length of jobs was incorrect. Expected 1 but got %d", len(jobs))
	} else {
		gotJob := jobs[0]
		expectedJob := &Job{}
		(*expectedJob) = *highPriorityJob
		expectedJob.status = StatusExecuting
		expectedJob.poolId = testPoolId
		if !reflect.DeepEqual(expectedJob, gotJob) {
			t.Errorf("Job returned by getNextJobs was incorrect.\n\tExpected: %+v\n\tBut got:  %+v", expectedJob, gotJob)
		}
	}
}

// TestStatusIsExecutingWhileExecuting tests that while a job is executing, its
// status is set to StatusExecuting.
func TestStatusIsExecutingWhileExecuting(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	// Create a pool with 4 workers
	pool, err := NewPool(&PoolConfig{
		NumWorkers: 4,
		BatchSize:  4,
		MinWait:    1 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		// Close the pool and wait for workers to finish
		pool.Close()
		if err := pool.Wait(); err != nil {
			t.Errorf("Unexpected error in pool.Wait(): %s", err.Error())
		}
	}()

	// Register some jobs which will set the value of some string index,
	// signal the wait group, and then wait for an exit signal before closing.
	// waitForJobs is a wait group which will wait for each job to set their string
	waitForJobs := sync.WaitGroup{}
	// jobsCanExit signals all jobs to exit when closed
	jobsCanExit := make(chan bool)
	data := make([]string, 4)
	setStringJob, err := RegisterType("setString", 0, func(i int) error {
		data[i] = "ok"
		waitForJobs.Done()
		// Wait for the signal before returning from this function
		for range jobsCanExit {
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Unexpected error in RegisterType: %s", err.Error())
	}

	// Queue up some jobs
	queuedJobs := make([]*Job, len(data))
	for i := 0; i < len(data); i++ {
		waitForJobs.Add(1)
		job, err := setStringJob.Schedule(100, time.Now(), i)
		if err != nil {
			t.Errorf("Unexpected error in Schedule: %s", err.Error())
		}
		queuedJobs[i] = job
	}

	// Start the pool
	if err := pool.Start(); err != nil {
		t.Errorf("Unexpected error in pool.Start(): %s", err.Error())
	}

	// Wait for the jobs to finish setting their data
	waitForJobs.Wait()

	// At this point, we expect the status of all jobs to be executing.
	for _, job := range queuedJobs {
		// Refresh the job and make sure its status is correct
		if err := job.Refresh(); err != nil {
			t.Errorf("Unexpected error in job.Refresh(): %s", err.Error())
		}
		expectStatusEquals(t, job, StatusExecuting)
	}

	// Signal that the jobs can now exit
	close(jobsCanExit)
}

// TestExecuteJobWithNoArguments registers and executes a job without any
// arguments and then checks that it executed correctly.
func TestExecuteJobWithNoArguments(t *testing.T) {
	testingSetUp()
	// defer testingTeardown()

	// Register a job type with a handler that expects 0 arguments
	data := ""
	setOkayJob, err := RegisterType("setOkay", 0, func() error {
		data = "ok"
		return nil
	})
	if err != nil {
		t.Fatalf("Unexpected error in RegisterType: %s", err.Error())
	}

	// Queue up a single job
	if _, err := setOkayJob.Schedule(100, time.Now(), nil); err != nil {
		t.Errorf("Unexpected error in Schedule(): %s", err.Error())
	}

	// Start the pool with 1 worker
	pool, err := NewPool(&PoolConfig{
		NumWorkers: 1,
		BatchSize:  1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Start(); err != nil {
		t.Errorf("Unexpected error in pool.Start(): %s", err.Error())
	}

	// Immediately close the pool and wait for workers to finish
	pool.Close()
	if err := pool.Wait(); err != nil {
		t.Errorf("Unexpected error in pool.Wait(): %s", err.Error())
	}

	// Make sure that data was set to "ok", indicating that the job executed
	// successfully.
	if data != "ok" {
		t.Errorf("Expected data to be \"ok\" but got \"%s\", indicating the job did not execute successfully.", data)
	}
}

// TestJobsWithHigherPriorityExecutedFirst creates two sets of jobs: one with lower priorities
// and one with higher priorities. Then it starts the worker pool and runs for exactly one iteration.
// Then it makes sure that the jobs with higher priorities were executed, and the lower priority ones
// were not.
func TestJobsWithHigherPriorityExecutedFirst(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	// Register some jobs which will simply set one of the values in data
	data := make([]string, 8)
	setStringJob, err := RegisterType("setString", 0, func(i int) error {
		data[i] = "ok"
		return nil
	})
	if err != nil {
		t.Fatalf("Unexpected error in RegisterType: %s", err.Error())
	}

	// Queue up some jobs
	queuedJobs := make([]*Job, len(data))
	for i := 0; i < len(data); i++ {
		// Lower indexes have higher priority and should be completed first
		job, err := setStringJob.Schedule(8-i, time.Now(), i)
		if err != nil {
			t.Errorf("Unexpected error in Schedule: %s", err.Error())
		}
		queuedJobs[i] = job
	}

	// Start the pool with 4 workers
	pool, err := NewPool(&PoolConfig{
		NumWorkers: 4,
		BatchSize:  4,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Start(); err != nil {
		t.Errorf("Unexpected error in pool.Start(): %s", err.Error())
	}

	// Immediately stop the pool to stop the workers from doing more jobs
	pool.Close()

	// Wait for the workers to finish
	if err := pool.Wait(); err != nil {
		t.Errorf("Unexpected error in pool.Wait(): %s", err.Error())
	}

	// Check that the first 4 values of data were set to "ok"
	// This would mean that the first 4 jobs (in order of priority)
	// were successfully executed.
	expectTestDataOk(t, data[:4])

	// Make sure all the other values of data are still blank
	expectTestDataBlank(t, data[4:])

	// Make sure the first four jobs we queued are marked as finished
	for _, job := range queuedJobs[0:4] {
		// Refresh the job and make sure its status is correct
		if err := job.Refresh(); err != nil {
			t.Errorf("Unexpected error in job.Refresh(): %s", err.Error())
		}
		expectStatusEquals(t, job, StatusFinished)
	}

	// Make sure the next four jobs we queued are marked as queued
	for _, job := range queuedJobs[4:] {
		// Refresh the job and make sure its status is correct
		if err := job.Refresh(); err != nil {
			t.Errorf("Unexpected error in job.Refresh(): %s", err.Error())
		}
		expectStatusEquals(t, job, StatusQueued)
	}
}

// TestJobsOnlyExecutedOnce creates a few jobs that increment a counter (each job
// has its own counter). Then it starts the pool and runs the query loop for at most two
// iterations. Then it checks that each job was executed only once by observing the counters.
func TestJobsOnlyExecutedOnce(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	// Register some jobs which will simply increment one of the values in data
	data := make([]int, 4)
	waitForJobs := sync.WaitGroup{}
	incrementJob, err := RegisterType("increment", 0, func(i int) error {
		data[i] += 1
		waitForJobs.Done()
		return nil
	})
	if err != nil {
		t.Fatalf("Unexpected error in RegisterType: %s", err.Error())
	}

	// Queue up some jobs
	for i := 0; i < len(data); i++ {
		waitForJobs.Add(1)
		if _, err := incrementJob.Schedule(100, time.Now(), i); err != nil {
			t.Errorf("Unexpected error in Schedule: %s", err.Error())
		}
	}

	// Start the pool with 4 workers
	pool, err := NewPool(&PoolConfig{
		NumWorkers: 4,
		BatchSize:  4,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Start(); err != nil {
		t.Errorf("Unexpected error in pool.Start(): %s", err.Error())
	}

	// Wait for the wait group, which tells us each job was executed at least once
	waitForJobs.Wait()
	// Close the pool, allowing for a max of one more iteration
	pool.Close()
	// Wait for the workers to finish
	if err := pool.Wait(); err != nil {
		t.Errorf("Unexpected error in pool.Wait(): %s", err.Error())
	}

	// Check that each value in data equals 1.
	// This would mean that each job was only executed once
	for i, datum := range data {
		if datum != 1 {
			t.Errorf(`Expected data[%d] to be 1 but got: %d`, i, datum)
		}
	}
}

// TestAllJobsExecuted creates many more jobs than workers. Then it starts
// the pool and continuously checks if every job was executed, it which case
// it exits successfully. If some of the jobs have not been executed after 1
// second, it breaks and reports an error. 1 second should be plenty of time
// to execute the jobs.
func TestAllJobsExecuted(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	// Create a pool with 4 workers
	pool, err := NewPool(&PoolConfig{
		NumWorkers: 4,
		BatchSize:  4,
		MinWait:    1 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		// Close the pool and wait for workers to finish
		pool.Close()
		if err := pool.Wait(); err != nil {
			t.Errorf("Unexpected error in pool.Wait(): %s", err.Error())
		}
	}()

	// Register some jobs which will simply set one of the elements in
	// data to "ok"
	dataMut := sync.Mutex{}
	data := make([]string, 100)
	setStringJob, err := RegisterType("setString", 0, func(i int) error {
		dataMut.Lock()
		data[i] = "ok"
		dataMut.Unlock()
		return nil
	})
	if err != nil {
		t.Fatalf("Unexpected error in RegisterType: %s", err.Error())
	}

	// Queue up some jobs
	for i := 0; i < len(data); i++ {
		if _, err := setStringJob.Schedule(100, time.Now(), i); err != nil {
			t.Errorf("Unexpected error in Schedule: %s", err.Error())
		}
	}

	// Start the pool
	if err := pool.Start(); err != nil {
		t.Errorf("Unexpected error in pool.Start(): %s", err.Error())
	}

	// Continuously check the data every 10 milliseconds. Eventually
	// we hope to see that everything was set to "ok". If 1 second has
	// passed, assume something went wrong.
	timeout := time.After(1 * time.Second)
	interval := time.Tick(10 * time.Millisecond)
	remainingJobs := len(data)
	for {
		select {
		case <-timeout:
			// More than 1 second has passed. Assume something went wrong.
			t.Errorf("1 second passed and %d jobs out of %d were not executed.", remainingJobs, len(data))
			break
		case <-interval:
			// Count the number of elements in data that equal "ok".
			// Anything that doesn't equal ok represents a job that hasn't been executed yet
			remainingJobs = len(data)
			dataMut.Lock()
			for _, datum := range data {
				if datum == "ok" {
					remainingJobs -= 1
				}
			}
			dataMut.Unlock()
			if remainingJobs == 0 {
				// Each item in data was set to "ok", so all the jobs were executed correctly.
				return
			}
		}
	}
}

// TestJobsAreNotExecutedUntilTime sets up a few jobs with a time parameter in the future
// Then it makes sure that those jobs are not executed until after that time.
func TestJobsAreNotExecutedUntilTime(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	// Create a pool with 4 workers
	pool, err := NewPool(&PoolConfig{
		NumWorkers: 4,
		BatchSize:  4,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		// Close the pool and wait for workers to finish
		pool.Close()
		if err := pool.Wait(); err != nil {
			t.Errorf("Unexpected error in pool.Wait(): %s", err.Error())
		}
	}()

	// Register some jobs which will set one of the elements in data
	// For this test, we want to execute two jobs at a time, so we'll
	// use a waitgroup.
	data := make([]string, 4)
	dataMut := sync.Mutex{}
	setStringJob, err := RegisterType("setString", 0, func(i int) error {
		dataMut.Lock()
		data[i] = "ok"
		dataMut.Unlock()
		return nil
	})
	if err != nil {
		t.Fatalf("Unexpected error in RegisterType: %s", err.Error())
	}

	// Queue up some jobs with a time parameter in the future
	currentTime := time.Now()
	timeDiff := 200 * time.Millisecond
	futureTime := currentTime.Add(timeDiff)
	for i := 0; i < len(data); i++ {
		if _, err := setStringJob.Schedule(100, futureTime, i); err != nil {
			t.Errorf("Unexpected error in Schedule: %s", err.Error())
		}
	}

	// Start the pool
	if err := pool.Start(); err != nil {
		t.Errorf("Unexpected error in pool.Start(): %s", err.Error())
	}

	// Continuously check the data every 10 milliseconds. Eventually
	// we hope to see that everything was set to "ok". We will check that
	// this condition is only true after futureTime has been reached, since
	// the jobs should not be executed before then.
	timeout := time.After(1 * time.Second)
	interval := time.Tick(10 * time.Millisecond)
	remainingJobs := len(data)
	for {
		select {
		case <-timeout:
			// More than 1 second has passed. Assume something went wrong.
			t.Errorf("1 second passed and %d jobs were not executed.", remainingJobs)
			t.FailNow()
		case <-interval:
			// Count the number of elements in data that equal "ok".
			// Anything that doesn't equal ok represents a job that hasn't been executed yet
			dataMut.Lock()
			remainingJobs = len(data)
			for _, datum := range data {
				if datum == "ok" {
					remainingJobs -= 1
				}
			}
			dataMut.Unlock()
			if remainingJobs == 0 {
				// Each item in data was set to "ok", so all the jobs were executed correctly.
				// Check that this happend after futureTime
				if time.Now().Before(futureTime) {
					t.Errorf("jobs were executed before their time parameter was reached.")
				}
				return
			}
		}
	}
}

// TestJobTimestamps creates and executes a job, then tests that the started and finished
// timestamps were correct.
func TestJobTimestamps(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	// Register a job type which will do nothing but sleep for some duration
	sleepJob, err := RegisterType("sleep", 0, func(d time.Duration) error {
		time.Sleep(d)
		return nil
	})
	if err != nil {
		t.Fatalf("Unexpected error in RegisterType: %s", err.Error())
	}

	// Queue up a single job
	sleepDuration := 10 * time.Millisecond
	job, err := sleepJob.Schedule(100, time.Now(), sleepDuration)
	if err != nil {
		t.Errorf("Unexpected error in sleepJob.Schedule(): %s", err.Error())
	}

	// Start a new pool with 1 worker
	pool, err := NewPool(&PoolConfig{
		NumWorkers: 1,
		BatchSize:  1,
	})
	if err != nil {
		t.Fatal(err)
	}
	poolStarted := time.Now()
	if err := pool.Start(); err != nil {
		t.Errorf("Unexpected error in pool.Start(): %s", err.Error())
	}

	// Immediately stop the pool and wait for workers to finish
	pool.Close()
	if err := pool.Wait(); err != nil {
		t.Errorf("Unexpected error in Pool.Wait(): %s", err.Error())
	}
	poolClosed := time.Now()

	// Update our copy of the job
	if err := job.Refresh(); err != nil {
		t.Errorf("Unexpected error in job.Refresh(): %s", err.Error())
	}

	// Make sure that the timestamps are correct
	expectTimeNotZero(t, job.Started())
	expectTimeBetween(t, job.Started(), poolClosed, poolStarted)
	expectTimeNotZero(t, job.Finished())
	expectTimeBetween(t, job.Finished(), poolClosed, poolStarted)
	expectDurationNotZero(t, job.Duration())
	expectDurationBetween(t, job.Duration(), sleepDuration, poolClosed.Sub(poolStarted))
}

// TestRecurringJob creates and executes a recurring job, then makes sure that the
// job is actually executed with the expected frequency.
func TestRecurringJob(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	// Create a new pool with 1 worker
	pool, err := NewPool(&PoolConfig{
		NumWorkers: 1,
		BatchSize:  1,
		MinWait:    1 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		// Close the pool and wait for workers to finish
		pool.Close()
		if err := pool.Wait(); err != nil {
			t.Errorf("Unexpected error in pool.Wait(): %s", err.Error())
		}
	}()

	// Register a job type which will simply send through to a channel
	jobFinished := make(chan bool)
	signalJob, err := RegisterType("signalJob", 0, func() error {
		jobFinished <- true
		return nil
	})
	if err != nil {
		t.Fatalf("Unexpected error in RegisterType: %s", err.Error())
	}

	// Schedule a recurring signalJob
	freq := 20 * time.Millisecond
	currentTime := time.Now()
	currentTimeUnix := currentTime.UTC().UnixNano()

	job, err := signalJob.ScheduleRecurring(100, currentTime, freq, nil)
	if err != nil {
		t.Errorf("Unexpected error in ScheduleRecurring: %s", err.Error())
	}

	// Start the pool
	if err := pool.Start(); err != nil {
		t.Errorf("Unexpected error in pool.Start(): %s", err.Error())
	}

	// Wait for three successful scheduled executions at the specified
	// frequency, with some tolerance for variation due to execution overhead.
	expectedSuccesses := 5
	expectedTimes := []int64{}
	for i := 0; i <= expectedSuccesses; i++ {
		expectedTimes = append(expectedTimes, currentTimeUnix+freq.Nanoseconds()*int64(i))
	}
	successCount := 0
	tolerance := 0.1
	timeoutDur := time.Duration(int64(float64(freq.Nanoseconds()) * (1 + tolerance)))
OuterLoop:
	for {
		timeout := time.Tick(timeoutDur)
		select {
		case <-jobFinished:
			// This means one more job was successfully executed
			successCount += 1
			if err := job.Refresh(); err != nil {
				t.Errorf("Unexpected error in job.Refresh(): %s", err.Error())
			}
			// Make sure the next scheduled job time parameter is correct
			if job.time != expectedTimes[successCount] {
				t.Errorf("job.time was wrong.\n\tExpected: %v\n\tBut got:  %v", expectedTimes[successCount], job.time)
			}
			// Make sure the job was started after the previous expected time
			expectedStartAfter := time.Unix(0, expectedTimes[successCount-1])
			expectTimeAfter(t, job.Started(), expectedStartAfter)
			// If we reached expectedSuccesses, we're done and the test passes!
			if successCount == expectedSuccesses {
				break OuterLoop
			}
		case <-timeout:
			t.Errorf("Expected %d jobs to execute within %v each, but only %d jobs executed successfully. There was a timeout for the %s job", expectedSuccesses, timeoutDur, successCount, humanize.Ordinal(successCount+1))
			t.FailNow()
		}
	}
}

// TestJobFailError creates and executes a job that is guaranteed to fail by returning an error,
// then tests that the error was captured and stored correctly and that the job status was
// set to failed.
func TestJobFailError(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	// Register a job type which will do nothing but sleep for some duration
	errorJob, err := RegisterType("errorJob", 0, func(msg string) error {
		return fmt.Errorf(msg)
	})
	if err != nil {
		t.Fatalf("Unexpected error in RegisterType: %s", err.Error())
	}
	testJobFail(t, errorJob)
}

// TestJobFailPanic creates and executes a job that is guaranteed to fail by panicking,
// then tests that the error was captured and stored correctly and that the job status
// was set to failed.
func TestJobFailPanic(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	// Register a job type which immediately panic
	panicJob, err := RegisterType("panicJob", 0, func(msg string) error {
		panic(errors.New(msg))
	})
	if err != nil {
		t.Fatalf("Unexpected error in RegisterType: %s", err.Error())
	}
	testJobFail(t, panicJob)
}

// testJobFail tests that jobs of the given jobType fail correctly. The given jobType must
// have a HandlerFunc which accepts a string argument and then always fails. The string argument
// should be the returned error value or the message sent to panic.
func testJobFail(t *testing.T, jobType *Type) {
	// Queue up a single job
	failMsg := "Test Job Failed!"
	job, err := jobType.Schedule(100, time.Now(), failMsg)
	if err != nil {
		t.Errorf("Unexpected error in %s.Schedule(): %s", jobType.String(), err.Error())
	}

	// Start a new pool with 1 worker
	pool, err := NewPool(&PoolConfig{
		NumWorkers: 1,
		BatchSize:  1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Start(); err != nil {
		t.Errorf("Unexpected error in pool.Start(): %s", err.Error())
	}

	// Immediately stop the pool and wait for workers to finish
	pool.Close()
	if err := pool.Wait(); err != nil {
		t.Errorf("Unexpected error in Pool.Wait(): %s", err.Error())
	}

	// Update our copy of the job
	if err := job.Refresh(); err != nil {
		t.Errorf("Unexpected error in job.Refresh(): %s", err.Error())
	}

	// Make sure that the error field is correct and that the job was
	// moved to the failed set
	expectJobFieldEquals(t, job, "error", failMsg, stringConverter)
	expectStatusEquals(t, job, StatusFailed)
}

// TestRetryJob creates and executes a job that is guaranteed to fail, then tests that
// the job is tried some number of times before finally failing.
func TestRetryJob(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	// Create a new pool with 4 worker
	pool, err := NewPool(&PoolConfig{
		NumWorkers: 4,
		BatchSize:  4,
		MinWait:    1 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		// Close the pool and wait for workers to finish
		pool.Close()
		if err := pool.Wait(); err != nil {
			t.Errorf("Unexpected error in pool.Wait(): %s", err.Error())
		}
	}()

	// Register a job type which will increment a counter with the number of tries
	tries := uint(0)
	triesMut := sync.Mutex{}
	retries := uint(5)
	expectedTries := retries + 1
	jobFailed := make(chan bool)
	countTriesJob, err := RegisterType("countTriesJob", retries, func() error {
		triesMut.Lock()
		tries += 1
		done := tries == expectedTries
		triesMut.Unlock()
		if done {
			jobFailed <- true
		}
		msg := fmt.Sprintf("job failed on the %s try", humanize.Ordinal(int(tries)))
		panic(msg)
	})
	if err != nil {
		t.Fatalf("Unexpected error in RegisterType: %s", err.Error())
	}

	// Queue up a single job
	if _, err := countTriesJob.Schedule(100, time.Now(), nil); err != nil {
		t.Errorf("Unexpected error in countTriesJob.Schedule(): %s", err.Error())
	}

	// Start the pool
	if err := pool.Start(); err != nil {
		t.Errorf("Unexpected error in pool.Start(): %s", err.Error())
	}

	// Wait for the job failed signal, or timeout if we don't receive it within 1 second
	timeout := time.After(1 * time.Second)
OuterLoop:
	for {
		select {
		case <-timeout:
			// More than 1 second has passed. Assume something went wrong.
			t.Errorf("1 second passed and the job never permanently failed. The job was tried %d times.", tries)
			t.FailNow()
		case <-jobFailed:
			if tries != expectedTries {
				t.Errorf("The job was not tried the right number of times. Expected %d but job was only tried %d times.", expectedTries, tries)
			} else {
				// The test should pass!
				break OuterLoop
			}
		}
	}
}

// TestStalePoolsArePurged tests that stale pools are properly purged when an active pool starts.
// It does this by manually instantiating a pool, queueing some jobs in it, and then causing it to
// go stale by changing its id (effectively preventing it from replying to pings).
func TestStalePoolsArePurged(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	// Create and start a pool with one worker
	stalePool, err := NewPool(&PoolConfig{
		NumWorkers:   1,
		BatchSize:    1,
		MinWait:      1 * time.Millisecond,
		StaleTimeout: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	stalePool.id = "stalePool"
	if err := stalePool.Start(); err != nil {
		t.Errorf("Unexpected error in stalePool.Start(): %s", err.Error())
	}

	// Create another pool with similar config but don't
	// start it yet
	newPool, err := NewPool(&PoolConfig{
		NumWorkers:   1,
		BatchSize:    1,
		MinWait:      1 * time.Millisecond,
		StaleTimeout: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	jobsCanFinish := make(chan bool)
	stalePoolNeedsClose := true
	defer func() {
		// Indicate that all outstanding jobs can finish by closing the channel
		close(jobsCanFinish)
		// Close both pools and wait for workers to finish
		newPool.Close()
		if err := newPool.Wait(); err != nil {
			t.Errorf("Unexpected error in newPool.Wait(): %s", err.Error())
		}
		if stalePoolNeedsClose {
			stalePool.Close()
		}
		if err := stalePool.Wait(); err != nil {
			t.Errorf("Unexpected error in stalePool.Wait(): %s", err.Error())
		}
	}()

	// Register a job type which will signal and then wait for a channel to close
	// before finishing
	jobStarted := make(chan bool)
	signalAndWaitJob, err := RegisterType("signalAndWaitJob", 0, func() error {
		jobStarted <- true
		for range jobsCanFinish {
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Unexpected error in RegisterType: %s", err.Error())
	}

	// Queue up a job
	job, err := signalAndWaitJob.Schedule(100, time.Now(), nil)
	if err != nil {
		t.Errorf("Error in signalAndWaitJob.Schedule: %s", err.Error())
	}

	// Wait for the job to start
	<-jobStarted

	// Now change the id of the stalePool so that it will no longer reply to pings properly
	oldId := stalePool.id
	oldPingKey := stalePool.pingKey()
	stalePool.Lock()
	stalePool.id = "invalidId"
	stalePool.Unlock()

	// Create a conn we can use to listen for the stale pool to be pinged
	ping := &redis.PubSubConn{Conn: redisPool.Get()}
	if err := ping.Subscribe(oldPingKey); err != nil {
		t.Errorf("Unexpected error in ping.Subscribe(): %s", err.Error())
	}
	pingChan := make(chan interface{})
	go func() {
		defer ping.Close()
		for {
			reply := ping.Receive()
			switch reply.(type) {
			case redis.Message:
				// The ping was received
				pingChan <- reply
				return
			case error:
				err := reply.(error)
				panic(err)
			}
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Start the new pool. We expect this to trigger a purge of the stale pool
	if err := newPool.Start(); err != nil {
		t.Errorf("Unexpected error in newPool.Start(): %s", err.Error())
	}

	// Wait for the stale pool to be pinged or timeout after 1 second
	timeout := time.After(1 * time.Second)
	select {
	case <-pingChan:
		// If we received a ping, close the stale pool and continue with the test
		stalePool.Close()
		stalePoolNeedsClose = false
	case <-timeout:
		fmt.Println("timeout")
		t.Errorf("1 second passed but the stale pool was never pinged")
		t.FailNow()
		return
	}

	// If we've reached here, the stale pool was pinged. We should wait to receive
	// from the channel again to indicate that the job was requeued and picked up by
	// the new pool.
	timeout = time.After(1 * time.Second)
	select {
	case <-jobStarted:
		// If the job started again, continue with the test
	case <-timeout:
		fmt.Println("timeout")
		t.Errorf("1 second passed but the job was never started again.")
		t.FailNow()
		return
	}

	// At this point, the stale pool should have been fully purged.
	expectSetDoesNotContain(t, Keys.ActivePools, oldId)
	expectJobFieldEquals(t, job, "poolId", newPool.id, stringConverter)
}

// expectTestDataOk reports an error via t.Errorf if any elements in data do not equal "ok". It is only
// used for tests in this file. Many of the tests use a slice of strings as data and queue up jobs to
// set one of the elements to "ok", so this makes checking them easier.
func expectTestDataOk(t *testing.T, data []string) {
	for i, datum := range data {
		if datum != "ok" {
			t.Errorf("Expected data[%d] to be \"ok\" but got: \"%s\"\ndata was: %v.", i, datum, data)
		}
	}
}

// expectTestDataBlank is like expectTestDataOk except it does the opposite. It reports an error if any
// of the elements in data were not blank.
func expectTestDataBlank(t *testing.T, data []string) {
	for i, datum := range data {
		if datum != "" {
			t.Errorf("Expected data[%d] to be \"\" but got: \"%s\"\ndata was: %v.", i, datum, data)
		}
	}
}

// expectTimeNotZero reports an error via t.Errorf if x is equal to the zero time.
func expectTimeNotZero(t *testing.T, x time.Time) {
	if x.IsZero() {
		t.Errorf("Expected time x to be non-zero but got zero.")
	}
}

// expectTimeAfter reports an error via t.Errorf if x is not after the given time.
func expectTimeAfter(t *testing.T, x, after time.Time) {
	if !x.After(after) {
		t.Errorf("time x was incorrect. Expected it to be after %v but got %v.", after, x)
	}
}

// expectTimeBefore reports an error via t.Errorf if x is not before the given time.
func expectTimeBefore(t *testing.T, x, before time.Time) {
	if !x.Before(before) {
		t.Errorf("time x was incorrect. Expected it to be before %v but got %v.", before, x)
	}
}

// expectTimeBetween reports an error via t.Errorf if x is not before and after the given times.
func expectTimeBetween(t *testing.T, x, before, after time.Time) {
	expectTimeBefore(t, x, before)
	expectTimeAfter(t, x, after)
}

// expectDurationNotZero reports an error via t.Errorf if d is equal to zero.
func expectDurationNotZero(t *testing.T, d time.Duration) {
	if d.Nanoseconds() == 0 {
		t.Errorf("Expected duration d to be non-zero but got zero.")
	}
}

// expectDurationBetween reports an error via t.Errorf if d is not more than min and less than max.
func expectDurationBetween(t *testing.T, d, min, max time.Duration) {
	if !(d > min) {
		t.Errorf("duration d was incorrect. Expected it to be more than %v but got %v.", min, d)
	}
	if !(d < max) {
		t.Errorf("duration d was incorrect. Expected it to be less than %v but got %v.", max, d)
	}
}
