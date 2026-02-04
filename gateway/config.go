package gateway

import (
	"os"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Gateway  GatewayConfig  `toml:"gateway"`
	Strategy StrategyConfig `toml:"strategy"`
	Nodes    []NodeConfig   `toml:"nodes"`
}

type GatewayConfig struct {
	Listen                string            `toml:"listen"`
	Shards                int               `toml:"shards"`
	MaxRetries            int               `toml:"max_retries"`
	MaxBodySize           int64             `toml:"max_body_size"`
	RetryNonIdempotent    bool              `toml:"retry_non_idempotent"`
	AdminAPIEnabled       bool              `toml:"admin_api_enabled"`
	AdminAPIToken         string            `toml:"admin_api_token"`
	ReadTimeout           Duration          `toml:"read_timeout"`
	WriteTimeout          Duration          `toml:"write_timeout"`
	IdleTimeout           Duration          `toml:"idle_timeout"`
	ResponseHeaderTimeout Duration          `toml:"response_header_timeout"`
	IdleConnTimeout       Duration          `toml:"idle_conn_timeout"`
	UpstreamTimeout       Duration          `toml:"upstream_timeout"`
	MaxIdleConns          int               `toml:"max_idle_conns"`
	MaxIdleConnsPerHost   int               `toml:"max_idle_conns_per_host"`
	MaxConnsPerHost       int               `toml:"max_conns_per_host"`
	HealthCheckDefault    HealthCheckConfig `toml:"health_check_default"`
	TriggerScript         string            `toml:"trigger_script"`
	TriggerTimeout        Duration          `toml:"trigger_timeout"`
	TriggerBodyLimit      int               `toml:"trigger_body_limit"`
	OpenAICheckKey        string            `toml:"openai_check_key"`
	OpenAICheckModel      string            `toml:"openai_check_model"`
	Retry                 RetryConfig       `toml:"retry"`
}

type RetryConfig struct {
	Enabled        bool `toml:"enabled"`
	EnablePost     bool `toml:"enable_post"`
	MaxRetries     int  `toml:"max_retries"`
	RetryOn5xx     bool `toml:"retry_on_5xx"`
	RetryOnError   bool `toml:"retry_on_error"`
	RetryOnTimeout bool `toml:"retry_on_timeout"`
}

type StrategyConfig struct {
	MinWeight               int32    `toml:"min_weight"`
	PenaltyFactor           float64  `toml:"penalty_factor"`
	RecoveryInterval        Duration `toml:"recovery_interval"`
	MaxPenaltyPerSecond     int32    `toml:"max_penalty_per_second"`
	ConnFactorEnabled       bool     `toml:"conn_factor_enabled"`
	ConnFactorSmoothing     int32    `toml:"conn_factor_smoothing"`
	ConnFactorSlope         float64  `toml:"conn_factor_slope"`
	ConnFactorSyncThreshold float64  `toml:"conn_factor_sync_threshold"`
	ConnFactorEMAAlpha      float64  `toml:"conn_factor_ema_alpha"`
	HashShard               bool     `toml:"hash_shard"`
}

type HealthCheckConfig struct {
	Interval Duration `toml:"interval"`
	Timeout  Duration `toml:"timeout"`
	Script   string   `toml:"script"`
}

type NodeConfig struct {
	ID          string `toml:"id"`
	Address     string `toml:"address"`
	Weight      int32  `toml:"weight"`
	CheckScript string `toml:"check_script"`
}

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(text []byte) error {
	parsed, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	d.Duration = parsed
	return nil
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Gateway.Shards <= 0 {
		cfg.Gateway.Shards = 1
	}
	if cfg.Gateway.MaxRetries < 0 {
		cfg.Gateway.MaxRetries = 0
	}
	if cfg.Gateway.MaxBodySize <= 0 {
		cfg.Gateway.MaxBodySize = 1 << 20
	}
	if cfg.Gateway.ReadTimeout.Duration <= 0 {
		cfg.Gateway.ReadTimeout = Duration{Duration: 15 * time.Second}
	}
	if cfg.Gateway.WriteTimeout.Duration <= 0 {
		cfg.Gateway.WriteTimeout = Duration{Duration: 30 * time.Second}
	}
	if cfg.Gateway.IdleTimeout.Duration <= 0 {
		cfg.Gateway.IdleTimeout = Duration{Duration: 60 * time.Second}
	}
	if cfg.Gateway.ResponseHeaderTimeout.Duration <= 0 {
		cfg.Gateway.ResponseHeaderTimeout = Duration{Duration: 10 * time.Second}
	}
	if cfg.Gateway.IdleConnTimeout.Duration <= 0 {
		cfg.Gateway.IdleConnTimeout = Duration{Duration: 90 * time.Second}
	}
	if cfg.Gateway.UpstreamTimeout.Duration <= 0 {
		cfg.Gateway.UpstreamTimeout = Duration{Duration: 5 * time.Second}
	}
	if cfg.Gateway.MaxIdleConns <= 0 {
		cfg.Gateway.MaxIdleConns = 256
	}
	if cfg.Gateway.MaxIdleConnsPerHost <= 0 {
		cfg.Gateway.MaxIdleConnsPerHost = 64
	}
	if cfg.Gateway.Retry.MaxRetries <= 0 {
		if cfg.Gateway.MaxRetries > 0 {
			cfg.Gateway.Retry.MaxRetries = cfg.Gateway.MaxRetries
		} else {
			cfg.Gateway.Retry.MaxRetries = 2
		}
	}
	if cfg.Gateway.TriggerTimeout.Duration <= 0 {
		cfg.Gateway.TriggerTimeout = Duration{Duration: 2 * time.Second}
	}
	if cfg.Gateway.TriggerBodyLimit <= 0 {
		cfg.Gateway.TriggerBodyLimit = 4096
	}
	if cfg.Strategy.MinWeight <= 0 {
		cfg.Strategy.MinWeight = 1
	}
	if cfg.Strategy.MaxPenaltyPerSecond < 0 {
		cfg.Strategy.MaxPenaltyPerSecond = 0
	}
	if cfg.Strategy.ConnFactorSmoothing < 0 {
		cfg.Strategy.ConnFactorSmoothing = 0
	}
	if cfg.Strategy.ConnFactorSlope <= 0 {
		cfg.Strategy.ConnFactorSlope = 0.4
	}
	if cfg.Strategy.ConnFactorSyncThreshold <= 0 {
		cfg.Strategy.ConnFactorSyncThreshold = 0.5
	}
	if cfg.Strategy.ConnFactorEMAAlpha <= 0 || cfg.Strategy.ConnFactorEMAAlpha > 1 {
		cfg.Strategy.ConnFactorEMAAlpha = 0.2
	}
	if cfg.Strategy.RecoveryInterval.Duration <= 0 {
		cfg.Strategy.RecoveryInterval = Duration{Duration: 10 * time.Second}
	}
	if cfg.Gateway.HealthCheckDefault.Interval.Duration <= 0 {
		cfg.Gateway.HealthCheckDefault.Interval = Duration{Duration: 10 * time.Second}
	}
	if cfg.Gateway.HealthCheckDefault.Timeout.Duration <= 0 {
		cfg.Gateway.HealthCheckDefault.Timeout = Duration{Duration: 2 * time.Second}
	}
	return &cfg, nil
}
