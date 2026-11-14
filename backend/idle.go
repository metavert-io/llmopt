package main

// Idle shutdown.
//
// Fly's proxy auto-stop stops an idle machine after only a few minutes without
// traffic, which can happen while someone is still reading or writing up a
// report. With LLMOPT_IDLE_SHUTDOWN set (e.g. "60m") and auto_stop_machines
// turned off, the app decides instead: it stays up until that long has passed
// since the last signed-in request (LLMOPT_IDLE_SHUTDOWN_VISITOR, default 10m,
// for anonymous visits and the boot itself), never while a request is in
// flight, and then stops the machine by asking supervisord (its parent) to shut
// down. supervisord exits 0, the machine stops, and Fly's proxy starts it again
// on the next request (auto_start_machines).

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type idleTracker struct {
	workWindow    time.Duration // how long signed-in activity keeps the machine up
	visitorWindow time.Duration // how long anything else (and boot) keeps it up
	now           func() time.Time
	stop          func() bool // stops the machine; false if it couldn't

	mu        sync.Mutex
	keepUntil time.Time
	inFlight  int
	stopping  bool
}

func newIdleTracker(workWindow, visitorWindow time.Duration, now func() time.Time, stop func() bool) *idleTracker {
	t := &idleTracker{workWindow: workWindow, visitorWindow: visitorWindow, now: now, stop: stop}
	t.keepUntil = now().Add(visitorWindow) // a wake-up keeps the machine up at least this long
	return t
}

// windowFor returns how long a request keeps the machine up: signed-in work
// gets the long window, anonymous visits the short one, and background polls
// (health checks, which open tabs send every few minutes) none at all.
func (t *idleTracker) windowFor(r *http.Request) time.Duration {
	if r.Method == http.MethodOptions || strings.HasPrefix(r.URL.Path, "/api/health") || r.URL.Path == "/health" {
		return 0
	}
	if r.Header.Get("Authorization") != "" {
		return t.workWindow
	}
	return t.visitorWindow
}

func (t *idleTracker) begin(window time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.inFlight++
	if until := t.now().Add(window); window > 0 && until.After(t.keepUntil) {
		t.keepUntil = until
	}
}

func (t *idleTracker) end(window time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.inFlight--
	// Count the end of a long request (a 10-minute test run) as activity too.
	if until := t.now().Add(window); window > 0 && until.After(t.keepUntil) {
		t.keepUntil = until
	}
}

// wrap records every request; nothing is stopped while any request is in flight.
func (t *idleTracker) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		window := t.windowFor(r)
		t.begin(window)
		defer t.end(window)
		next.ServeHTTP(w, r)
	})
}

// check stops the machine if it has been idle long enough. It reports whether
// it asked for a stop.
func (t *idleTracker) check() bool {
	t.mu.Lock()
	idle := !t.stopping && t.inFlight == 0 && t.now().After(t.keepUntil)
	if idle {
		t.stopping = true
	}
	t.mu.Unlock()
	if !idle {
		return false
	}
	log.Printf("Idle shutdown: no activity since the keep-alive window ended; stopping the machine")
	if t.stop() {
		return true
	}
	// Couldn't stop (not under supervisord): stay up and try again later.
	t.mu.Lock()
	t.stopping = false
	t.keepUntil = t.now().Add(t.visitorWindow)
	t.mu.Unlock()
	return false
}

func (t *idleTracker) run(interval time.Duration) {
	for range time.Tick(interval) {
		if t.check() {
			return
		}
	}
}

// stopViaSupervisord asks the parent supervisord to shut everything down,
// which ends the machine's main process. It refuses unless the parent really
// is supervisord, so a local run can never signal the user's shell.
func stopViaSupervisord() bool {
	ppid := os.Getppid()
	cmdline, err := os.ReadFile("/proc/" + strconv.Itoa(ppid) + "/cmdline")
	if err != nil || !strings.Contains(string(cmdline), "supervisord") {
		log.Printf("Idle shutdown: parent process %d is not supervisord; not stopping", ppid)
		return false
	}
	if err := syscall.Kill(ppid, syscall.SIGTERM); err != nil {
		log.Printf("Idle shutdown: signalling supervisord failed: %v", err)
		return false
	}
	return true
}

// startIdleShutdown wraps the handler with idle tracking when
// LLMOPT_IDLE_SHUTDOWN is set; otherwise it returns the handler unchanged.
func startIdleShutdown(h http.Handler) http.Handler {
	raw := os.Getenv("LLMOPT_IDLE_SHUTDOWN")
	if raw == "" {
		return h
	}
	work, err := time.ParseDuration(raw)
	if err != nil || work <= 0 {
		log.Printf("Idle shutdown disabled: invalid LLMOPT_IDLE_SHUTDOWN %q", raw)
		return h
	}
	visitor := 10 * time.Minute
	if v := os.Getenv("LLMOPT_IDLE_SHUTDOWN_VISITOR"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			visitor = d
		}
	}
	if visitor > work {
		visitor = work
	}
	t := newIdleTracker(work, visitor, time.Now, stopViaSupervisord)
	go t.run(time.Minute)
	log.Printf("Idle shutdown enabled: stops after %s without signed-in activity (%s for visitors)", work, visitor)
	return t.wrap(h)
}
