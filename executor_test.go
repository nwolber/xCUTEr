package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/nwolber/xCUTEr/job"
)

var (
	gracePeriod = time.Second * 10
)

func expect(t *testing.T, text string, got, want int) {
	if got != want {
		t.Errorf("%s want: %d, got %d", text, want, got)
	}
}

func expectExecutor(t *testing.T, e *executor, text string, inactive, scheduled, running, completed int) {
	expect(t, fmt.Sprintf("%s - inactive", text), len(e.inactive), inactive)
	expect(t, fmt.Sprintf("%s - scheduled", text), len(e.scheduled), scheduled)
	expect(t, fmt.Sprintf("%s - running", text), len(e.running), running)
	expect(t, fmt.Sprintf("%s - completed", text), len(e.completed), completed)
}

func TestMain(m *testing.M) {
	log.SetOutput(ioutil.Discard)
	os.Exit(m.Run())
}

func TestAddOnce(t *testing.T) {
	e := newExecutor(nil)
	done := make(chan struct{})
	e.run = func(info *runInfo) {
		if info == nil {
			t.Error("want: runInfo, got: nil")
		}
		close(done)
	}

	e.Add(&jobInfo{
		c: &job.Config{
			Schedule: "once",
		},
	})

	select {
	case <-done:
	case <-time.After(gracePeriod):
		t.Error("expected job to be run immediatelly")
	}
}

func TestRunOnce(t *testing.T) {
	e := newExecutor(context.Background())

	wait := make(chan struct{})
	done := make(chan struct{})
	e.Add(&jobInfo{
		file: "test.job",
		c: &job.Config{
			Schedule: "once",
		},
		f: func(ctx context.Context) (context.Context, error) {
			done <- struct{}{}
			<-wait
			close(done)
			return nil, nil
		},
	})

	select {
	case <-done:
	case <-time.After(gracePeriod):
		t.Fatal("expected job to be run immediatelly")
	}

	expectExecutor(t, e, "running", 0, 0, 1, 0)

	close(wait)

	<-done

	expectExecutor(t, e, "done", 0, 0, 0, 1)
}

func TestRunTwice(t *testing.T) {
	e := newExecutor(context.Background())

	wait := make(chan struct{})
	done := make(chan struct{})
	j := &jobInfo{
		file: "test.job",
		c: &job.Config{
			Name:     "Test Job",
			Schedule: "once",
		},
		f: func(ctx context.Context) (context.Context, error) {
			done <- struct{}{}
			<-wait
			close(done)
			return nil, nil
		},
	}
	expectExecutor(t, e, "before", 0, 0, 0, 0)

	e.Add(j)

	select {
	case <-done:
	case <-time.After(gracePeriod):
		t.Fatal("expected job to be run immediatelly")
	}

	expectExecutor(t, e, "running", 0, 0, 1, 0)

	e.Add(j)

	select {
	case _, ok := <-done:
		if ok {
			t.Fatal("expected job not to be run twice")
		}
	case <-time.After(time.Second):
	}

	expectExecutor(t, e, "running 2", 0, 0, 1, 0)

	close(wait)
	<-done

	expectExecutor(t, e, "done", 0, 0, 0, 1)
}

func TestMaxCompleted(t *testing.T) {
	e := newExecutor(context.Background())
	want := 2
	e.maxCompleted = want

	done := make(chan struct{})
	j := &jobInfo{
		file: "test.job",
		c: &job.Config{
			Name:     "Test Job",
			Schedule: "once",
		},
		f: func(ctx context.Context) (context.Context, error) {
			done <- struct{}{}
			return nil, nil
		},
	}

	for i := 0; i < want+2; i++ {
		e.Add(j)
		<-done
	}

	if got := len(e.getCompleted()); got != want {
		t.Errorf("expected %d jobs in completed list, got %d", want, got)
	}
}

func TestAddSchedule(t *testing.T) {
	const (
		file         = "test.job"
		wantSchedule = "@every 10s"
	)

	waitBeforeWake := make(chan struct{})
	e := newExecutor(context.Background())
	e.schedule = func(schedule string, f func()) (string, error) {
		go func() {
			<-waitBeforeWake
			f()
		}()
		return "TEST-ID", nil
	}

	wait := make(chan struct{})
	done := make(chan struct{})
	j := &jobInfo{
		file: file,
		c: &job.Config{
			Name:     "Test Job",
			Schedule: wantSchedule,
		},
		f: func(ctx context.Context) (context.Context, error) {
			done <- struct{}{}
			<-wait
			done <- struct{}{}
			return nil, nil
		},
	}
	e.Add(j)

	expectExecutor(t, e, "sleeping", 0, 1, 0, 0)

	close(waitBeforeWake)
	select {
	case <-done:
	case <-time.After(gracePeriod):
		t.Fatal("expected job to get scheduled")
	}

	expectExecutor(t, e, "running", 0, 1, 1, 0)

	close(wait)

	select {
	case <-done:
	case <-time.After(gracePeriod):
		t.Fatal("expected job to get scheduled")
	}

	expectExecutor(t, e, "done", 0, 1, 0, 1)
}

