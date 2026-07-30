//go:build windows

package agentservice

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func IsService() (bool, error) {
	return svc.IsWindowsService()
}

func Run(name string, run RunFunc) error {
	if run == nil {
		return errors.New("service run function is required")
	}
	return svc.Run(name, &handler{run: run})
}

func Install(name, displayName, executablePath, configPath string) error {
	executablePath, err := filepath.Abs(executablePath)
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}
	configPath, err = filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("resolve config path: %w", err)
	}
	if _, err := os.Stat(executablePath); err != nil {
		return fmt.Errorf("inspect executable: %w", err)
	}
	if _, err := os.Stat(configPath); err != nil {
		return fmt.Errorf("inspect config: %w", err)
	}

	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to Windows service manager: %w", err)
	}
	defer manager.Disconnect()
	if existing, openErr := manager.OpenService(name); openErr == nil {
		existing.Close()
		return fmt.Errorf("service %s is already installed", name)
	}
	service, err := manager.CreateService(name, executablePath, mgr.Config{
		DisplayName: displayName,
		Description: "Receives DayZ Behaviour telemetry locally and forwards it durably to the central service.",
		StartType:   mgr.StartAutomatic,
	}, "run", "-config", configPath)
	if err != nil {
		return fmt.Errorf("create Windows service: %w", err)
	}
	defer service.Close()
	return nil
}

func Start(name string) error {
	service, cleanup, err := openService(name)
	if err != nil {
		return err
	}
	defer cleanup()
	if err := service.Start(); err != nil {
		return fmt.Errorf("start Windows service: %w", err)
	}
	return nil
}

func Stop(name string) error {
	service, cleanup, err := openService(name)
	if err != nil {
		return err
	}
	defer cleanup()
	status, err := service.Query()
	if err != nil {
		return fmt.Errorf("query Windows service: %w", err)
	}
	if status.State == svc.Stopped {
		return nil
	}
	if _, err := service.Control(svc.Stop); err != nil {
		return fmt.Errorf("stop Windows service: %w", err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		status, err = service.Query()
		if err != nil {
			return fmt.Errorf("query Windows service while stopping: %w", err)
		}
		if status.State == svc.Stopped {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return errors.New("Windows service did not stop within 30 seconds")
}

func Query(name string) (Status, error) {
	service, cleanup, err := openService(name)
	if err != nil {
		return Status{}, err
	}
	defer cleanup()
	status, err := service.Query()
	if err != nil {
		return Status{}, fmt.Errorf("query Windows service: %w", err)
	}
	return Status{State: stateName(status.State)}, nil
}

func Uninstall(name string) error {
	service, cleanup, err := openService(name)
	if err != nil {
		return err
	}
	defer cleanup()
	status, queryErr := service.Query()
	if queryErr == nil && status.State != svc.Stopped {
		if _, controlErr := service.Control(svc.Stop); controlErr == nil {
			deadline := time.Now().Add(30 * time.Second)
			for time.Now().Before(deadline) {
				status, queryErr = service.Query()
				if queryErr != nil || status.State == svc.Stopped {
					break
				}
				time.Sleep(300 * time.Millisecond)
			}
		}
	}
	if err := service.Delete(); err != nil {
		return fmt.Errorf("delete Windows service: %w", err)
	}
	return nil
}

type handler struct {
	run RunFunc
}

func (h *handler) Execute(_ []string, requests <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	changes <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- h.run(ctx) }()
	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	for {
		select {
		case request := <-requests:
			switch request.Cmd {
			case svc.Interrogate:
				changes <- request.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				cancel()
				if err := <-done; err != nil {
					return true, 1
				}
				return false, 0
			}
		case err := <-done:
			if err != nil {
				return true, 1
			}
			return false, 0
		}
	}
}

func openService(name string) (*mgr.Service, func(), error) {
	manager, err := mgr.Connect()
	if err != nil {
		return nil, nil, fmt.Errorf("connect to Windows service manager: %w", err)
	}
	service, err := manager.OpenService(name)
	if err != nil {
		manager.Disconnect()
		return nil, nil, fmt.Errorf("open Windows service %s: %w", name, err)
	}
	cleanup := func() {
		service.Close()
		manager.Disconnect()
	}
	return service, cleanup, nil
}

func stateName(state svc.State) string {
	switch state {
	case svc.Stopped:
		return "stopped"
	case svc.StartPending:
		return "start_pending"
	case svc.StopPending:
		return "stop_pending"
	case svc.Running:
		return "running"
	case svc.ContinuePending:
		return "continue_pending"
	case svc.PausePending:
		return "pause_pending"
	case svc.Paused:
		return "paused"
	default:
		return fmt.Sprintf("unknown_%d", state)
	}
}
