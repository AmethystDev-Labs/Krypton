package gateway

import (
	"context"
	"errors"
	"os"
	"time"

	"go.starlark.net/starlark"
)

type TriggerResult struct {
	Score   *int32
	Penalty *int32
	Reward  *int32
	Retry   *bool
	Message string
}

func runTrigger(ctx context.Context, cfg *Config, node *Node, req *triggerRequest, resp *triggerResponse) (*TriggerResult, error) {
	if cfg.Gateway.TriggerScript == "" {
		return nil, nil
	}
	if _, err := os.Stat(cfg.Gateway.TriggerScript); err != nil {
		return nil, err
	}

	timeout := cfg.Gateway.TriggerTimeout.Duration
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	thread := &starlark.Thread{Name: "trigger_check"}
	done := make(chan error, 1)
	var result *TriggerResult

	go func() {
		predeclared := starlark.StringDict{
			"log":    MakeLogModule(),
			"config": MakeConfigModule(cfg),
		}
		globals, err := starlark.ExecFile(thread, cfg.Gateway.TriggerScript, nil, predeclared)
		if err != nil {
			done <- err
			return
		}
		fn, ok := globals["trigger"]
		if !ok {
			done <- errors.New("trigger() not found")
			return
		}
		ctxDict := starlark.NewDict(3)
		_ = ctxDict.SetKey(starlark.String("node"), nodeToDict(node))
		_ = ctxDict.SetKey(starlark.String("request"), req.toDict())
		_ = ctxDict.SetKey(starlark.String("response"), resp.toDict())

		v, err := starlark.Call(thread, fn, starlark.Tuple{ctxDict}, nil)
		if err != nil {
			done <- err
			return
		}
		result = parseTriggerResult(v)
		done <- nil
	}()

	select {
	case <-ctx.Done():
		thread.Cancel(ctx.Err().Error())
		return nil, ctx.Err()
	case err := <-done:
		if err != nil {
			return nil, err
		}
		return result, nil
	case <-time.After(timeout):
		thread.Cancel("trigger timeout")
		return nil, context.DeadlineExceeded
	}
}

type triggerRequest struct {
	Method  string
	Path    string
	Headers map[string]string
}

type triggerResponse struct {
	Status int
	Body   string
}

func (t *triggerRequest) toDict() *starlark.Dict {
	d := starlark.NewDict(3)
	_ = d.SetKey(starlark.String("method"), starlark.String(t.Method))
	_ = d.SetKey(starlark.String("path"), starlark.String(t.Path))
	_ = d.SetKey(starlark.String("headers"), mapToDict(t.Headers))
	return d
}

func (t *triggerResponse) toDict() *starlark.Dict {
	d := starlark.NewDict(2)
	_ = d.SetKey(starlark.String("status"), starlark.MakeInt(t.Status))
	_ = d.SetKey(starlark.String("body"), starlark.String(t.Body))
	return d
}

func nodeToDict(n *Node) *starlark.Dict {
	d := starlark.NewDict(3)
	_ = d.SetKey(starlark.String("id"), starlark.String(n.ID))
	_ = d.SetKey(starlark.String("address"), starlark.String(n.Address))
	_ = d.SetKey(starlark.String("weight"), starlark.MakeInt(int(n.InitialWeight)))
	return d
}

func mapToDict(m map[string]string) *starlark.Dict {
	d := starlark.NewDict(len(m))
	for k, v := range m {
		_ = d.SetKey(starlark.String(k), starlark.String(v))
	}
	return d
}

func parseTriggerResult(v starlark.Value) *TriggerResult {
	if v == nil || v == starlark.None {
		return nil
	}
	res := &TriggerResult{}
	switch val := v.(type) {
	case starlark.Int:
		i, _ := val.Int64()
		iv := int32(i)
		res.Penalty = &iv
	case *starlark.Dict:
		if score, ok := dictGetNumber(val, "score"); ok {
			iv := int32(score)
			res.Score = &iv
		}
		if p, ok := dictGetNumber(val, "penalty"); ok {
			iv := int32(p)
			res.Penalty = &iv
		}
		if r, ok := dictGetNumber(val, "reward"); ok {
			iv := int32(r)
			res.Reward = &iv
		}
		if rb, ok := dictGetBool(val, "retry"); ok {
			res.Retry = &rb
		}
		if msg, ok := dictGetString(val, "message"); ok {
			res.Message = msg
		}
	}
	return res
}

func dictGetBool(d *starlark.Dict, key string) (bool, bool) {
	v, ok, _ := d.Get(starlark.String(key))
	if !ok {
		return false, false
	}
	switch b := v.(type) {
	case starlark.Bool:
		return bool(b), true
	default:
		return false, false
	}
}
