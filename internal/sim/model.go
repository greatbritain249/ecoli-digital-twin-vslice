// Package sim provides an intentionally simplified, coarse-grained E. coli
// physiology model. It is NOT the vEcoli model and is not validated for
// experimental prediction. Units and assumptions are documented in MODEL.md.
package sim

import (
	"errors"
	"math"
	"sync"
	"time"
)

type Medium string

const (
	Rich    Medium = "rich"
	Limited Medium = "limited"
	Starved Medium = "starved"
)

func ValidMedium(m Medium) bool { return m == Rich || m == Limited || m == Starved }

type Snapshot struct {
	TimeMin             float64 `json:"time_min"`
	Medium              Medium  `json:"medium"`
	Running             bool    `json:"running"`
	GlucoseMM           float64 `json:"glucose_mM"`
	ATPMM               float64 `json:"atp_mM"`
	BiomassFG           float64 `json:"biomass_fg"`
	GrowthPerHour       float64 `json:"growth_per_hour"`
	TranslationActivity float64 `json:"translation_activity"`
	EnergyIndex         float64 `json:"energy_index"`
	DivisionProgress    float64 `json:"division_progress"`
	Divisions           int     `json:"divisions"`
	Model               string  `json:"model"`
	Source              string  `json:"source,omitempty"`
}

const ModelName = "educational-coarse-grained-v1"
const initialBiomass = 280.0

// Initial returns a new independent session's state.
func Initial(m Medium) Snapshot {
	glucose := 3.4
	if m == Limited {
		glucose = 0.85
	}
	if m == Starved {
		glucose = 0.15
	}
	s := Snapshot{Medium: m, GlucoseMM: glucose, ATPMM: 2.4, BiomassFG: initialBiomass,
		DivisionProgress: 0, Model: ModelName}
	return Recalculate(s)
}

func clamp(x, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, x)) }

func Recalculate(s Snapshot) Snapshot {
	n := s.GlucoseMM / (0.65 + s.GlucoseMM)
	energy := clamp(s.ATPMM/3.0, 0, 1)
	// Dimensionless indicators for interactive rendering; not measurements.
	s.TranslationActivity = clamp((0.08+0.92*n)*(0.18+0.82*energy), 0, 1)
	s.EnergyIndex = energy
	s.GrowthPerHour = 0.93 * n * s.TranslationActivity
	s.Model = ModelName
	return s
}

// Step advances dt simulated minutes with nutrient-dependent ATP production,
// consumption, growth, and an illustrative division event at doubled biomass.
// No random noise: tests and replays are reproducible.
func Step(s Snapshot, dt float64) Snapshot {
	if dt <= 0 || !s.Running {
		return s
	}
	if dt > 5 {
		dt = 5
	}
	feed := 0.0
	switch s.Medium {
	case Rich:
		feed = 0.060
	case Limited:
		feed = 0.006
	case Starved:
		feed = 0.0
	}
	uptake := 0.028 * s.GlucoseMM / (0.35 + s.GlucoseMM) * clamp(s.BiomassFG/initialBiomass, 0.5, 2)
	s.GlucoseMM = clamp(s.GlucoseMM+dt*(feed*(1-s.GlucoseMM/5.0)-uptake), 0, 7)
	n := s.GlucoseMM / (0.65 + s.GlucoseMM)
	// Producing ATP requires accessible substrate; maintenance consumes it.
	s.ATPMM = clamp(s.ATPMM+dt*(0.024*n-0.016-0.010*s.TranslationActivity), 0.15, 3.5)
	s = Recalculate(s)
	ratio := math.Exp(s.GrowthPerHour * dt / 60.0)
	s.BiomassFG *= ratio
	s.DivisionProgress = clamp(s.BiomassFG/initialBiomass-1, 0, 1)
	for s.BiomassFG >= initialBiomass*2 {
		s.BiomassFG /= 2
		s.Divisions++
		s.DivisionProgress = 0
	}
	s.TimeMin += dt
	return s
}

// Session owns a simulation clock and supports several independent SSE clients.
// The in-memory store is intentionally limited to a single API process.
type Session struct {
	mu          sync.RWMutex
	state       Snapshot
	initial     Medium
	clients     map[chan Snapshot]struct{}
	lastAccess  time.Time
	closed      bool
	stop        chan struct{}
	replay      []Snapshot
	replayIndex int
}

