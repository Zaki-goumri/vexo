package config

import "os"

type Config struct {
	DataDir     string
	APIAddr     string
	ConsoleAddr string
	P2PAddr     string
}

func Load() *Config {
	return &Config{
		DataDir:     getenv("VEXO_DATA_DIR", "volume"),
		APIAddr:     getenv("VEXO_API_ADDR", ":9090"),
		ConsoleAddr: getenv("VEXO_CONSOLE_ADDR", ":9091"),
		P2PAddr:     getenv("VEXO_P2P_ADDR", ":3001"),
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
