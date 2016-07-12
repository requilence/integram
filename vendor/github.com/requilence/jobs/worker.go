// Copyright 2015 Alex Browne.  All rights reserved.
// Use of this source code is governed by the MIT
// license, which can be found in the LICENSE file.

package jobs

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"reflect"
	"runtime"
	"sync"
	"time"
)

var (
	dunno     = []byte("???")
	centerDot = []byte("·")
	dot       = []byte(".")
	slash     = []byte("/")
)

// worker continuously executes jobs within its own goroutine.
// The jobs chan is shared between all jobs. To stop the worker,
// simply close the jobs channel.
type worker struct {
	jobs           chan *Job
	wg             *sync.WaitGroup
	afterFunc      func(*Job)
	middlewareFunc func(chan bool, *Job, *[]reflect.Value)
}

// start starts a goroutine in which the worker will continuously
// execute jobs until the jobs channel is closed.
func (w *worker) start() {
	go func() {
		for job := range w.jobs {
			w.doJob(job)
		}
		w.wg.Done()
	}()
}

// stack returns a nicely formated stack frame, skipping skip frames
func stack(skip int) []byte {
	buf := new(bytes.Buffer) // the returned data
	// As we loop, we open files and read them. These variables record the currently
	// loaded file.
	var lines [][]byte
	var lastFile string
	for i := skip; ; i++ { // Skip the expected number of frames
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		// Print this much at least.  If we can't find the source, it won't show.
		fmt.Fprintf(buf, "%s:%d (0x%x)\n", file, line, pc)
		if file != lastFile {
			data, err := ioutil.ReadFile(file)
			if err != nil {
				continue
			}
			lines = bytes.Split(data, []byte{'\n'})
			lastFile = file
		}
		fmt.Fprintf(buf, "\t%s: %s\n", function(pc), source(lines, line))
	}
	return buf.Bytes()
}

// source returns a space-trimmed slice of the n'th line.
func source(lines [][]byte, n int) []byte {
	n-- // in stack trace, lines are 1-indexed but our array is 0-indexed
	if n < 0 || n >= len(lines) {
		return dunno
	}
	return bytes.TrimSpace(lines[n])
}

// function returns, if possible, the name of the function containing the PC.
func function(pc uintptr) []byte {
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return dunno
	}
	name := []byte(fn.Name())
	// The name includes the path name to the package, which is unnecessary
	// since the file name is already included.  Plus, it has center dots.
	// That is, we see
	//	runtime/debug.*T·ptrmethod
	// and want
	//	*T.ptrmethod
	// Also the package path might contains dot (e.g. code.google.com/...),
	// so first eliminate the path prefix
	if lastslash := bytes.LastIndex(name, slash); lastslash >= 0 {
		name = name[lastslash+1:]
	}
	if period := bytes.Index(name, dot); period >= 0 {
		name = name[period+1:]
	}
	name = bytes.Replace(name, centerDot, dot, -1)
	return name
}

// doJob executes the given job. It also sets the status and timestamps for
// the job appropriately depending on the outcome of the execution.
func (w *worker) doJob(job *Job) {
	channel := make(chan bool)

	if w.afterFunc != nil {
		defer w.afterFunc(job)
	}
	if w.middlewareFunc != nil {
		defer func() {
			channel <- true
		}()
	}

	defer func() {
		if r := recover(); r != nil {
			// Get a reasonable error message from the panic
			var err error
			var ok bool
			if err, ok = r.(error); !ok {
				err = errors.New(fmt.Sprint(r))
			}

			stack := stack(3)
			fmt.Printf("Panic recovery -> %s\n%s\n", err, stack)

			if err := setJobError(job, err); err != nil {

				// Nothing left to do but panic
				panic(err)
			}
		}
	}()

	// Set the started field and save the job
	job.started = time.Now().UTC().UnixNano()
	t0 := newTransaction()
	t0.setJobField(job, "started", job.started)
	if err := t0.exec(); err != nil {
		if err := setJobError(job, err); err != nil {

			// NOTE: panics will be caught by the recover statment above
			panic(err)
		}
		return
	}
	// Use reflection to instantiate arguments for the handler
	//handlerArgs := []reflect.Value{}

	// Call the handler using the arguments we just instantiated

	handlerType := reflect.TypeOf(job.typ.handler)
	handlerArgs := make([]reflect.Value, handlerType.NumIn())

	if len(handlerArgs) > 0 {
		handlerArgsInterfaces := make([]interface{}, handlerType.NumIn())

		for i := 0; i < handlerType.NumIn(); i++ {

			dataVal := reflect.Zero(handlerType.In(i))
			//fmt.Printf("handler arg %d: %v,  %v, %v\n", i, dataVal.String(), dataVal.Kind().String(), dataVal.Interface())
			handlerArgsInterfaces[i] = dataVal.Interface()
		}

		if err := decode(job.data, &handlerArgsInterfaces); err != nil {
			if err := setJobError(job, err); err != nil {
				// NOTE: panics will be caught by the recover statment above
				panic(err)
			}
			return
		}

		for i := 0; i < len(handlerArgsInterfaces); i++ {

			if handlerType.In(i).Kind() == reflect.Ptr {
				handlerArgs[i] = reflect.ValueOf(handlerArgsInterfaces[i])
			} else {
				handlerArgs[i] = reflect.ValueOf(handlerArgsInterfaces[i])
			}
		}

	}

	handlerVal := reflect.ValueOf(job.typ.handler)
	if w.middlewareFunc != nil {
		go w.middlewareFunc(channel, job, &handlerArgs)
		<-channel
	}
	//	d := handlerArgs[0]
	returnVals := handlerVal.Call(handlerArgs)
	// Set the finished timestamp
	job.finished = time.Now().UTC().UnixNano()

	// Check if the error return value was nil
	if !returnVals[0].IsNil() {
		err := returnVals[0].Interface().(error)

		if err := setJobError(job, err); err != nil {
			// NOTE: panics will be caught by the recover statment above
			panic(err)
		}
		return
	}
	t1 := newTransaction()
	t1.setJobField(job, "finished", job.finished)
	if job.IsRecurring() {
		// If the job is recurring, reschedule and set status to queued
		job.time = job.NextTime()
		t1.setJobField(job, "time", job.time)
		t1.addJobToTimeIndex(job)
		t1.setStatus(job, StatusQueued)
	} else {
		// Otherwise, set status to finished
		t1.setStatus(job, StatusFinished)
	}
	if err := t1.exec(); err != nil {
		if err := setJobError(job, err); err != nil {
			// NOTE: panics will be caught by the recover statment above
			panic(err)
		}
		return
	}
}

func setJobError(job *Job, err error) error {
	fmt.Println("Job failed")
	job.err = err
	// Start a new transaction
	t := newTransaction()
	// Set the job error field
	t.setJobField(job, "error", err.Error())
	// Either queue the job for retry or mark it as failed depending
	// on how many retries the job has left
	t.retryOrFailJob(job, nil)
	if err := t.exec(); err != nil {
		return err
	}
	return nil
}
