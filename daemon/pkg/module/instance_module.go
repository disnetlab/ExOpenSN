package module

import (
	"NodeDaemon/config"
	"NodeDaemon/data"
	"NodeDaemon/model"
	"encoding/json"
	"strings"

	"NodeDaemon/share/dir"
	"NodeDaemon/share/key"
	"NodeDaemon/share/signal"
	"NodeDaemon/utils"
	"context"
	"fmt"
	"sync"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/sirupsen/logrus"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const MaskGroupSizeLog2 = 5
const Mask32 = 0x1F
const CPUPeriodUs = 5000

var StopTimeoutSecond = 20
var InstanceParallelLimitChan chan uint8

var (
	cpuCoreInex              = 0
	cpuCoreMutex *sync.Mutex = &sync.Mutex{}
)

func GetCPUCoreIndex() int {
	cpuCoreMutex.Lock()
	defer cpuCoreMutex.Unlock()
	ret := (cpuCoreInex + 1) % key.SelfNode.CPUCore
	cpuCoreInex = ret
	return ret
}

func Max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func GenerateCoreIndexMaskHex(coreIndex int, totalCore int) string {
	sb := strings.Builder{}
	groupCount := totalCore >> MaskGroupSizeLog2
	hasMod := totalCore & Mask32
	flipGroup := coreIndex >> MaskGroupSizeLog2
	coreBitIndex := coreIndex & Mask32

	if hasMod != 0 {
		groupCount++
	}

	for i := groupCount - 1; i >= 0; i-- {
		if flipGroup == i {
			sb.WriteString(fmt.Sprintf("%08x", 1<<coreBitIndex))
		} else {
			sb.WriteString("00000000")
		}
		if i != 0 {
			sb.WriteByte(',')
		}
	}
	return sb.String()[((32-hasMod)&Mask32)>>2:]

}

func CreateContainer(instance *model.Instance) error {
	containerID := fmt.Sprintf("%s_%s", instance.Type, instance.InstanceID)
	localCpuCoreIndex := GetCPUCoreIndex()

	err := utils.DoWithRetry(func() error {

		containerConfig := &container.Config{
			Hostname: instance.InstanceID,
			Image:    instance.Image,
			Env: []string{
				fmt.Sprintf("IRQ_MASK=%s", GenerateCoreIndexMaskHex(localCpuCoreIndex, key.SelfNode.CPUCore)),
				// "IRQ_MASK=00,00000000",
			},
			StopTimeout: &StopTimeoutSecond,
		}

		hostConfig := &container.HostConfig{
			Privileged:  true,
			NetworkMode: "none",
			Binds: []string{
				fmt.Sprintf("%s:%s", dir.MountShareData, "/share"),
			},
			Resources: container.Resources{
				// NanoCPUs:   instance.Resource.NanoCPU,
				Memory:     instance.Resource.MemoryByte,
				CPUPeriod:  CPUPeriodUs,
				CPUQuota:   Max(1001, instance.Resource.NanoCPU*CPUPeriodUs/1e9+1),
				CpusetCpus: fmt.Sprintf("%d", localCpuCoreIndex),
			},
		}

		_, err := utils.DockerClient.ContainerCreate(
			context.Background(),
			client.ContainerCreateOptions{
				Config:     containerConfig,
				HostConfig: hostConfig,
				Name:       containerID,
			},
		)

		if err != nil {
			return err
		}

		return err
	}, 4)

	if err != nil {
		return fmt.Errorf("create container %s Error: %s", containerID, err.Error())
	}
	return nil
}

func StartContainer(instance *model.Instance) (int, error) {

	containerID := fmt.Sprintf("%s_%s", instance.Type, instance.InstanceID)
	pid := 0
	err := utils.DoWithRetry(func() error {
		_, err := utils.DockerClient.ContainerStart(
			context.Background(),
			containerID,
			client.ContainerStartOptions{},
		)
		return err
	}, 4)
	if err != nil {
		return pid, fmt.Errorf("start container %s error: %s", containerID, err.Error())
	}

	err = utils.DoWithRetry(func() error {
		info, err := utils.DockerClient.ContainerInspect(context.Background(), containerID, client.ContainerInspectOptions{})
		if err != nil {
			return err
		}
		pid = info.Container.State.Pid
		return nil
	}, 4)

	if err != nil {
		return pid, fmt.Errorf("get pid of container %s error: %s", containerID, err.Error())
	}

	return pid, nil
}

func StopContainer(instance *model.Instance) error {

	containerID := fmt.Sprintf("%s_%s", instance.Type, instance.InstanceID)

	err := utils.DoWithRetry(func() error {
		_, err := utils.DockerClient.ContainerStop(context.Background(), containerID, client.ContainerStopOptions{})
		return err
	}, 4)

	if err != nil {
		err := fmt.Errorf(
			"stop container %s error: %s",
			containerID,
			err.Error(),
		)
		return err
	}

	return nil
}

func RemoveContainer(instance *model.Instance) error {
	containerID := fmt.Sprintf("%s_%s", instance.Type, instance.InstanceID)
	err := utils.DoWithRetry(func() error {
		_, err := utils.DockerClient.ContainerRemove(
			context.Background(),
			containerID,
			client.ContainerRemoveOptions{Force: true},
		)
		return err
	}, 4)

	if err != nil {
		err := fmt.Errorf(
			"stop container %s error: %s",
			containerID,
			err.Error(),
		)
		return err
	}

	return nil
}

func UpdateInstanceState(oldInstance, newInstance *model.Instance) error {
	var err error
	InstanceParallelLimitChan <- 1
	defer func() {
		<-InstanceParallelLimitChan
	}()
	isRunning := oldInstance.IsRunning()
	isCreated := oldInstance.IsCreated()

	shouldRunning := newInstance.Start
	shouldCreate := newInstance.InstanceID != ""

	if isCreated != shouldCreate {
		if shouldCreate {
			err = CreateContainer(newInstance)
		} else {
			StopContainer(oldInstance)
			data.DeleteInstancePid(oldInstance.InstanceID)
			err = RemoveContainer(oldInstance)
		}
	}

	if err != nil {
		return fmt.Errorf("update create state error: %s", err.Error())
	}

	if shouldRunning != isRunning {
		if shouldRunning {
			var pid int
			pid, err = StartContainer(newInstance)
			data.SetInstancePid(newInstance.InstanceID, pid)
		} else if shouldCreate {
			err = StopContainer(oldInstance)
			data.DeleteInstancePid(oldInstance.InstanceID)
		}
	}
	if err != nil {
		return fmt.Errorf("update running state error: %s", err.Error())
	}
	return nil
}

const InstanceModuleContainerName = "instance_manager"

type InstanceModule struct {
	Base
}

func watchInstanceDaemon(sigChan chan int, errChan chan error) {
	for {
		ctx, cancel := context.WithCancel(context.Background())
		watchChan := utils.EtcdClient.Watch(ctx, key.NodeInstanceListKeySelf, clientv3.WithPrefix(), clientv3.WithPrevKV())

		for {
			select {
			case sig := <-sigChan:
				if sig == signal.STOP_SIGNAL {
					cancel()
					return
				}
			case res := <-watchChan:
				for _, v := range res.Events {
					go func(v *clientv3.Event) {
						etcdKey := ""
						oldInstance := new(model.Instance)
						newInstance := new(model.Instance)
						if v.PrevKv != nil && len(v.PrevKv.Value) > 0 {
							etcdKey = string(v.PrevKv.Key)
							err := json.Unmarshal(v.PrevKv.Value, oldInstance)
							if err != nil {
								errMsg := fmt.Sprintf("Parse Instance Info From %s Error: %s", etcdKey, err.Error())
								logrus.Error(errMsg)
								return
							}
						}
						if v.Kv != nil && len(v.Kv.Value) > 0 {
							etcdKey = string(v.Kv.Key)
							err := json.Unmarshal(v.Kv.Value, newInstance)
							if err != nil {
								errMsg := fmt.Sprintf("Parse Instance Info From %s Error: %s", etcdKey, err.Error())
								logrus.Error(errMsg)
								return
							}
						}
						err := UpdateInstanceState(oldInstance, newInstance)
						if err != nil {
							errMsg := fmt.Sprintf("Update Instance %s State Error: %s", etcdKey, err.Error())
							logrus.Error(errMsg)
						} else {
							logrus.Infof("Update Instance %s State Success", etcdKey)
						}
					}(v)
				}

			}
		}
	}
}

func CreateInstanceManagerModule() *InstanceModule {
	InstanceParallelLimitChan = make(chan uint8, config.GlobalConfig.App.ParallelNum)
	return &InstanceModule{
		Base{
			sigChan:    make(chan int),
			errChan:    make(chan error),
			running:    false,
			daemonFunc: watchInstanceDaemon,
			wg:         new(sync.WaitGroup),
			ModuleName: "InstanceManage",
		},
	}
}
