package api

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// LAN mode's brute-force limit (ADR-0046 follow-up, issue #246). Fixed
// constants rather than configuration: the defaults are meant to be
// unnoticeable to the owner and useless to a guesser.
const (
	// attemptLimit is how many failed attempts one address may make inside
	// attemptWindow before it is blocked.
	attemptLimit = 5
	// attemptWindow is how long a failure counts; it is measured from the
	// address's latest failure, so a quiet address starts clean.
	attemptWindow = 10 * time.Minute
	// attemptBaseDelay is the block after the attemptLimit-th failure; each
	// further failure doubles it, up to attemptMaxDelay.
	attemptBaseDelay = 30 * time.Second
	attemptMaxDelay  = 15 * time.Minute
)

type attemptRecord struct {
	failures     int
	lastFailure  time.Time
	blockedUntil time.Time
}

// attemptLimiter counts failed authentication attempts per source address,
// in memory: a restart forgets them, which is fine for a throttle. The clock
// is injectable for tests.
type attemptLimiter struct {
	mu      sync.Mutex
	now     func() time.Time
	records map[string]*attemptRecord
}

func newAttemptLimiter(now func() time.Time) *attemptLimiter {
	if now == nil {
		now = time.Now
	}
	return &attemptLimiter{now: now, records: map[string]*attemptRecord{}}
}

// blockedFor reports how long addr must still wait; zero means it may try.
func (l *attemptLimiter) blockedFor(addr string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	rec := l.records[addr]
	if rec == nil {
		return 0
	}
	now := l.now()
	if now.Before(rec.blockedUntil) {
		return rec.blockedUntil.Sub(now)
	}
	if now.Sub(rec.lastFailure) > attemptWindow {
		delete(l.records, addr)
	}
	return 0
}

// fail records a failed attempt from addr, blocking it once it has made
// attemptLimit of them and for longer with every further one.
func (l *attemptLimiter) fail(addr string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for k, rec := range l.records {
		if now.After(rec.blockedUntil) && now.Sub(rec.lastFailure) > attemptWindow {
			delete(l.records, k)
		}
	}
	rec := l.records[addr]
	if rec == nil {
		rec = &attemptRecord{}
		l.records[addr] = rec
	}
	rec.failures++
	rec.lastFailure = now
	if rec.failures >= attemptLimit {
		delay := attemptBaseDelay
		for i := attemptLimit; i < rec.failures && delay < attemptMaxDelay; i++ {
			delay *= 2
		}
		if delay > attemptMaxDelay {
			delay = attemptMaxDelay
		}
		rec.blockedUntil = now.Add(delay)
	}
}

// reset forgets addr's failures after a successful sign-in.
func (l *attemptLimiter) reset(addr string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.records, addr)
}

// remoteAddr is the peer address with the port stripped. X-Forwarded-For is
// deliberately not read: any client could forge it to dodge the limit.
func remoteAddr(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func writeTooManyAttempts(w http.ResponseWriter, wait time.Duration) {
	secs := int((wait + time.Second - 1) / time.Second)
	w.Header().Set("Retry-After", strconv.Itoa(secs))
	http.Error(w, "too many failed attempts, try again later", http.StatusTooManyRequests)
}

// statusRecorder remembers the status a handler wrote.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}
