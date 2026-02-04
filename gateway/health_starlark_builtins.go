package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

func MakeHttpModule(timeout time.Duration) starlark.Value {
	return starlarkstruct.FromStringDict(starlark.String("http"), starlark.StringDict{
		"get": starlark.NewBuiltin("http.get", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			var url string
			var headers *starlark.Dict
			if err := starlark.UnpackArgs("http.get", args, kwargs, "url", &url, "headers?", &headers); err != nil {
				return starlark.None, err
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			client := &http.Client{Timeout: timeout}
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return starlark.None, err
			}
			applyHeaders(req, headers)
			resp, err := client.Do(req)
			if err != nil {
				return starlark.None, err
			}
			defer resp.Body.Close()

			bodyBytes, err := readLimited(resp.Body, 1<<20)
			if err != nil {
				return starlark.None, err
			}

			result := starlark.NewDict(3)
			_ = result.SetKey(starlark.String("status"), starlark.MakeInt(resp.StatusCode))
			_ = result.SetKey(starlark.String("text"), starlark.String(string(bodyBytes)))

			var data interface{}
			if err := json.Unmarshal(bodyBytes, &data); err == nil {
				_ = result.SetKey(starlark.String("json"), convertToStarlark(data))
			} else {
				_ = result.SetKey(starlark.String("json"), starlark.None)
			}

			return result, nil
		}),
		"post_json": starlark.NewBuiltin("http.post_json", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			var url string
			var headers *starlark.Dict
			var payload starlark.Value
			if err := starlark.UnpackArgs("http.post_json", args, kwargs, "url", &url, "json", &payload, "headers?", &headers); err != nil {
				return starlark.None, err
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			goPayload := convertFromStarlark(payload)
			bodyBytes, err := json.Marshal(goPayload)
			if err != nil {
				return starlark.None, err
			}

			client := &http.Client{Timeout: timeout}
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
			if err != nil {
				return starlark.None, err
			}
			applyHeaders(req, headers)
			if req.Header.Get("Content-Type") == "" {
				req.Header.Set("Content-Type", "application/json")
			}

			resp, err := client.Do(req)
			if err != nil {
				return starlark.None, err
			}
			defer resp.Body.Close()

			respBytes, err := readLimited(resp.Body, 1<<20)
			if err != nil {
				return starlark.None, err
			}

			result := starlark.NewDict(3)
			_ = result.SetKey(starlark.String("status"), starlark.MakeInt(resp.StatusCode))
			_ = result.SetKey(starlark.String("text"), starlark.String(string(respBytes)))

			var data interface{}
			if err := json.Unmarshal(respBytes, &data); err == nil {
				_ = result.SetKey(starlark.String("json"), convertToStarlark(data))
			} else {
				_ = result.SetKey(starlark.String("json"), starlark.None)
			}

			return result, nil
		}),
		"get_json": starlark.NewBuiltin("http.get_json", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			var url string
			var headers *starlark.Dict
			if err := starlark.UnpackArgs("http.get_json", args, kwargs, "url", &url, "headers?", &headers); err != nil {
				return starlark.None, err
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			client := &http.Client{Timeout: timeout}
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return starlark.None, err
			}
			applyHeaders(req, headers)
			resp, err := client.Do(req)
			if err != nil {
				return starlark.None, err
			}
			defer resp.Body.Close()

			var data interface{}
			limited := io.LimitReader(resp.Body, 1<<20)
			if err := json.NewDecoder(limited).Decode(&data); err != nil {
				return starlark.None, fmt.Errorf("http.get_json: %w", err)
			}
			return convertToStarlark(data), nil
		}),
	})
}

func MakeLogModule() starlark.Value {
	return starlarkstruct.FromStringDict(starlark.String("log"), starlark.StringDict{
		"info": starlark.NewBuiltin("log.info", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			var msg string
			if err := starlark.UnpackArgs("log.info", args, kwargs, "msg", &msg); err != nil {
				return starlark.None, err
			}
			Infof("%s", msg)
			return starlark.None, nil
		}),
		"warn": starlark.NewBuiltin("log.warn", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			var msg string
			if err := starlark.UnpackArgs("log.warn", args, kwargs, "msg", &msg); err != nil {
				return starlark.None, err
			}
			Warnf("%s", msg)
			return starlark.None, nil
		}),
		"error": starlark.NewBuiltin("log.error", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			var msg string
			if err := starlark.UnpackArgs("log.error", args, kwargs, "msg", &msg); err != nil {
				return starlark.None, err
			}
			Errorf("%s", msg)
			return starlark.None, nil
		}),
	})
}

func applyHeaders(req *http.Request, headers *starlark.Dict) {
	if headers == nil {
		return
	}
	for _, item := range headers.Items() {
		k, ok := starlark.AsString(item[0])
		if !ok {
			continue
		}
		v, ok := starlark.AsString(item[1])
		if !ok {
			continue
		}
		req.Header.Set(k, v)
	}
}

func convertFromStarlark(v starlark.Value) interface{} {
	switch val := v.(type) {
	case starlark.NoneType:
		return nil
	case starlark.Bool:
		return bool(val)
	case starlark.Int:
		i, _ := val.Int64()
		return i
	case starlark.Float:
		return float64(val)
	case starlark.String:
		return string(val)
	case *starlark.Dict:
		out := make(map[string]interface{})
		for _, item := range val.Items() {
			k, ok := starlark.AsString(item[0])
			if !ok {
				continue
			}
			out[k] = convertFromStarlark(item[1])
		}
		return out
	case *starlark.List:
		list := make([]interface{}, val.Len())
		it := val.Iterate()
		defer it.Done()
		var item starlark.Value
		i := 0
		for it.Next(&item) {
			list[i] = convertFromStarlark(item)
			i++
		}
		return list
	default:
		return fmt.Sprintf("%v", val)
	}
}

func readLimited(r io.Reader, limit int64) ([]byte, error) {
	limited := io.LimitReader(r, limit)
	return io.ReadAll(limited)
}

func convertToStarlark(v interface{}) starlark.Value {
	switch x := v.(type) {
	case nil:
		return starlark.None
	case bool:
		return starlark.Bool(x)
	case float64:
		return starlark.Float(x)
	case string:
		return starlark.String(x)
	case map[string]interface{}:
		dict := starlark.NewDict(len(x))
		for k, v := range x {
			_ = dict.SetKey(starlark.String(k), convertToStarlark(v))
		}
		return dict
	case []interface{}:
		list := make([]starlark.Value, len(x))
		for i, v := range x {
			list[i] = convertToStarlark(v)
		}
		return starlark.NewList(list)
	default:
		return starlark.String(fmt.Sprintf("%v", x))
	}
}
