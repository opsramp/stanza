package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/kardianos/service"
	"github.com/opsramp/stanza/agent"
	"go.uber.org/zap"
)

// AgentService is a service that runs the stanza agent.
type AgentService struct {
	cancel context.CancelFunc
	agent  *agent.LogAgent
}

// Start will start the stanza agent.
func (a *AgentService) Start(_ service.Service) error {
	a.agent.Info("Starting Opsramp agent")
	if err := a.agent.Start(); err != nil {
		a.agent.Errorw("Failed to start Opsramp agent", zap.Any("error", err))
		a.cancel()
		return nil
	}

	a.agent.Info("Opsramp agent started")
	return nil
}

// Stop will stop the opsramp agent.
func (a *AgentService) Stop(_ service.Service) error {
	a.agent.Info("Stopping opsramp agent")
	if err := a.agent.Stop(); err != nil {
		a.agent.Errorw("Failed to stop opsramp agent gracefully", zap.Any("error", err))
		a.cancel()
		return nil
	}

	a.agent.Info("Opsramp agent stopped")
	a.cancel()
	return nil
}

// newAgentService creates a new agent service with the provided agent.
func newAgentService(ctx context.Context, agent *agent.LogAgent, cancel context.CancelFunc) (service.Service, error) {
	agentService := &AgentService{cancel, agent}
	config := &service.Config{
		Name:        "opsramp",
		DisplayName: "OpsRamp Log Agent",
		Description: "Monitors and processes log entries",
		Option: service.KeyValue{
			"RunWait": func() {
				var sigChan = make(chan os.Signal, 3)
				signal.Notify(sigChan, syscall.SIGTERM, os.Interrupt)
				select {
				case <-sigChan:
				case <-ctx.Done():
				}
			},
		},
	}

	service, err := service.New(agentService, config)
	if err != nil {
		return nil, err
	}

	return service, nil
}
