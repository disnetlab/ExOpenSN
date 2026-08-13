// This is the orchestration layer of the energy extension.
// It is the piece that connects all the other pieces together and runs them repeatedly

package module

import (
	"NodeDaemon/config"
	"NodeDaemon/model"
	"NodeDaemon/pkg/energy"
	"NodeDaemon/pkg/synchronizer"
	"NodeDaemon/share/key"
	"NodeDaemon/share/signal"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// OpenSN instance model already has an Extra field CHECK!!!
// Extra keys read from Instance.Extra for satellite power specs. Keep these
// string literals in sync with the equivalent constants on the Python side
// (instance_types.py), same convention as EX_TLE0_KEY etc.
const (
	ExPanelAreaKey      = "panel_area_m2"
	ExCellEfficiencyKey = "cell_efficiency"
	ExSystemEffKey      = "system_efficiency"
	ExIdlePowerKey      = "idle_power_w"
	ExMaxPowerKey       = "max_power_w"
)

type PowerModule struct {
	Base
}

func CreatePowerModule() *PowerModule {
	return &PowerModule{
		Base{
			sigChan:    make(chan int),
			errChan:    make(chan error),
			wg:         new(sync.WaitGroup),
			daemonFunc: powerDaemonFunc,
			running:    false,
			ModuleName: "PowerModule",
		},
	}
}

// specFromExtra parses a satellite's power hardware spec out of its Extra
// map. Returns ok=false (not an error) when any required key is missing —
// this is how ground-station / router instances, which have no power spec,
// are silently skipped rather than needing an explicit type check against
// a satellite-type constant.
func specFromExtra(extra map[string]string) (energy.PowerSpecInput, bool) {
	keys := []string{ExPanelAreaKey, ExCellEfficiencyKey, ExSystemEffKey, ExIdlePowerKey, ExMaxPowerKey}
	values := make(map[string]float64, len(keys))
	for _, k := range keys {
		raw, present := extra[k]
		if !present {
			return energy.PowerSpecInput{}, false
		}
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			logrus.Warnf("Power spec key %s has non-numeric value %q, skipping instance", k, raw)
			return energy.PowerSpecInput{}, false
		}
		values[k] = parsed
	}
	return energy.PowerSpecInput{
		PanelAreaM2:      values[ExPanelAreaKey],
		CellEfficiency:   values[ExCellEfficiencyKey],
		SystemEfficiency: values[ExSystemEffKey],
		IdlePowerW:       values[ExIdlePowerKey],
		MaxPowerW:        values[ExMaxPowerKey],
	}, true
}

func powerDaemonFunc(sigChan chan int, errChan chan error) {
	generationModel := energy.NewFullPowerModel()
	consumptionModel := energy.NewLinearConsumptionModel()

	for {
		select {
		case sig := <-sigChan:
			if sig == signal.STOP_SIGNAL {
				return
			}
		case <-time.After(time.Duration(config.GlobalConfig.App.MonitorInterval) * time.Second):
			// tick, same cadence as MonitorModule — power state is cheap to
			// recompute and should stay roughly in sync with other status
			// data rather than running on its own unrelated interval
		}
		
		// What instances are running on this node? Eg. Satellites/Ground Stations
		instances, err := synchronizer.GetInstanceList(key.NodeIndex)
		if err != nil {
			logrus.Errorf("PowerModule: get instance list of node %d error: %s", key.NodeIndex, err.Error())
			errChan <- err
			continue
		}
		logrus.Infof("PowerModule DEBUG: node_index=%d instances_found=%d", key.NodeIndex, len(instances))
		
		// Get sun angle and u(t) from etcd database
		solarInputs, err := synchronizer.GetAllInstanceSolarInput()
		if err != nil {
			logrus.Errorf("PowerModule: get solar input error: %s", err.Error())
			errChan <- err
			continue
		}

		for _, instance := range instances {
			spec, ok := specFromExtra(instance.Extra)
			if !ok {
				continue // not a power-modeled instance (e.g. ground station)... we don't wanna model a GS as a satellite
			}

			input, hasInput := solarInputs[instance.InstanceID]
			if !hasInput {
				logrus.Infof("PowerModule DEBUG: instance %s has valid spec but no solar_input entry found", instance.InstanceID)
				continue
			}

			// call the energy caclulation models
			generated := generationModel.Generate(spec, input.SunAngleRad)
			consumed := consumptionModel.Consume(spec, input.Utilization)

			status := model.PowerStatus{
				GeneratedPowerW: generated,
				ConsumedPowerW:  consumed,
				NetPowerW:       generated - consumed,
			}
			
			// Write Result back into etcd
			err = synchronizer.PutInstancePower(instance.InstanceID, status)
			if err != nil {
				logrus.Errorf("PowerModule: put power status for %s error: %s", instance.InstanceID, err.Error())
			} else {
				logrus.Infof("PowerModule DEBUG: successfully wrote power status for %s: gen=%.2fW consume=%.2fW", instance.InstanceID, generated, consumed)
			}
		}
	}
}