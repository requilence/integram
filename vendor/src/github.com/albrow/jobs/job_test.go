// Copyright 2015 Alex Browne.  All rights reserved.
// Use of this source code is governed by the MIT
// license, which can be found in the LICENSE file.

package jobs

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestJobSave(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	// Create and save a test job
	job, err := createTestJob()
	if err != nil {
		t.Fatal(err)
	}
	job.started = 1
	job.finished = 5
	job.freq = 10
	job.retries = 3
	job.poolId = "testPool"
	if err := job.save(); err != nil {
		t.Errorf("Unexpected error saving job: %s", err.Error())
	}

	// Make sure the main hash was saved correctly
	expectJobFieldEquals(t, job, "data", job.data, nil)
	expectJobFieldEquals(t, job, "type", job.typ.name, stringConverter)
	expectJobFieldEquals(t, job, "time", job.time, int64Converter)
	expectJobFieldEquals(t, job, "freq", job.freq, int64Converter)
	expectJobFieldEquals(t, job, "priority", job.priority, intConverter)
	expectJobFieldEquals(t, job, "started", job.started, int64Converter)
	expectJobFieldEquals(t, job, "finished", job.finished, int64Converter)
	expectJobFieldEquals(t, job, "retries", job.retries, uintConverter)
	expectJobFieldEquals(t, job, "poolId", job.poolId, stringConverter)

	// Make sure the job status was correct
	expectStatusEquals(t, job, StatusSaved)

	// Make sure the job was indexed by its time correctly
	expectJobInTimeIndex(t, job)
}

func TestJobFindById(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	// Create and save a test job
	job, err := createTestJob()
	if err != nil {
		t.Fatal(err)
	}
	job.started = 1
	job.finished = 5
	job.freq = 10
	job.retries = 3
	job.poolId = "testPool"
	if err := job.save(); err != nil {
		t.Errorf("Unexpected error saving job: %s", err.Error())
	}

	// Find the job in the database
	jobCopy, err := FindById(job.id)
	if err != nil {
		t.Errorf("Unexpected error in FindById: %s", err)
	}
	if !reflect.DeepEqual(jobCopy, job) {
		t.Errorf("Found job was not correct.\n\tExpected: %+v\n\tBut got:  %+v", job, jobCopy)
	}

	// Attempting to find a job that doesn't exist should return an error
	fakeId := "foobar"
	if _, err := FindById(fakeId); err == nil {
		t.Error("Expected error when FindById was called with a fake id but got none.")
	} else if _, ok := err.(ErrorJobNotFound); !ok {
		t.Errorf("Expected error to have type ErrorJobNotFound, but got %T", err)
	} else if !strings.Contains(err.Error(), fakeId) {
		t.Error("Expected error message to contain the fake id but it did not.")
	}
}

func TestJobRefresh(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	// Create and save a job
	job, err := createAndSaveTestJob()
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	// Get a copy of that job directly from database
	jobCopy := &Job{}
	tx := newTransaction()
	tx.scanJobById(job.id, jobCopy)
	if err := tx.exec(); err != nil {
		t.Errorf("Unexpected error in tx.exec(): %s", err.Error())
	}

	// Modify and save the copy
	newPriority := jobCopy.priority + 100
	jobCopy.priority = newPriority
	if err := jobCopy.save(); err != nil {
		t.Errorf("Unexpected error in jobCopy.save(): %s", err.Error())
	}

	// Refresh the original job
	if err := job.Refresh(); err != nil {
		t.Errorf("Unexpected error in job.Refresh(): %s", err.Error())
	}

	// Now the original and the copy should b equal
	if !reflect.DeepEqual(job, jobCopy) {
		t.Errorf("Expected job to equal jobCopy but it did not.\n\tExpected %+v\n\tBut got  %+v", jobCopy, job)
	}
}

func TestJobenqueue(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	// Run through a set of possible state paths and make sure the result is
	// always what we expect
	statePaths := []statePath{
		{
			steps: []func(*Job) error{
				// Just call enqueue after creating a new job
				enqueueJob,
			},
			expected: StatusQueued,
		},
		{
			steps: []func(*Job) error{
				// Call enqueue, then Cancel, then enqueue again
				enqueueJob,
				cancelJob,
				enqueueJob,
			},
			expected: StatusQueued,
		},
	}
	testJobStatePaths(t, statePaths)
}

func TestJobCancel(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	// Run through a set of possible state paths and make sure the result is
	// always what we expect
	statePaths := []statePath{
		{
			steps: []func(*Job) error{
				// Just call Cancel after creating a new job
				cancelJob,
			},
			expected: StatusCancelled,
		},
		{
			steps: []func(*Job) error{
				// Call Cancel, then enqueue, then Cancel again
				cancelJob,
				enqueueJob,
				cancelJob,
			},
			expected: StatusCancelled,
		},
	}
	testJobStatePaths(t, statePaths)
}

