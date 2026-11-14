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
	"context"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
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

// isPoll reports background requests that keep nothing up: preflights and
// health checks (Fly's, and the ones open tabs send every few minutes).
func isPoll(r *http.Request) bool {
	return r.Method == http.MethodOptions || strings.HasPrefix(r.URL.Path, "/api/health") || r.URL.Path == "/health"
}

// signedInKey marks, in a request's context, whether the request turned out to
// be signed-in work. The auth wrappers set it once a token has been checked; a
// mere Authorization header doesn't count.
type signedInKey struct{}

type signedInFlag struct {
	mu  sync.Mutex
	set bool
}

// markSignedIn records that the request passed authentication (no-op outside
// idle tracking).
func markSignedIn(ctx context.Context) {
	if f, ok := ctx.Value(signedInKey{}).(*signedInFlag); ok {
		f.mu.Lock()
		f.set = true
		f.mu.Unlock()
	}
}

func (f *signedInFlag) get() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.set
}

// begin admits a request, unless the machine is already stopping. Requests
// count as visits until they turn out to be signed-in work (see end).
func (t *idleTracker) begin(window time.Duration) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stopping {
		return false
	}
	t.inFlight++
	if until := t.now().Add(window); window > 0 && until.After(t.keepUntil) {
		t.keepUntil = until
	}
	return true
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

// wrap records every request; nothing is stopped while any request is in
// flight. A request counts as signed-in work (the long window) only when the
// app's auth wrapper marked it so; once the machine has decided to stop, new
// requests are turned away with 503 (the page retries, which wakes it again).
func (t *idleTracker) wrap(next http.Handler) http.Handler {
	return t.wrapWith(next, func(r *http.Request, signedIn bool, _ int) bool { return signedIn })
}

// wrapWith is wrap with a custom test for signed-in work, given whether the
// app marked the request and the response status.
func (t *idleTracker) wrapWith(next http.Handler, isWork func(r *http.Request, marked bool, status int) bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		poll := isPoll(r)
		start := t.visitorWindow
		if poll {
			start = 0
		}
		if !t.begin(start) {
			w.Header().Set("Retry-After", "5")
			http.Error(w, "the server is restarting; try again in a few seconds", http.StatusServiceUnavailable)
			return
		}
		flag := &signedInFlag{}
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		window := start
		defer func() { t.end(window) }()
		next.ServeHTTP(rec, r.WithContext(context.WithValue(r.Context(), signedInKey{}, flag)))
		if !poll && isWork(r, flag.get(), rec.status) {
			window = t.workWindow
		}
	})
}

// statusRecorder notes the response status and keeps streaming working.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wroteHeader {
		s.status, s.wroteHeader = code, true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	s.wroteHeader = true
	return s.ResponseWriter.Write(b)
}

func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

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

// lastSaaSProxy forwards LastSaaS's routes (sign-in, billing, webhooks, admin)
// to LastSaaS through the idle tracker, so they count as activity and as work
// in flight: shutdown stops LastSaaS too. Caddy sends those routes here (and
// straight to LastSaaS if this proxy is down). The forwarding headers Caddy set
// are passed on unchanged, so LastSaaS still sees the real client address.
func lastSaaSProxy(upstream *url.URL, t *idleTracker) http.Handler {
	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(upstream)
			pr.Out.Host = pr.In.Host
			for _, h := range []string{"X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto", "Forwarded"} {
				if v, ok := pr.In.Header[h]; ok {
					pr.Out.Header[h] = v
				}
			}
		},
		FlushInterval: -1,
	}
	if t == nil {
		return rp
	}
	// LastSaaS checks its own tokens: a signed-in request it answers without
	// an auth error counts as work.
	return t.wrapWith(rp, func(r *http.Request, _ bool, status int) bool {
		return r.Header.Get("Authorization") != "" && status < 400
	})
}

// startLastSaaSProxy serves lastSaaSProxy on LLMOPT_LASTSAAS_PROXY (e.g.
// 127.0.0.1:8092) for LLMOPT_LASTSAAS_UPSTREAM (e.g. http://127.0.0.1:8091),
// when both are set.
func startLastSaaSProxy(t *idleTracker) {
	listen, up := os.Getenv("LLMOPT_LASTSAAS_PROXY"), os.Getenv("LLMOPT_LASTSAAS_UPSTREAM")
	if listen == "" || up == "" {
		return
	}
	upstream, err := url.Parse(up)
	if err != nil || upstream.Host == "" {
		log.Printf("LastSaaS proxy disabled: invalid LLMOPT_LASTSAAS_UPSTREAM %q", up)
		return
	}
	go func() {
		log.Printf("LastSaaS proxy on %s → %s (idle tracking: %v)", listen, upstream, t != nil)
		log.Printf("LastSaaS proxy stopped: %v", http.ListenAndServe(listen, lastSaaSProxy(upstream, t)))
	}()
}

// startIdleShutdown wraps the handler with idle tracking when
// LLMOPT_IDLE_SHUTDOWN is set; otherwise it returns the handler unchanged. It
// also starts the LastSaaS proxy when configured.
func startIdleShutdown(h http.Handler) http.Handler {
	raw := os.Getenv("LLMOPT_IDLE_SHUTDOWN")
	if raw == "" {
		startLastSaaSProxy(nil)
		return h
	}
	work, err := time.ParseDuration(raw)
	if err != nil || work <= 0 {
		log.Printf("Idle shutdown disabled: invalid LLMOPT_IDLE_SHUTDOWN %q", raw)
		startLastSaaSProxy(nil)
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
	startLastSaaSProxy(t)
	log.Printf("Idle shutdown enabled: stops after %s without signed-in activity (%s for visitors)", work, visitor)
	return t.wrap(h)
}
