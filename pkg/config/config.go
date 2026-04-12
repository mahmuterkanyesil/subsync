package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	//Media
	WatchDirs []string

	//Redis
	RedisURL string

	//Database
	StateDBPath string
	ProgressDir string

	//API
	APIPort string

	//Worker
	BatchSize int
	ScanIntervalSec int
	EmbedIntervalSec int
	WorkerConcurrency int
}

func Load() *Config {
	return &Config{
		WatchDirs:         strings.Split(getEnv("WATCH_DIRS", "/tv:/movies"), string(os.PathListSeparator)),
        RedisURL:          getEnv("REDIS_URL", "redis://localhost:6379/0"),
        StateDBPath:       getEnv("STATE_DB_PATH", "./state.db"),
        ProgressDir:       getEnv("PROGRESS_DIR", "./progress"),
        APIPort:           getEnv("API_PORT", "8080"),
        BatchSize:         getEnvInt("BATCH_SIZE", 500),
        ScanIntervalSec:   getEnvInt("SCAN_INTERVAL_SEC", 30),
        EmbedIntervalSec:  getEnvInt("EMBED_INTERVAL_SEC", 600),
        WorkerConcurrency: getEnvInt("WORKER_CONCURRENCY", 2),
	}
}

func getEnv(key string, defaultValue string) string {
	if val:= os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if val:= os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultValue
}
