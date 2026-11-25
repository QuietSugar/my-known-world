package bitwarden

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// AutoDetectConfig 自动探测 Bitwarden 配置：
// 1. 优先从环境变量读取
// 2. 其次尝试从宿主机已有的 hall-ops.db 自动继承配置，无缝平滑过渡
func AutoDetectConfig() Config {
	cfg := Config{
		ServerURL:      os.Getenv("BITWARDEN_SERVER_URL"),
		ClientID:       os.Getenv("BITWARDEN_CLIENT_ID"),
		ClientSecret:   os.Getenv("BITWARDEN_CLIENT_SECRET"),
		MasterPassword: os.Getenv("BITWARDEN_MASTER_PASSWORD"),
	}
	if cfg.ServerURL == "" {
		cfg.ServerURL = os.Getenv("VAULTWARDEN_URL")
	}

	if cfg.Enabled() && cfg.ClientID != "" && cfg.ClientSecret != "" {
		return cfg
	}

	// 尝试从 hall-ops.db 自动读取已有的配置 (使用系统已有的 sqlite3 命令，避免引入 CGO)
	home, _ := os.UserHomeDir()
	candidatePaths := []string{
		os.Getenv("HALL_OPS_DB"),
		filepath.Join(home, "deploy/hall-ops/data/hall-ops.db"),
		"/home/dev/deploy/hall-ops/data/hall-ops.db",
	}

	for _, p := range candidatePaths {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			if loaded := loadFromCLI(p); loaded.Enabled() {
				if cfg.ServerURL == "" {
					cfg.ServerURL = loaded.ServerURL
				}
				if cfg.ClientID == "" {
					cfg.ClientID = loaded.ClientID
				}
				if cfg.ClientSecret == "" {
					cfg.ClientSecret = loaded.ClientSecret
				}
				if cfg.MasterPassword == "" {
					cfg.MasterPassword = loaded.MasterPassword
				}
				return cfg
			}
		}
	}

	return cfg
}

func loadFromCLI(dbPath string) Config {
	cmd := exec.Command("sqlite3", dbPath, "SELECT key, value FROM app_kv WHERE app_name = 'bitwarden_search' AND key LIKE '_config/%';")
	out, err := cmd.Output()
	if err != nil {
		return Config{}
	}

	var cfg Config
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		k := parts[0]
		rawVal := parts[1]
		var strVal string
		if err := json.Unmarshal([]byte(rawVal), &strVal); err != nil {
			strVal = strings.Trim(rawVal, "\"")
		}
		switch {
		case strings.HasSuffix(k, "/server_url"):
			cfg.ServerURL = strVal
		case strings.HasSuffix(k, "/client_id"):
			cfg.ClientID = strVal
		case strings.HasSuffix(k, "/client_secret"):
			cfg.ClientSecret = strVal
		case strings.HasSuffix(k, "/master_password"):
			cfg.MasterPassword = strVal
		}
	}
	return cfg
}

