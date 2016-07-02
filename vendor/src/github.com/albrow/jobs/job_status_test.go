// Copyright 2015 Alex Browne.  All rights reserved.
// Use of this source code is governed by the MIT
// license, which can be found in the LICENSE file.

package jobs

import (
	"reflect"
	"testing"
)

func TestStatusCount(t *testing.T) {
	testingSetUp()
	defer testingTeardown()
	jobs, err := createAndSaveTestJobs(5)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	for _, status := range possibleStatuses {
		if status == StatusDestroyed {
			// Skip this one, since destroying a job means erasing all records from the database
			continue
		}
		for _, job := range jobs {
			job.setStatus(status)
		}
		count, err := status.Count()
		if err != nil {
			t.Errorf("Unexpected error in status.Count(): %s", err.Error())
		}
		if count != len(jobs) {
			t.Errorf("Expected %s.Count() to return %d after setting job statuses to %s, but got %d", status, len(jobs), status, count)
		}
	}
}

func TestStatusJobIds(t *testing.T) {
	testingSetUp()
	defer testingTeardown()
	jobs, err := createAndSaveTestJobs(5)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	jobIds := make([]string, len(jobs))
	for i, job := range jobs {
		jobIds[i] = job.id
	}
	for _, status := range possibleStatuses {
		if status == StatusDestroyed {
			// Skip this one, since destroying a job means erasing all records from the database
			continue
		}
		for _, job := range jobs {
			job.setStatus(status)
		}
		gotIds, err := status.JobIds()
		if err != nil {
			t.Errorf("Unexpected error in status.JobIds(): %s", err.Error())
		}
		if len(gotIds) != len(jobIds) {
			t.Errorf("%s.JobIds() was incorrect. Expected slice of length %d but got %d", len(jobIds), len(gotIds))
		}
		if !reflect.DeepEqual(jobIds, gotIds) {
			t.Errorf("%s.JobIds() was incorrect. Expected %v but got %v", status, jobIds, gotIds)
		}
	}
}

func TestStatusJobs(t *testing.T) {
	testingSetUp()
	defer testingTeardown()
	jobs, err := createAndSaveTestJobs(5)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	for _, status := range possibleStatuses {
		if status == StatusDestroyed {
			// Skip this one, since destroying a job means erasing all records from the database
			continue
		}
		for _, job := range jobs {
			job.setStatus(status)
		}
		gotJobs, err := status.Jobs()
		if err != nil {
			t.Errorf("Unexpected error in status.Jobs(): %s", err.Error())
		}
		if len(gotJobs) != len(jobs) {
			t.Errorf("%s.Jobs() was incorrect. Expected slice of length %d but got %d", len(jobs), len(gotJobs))
		}
		if !reflect.DeepEqual(jobs, gotJobs) {
			t.Errorf("%s.Jobs() was incorrect. Expected %v but got %v", status, jobs, gotJobs)
		}
	}
}
