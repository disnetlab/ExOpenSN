TYPE_SATELLITE = "Satellite"

# Type Satellite Extra Fields
EX_TLE0_KEY = "TLE_0"
EX_TLE1_KEY = "TLE_1"
EX_TLE2_KEY = "TLE_2"
EX_ORBIT_INDEX = "OrbitIndex"
EX_SATELLITE_INDEX = "SatelliteIndex"
EX_AREA_KEY = "Area"


TYPE_GROUND_STATION = "GroundStation"
TYPE_GROUND_TERMINAL = "GroundTerminal"

# Type GroundStation Extra Fields
EX_LATITUDE_KEY = "latitude"
EX_LONGITUDE_KEY = "longitude"
EX_ALTITUDE_KEY = "altitude"

# ---------------------------------------------------------------------------
# These string values MUST match
# the Go-side constants in daemon/pkg/module/power_module.go
# (ExPanelAreaKey etc.) exactly, since both languages read/write the same
# Instance.Extra map keys.
# ---------------------------------------------------------------------------

EX_PANEL_AREA_KEY = "panel_area_m2"
EX_CELL_EFFICIENCY_KEY = "cell_efficiency"
EX_SYSTEM_EFFICIENCY_KEY = "system_efficiency"
EX_IDLE_POWER_KEY = "idle_power_w"
EX_MAX_POWER_KEY = "max_power_w"