func TestAddScheduleError(t *testing.T) {
	const (
		file = "test.job"
	)

	e := newExecutor(context.Background())
	e.schedule = func(schedule string, f func()) (string, error) {
		return "", errors.New("test error")
	}

	j := &jobInfo{
		file: file,
		c: &job.Config{
			Name: "Test Job",
		},
	}
	if err := e.Add(j); err == nil {
		t.Error("expected error, got nil")
	}

	expectExecutor(t, e, "after", 0, 0, 0, 0)
}

func TestRemoveBeforeWake(t *testing.T) {
	const (
		file = "test.job"
	)

	waitBeforeWake := make(chan struct{})
	e := newExecutor(context.Background())
	e.schedule = func(schedule string, f func()) (string, error) {
		go func() {
			<-waitBeforeWake
		}()
		return "TEST-ID", nil
	}

	j := &jobInfo{
		file: file,
		c: &job.Config{
			Name: "Test Job",
		},
	}
	e.Add(j)

	expectExecutor(t, e, "before", 0, 1, 0, 0)

	e.Remove(file)

	expectExecutor(t, e, "after", 0, 0, 0, 0)
}

func TestRemoveWhileRunning(t *testing.T) {
	const (
		file = "test.job"
	)

	waitBeforeWake := make(chan struct{})
	e := newExecutor(context.Background())
	e.schedule = func(schedule string, f func()) (string, error) {
		go func() {
			<-waitBeforeWake
			f()
		}()
		return "TEST-ID", nil
	}

	wait := make(chan struct{})
	done := make(chan struct{})
	j := &jobInfo{
		file: file,
		c: &job.Config{
			Name: "Test Job",
		},
		f: func(ctx context.Context) (context.Context, error) {
			done <- struct{}{}
			<-wait
			return nil, nil
		},
	}
	e.Add(j)

	expectExecutor(t, e, "sleeping", 0, 1, 0, 0)

	close(waitBeforeWake)
	select {
	case <-done:
	case <-time.After(gracePeriod):
		t.Fatal("expected job to get scheduled")
	}
	expectExecutor(t, e, "running", 0, 1, 1, 0)

	e.Remove(file)

	expectExecutor(t, e, "after", 0, 0, 0, 0)
}

func TestRemoveAfterDone(t *testing.T) {
	const (
		file = "test.job"
	)

	waitBeforeWake := make(chan struct{})
	e := newExecutor(context.Background())
	e.schedule = func(schedule string, f func()) (string, error) {
		go func() {
			<-waitBeforeWake
			f()
		}()
		return "TEST-ID", nil
	}

	wait := make(chan struct{})
	done := make(chan struct{})
	j := &jobInfo{
		file: file,
		c: &job.Config{
			Name: "Test Job",
		},
		f: func(ctx context.Context) (context.Context, error) {
			done <- struct{}{}
			<-wait
			done <- struct{}{}
			return nil, nil
		},
	}
	e.Add(j)

	expectExecutor(t, e, "sleeping", 0, 1, 0, 0)

	close(waitBeforeWake)
	select {
	case <-done:
	case <-time.After(gracePeriod):
		t.Fatal("expected job to get scheduled")
	}

	expectExecutor(t, e, "awake", 0, 1, 1, 0)

	close(wait)

	select {
	case <-done:
	case <-time.After(gracePeriod):
		t.Fatal("expected job to get scheduled")
	}

	e.Remove(file)

	expectExecutor(t, e, "done", 0, 0, 0, 1)
}

func TestAddInactive(t *testing.T) {
	const (
		file = "test.job"
	)

	waitBeforeWake := make(chan struct{})
	e := newExecutor(context.Background())
	e.manualActive = true
	e.schedule = func(schedule string, f func()) (string, error) {
		<-waitBeforeWake
		return "TEST-ID", nil
	}

	j := &jobInfo{
		file: file,
		c: &job.Config{
			Name: "Test Job",
		},
	}
	e.Add(j)

	expectExecutor(t, e, "inactive", 1, 0, 0, 0)
}

func TestRemoveInactive(t *testing.T) {
	const (
		file = "test.job"
	)

	waitBeforeWake := make(chan struct{})
	e := newExecutor(context.Background())
	e.manualActive = true
	e.schedule = func(schedule string, f func()) (string, error) {
		<-waitBeforeWake
		return "TEST-ID", nil
	}

	j := &jobInfo{
		file: file,
		c: &job.Config{
			Name: "Test Job",
		},
	}
	e.Add(j)
	e.Remove(file)

	expectExecutor(t, e, "after", 0, 0, 0, 0)
}
