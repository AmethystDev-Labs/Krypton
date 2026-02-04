package main

import (
	"context"
	"net/http"
	"time"

	"krypton/gateway"
)

func main() {
	cfg, err := gateway.LoadConfig("config.toml")
	if err != nil {
		gateway.Errorf("load config: %v", err)
		return
	}

	gateway.SetLogLevelFromEnv()

	balancer, err := gateway.NewBalancer(cfg)
	if err != nil {
		gateway.Errorf("init balancer: %v", err)
		return
	}

	health := gateway.NewHealthChecker(cfg, balancer)
	go health.Run(context.Background())

	var handler http.Handler = balancer
	if cfg.Gateway.AdminAPIEnabled {
		if cfg.Gateway.AdminAPIToken == "" {
			gateway.Errorf("admin_api_enabled requires admin_api_token")
			return
		}
		mux := http.NewServeMux()
		mux.Handle("/", balancer)
		mux.Handle("/.krypton/", gateway.NewAdminHandler("config.toml", balancer))
		handler = mux
	}

	server := &http.Server{
		Addr:              cfg.Gateway.Listen,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.Gateway.ReadTimeout.Duration,
		WriteTimeout:      cfg.Gateway.WriteTimeout.Duration,
		IdleTimeout:       cfg.Gateway.IdleTimeout.Duration,
	}

	gateway.Infof("krypton listening on %s", cfg.Gateway.Listen)
	if err := server.ListenAndServe(); err != nil {
		gateway.Errorf("server: %v", err)
	}
}
