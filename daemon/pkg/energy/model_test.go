package energy

import (
	"math"
	"testing"
)

func almostEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}

// Go recognises functions beginning with 'Test' as tests when they are in a _test.go file.

func TestFullPowerModel_MaxGenerationAtNormalIncidence(t *testing.T) {
	m := NewFullPowerModel()   // creates the solar generation model
	spec := PowerSpecInput{
		PanelAreaM2:      10.0,
		CellEfficiency:   0.30,
		SystemEfficiency: 0.85,
	}
	// gamma = 0: sun directly overhead, cos(0) = 1, reflectivity near its
	// minimum (~4%); this should be close to the theoretical max for this
	// spec, not exactly equal, since even at 0 degrees there's the ~4%
	// baseline reflectivity loss.
	got := m.Generate(spec, 0)

	// maximum before reflectivity losses.
	theoreticalMax := spec.PanelAreaM2 * spec.CellEfficiency * SolarConstantWPerM2 * spec.SystemEfficiency

	// test failure
	if got >= theoreticalMax {
		t.Errorf("generated power %f should be less than theoretical max %f (reflectivity loss must reduce it)", got, theoreticalMax)
	}

	// Allow an error of up to 1% of the theoretical maximum
	if !almostEqual(got, theoreticalMax*0.96, theoreticalMax*0.01) {
		t.Errorf("expected ~96%% of theoretical max at normal incidence, got %f (%.1f%% of max)", got, 100*got/theoreticalMax)
	}
}

// Eclipse Test
func TestFullPowerModel_ZeroAtEclipse(t *testing.T) {
	m := NewFullPowerModel()
	spec := PowerSpecInput{PanelAreaM2: 10, CellEfficiency: 0.3, SystemEfficiency: 0.85}

	// Note: uses almostEqual rather than exact `!= 0`, since float64
	// trig near pi/2 doesn't land on exact zero (see epsilon comment in
	// Generate())
	got := m.Generate(spec, math.Pi/2) // sun exactly at panel edge, no useful sunlight hitting the panel.	
	if !almostEqual(got, 0, 1e-6) {
		t.Errorf("expected ~0 power at 90 degree incidence, got %f", got)
	}

	got = m.Generate(spec, math.Pi) // sun fully behind panel
	if !almostEqual(got, 0, 1e-6) {
		t.Errorf("expected ~0 power at 180 degree incidence (eclipse), got %f", got)
	}

	got = m.Generate(spec, math.Pi/2+0.01) // just past the usable panel-facing region.
	if !almostEqual(got, 0, 1e-6) {
		t.Errorf("expected ~0 power just past 90 degrees, got %f", got)
	}
}

// As the Sun moves further away from the panel's normal direction, generated power should decrease.
func TestFullPowerModel_MonotonicallyDecreasesWithAngle(t *testing.T) {
	m := NewFullPowerModel()
	spec := PowerSpecInput{PanelAreaM2: 10, CellEfficiency: 0.3, SystemEfficiency: 0.85}

	angles := []float64{0, math.Pi / 8, math.Pi / 4, 3 * math.Pi / 8}   // 0, 22.5, 45, 67.5
	prev := math.Inf(1)
	for _, angle := range angles {
		got := m.Generate(spec, angle)   // get the generated power for current angle
		if got >= prev {
			t.Errorf("expected generation to strictly decrease as angle increases: at %.4f rad got %f, previous was %f", angle, got, prev)
		}
		prev = got
	}
}

// A satellite with zero solar-panel area cannot generate power even though Sun is perfectly aligned
func TestFullPowerModel_ZeroPanelAreaGivesZeroPower(t *testing.T) {
	m := NewFullPowerModel()
	spec := PowerSpecInput{PanelAreaM2: 0, CellEfficiency: 0.3, SystemEfficiency: 0.85}
	got := m.Generate(spec, 0)
	if got != 0 {
		t.Errorf("expected 0 power with 0 panel area, got %f", got)
	}
}

// Proof of Extensibility
func TestFullPowerModel_InjectableReflectivity(t *testing.T) {
	// Confirms the strategy-pattern injection point actually works: swap
	// in a reflectivity function that always returns 0 and check the
	// result changes accordingly.
	m := &FullPowerModel{ReflectivityFn: func(gamma float64) float64 { return 0 }}
	spec := PowerSpecInput{PanelAreaM2: 10, CellEfficiency: 0.3, SystemEfficiency: 0.85}
	got := m.Generate(spec, 0)
	theoreticalMax := spec.PanelAreaM2 * spec.CellEfficiency * SolarConstantWPerM2 * spec.SystemEfficiency
	if !almostEqual(got, theoreticalMax, 0.001) {
		t.Errorf("with zero reflectivity loss injected, expected exactly theoretical max %f, got %f", theoreticalMax, got)
	}
}

func TestReflectivityLoss_IncreasesTowardGrazingAngle(t *testing.T) {
	low := ReflectivityLoss(0.1)
	high := ReflectivityLoss(1.4) // ~80 degrees, near grazing
	if high <= low {
		t.Errorf("expected reflectivity loss to increase at grazing angles: at 0.1 rad got %f, at 1.4 rad got %f", low, high)
	}
}

func TestLinearConsumptionModel_IdleAndMax(t *testing.T) {
	m := NewLinearConsumptionModel()
	spec := PowerSpecInput{IdlePowerW: 15, MaxPowerW: 120}

	if got := m.Consume(spec, 0); got != 15 {
		t.Errorf("expected idle power 15 at u=0, got %f", got)
	}
	if got := m.Consume(spec, 1); got != 120 {
		t.Errorf("expected max power 120 at u=1, got %f", got)
	}
	if got := m.Consume(spec, 0.5); got != 67.5 {
		t.Errorf("expected midpoint power 67.5 at u=0.5, got %f", got)
	}
}

// Check with invalid input
func TestLinearConsumptionModel_ClampsOutOfRangeUtilization(t *testing.T) {
	m := NewLinearConsumptionModel()
	spec := PowerSpecInput{IdlePowerW: 15, MaxPowerW: 120}

	if got := m.Consume(spec, -0.5); got != 15 {
		t.Errorf("expected negative utilization to clamp to idle power 15, got %f", got)
	}
	if got := m.Consume(spec, 1.5); got != 120 {
		t.Errorf("expected over-1.0 utilization to clamp to max power 120, got %f", got)
	}
}
