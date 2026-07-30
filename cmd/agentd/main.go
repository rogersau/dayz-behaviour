package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/rogersau/dayz-behaviour/internal/agent"
	"github.com/rogersau/dayz-behaviour/internal/agentservice"
	"golang.org/x/term"
)

const (
	serviceName        = "DayZBehaviourAgent"
	serviceDisplayName = "DayZ Behaviour Agent"
)

func main() {
	if err := execute(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "dayz-behaviour-agent:", err)
		os.Exit(1)
	}
}

func execute(args []string) error {
	command := "run"
	if len(args) > 0 && args[0] != "" && args[0][0] != '-' {
		command = args[0]
		args = args[1:]
	}
	switch command {
	case "run":
		return runCommand(args)
	case "init":
		return initCommand(args)
	case "install":
		return installCommand(args)
	case "start":
		return agentservice.Start(serviceName)
	case "stop":
		return agentservice.Stop(serviceName)
	case "status":
		status, err := agentservice.Query(serviceName)
		if err != nil {
			return err
		}
		fmt.Println(status.State)
		return nil
	case "uninstall":
		return agentservice.Uninstall(serviceName)
	case "generate-credential":
		value, err := randomCredential()
		if err != nil {
			return err
		}
		fmt.Println(value)
		return nil
	case "version":
		fmt.Println(agent.Version)
		return nil
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

func runCommand(args []string) error {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	configPath := flags.String("config", defaultConfigPath(), "path to the agent JSON configuration")
	if err := flags.Parse(args); err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	run := func(ctx context.Context) error {
		config, err := agent.LoadConfig(*configPath)
		if err != nil {
			return err
		}
		runtime, err := agent.NewRuntime(config, logger)
		if err != nil {
			return err
		}
		return runtime.Run(ctx)
	}

	isService, err := agentservice.IsService()
	if err != nil {
		return fmt.Errorf("detect Windows service context: %w", err)
	}
	if isService {
		return agentservice.Run(serviceName, run)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return run(ctx)
}

func initCommand(args []string) error {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	configPath := flags.String("config", defaultConfigPath(), "path to create")
	serverID := flags.String("server-id", "", "server ID configured in the DayZ mod")
	upstreamURL := flags.String("upstream-url", "", "central DayZ Behaviour base URL")
	dayZSpoolDir := flags.String("dayz-spool-dir", "", "optional DayZBehaviourProbe spool directory")
	force := flags.Bool("force", false, "overwrite an existing configuration")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *serverID == "" || *upstreamURL == "" {
		return errors.New("init requires -server-id and -upstream-url")
	}
	if !*force {
		if _, err := os.Stat(*configPath); err == nil {
			return fmt.Errorf("configuration already exists at %s; use -force to replace it", *configPath)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect configuration path: %w", err)
		}
	}
	upstreamCredential, err := readUpstreamCredential()
	if err != nil {
		return err
	}
	localCredential, err := randomCredential()
	if err != nil {
		return err
	}
	config := agent.DefaultConfig()
	config.ServerID = *serverID
	config.UpstreamURL = *upstreamURL
	config.UpstreamBearerToken = upstreamCredential
	config.LocalIngestToken = localCredential
	config.DayZSpoolDir = *dayZSpoolDir
	if err := config.Validate(); err != nil {
		return err
	}
	if err := agent.SaveConfig(*configPath, config); err != nil {
		return err
	}
	fmt.Printf("Created %s\n", *configPath)
	fmt.Printf("Set the DayZ mod ingest_token to: %s\n", localCredential)
	fmt.Println("Set the DayZ mod endpoint to: http://127.0.0.1:8080/")
	return nil
}

func installCommand(args []string) error {
	flags := flag.NewFlagSet("install", flag.ContinueOnError)
	configPath := flags.String("config", defaultConfigPath(), "path to the agent JSON configuration")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if _, err := agent.LoadConfig(*configPath); err != nil {
		return err
	}
	executablePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	if err := agentservice.Install(serviceName, serviceDisplayName, executablePath, *configPath); err != nil {
		return err
	}
	fmt.Printf("Installed %s. Run this executable with the start command to start it.\n", serviceDisplayName)
	return nil
}

func defaultConfigPath() string {
	executablePath, err := os.Executable()
	if err != nil {
		return "dayz-behaviour-agent.json"
	}
	return filepath.Join(filepath.Dir(executablePath), "dayz-behaviour-agent.json")
}

func readUpstreamCredential() (string, error) {
	if value := strings.TrimSpace(os.Getenv("DBA_AGENT_UPSTREAM_TOKEN")); value != "" {
		return value, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New("set DBA_AGENT_UPSTREAM_TOKEN or run init in an interactive terminal")
	}
	fmt.Print("Paste the credential issued for this server: ")
	value, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("read upstream credential: %w", err)
	}
	credential := strings.TrimSpace(string(value))
	if credential == "" {
		return "", errors.New("upstream credential is required")
	}
	return credential, nil
}

func randomCredential() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate local ingest credential: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func printUsage() {
	fmt.Print(`DayZ Behaviour Agent

Commands:
  init                Create a configuration file and local DayZ credential
  run                 Run in the foreground, or under the Windows service manager
  install             Install as an automatic Windows service
  start               Start the installed Windows service
  stop                Stop the installed Windows service
  status              Print the installed Windows service state
  uninstall           Remove the Windows service
  generate-credential Generate a random per-server central credential
  version             Print the agent version
`)
}
