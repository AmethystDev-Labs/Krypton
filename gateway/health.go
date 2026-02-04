package gateway

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"go.starlark.net/starlark"
)

type HealthResult struct {
	Score   int32
	Status  string
	Message string
	Labels  map[string]string
}

type HealthChecker struct {
	cfg      *Config
	balancer *Balancer
}

func NewHealthChecker(cfg *Config, balancer *Balancer) *HealthChecker {
	return &HealthChecker{
		cfg:      cfg,
		balancer: balancer,
	}
}

func (h *HealthChecker) Run(ctx context.Context) {
	ticker := time.NewTicker(h.cfg.Gateway.HealthCheckDefault.Interval.Duration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.runOnce(ctx)
		}
	}
}

func (h *HealthChecker) runOnce(ctx context.Context) {
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup

	h.balancer.ForEachNode(func(n *Node) {
		wg.Add(1)
		sem <- struct{}{}
		go func(node *Node) {
			defer wg.Done()
			defer func() { <-sem }()

			checkCfg := h.cfg.Gateway.HealthCheckDefault
			if node.checkScript != "" {
				checkCfg.Script = node.checkScript
			}
			score, err := runStarlarkCheck(ctx, checkCfg, node, h.cfg)
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					Warnf("health check timeout node=%s err=%v", node.ID, err)
				} else {
					Warnf("health check error node=%s err=%v", node.ID, err)
				}
			}
			node.SetActiveScore(score)
			node.SyncWeight(node.PassiveScore(), node.ActiveScore())
			Infof("health check ok node=%s score=%d passive=%.0f active=%.0f", node.ID, score, node.PassiveScore(), node.ActiveScore())
		}(n)
	})

	wg.Wait()
}

func runStarlarkCheck(ctx context.Context, cfg HealthCheckConfig, n *Node, fullCfg *Config) (int32, error) {
	if cfg.Script == "" {
		return 100, nil
	}
	if _, err := os.Stat(cfg.Script); err != nil {
		return 100, err
	}

	timeout := cfg.Timeout.Duration
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	thread := &starlark.Thread{Name: "health_check"}
	done := make(chan error, 1)
	var result HealthResult

	go func() {
		predeclared := starlark.StringDict{
			"http":   MakeHttpModule(timeout),
			"log":    MakeLogModule(),
			"config": MakeConfigModule(fullCfg),
		}
		globals, err := starlark.ExecFile(thread, cfg.Script, nil, predeclared)
		if err != nil {
			done <- err
			return
		}
		fn, ok := globals["check"]
		if !ok {
			done <- errors.New("check() not found")
			return
		}
		nodeCtx := starlark.NewDict(4)
		_ = nodeCtx.SetKey(starlark.String("id"), starlark.String(n.ID))
		_ = nodeCtx.SetKey(starlark.String("address"), starlark.String(n.Address))
		_ = nodeCtx.SetKey(starlark.String("weight"), starlark.MakeInt(int(n.InitialWeight)))

		v, err := starlark.Call(thread, fn, starlark.Tuple{nodeCtx}, nil)
		if err != nil {
			done <- err
			return
		}
		result = parseHealthResult(v)
		done <- nil
	}()

	select {
	case <-ctx.Done():
		thread.Cancel(ctx.Err().Error())
		return 100, ctx.Err()
	case err := <-done:
		if err != nil {
			return 100, err
		}
		return result.Score, nil
	case <-time.After(timeout):
		thread.Cancel("health check timeout")
		return 0, context.DeadlineExceeded
	}
}

func parseHealthResult(v starlark.Value) HealthResult {
	res := HealthResult{
		Score:  100,
		Status: "healthy",
		Labels: map[string]string{},
	}
	switch val := v.(type) {
	case starlark.Int:
		score, _ := val.Int64()
		res.Score = int32(score)
	case starlark.Float:
		res.Score = int32(val)
	case *starlark.Dict:
		if s, ok := dictGetString(val, "status"); ok {
			res.Status = s
		}
		if m, ok := dictGetString(val, "message"); ok {
			res.Message = m
		}
		if score, ok := dictGetNumber(val, "score"); ok {
			res.Score = int32(score)
		}
		if labels, ok := dictGetDict(val, "labels"); ok {
			res.Labels = labels
		}
	}
	if res.Score < 0 {
		res.Score = 0
	}
	if res.Score > 100 {
		res.Score = 100
	}
	return res
}

func dictGetString(d *starlark.Dict, key string) (string, bool) {
	v, ok, _ := d.Get(starlark.String(key))
	if !ok {
		return "", false
	}
	s, ok := starlark.AsString(v)
	return s, ok
}

func dictGetNumber(d *starlark.Dict, key string) (int64, bool) {
	v, ok, _ := d.Get(starlark.String(key))
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case starlark.Int:
		i, _ := n.Int64()
		return i, true
	case starlark.Float:
		return int64(n), true
	default:
		return 0, false
	}
}

func dictGetDict(d *starlark.Dict, key string) (map[string]string, bool) {
	v, ok, _ := d.Get(starlark.String(key))
	if !ok {
		return nil, false
	}
	dict, ok := v.(*starlark.Dict)
	if !ok {
		return nil, false
	}
	out := make(map[string]string)
	for _, item := range dict.Items() {
		k, ok := starlark.AsString(item[0])
		if !ok {
			continue
		}
		val, ok := starlark.AsString(item[1])
		if !ok {
			continue
		}
		out[k] = val
	}
	return out, true
}
