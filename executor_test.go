package main

import (
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

	if e.getRunning("test.job") == nil {
		t.Error("expected job in running state")
	}

	close(wait)

	<-done

	if e.getRunning("test.job") != nil {
		t.Error("expected job to be no longer in running state")
	}

	if len(e.getCompleted()) != 1 {
		t.Error("expected job in completed list")
	}
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

	e.Add(j)

	select {
	case <-done:
	case <-time.After(gracePeriod):
		t.Fatal("expected job to be run immediatelly")
	}

	if e.getRunning("test.job") == nil {
		t.Error("expected job in running state")
	}

	e.Add(j)

	select {
	case _, ok := <-done:
		if ok {
			t.Fatal("expected job not to be run twice")
		}
	case <-time.After(time.Second):
	}

	close(wait)

	<-done

	if e.getRunning("test.job") != nil {
		t.Error("expected job to be no longer in running state")
	}

	if len(e.getCompleted()) != 1 {
		t.Error("expected job in completed list")
	}
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
		if schedule != wantSchedule {
			t.Errorf("want schedule: %s, got: %s", wantSchedule, schedule)
		}

		if f == nil {
			t.Fatal("exepected a func got nil")
		}

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

	expect(t, "sleeping - scheduled", len(e.scheduled), 1)
	expect(t, "sleeping - inactive", len(e.inactive), 0)
	expect(t, "sleeping - running", len(e.running), 0)
	expect(t, "sleeping - completed", len(e.completed), 0)

	close(waitBeforeWake)
	select {
	case <-done:
	case <-time.After(gracePeriod):
		t.Fatal("expected job to get scheduled")
	}

	expect(t, "awake - scheduled", len(e.scheduled), 1)
	expect(t, "awake - inactive", len(e.inactive), 0)
	expect(t, "awake - running", len(e.running), 1)
	expect(t, "awake - completed", len(e.completed), 0)

	close(wait)

	select {
	case <-done:
	case <-time.After(gracePeriod):
		t.Fatal("expected job to get scheduled")
	}

	expect(t, "done - scheduled", len(e.scheduled), 1)
	expect(t, "done - inactive", len(e.inactive), 0)
	expect(t, "done - running", len(e.running), 0)
	expect(t, "done - completed", len(e.completed), 1)
}

