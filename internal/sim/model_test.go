package sim

import (
	"testing"
	"time"
)

func TestGrowthAndStarvation(t *testing.T) {
	a := Initial(Rich)
	a.Running = true
	b := Initial(Starved)
	b.Running = true
	for i := 0; i < 30; i++ {
		a = Step(a, 2)
		b = Step(b, 2)
	}
	if a.BiomassFG <= b.BiomassFG {
		t.Fatalf("rich biomass %f not > starved %f", a.BiomassFG, b.BiomassFG)
	}
	if a.TimeMin != 60 || b.TimeMin != 60 {
		t.Fatal("clock mismatch")
	}
	if b.GlucoseMM >= 0.15 {
		t.Fatal("starved glucose failed to decrease")
	}
	if a.ATPMM <= 0 || b.ATPMM <= 0 {
		t.Fatal("negative ATP")
	}
}
func TestNoAdvancementWhenPaused(t *testing.T) {
	a := Initial(Rich)
	b := Step(a, 2)
	if a.TimeMin != b.TimeMin {
		t.Fatal("paused simulation advanced")
	}
}
func TestIndependentSessionsAndActions(t *testing.T) {
	s1, _ := NewSession(Rich)
	defer s1.Close()
	s2, _ := NewSession(Limited)
	defer s2.Close()
	_, err := s1.Action("set_medium", Starved)
	if err != nil || s2.Snapshot().Medium != Limited {
		t.Fatal("sessions not isolated")
	}
	if _, err = s1.Action("set_medium", "bogus"); err == nil {
		t.Fatal("accepted bogus medium")
	}
	if _, err = s1.Action("invalid", Rich); err == nil {
		t.Fatal("accepted bogus action")
	}
}

func TestReplayProgressAndReset(t *testing.T) {
	frames := []Snapshot{
		{TimeMin: 0, Medium: Rich, GlucoseMM: 3, ATPMM: 2.4, BiomassFG: 280, TranslationActivity: .6},
		{TimeMin: 2, Medium: Rich, GlucoseMM: 2.8, ATPMM: 2.3, BiomassFG: 282, TranslationActivity: .59},
		{TimeMin: 4, Medium: Starved, GlucoseMM: .5, ATPMM: 1.4, BiomassFG: 283, TranslationActivity: .3},
	}
	session, err := NewReplaySession(frames, "vEcoli test trace (not measured)")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if session.Snapshot().Source == "" {
		t.Fatal("missing replay provenance")
	}
	if _, err := session.Action("set_medium", Limited); err == nil {
		t.Fatal("changed medium in a fixed trace")
	}
	_, _ = session.Action("start", Rich)
	// Test direct model stepping by inspecting an actual session clock tick.
	time.Sleep(1050 * time.Millisecond)
	if got := session.Snapshot().TimeMin; got != 4 {
		t.Fatalf("replay did not advance to end: %f", got)
	}
	if session.Snapshot().Running {
		t.Fatal("replay should pause when complete")
	}
	s, err := session.Action("reset", Rich)
	if err != nil || s.TimeMin != 0 {
		t.Fatal("replay reset failed")
	}
}
func TestRejectInvalidReplay(t *testing.T) {
	a := Snapshot{TimeMin: 0, Medium: Rich, GlucoseMM: 1, ATPMM: 2, BiomassFG: 280, TranslationActivity: .6}
	if _, err := NewReplaySession([]Snapshot{a, a}, "invalid"); err == nil {
		t.Fatal("accepted repeated time")
	}
}
