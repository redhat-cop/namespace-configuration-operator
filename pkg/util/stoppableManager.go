package util

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("controller_locked_object")

type StoppableManager struct {
	manager.Manager
	stopChannel chan struct{}
}

func (sm *StoppableManager) Stop() {
	close(sm.stopChannel)
}

func (sm *StoppableManager) Start() {
	go sm.Manager.Start(sm.stopChannel)
}

func NewStoppableManager(parentManager manager.Manager) (StoppableManager, error) {
	manager, err := manager.New(parentManager.GetConfig(), manager.Options{})
	if err != nil {
		return StoppableManager{}, err
	}
	return StoppableManager{
		Manager:     manager,
		stopChannel: make(chan struct{}),
	}, nil
}