func TestRemoveBeforeWake(t *testing.T) {
	const (
		file         = "test.job"
		wantSchedule = "@every 10s"
	)

	waitBeforeWake := make(chan struct{})
	e := newExecutor(context.Background())
	e.schedule = func(schedule string, f func()) (string, error) {
		if schedule != wantSchedule {
			t.Errorf("want schedule: %s, got: %s", wantSchedule, schedule)
		}

		if f == nil {
			t.Fatal("exepected a func got nil")
		}

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

	expect(t, "sleeping - scheduled", len(e.scheduled), 1)
	expect(t, "sleeping - inactive", len(e.inactive), 0)
	expect(t, "sleeping - running", len(e.running), 0)
	expect(t, "sleeping - completed", len(e.completed), 0)

	e.Remove(file)

	expect(t, "sleeping - scheduled", len(e.scheduled), 0)
	expect(t, "sleeping - inactive", len(e.inactive), 0)
	expect(t, "sleeping - running", len(e.running), 0)
	expect(t, "sleeping - completed", len(e.completed), 0)

	close(waitBeforeWake)
}

func TestRemoveWhileRunning(t *testing.T) {
	const (
		file         = "test.job"
		wantSchedule = "@every 10s"
	)

	waitBeforeWake := make(chan struct{})
	e := newExecutor(context.Background())
	e.schedule = func(schedule string, f func()) (string, error) {
		if schedule != wantSchedule {
			t.Errorf("want schedule: %s, got: %s", wantSchedule, schedule)
		}

		if f == nil {
			t.Fatal("exepected a func got nil")
		}

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

	expect(t, "sleeping - scheduled", len(e.scheduled), 1)
	expect(t, "sleeping - inactive", len(e.inactive), 0)
	expect(t, "sleeping - running", len(e.running), 0)
	expect(t, "sleeping - completed", len(e.completed), 0)

	close(waitBeforeWake)
	select {
	case <-done:
	case <-time.After(gracePeriod):
		t.Fatal("expected job to get scheduled")
	}

	e.Remove(file)

	expect(t, "awake - scheduled", len(e.scheduled), 0)
	expect(t, "awake - inactive", len(e.inactive), 0)
	expect(t, "awake - running", len(e.running), 0)
	expect(t, "awake - completed", len(e.completed), 0)
}

func TestRemoveAfterDone(t *testing.T) {
	const (
		file         = "test.job"
		wantSchedule = "@every 10s"
	)

	waitBeforeWake := make(chan struct{})
	e := newExecutor(context.Background())
	e.schedule = func(schedule string, f func()) (string, error) {
		if schedule != wantSchedule {
			t.Errorf("want schedule: %s, got: %s", wantSchedule, schedule)
		}

		if f == nil {
			t.Fatal("exepected a func got nil")
		}

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

	expect(t, "sleeping - scheduled", len(e.scheduled), 1)
	expect(t, "sleeping - inactive", len(e.inactive), 0)
	expect(t, "sleeping - running", len(e.running), 0)
	expect(t, "sleeping - completed", len(e.completed), 0)

	close(waitBeforeWake)
	select {
	case <-done:
	case <-time.After(gracePeriod):
		t.Fatal("expected job to get scheduled")
	}

	expect(t, "awake - scheduled", len(e.scheduled), 1)
	expect(t, "awake - inactive", len(e.inactive), 0)
	expect(t, "awake - running", len(e.running), 1)
	expect(t, "awake - completed", len(e.completed), 0)

	close(wait)

	select {
	case <-done:
	case <-time.After(gracePeriod):
		t.Fatal("expected job to get scheduled")
	}

	e.Remove(file)

	expect(t, "done - scheduled", len(e.scheduled), 0)
	expect(t, "done - inactive", len(e.inactive), 0)
	expect(t, "done - running", len(e.running), 0)
	expect(t, "done - completed", len(e.completed), 1)
}

func TestAddInactive(t *testing.T) {
	const (
		file         = "test.job"
		wantSchedule = "@every 10s"
	)

	waitBeforeWake := make(chan struct{})
	e := newExecutor(context.Background())
	e.manualActive = true
	e.schedule = func(schedule string, f func()) (string, error) {
		if schedule != wantSchedule {
			t.Errorf("want schedule: %s, got: %s", wantSchedule, schedule)
		}

		if f == nil {
			t.Fatal("exepected a func got nil")
		}

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

	expect(t, "sleeping - scheduled", len(e.scheduled), 0)
	expect(t, "sleeping - inactive", len(e.inactive), 1)
	expect(t, "sleeping - running", len(e.running), 0)
	expect(t, "sleeping - completed", len(e.completed), 0)
}

func TestRemoveInactive(t *testing.T) {
	const (
		file         = "test.job"
		wantSchedule = "@every 10s"
	)

	waitBeforeWake := make(chan struct{})
	e := newExecutor(context.Background())
	e.manualActive = true
	e.schedule = func(schedule string, f func()) (string, error) {
		go func() {
			<-waitBeforeWake
			f()
		}()
		return "TEST-ID", nil
	}

	done := make(chan struct{})
	j := &jobInfo{
		file: file,
		c: &job.Config{
			Name:     "Test Job",
			Schedule: wantSchedule,
		},
		f: func(ctx context.Context) (context.Context, error) {
			done <- struct{}{}
			return nil, nil
		},
	}
	e.Add(j)
	e.Remove(file)

	expect(t, "sleeping - scheduled", len(e.scheduled), 0)
	expect(t, "sleeping - inactive", len(e.inactive), 0)
	expect(t, "sleeping - running", len(e.running), 0)
	expect(t, "sleeping - completed", len(e.completed), 0)
}
