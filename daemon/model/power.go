package model

// PowerSpec holds the static, per-satellite hardware specification used by
// my Power Model. These values come from Instance.Extra (parsed from
// strings) at topology-load time and do not change during the simulation -
// same lifecycle as TLE data.
//
// Extra key names (add to instance_types.py on the Python side too, so both
// languages agree on the string keys used in Instance.Extra):
//   panel_area_m2       -> PanelAreaM2
//   cell_efficiency     -> CellEfficiency
//   system_efficiency   -> SystemEfficiency
//   idle_power_w        -> IdlePowerW
//   max_power_w         -> MaxPowerW
type PowerSpec struct {
	PanelAreaM2      float64 `json:"panel_area_m2"`
	CellEfficiency   float64 `json:"cell_efficiency"`
	SystemEfficiency float64 `json:"system_efficiency"`
	IdlePowerW       float64 `json:"idle_power_w"`	
	MaxPowerW        float64 `json:"max_power_w"`
}

// SolarInput is written by the Python Topology Configurator every tick
// (pyephem computes the sun-incidence angle from the satellite's existing
// position — no orbital/solar geometry math happens in Go). This is the
// power-model equivalent of Position: computed in Python, pushed to the
// daemon, consumed by a Go module.
type SolarInput struct {
	SunAngleRad float64 `json:"sun_angle_rad"` // gamma, radians, 0 = sun directly overhead panel
	Utilization float64 `json:"utilization"`   // u(t), 0..1, workload-driven or user-defined
}

// PowerStatus is the computed, per-tick result written by PowerModule
// (Go side) after combining PowerSpec + SolarInput. This is what the web
// console / KV DB consumers should read for "current" power state.
type PowerStatus struct {
	GeneratedPowerW float64 `json:"generated_power_w"`
	ConsumedPowerW  float64 `json:"consumed_power_w"`
	NetPowerW       float64 `json:"net_power_w"` // generated - consumed, convenience field
}
