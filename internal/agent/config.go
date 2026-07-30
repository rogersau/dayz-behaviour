package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Version is replaced at release build time using -ldflags -X.
var Version = "dev"

const (
	defaultMaxQueueBytes   int64 = 5 * 1024 * 1024 * 1024
	defaultMaxQueueBatches       = 250_000
)

type Config struct {
	ListenAddr            string `json:"listen_addr"`
	ServerID              string `json:"server_id"`
	LocalIngestToken      string `json:"local_ingest_token"`
	UpstreamURL           string `json:"upstream_url"`
	UpstreamBearerToken   string `json:"upstream_bearer_token"`
	DataDir               string `json:"data_dir"`
	DayZSpoolDir          string `json:"dayz_spool_dir,omitempty"`
	MaxQueueBytes         int64  `json:"max_queue_bytes"`
	MaxQueueBatches       int    `json:"max_queue_batches"`
	UploadIntervalSeconds int    `json:"upload_interval_seconds"`
	RequestTimeoutSeconds int    `json:"request_timeout_seconds"`
	MinimumRetrySeconds   int    `json:"minimum_retry_seconds"`
	MaximumRetrySeconds   int    `json:"maximum_retry_seconds"`
}

func DefaultConfig() Config {
	return Config{
		ListenAddr:            "127.0.0.1:8080",
		DataDir:               "./data",
		MaxQueueBytes:         defaultMaxQueueBytes,
		MaxQueueBatches:       defaultMaxQueueBatches,
		UploadIntervalSeconds: 1,
		RequestTimeoutSeconds: 15,
		MinimumRetrySeconds:   1,
		MaximumRetrySeconds:   300,
	}
}

func LoadConfig(path string) (Config, error) {
	config := DefaultConfig()
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("read agent config: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("decode agent config: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return Config{}, fmt.Errorf("decode agent config: %w", err)
	}

	applySecretEnvironment(&config)
	if err := config.resolvePaths(filepath.Dir(path)); err != nil {
		return Config{}, err
	}
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func SaveConfig(path string, config Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("encode agent config: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write agent config: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("restrict agent config permissions: %w", err)
	}
	return nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.ServerID) == "" {
		return errors.New("agent config server_id is required")
	}
	if strings.TrimSpace(c.LocalIngestToken) == "" {
		return errors.New("agent config local_ingest_token is required")
	}
	if strings.TrimSpace(c.UpstreamBearerToken) == "" {
		return errors.New("agent config upstream_bearer_token is required")
	}
	if strings.TrimSpace(c.DataDir) == "" {
		return errors.New("agent config data_dir is required")
	}
	if err := validateLoopbackAddress(c.ListenAddr); err != nil {
		return err
	}
	upstream, err := url.Parse(c.UpstreamURL)
	if err != nil || upstream.Host == "" {
		return errors.New("agent config upstream_url must be an absolute URL")
	}
	if upstream.User != nil || upstream.RawQuery != "" || upstream.Fragment != "" {
		return errors.New("agent config upstream_url must not contain credentials, a query string, or a fragment")
	}
	if upstream.Scheme != "https" && !(upstream.Scheme == "http" && isLoopbackHost(upstream.Hostname())) {
		return errors.New("agent config upstream_url must use HTTPS unless it targets loopback")
	}
	if c.MaxQueueBytes <= 0 {
		return errors.New("agent config max_queue_bytes must be greater than zero")
	}
	if c.MaxQueueBatches <= 0 {
		return errors.New("agent config max_queue_batches must be greater than zero")
	}
	if c.UploadIntervalSeconds <= 0 || c.RequestTimeoutSeconds <= 0 {
		return errors.New("agent upload interval and request timeout must be greater than zero")
	}
	if c.MinimumRetrySeconds <= 0 || c.MaximumRetrySeconds < c.MinimumRetrySeconds {
		return errors.New("agent retry bounds are invalid")
	}
	return nil
}

func (c Config) UpstreamEndpoint() string {
	base := strings.TrimRight(c.UpstreamURL, "/")
	if strings.HasSuffix(base, "/v1/telemetry/batches") {
		return base
	}
	return base + "/v1/telemetry/batches"
}

func (c Config) UploadInterval() time.Duration {
	return time.Duration(c.UploadIntervalSeconds) * time.Second
}

func (c Config) RequestTimeout() time.Duration {
	return time.Duration(c.RequestTimeoutSeconds) * time.Second
}

func (c Config) MinimumRetry() time.Duration {
	return time.Duration(c.MinimumRetrySeconds) * time.Second
}

func (c Config) MaximumRetry() time.Duration {
	return time.Duration(c.MaximumRetrySeconds) * time.Second
}

func (c *Config) resolvePaths(base string) error {
	var err error
	c.DataDir, err = resolvePath(base, c.DataDir)
	if err != nil {
		return fmt.Errorf("resolve data_dir: %w", err)
	}
	if strings.TrimSpace(c.DayZSpoolDir) != "" {
		c.DayZSpoolDir, err = resolvePath(base, c.DayZSpoolDir)
		if err != nil {
			return fmt.Errorf("resolve dayz_spool_dir: %w", err)
		}
	}
	return nil
}

func resolvePath(base, path string) (string, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	return filepath.Abs(path)
}

func applySecretEnvironment(config *Config) {
	if value := os.Getenv("DBA_AGENT_LOCAL_TOKEN"); value != "" {
		config.LocalIngestToken = value
	}
	if value := os.Getenv("DBA_AGENT_UPSTREAM_TOKEN"); value != "" {
		config.UpstreamBearerToken = value
	}
}

func validateLoopbackAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("agent config listen_addr must include a valid host and port: %w", err)
	}
	if !isLoopbackHost(host) {
		return errors.New("agent config listen_addr must bind to loopback")
	}
	return nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(strings.TrimSpace(host), "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("config contains multiple JSON values")
}
