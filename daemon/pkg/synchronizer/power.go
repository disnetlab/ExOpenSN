// This file is the communication layer between the PowerModule and etcd.
// Its only job is to read and write energy-related data in etcd.

package synchronizer

import (
	"NodeDaemon/model"
	"NodeDaemon/share/key"
	"NodeDaemon/utils"
	"context"
	"encoding/json"
	"fmt"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// --- SolarInput: written by Python (pyephem), then read by PowerModule ---
// Mirrors position.go's GetAllInstancePosition / GetInstancePosition exactly,

func GetAllInstanceSolarInput() (map[string]model.SolarInput, error) {
	baseKey := key.InstanceSolarInputKey
	// Give me every key beginning with baseKey for solar input
	list, err := utils.EtcdClient.Get(
		context.Background(),
		baseKey,
		clientv3.WithPrefix(),
	)
	if err != nil {
		return nil, fmt.Errorf("get all instance solar input error: %s", err.Error())
	}
	result := make(map[string]model.SolarInput)
	for _, v := range list.Kvs {
		input := model.SolarInput{}
		err = json.Unmarshal(v.Value, &input)
		if err != nil {
			return nil, fmt.Errorf("unmarshal solar input error: %s", err.Error())
		}
		instanceID, _ := utils.GetEtcdLastKey(string(v.Key))
		result[instanceID] = input
	}
	return result, nil
}

// Store one satellite's solar input
func PutInstanceSolarInput(instanceID string, input model.SolarInput) error {
	data, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("marshal solar input error: %s", err.Error())
	}
	putKey := fmt.Sprintf("%s/%s", key.InstanceSolarInputKey, instanceID)
	_, err = utils.EtcdClient.Put(context.Background(), putKey, string(data))
	if err != nil {
		return fmt.Errorf("put instance %s solar input error: %s", instanceID, err.Error())
	}
	return nil
}

// After the PowerModule calculates power generation and consumption, it creates a PowerStatus (has data of gen, con, net power)

// this function retrieves all satellites' power statuses
// we don't use GetInstancePower() in a loop cuz that's inefficient, multiple etcd requests vs 1 
func GetAllInstancePower() (map[string]model.PowerStatus, error) {
	baseKey := key.InstancePowerKey
	list, err := utils.EtcdClient.Get(
		context.Background(),
		baseKey,
		clientv3.WithPrefix(),
	)
	if err != nil {
		return nil, fmt.Errorf("get all instance power error: %s", err.Error())
	}
	result := make(map[string]model.PowerStatus)
	for _, v := range list.Kvs {
		status := model.PowerStatus{}
		err = json.Unmarshal(v.Value, &status)
		if err != nil {
			return nil, fmt.Errorf("unmarshal power status error: %s", err.Error())
		}
		instanceID, _ := utils.GetEtcdLastKey(string(v.Key))
		result[instanceID] = status
	}
	return result, nil
}

// Retrieve one satellite's power status
func GetInstancePower(instanceID string) (model.PowerStatus, error) {
	putKey := fmt.Sprintf("%s/%s", key.InstancePowerKey, instanceID)
	info, err := utils.EtcdClient.Get(context.Background(), putKey)
	if err != nil {
		return model.PowerStatus{}, fmt.Errorf("get instance %s power error: %s", instanceID, err.Error())
	}
	if len(info.Kvs) <= 0 {
		return model.PowerStatus{}, fmt.Errorf("instance %s power not found", instanceID)
	}
	status := model.PowerStatus{}
	err = json.Unmarshal(info.Kvs[0].Value, &status)
	if err != nil {
		return model.PowerStatus{}, fmt.Errorf("unmarshal power status error: %s", err.Error())
	}
	return status, nil
}

func PutInstancePower(instanceID string, status model.PowerStatus) error {
	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("marshal power status error: %s", err.Error())
	}
	putKey := fmt.Sprintf("%s/%s", key.InstancePowerKey, instanceID)
	_, err = utils.EtcdClient.Put(context.Background(), putKey, string(data))
	if err != nil {
		return fmt.Errorf("put instance %s power error: %s", instanceID, err.Error())
	}
	return nil
}
