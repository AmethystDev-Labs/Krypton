package gateway

import (
	"encoding/json"
	"net/http"
	"os"
)

type AdminHandler struct {
	cfgPath  string
	balancer *Balancer
}

func NewAdminHandler(cfgPath string, balancer *Balancer) *AdminHandler {
	return &AdminHandler{
		cfgPath:  cfgPath,
		balancer: balancer,
	}
}

func (h *AdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.balancer.config.Gateway.AdminAPIEnabled {
		http.NotFound(w, r)
		return
	}
	token := h.balancer.config.Gateway.AdminAPIToken
	if token == "" {
		http.Error(w, "admin token required", http.StatusForbidden)
		return
	}
	if r.Header.Get("X-Krypton-Token") != token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.URL.Path {
	case "/.krypton/health":
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	case "/.krypton/reload/config":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := h.reloadConfig(); err != nil {
			Errorf("admin reload config err=%v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"status": "error", "message": err.Error()})
			return
		}
		Infof("admin reload config ok")
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	case "/.krypton/reload/scripts":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := h.validateScripts(); err != nil {
			Errorf("admin reload scripts err=%v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"status": "error", "message": err.Error()})
			return
		}
		Infof("admin reload scripts ok")
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	default:
		http.NotFound(w, r)
		return
	}
}

func (h *AdminHandler) reloadConfig() error {
	cfg, err := LoadConfig(h.cfgPath)
	if err != nil {
		return err
	}
	return h.balancer.ApplyConfig(cfg)
}

func (h *AdminHandler) validateScripts() error {
	cfg := h.balancer.config
	if cfg.Gateway.HealthCheckDefault.Script != "" {
		if _, err := os.Stat(cfg.Gateway.HealthCheckDefault.Script); err != nil {
			return err
		}
	}
	if cfg.Gateway.TriggerScript != "" {
		if _, err := os.Stat(cfg.Gateway.TriggerScript); err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, code int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
