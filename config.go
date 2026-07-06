package main

import (
	"fmt"
	"os"
)

const (
	defaultBaseURL = "https://api.deepseek.com"
	// Deliberately not 127.0.0.1:11434: Raycast's built-in Local Models
	// feature treats that exact string as "use the real installed Ollama
	// app" and bypasses this server entirely (see docs/adr/0003).
	defaultListenAddr = "127.0.0.1:11435"
)

type config struct {
	apiKey     string
	baseURL    string
	listenAddr string
}

func loadConfig() (config, error) {
	cfg := config{
		apiKey:     os.Getenv("DEEPSEEK_API_KEY"),
		baseURL:    os.Getenv("DEEPSEEK_BASE_URL"),
		listenAddr: os.Getenv("OLLAMA_HOST"),
	}
	if cfg.apiKey == "" {
		return config{}, fmt.Errorf("DEEPSEEK_API_KEY is required")
	}
	if cfg.baseURL == "" {
		cfg.baseURL = defaultBaseURL
	}
	if cfg.listenAddr == "" {
		cfg.listenAddr = defaultListenAddr
	}
	return cfg, nil
}