func TestJobReschedule(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	// Create and save a new job, then make sure that the time
	// parameter is set correctly when we call reschedule.
	job, err := createAndSaveTestJob()
	if err != nil {
		t.Fatalf("Unexpected error in createAndSaveTestJob(): %s", err.Error())
	}
	currentTime := time.Now()
	unixNanoTime := currentTime.UTC().UnixNano()
	if err := job.Reschedule(currentTime); err != nil {
		t.Errorf("Unexpected error in job.Reschedule: %s", err.Error())
	}
	expectJobFieldEquals(t, job, "time", unixNanoTime, int64Converter)
	expectJobInTimeIndex(t, job)

	// Run through a set of possible state paths and make sure the result is
	// always what we expect
	statePaths := []statePath{
		{
			steps: []func(*Job) error{
				// Just call Reschedule after creating a new job
				rescheduleJob,
			},
			expected: StatusQueued,
		},
		{
			steps: []func(*Job) error{
				// Call Cancel, then reschedule
				cancelJob,
				rescheduleJob,
			},
			expected: StatusQueued,
		},
	}
	testJobStatePaths(t, statePaths)
}

func TestJobDestroy(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	// Run through a set of possible state paths and make sure the result is
	// always what we expect
	statePaths := []statePath{
		{
			steps: []func(*Job) error{
				// Just call Destroy after creating a new job
				destroyJob,
			},
			expected: StatusDestroyed,
		},
		{
			steps: []func(*Job) error{
				// Call Destroy after cancel
				cancelJob,
				destroyJob,
			},
			expected: StatusDestroyed,
		},
		{
			steps: []func(*Job) error{
				// Call Destroy after enqueue
				enqueueJob,
				destroyJob,
			},
			expected: StatusDestroyed,
		},
		{
			steps: []func(*Job) error{
				// Call Destroy after enqueue then cancel
				enqueueJob,
				cancelJob,
				destroyJob,
			},
			expected: StatusDestroyed,
		},
	}
	testJobStatePaths(t, statePaths)
}

func TestJobSetError(t *testing.T) {
	testingSetUp()
	defer testingTeardown()

	job, err := createAndSaveTestJob()
	if err != nil {
		t.Fatalf("Unexpected error in createAndSaveTestJob(): %s", err.Error())
	}
	testErr := errors.New("Test Error")
	if err := job.setError(testErr); err != nil {
		t.Errorf("Unexpected error in job.setError(): %s", err.Error())
	}
	expectJobFieldEquals(t, job, "error", testErr.Error(), stringConverter)
}

// statePath represents a path through which a job can travel, where each step
// potentially modifies its status. expected is what we expect the job status
// to be after the last step.
type statePath struct {
	steps    []func(*Job) error
	expected Status
}

var (
	// Some easy to use step functions
	enqueueJob = func(j *Job) error {
		return j.enqueue()
	}
	cancelJob = func(j *Job) error {
		return j.Cancel()
	}
	destroyJob = func(j *Job) error {
		return j.Destroy()
	}
	rescheduleJob = func(j *Job) error {
		return j.Reschedule(time.Now())
	}
)

// testJobStatePaths will for each statePath run through the steps, make sure
// there were no errors at any step, and check that the status after the last
// step is what we expect.
func testJobStatePaths(t *testing.T, statePaths []statePath) {
	for _, statePath := range statePaths {
		testingSetUp()
		defer testingTeardown()
		// Create a new test job
		job, err := createAndSaveTestJob()
		if err != nil {
			t.Fatal(err)
		}
		// Run the job through each step
		for _, step := range statePath.steps {
			if err := step(job); err != nil {
				t.Errorf("Unexpected error in step %v: %s", step, err)
			}
		}
		expectStatusEquals(t, job, statePath.expected)
	}
}

func TestScanJob(t *testing.T) {
	testingSetUp()
	defer testingTeardown()
	job, err := createAndSaveTestJob()
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	conn := redisPool.Get()
	defer conn.Close()
	replies, err := conn.Do("HGETALL", job.Key())
	if err != nil {
		t.Errorf("Unexpected error in HGETALL: %s", err.Error())
	}
	jobCopy := &Job{id: job.id}
	if err := scanJob(replies, jobCopy); err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	if !reflect.DeepEqual(job, jobCopy) {
		t.Errorf("Result of scanJob was incorrect.\n\tExpected %+v\n\tbut got  %+v", job, jobCopy)
	}
}
