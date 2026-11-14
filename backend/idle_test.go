package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func newTestIdle(t *testing.T) (*idleTracker, *fakeClock, *int) {
	t.Helper()
	clock := &fakeClock{t: time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)}
	stops := 0
	tr := newIdleTracker(60*time.Minute, 10*time.Minute, clock.now, func() bool { stops++; return true })
	return tr, clock, &stops
}

func serve(tr *idleTracker, method, path string, auth bool) {
	h := tr.wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(method, path, nil)
	if auth {
		req.Header.Set("Authorization", "Bearer token")
	}
	h.ServeHTTP(httptest.NewRecorder(), req)
}

func TestIdle_BootKeepsMachineUpForVisitorWindow(t *testing.T) {
	tr, clock, stops := newTestIdle(t)
	clock.advance(9 * time.Minute)
	if tr.check() || *stops != 0 {
		t.Fatal("stopped before the visitor window after boot ended")
	}
	clock.advance(2 * time.Minute)
	if !tr.check() || *stops != 1 {
		t.Fatal("should stop once the boot window has passed with no activity")
	}
	if tr.check() || *stops != 1 {
		t.Error("must ask for a stop only once")
	}
}

func TestIdle_SignedInWorkKeepsMachineUpLonger(t *testing.T) {
	tr, clock, stops := newTestIdle(t)
	serve(tr, "GET", "/api/analyses", true)
	clock.advance(59 * time.Minute)
	if tr.check() {
		t.Fatal("stopped during the work window")
	}
	serve(tr, "POST", "/api/test", true) // more work resets the window
	clock.advance(59 * time.Minute)
	if tr.check() {
		t.Fatal("stopped although work happened 59 minutes ago")
	}
	clock.advance(2 * time.Minute)
	if !tr.check() || *stops != 1 {
		t.Fatal("should stop an hour after the last signed-in request")
	}
}

func TestIdle_HealthPollsDontKeepMachineUp(t *testing.T) {
	tr, clock, _ := newTestIdle(t)
	for i := 0; i < 3; i++ {
		clock.advance(5 * time.Minute)
		serve(tr, "GET", "/api/health/providers", true) // what an open tab polls
		serve(tr, "OPTIONS", "/api/test", false)
	}
	if !tr.check() {
		t.Error("background health polls must not keep the machine up")
	}
}

func TestIdle_VisitorsGetShortWindow(t *testing.T) {
	tr, clock, _ := newTestIdle(t)
	clock.advance(8 * time.Minute)
	serve(tr, "GET", "/share/abc123", false)
	clock.advance(9 * time.Minute)
	if tr.check() {
		t.Fatal("stopped during a visitor's window")
	}
	clock.advance(2 * time.Minute)
	if !tr.check() {
		t.Fatal("a visitor should only keep the machine up for the short window")
	}
}

func TestIdle_NeverStopsDuringARequest(t *testing.T) {
	tr, clock, stops := newTestIdle(t)
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	h := tr.wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
	}))
	go func() {
		req := httptest.NewRequest("POST", "/api/test", nil)
		req.Header.Set("Authorization", "Bearer token")
		h.ServeHTTP(httptest.NewRecorder(), req)
		close(done)
	}()
	<-started
	clock.advance(3 * time.Hour) // a request far longer than any window
	if tr.check() || *stops != 0 {
		t.Fatal("stopped while a request was in flight")
	}
	close(release)
	<-done
	clock.advance(59 * time.Minute)
	if tr.check() {
		t.Fatal("the end of a long request should count as activity")
	}
	clock.advance(2 * time.Minute)
	if !tr.check() {
		t.Fatal("should stop after the window following the request")
	}
}

func TestIdle_FailedStopRetriesLater(t *testing.T) {
	clock := &fakeClock{t: time.Now()}
	attempts := 0
	tr := newIdleTracker(time.Hour, 10*time.Minute, clock.now, func() bool { attempts++; return false })
	clock.advance(11 * time.Minute)
	if tr.check() {
		t.Fatal("check should report no stop when stopping failed")
	}
	if tr.check() {
		t.Fatal("should wait another window before retrying")
	}
	clock.advance(11 * time.Minute)
	tr.check()
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
}

func TestStartIdleShutdown_DisabledWithoutEnv(t *testing.T) {
	os.Unsetenv("LLMOPT_IDLE_SHUTDOWN")
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	if got := startIdleShutdown(h); got == nil {
		t.Fatal("nil handler")
	}
	t.Setenv("LLMOPT_IDLE_SHUTDOWN", "not-a-duration")
	startIdleShutdown(h) // must not panic or enable
}

func TestStopViaSupervisord_RefusesOtherParents(t *testing.T) {
	// The test runner's parent is not supervisord, so nothing may be signalled.
	if stopViaSupervisord() {
		t.Fatal("must refuse to signal a parent that isn't supervisord")
	}
}
