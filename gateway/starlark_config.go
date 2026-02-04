package gateway

import (
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

func MakeConfigModule(cfg *Config) starlark.Value {
	return starlarkstruct.FromStringDict(starlark.String("config"), starlark.StringDict{
		"get": starlark.NewBuiltin("config.get", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			var key string
			var def starlark.Value = starlark.None
			if err := starlark.UnpackArgs("config.get", args, kwargs, "key", &key, "default?", &def); err != nil {
				return starlark.None, err
			}
			val, ok := lookupConfigValue(cfg, key)
			if !ok {
				return def, nil
			}
			return convertToStarlark(val), nil
		}),
	})
}

func lookupConfigValue(cfg *Config, key string) (interface{}, bool) {
	if key == "" {
		return nil, false
	}
	parts := strings.Split(key, ".")
	var cur interface{} = configToMap(cfg)
	for _, p := range parts {
		switch node := cur.(type) {
		case map[string]interface{}:
			next, ok := node[p]
			if !ok {
				return nil, false
			}
			cur = next
		default:
			return nil, false
		}
	}
	return cur, true
}

func configToMap(cfg *Config) map[string]interface{} {
	gw := cfg.Gateway
	st := cfg.Strategy
	hc := gw.HealthCheckDefault
	nodes := make([]interface{}, 0, len(cfg.Nodes))
	for _, n := range cfg.Nodes {
		nodes = append(nodes, map[string]interface{}{
			"id":           n.ID,
			"address":      n.Address,
			"weight":       n.Weight,
			"check_script": n.CheckScript,
		})
	}

	return map[string]interface{}{
		"gateway": map[string]interface{}{
			"listen":                  gw.Listen,
			"shards":                  gw.Shards,
			"max_retries":             gw.MaxRetries,
			"max_body_size":           gw.MaxBodySize,
			"retry_non_idempotent":    gw.RetryNonIdempotent,
			"admin_api_enabled":       gw.AdminAPIEnabled,
			"read_timeout":            gw.ReadTimeout.Duration.String(),
			"write_timeout":           gw.WriteTimeout.Duration.String(),
			"idle_timeout":            gw.IdleTimeout.Duration.String(),
			"response_header_timeout": gw.ResponseHeaderTimeout.Duration.String(),
			"idle_conn_timeout":       gw.IdleConnTimeout.Duration.String(),
			"upstream_timeout":        gw.UpstreamTimeout.Duration.String(),
			"max_idle_conns":          gw.MaxIdleConns,
			"max_idle_conns_per_host": gw.MaxIdleConnsPerHost,
			"max_conns_per_host":      gw.MaxConnsPerHost,
			"health_check_default": map[string]interface{}{
				"interval": hc.Interval.Duration.String(),
				"timeout":  hc.Timeout.Duration.String(),
				"script":   hc.Script,
			},
			"trigger_script":     gw.TriggerScript,
			"trigger_timeout":    gw.TriggerTimeout.Duration.String(),
			"trigger_body_limit": gw.TriggerBodyLimit,
			"openai_check_key":   gw.OpenAICheckKey,
			"openai_check_model": gw.OpenAICheckModel,
		},
		"strategy": map[string]interface{}{
			"min_weight":        st.MinWeight,
			"penalty_factor":    st.PenaltyFactor,
			"recovery_interval": st.RecoveryInterval.Duration.String(),
			"hash_shard":        st.HashShard,
		},
		"nodes": nodes,
	}
}