func NewSession(m Medium) (*Session, error) {
	if !ValidMedium(m) {
		return nil, errors.New("invalid medium: must be rich, limited, or starved")
	}
	ss := &Session{state: Initial(m), initial: m, clients: make(map[chan Snapshot]struct{}), lastAccess: time.Now(), stop: make(chan struct{})}
	go ss.loop()
	return ss, nil
}

// NewReplaySession streams a normalized trajectory exported from a separate
// scientific model. The source must supply units, provenance and valid values.
// This is replay, not a live vEcoli process or a calibrated model.
func NewReplaySession(frames []Snapshot, source string) (*Session, error) {
	if len(frames) < 2 || len(frames) > 50000 {
		return nil, errors.New("replay requires 2-50000 frames")
	}
	last := -1.0
	for i := range frames {
		f := &frames[i]
		if math.IsNaN(f.TimeMin) || math.IsInf(f.TimeMin, 0) || f.TimeMin < 0 || f.TimeMin <= last {
			return nil, errors.New("replay times must strictly increase")
		}
		for _, v := range []float64{f.GlucoseMM, f.ATPMM, f.BiomassFG, f.TranslationActivity} {
			if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
				return nil, errors.New("invalid replay value")
			}
		}
		if f.TranslationActivity > 1 || f.ATPMM > 100 || f.BiomassFG <= 0 {
			return nil, errors.New("out-of-range replay values")
		}
		if !ValidMedium(f.Medium) {
			f.Medium = Rich
		}
		f.Running = false
		f.Model = "imported-trace"
		f.Source = source
		f.EnergyIndex = clamp(f.ATPMM/3, 0, 1)
		last = f.TimeMin
	}
	ss := &Session{state: frames[0], initial: frames[0].Medium, clients: make(map[chan Snapshot]struct{}), lastAccess: time.Now(), stop: make(chan struct{}), replay: frames}
	go ss.loop()
	return ss, nil
}

func (s *Session) loop() {
	tick := time.NewTicker(450 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-tick.C:
			s.mu.Lock()
			if s.closed {
				s.mu.Unlock()
				return
			}
			if s.state.Running {
				if len(s.replay) > 0 {
					if s.replayIndex+1 < len(s.replay) {
						s.replayIndex++
						s.state = s.replay[s.replayIndex]
						s.state.Running = s.replayIndex+1 < len(s.replay)
					} else {
						s.state.Running = false
					}
				} else {
					s.state = Step(s.state, 2.0)
				}
				s.broadcastLocked()
			}
			s.mu.Unlock()
		}
	}
}
func (s *Session) broadcastLocked() {
	for ch := range s.clients {
		select {
		case ch <- s.state:
		default:
			// Drop an obsolete snapshot to keep bounded queues and deliver latest.
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- s.state:
			default:
			}
		}
	}
}
func (s *Session) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastAccess = time.Now()
	return s.state
}
func (s *Session) Action(action string, m Medium) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Snapshot{}, errors.New("simulation closed")
	}
	switch action {
	case "start":
		s.state.Running = true
	case "pause":
		s.state.Running = false
	case "reset":
		if len(s.replay) > 0 {
			s.replayIndex = 0
			s.state = s.replay[0]
		} else {
			s.state = Initial(s.initial)
		}
	case "set_medium":
		if len(s.replay) > 0 {
			return Snapshot{}, errors.New("cannot change medium in recorded trajectory")
		}
		if !ValidMedium(m) {
			return Snapshot{}, errors.New("invalid medium")
		}
		s.state.Medium = m
		// Changing medium affects feeding rate, not intracellular concentrations instantly.
	default:
		return Snapshot{}, errors.New("action must be start, pause, reset, or set_medium")
	}
	s.lastAccess = time.Now()
	s.broadcastLocked()
	return s.state, nil
}
func (s *Session) Subscribe() (<-chan Snapshot, func()) {
	ch := make(chan Snapshot, 2)
	s.mu.Lock()
	if !s.closed {
		s.clients[ch] = struct{}{}
		ch <- s.state
	}
	s.lastAccess = time.Now()
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.clients, ch)
		// Not closing the channel avoids a race with subscribers selecting on it.
	}
}
func (s *Session) Expired(ttl time.Duration) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Since(s.lastAccess) > ttl && len(s.clients) == 0
}
func (s *Session) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		close(s.stop)
	}
}

// Done is closed when the session is deleted or the server shuts down.
func (s *Session) Done() <-chan struct{} { return s.stop }
