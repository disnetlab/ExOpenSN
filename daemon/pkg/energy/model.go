// This file implements the energy model for one satellite. It has two separate jobs:

// Generation: How many watts the solar panel produces from sunlight.
// Consumption: How many watts the satellite uses based on its utilization.

// The important design idea is that these are models, not the simulation itself. They take inputs and return a number. 
// They do not know about Docker, satellites, or the rest of OpenSN.

//
// Design notes:
//   - Generation and consumption are separate, independently swappable
//     strategy interfaces (Strategy pattern) — a new generation model does
//     not require touching consumption code, and vice versa.
//   - Both are pure functions: given the same inputs, they always return
//     the same output, with no side effects and no hidden state. This makes
//     them trivially unit-testable without a running simulation, a real
//     etcd instance, or any containers.
//   - Sun-incidence angle (gamma) is NOT computed here. It is computed in
//     Python (pyephem) from the satellite's existing Position, and arrives
//     as part of model.SolarInput. This package only does the physics that
//     doesn't require orbital/solar geometry libraries.
package energy

import "math"

// Solar Constant refers to the power of the Sun at the Earth, per square metre 
// So if a panel is 1 m^2 and is perfectly efficient and pointed directly at Sun, starting theoretical input would be 1361 W
// before accounting for efficiency and angle.
const SolarConstantWPerM2 = 1361.0 // B0, W/m^2 at 1 AU, fixed

// GenerationModel computes instantaneous generated power (Watts) for one
// satellite, given its static hardware spec and the current sun-incidence
// angle. Implementations must be pure functions (no I/O, no mutation)
// using an interface cuz might have different models later (Extensible)
type GenerationModel interface {
	Generate(spec PowerSpecInput, sunAngleRad float64) float64
}

// ConsumptionModel computes instantaneous consumed power (Watts) for one
// satellite, given its static hardware spec and current utilization u(t).
type ConsumptionModel interface {
	Consume(spec PowerSpecInput, utilization float64) float64
}

// PowerSpecInput mirrors model.PowerSpec. Declared separately here so this
// package has no import dependency on the model package's other unrelated
// types. (Separation of Responsibilities)
type PowerSpecInput struct {
	PanelAreaM2      float64
	CellEfficiency   float64
	SystemEfficiency float64
	IdlePowerW       float64
	MaxPowerW        float64
}

// ReflectivityLoss is a standard empirical estimate for angle-dependent
// panel-surface reflectivity loss (rho(gamma)). 
// Claude came up with it. THIS IS A PLACEHOLDER! FIND SOMETHING RIGOROUS:

//   rho(gamma) = rho0 + (1 - rho0) * sin(gamma)^n
//
// rho0 ~ 0.04 (typical AR-coated space-grade cell reflectance at normal
// incidence), n = 5 (reflectivity stays low until fairly grazing angles,
// then rises sharply) - standard shape used in solar-panel power-loss
// approximations.
func ReflectivityLoss(sunAngleRad float64) float64 {
	const rho0 = 0.04
	const n = 5.0
	return rho0 + (1-rho0)*math.Pow(math.Sin(sunAngleRad), n)
}

// FullPowerModel implements the equation from the energy extension design:
//
//   P = A * eta_sc * B0 * eta_sys * cos(gamma) * (1 - rho(gamma))
//
// ReflectivityFn is injectable so a different reflectivity estimate can be
// swapped in (e.g. a lookup-table model, or a per-material formula) without
// changing this struct's Generate logic (Open/Closed principle)
type FullPowerModel struct {
	ReflectivityFn func(sunAngleRad float64) float64
}

// NewFullPowerModel returns a FullPowerModel using the standard
// ReflectivityLoss estimate. Use this unless you have a specific reason to
// inject a different reflectivity function.
func NewFullPowerModel() *FullPowerModel {
	return &FullPowerModel{ReflectivityFn: ReflectivityLoss}
}

func (m *FullPowerModel) Generate(spec PowerSpecInput, sunAngleRad float64) float64 {
	// Satellite is past the terminator / sun below panel plane: no
	// generation. cos(gamma) would go negative past 90 degrees, which is
	// not physically meaningful here — clamp to zero rather than letting
	// negative "generation" leak into downstream sums.
	//
	// Uses a small epsilon rather than a strict `<= 0` comparison:
	// float64 trig functions don't return exact zero at pi/2 (e.g.
	// math.Cos(math.Pi/2) is ~6e-17, not 0, due to floating-point
	// representation of pi/2 itself) — a strict comparison would let a
	// physically-zero case sneak through as a tiny nonzero generation
	// value instead of clamping cleanly.
	const epsilon = 1e-9
	cosGamma := math.Cos(sunAngleRad)
	if cosGamma <= epsilon {
		return 0
	}
	rho := m.ReflectivityFn(sunAngleRad)
	return spec.PanelAreaM2 *
		spec.CellEfficiency *
		SolarConstantWPerM2 *
		spec.SystemEfficiency *
		cosGamma *
		(1 - rho)
}

// LinearConsumptionModel implements:
//
//   P_consume(t) = P_idle + (P_max - P_idle) * u(t)
//
// utilization is expected in [0, 1]; values outside that range are clamped
// so a bad upstream u(t) value can't produce a negative or over-max power
// draw.
type LinearConsumptionModel struct{}

func NewLinearConsumptionModel() *LinearConsumptionModel {
	return &LinearConsumptionModel{}
}

func (m *LinearConsumptionModel) Consume(spec PowerSpecInput, utilization float64) float64 {
	u := utilization
	if u < 0 {
		u = 0
	}
	if u > 1 {
		u = 1
	}
	return spec.IdlePowerW + (spec.MaxPowerW-spec.IdlePowerW)*u
}
