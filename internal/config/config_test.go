package config

import (
	"bytes"
	"log"
	"strings"
	"testing"
	"time"
)

var configEnvKeys = []string{
	"HTTP_ADDR",
	"HTTP_READ_TIMEOUT",
	"HTTP_WRITE_TIMEOUT",
	"HTTP_IDLE_TIMEOUT",
	"HTTP_SHUTDOWN_TIMEOUT",
	"DATABASE_URL",
	"DB_MAX_CONNS",
	"DB_MIN_CONNS",
	"DB_MAX_CONN_LIFETIME",
	"DB_MAX_CONN_IDLE_TIME",
	"JWT_SECRET",
	"EVENT_PRODUCER",
}

func TestLoadUsesDefaultsForOptionalValues(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/company")
	t.Setenv("JWT_SECRET", "secret")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.HTTP.Addr != ":8080" {
		t.Fatalf("expected default HTTP addr, got %q", cfg.HTTP.Addr)
	}
	if cfg.HTTP.ReadTimeout != 5*time.Second {
		t.Fatalf("expected default read timeout, got %s", cfg.HTTP.ReadTimeout)
	}
	if cfg.HTTP.WriteTimeout != 10*time.Second {
		t.Fatalf("expected default write timeout, got %s", cfg.HTTP.WriteTimeout)
	}
	if cfg.HTTP.IdleTimeout != 60*time.Second {
		t.Fatalf("expected default idle timeout, got %s", cfg.HTTP.IdleTimeout)
	}
	if cfg.HTTP.ShutdownTimeout != 10*time.Second {
		t.Fatalf("expected default shutdown timeout, got %s", cfg.HTTP.ShutdownTimeout)
	}
	if cfg.Database.URL != "postgres://user:pass@localhost:5432/company" {
		t.Fatalf("expected configured database URL, got %q", cfg.Database.URL)
	}
	if cfg.Database.MaxConns != 10 {
		t.Fatalf("expected default max conns, got %d", cfg.Database.MaxConns)
	}
	if cfg.Database.MinConns != 1 {
		t.Fatalf("expected default min conns, got %d", cfg.Database.MinConns)
	}
	if cfg.Database.MaxConnLifetime != time.Hour {
		t.Fatalf("expected default max conn lifetime, got %s", cfg.Database.MaxConnLifetime)
	}
	if cfg.Database.MaxConnIdleTime != 30*time.Minute {
		t.Fatalf("expected default max conn idle time, got %s", cfg.Database.MaxConnIdleTime)
	}
	if cfg.Auth.JWTSecret != "secret" {
		t.Fatalf("expected configured JWT secret, got %q", cfg.Auth.JWTSecret)
	}
	if cfg.Events.Producer != "log" {
		t.Fatalf("expected default event producer, got %q", cfg.Events.Producer)
	}
}

func TestLoadUsesEnvironmentOverrides(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("HTTP_ADDR", "127.0.0.1:9090")
	t.Setenv("HTTP_READ_TIMEOUT", "1s")
	t.Setenv("HTTP_WRITE_TIMEOUT", "2s")
	t.Setenv("HTTP_IDLE_TIMEOUT", "3s")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "4s")
	t.Setenv("DATABASE_URL", "postgres://override")
	t.Setenv("DB_MAX_CONNS", "25")
	t.Setenv("DB_MIN_CONNS", "5")
	t.Setenv("DB_MAX_CONN_LIFETIME", "6m")
	t.Setenv("DB_MAX_CONN_IDLE_TIME", "7m")
	t.Setenv("JWT_SECRET", "override-secret")
	t.Setenv("EVENT_PRODUCER", "kafka")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.HTTP.Addr != "127.0.0.1:9090" {
		t.Fatalf("expected overridden HTTP addr, got %q", cfg.HTTP.Addr)
	}
	if cfg.HTTP.ReadTimeout != time.Second {
		t.Fatalf("expected overridden read timeout, got %s", cfg.HTTP.ReadTimeout)
	}
	if cfg.HTTP.WriteTimeout != 2*time.Second {
		t.Fatalf("expected overridden write timeout, got %s", cfg.HTTP.WriteTimeout)
	}
	if cfg.HTTP.IdleTimeout != 3*time.Second {
		t.Fatalf("expected overridden idle timeout, got %s", cfg.HTTP.IdleTimeout)
	}
	if cfg.HTTP.ShutdownTimeout != 4*time.Second {
		t.Fatalf("expected overridden shutdown timeout, got %s", cfg.HTTP.ShutdownTimeout)
	}
	if cfg.Database.URL != "postgres://override" {
		t.Fatalf("expected overridden database URL, got %q", cfg.Database.URL)
	}
	if cfg.Database.MaxConns != 25 {
		t.Fatalf("expected overridden max conns, got %d", cfg.Database.MaxConns)
	}
	if cfg.Database.MinConns != 5 {
		t.Fatalf("expected overridden min conns, got %d", cfg.Database.MinConns)
	}
	if cfg.Database.MaxConnLifetime != 6*time.Minute {
		t.Fatalf("expected overridden max conn lifetime, got %s", cfg.Database.MaxConnLifetime)
	}
	if cfg.Database.MaxConnIdleTime != 7*time.Minute {
		t.Fatalf("expected overridden max conn idle time, got %s", cfg.Database.MaxConnIdleTime)
	}
	if cfg.Auth.JWTSecret != "override-secret" {
		t.Fatalf("expected overridden JWT secret, got %q", cfg.Auth.JWTSecret)
	}
	if cfg.Events.Producer != "kafka" {
		t.Fatalf("expected overridden event producer, got %q", cfg.Events.Producer)
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("JWT_SECRET", "secret")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "DATABASE_URL is required" {
		t.Fatalf("expected DATABASE_URL error, got %v", err)
	}
}

func TestLoadRequiresJWTSecret(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://localhost")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "JWT_SECRET is required" {
		t.Fatalf("expected JWT_SECRET error, got %v", err)
	}
}

func TestLoadFallsBackForInvalidOptionalValues(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://localhost")
	t.Setenv("JWT_SECRET", "secret")
	t.Setenv("DB_MAX_CONNS", "not-an-int")
	t.Setenv("DB_MIN_CONNS", "also-not-an-int")
	t.Setenv("HTTP_READ_TIMEOUT", "not-a-duration")
	t.Setenv("DB_MAX_CONN_IDLE_TIME", "also-not-a-duration")

	logOutput := captureLogOutput(t, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}

		if cfg.Database.MaxConns != 10 {
			t.Fatalf("expected fallback max conns, got %d", cfg.Database.MaxConns)
		}
		if cfg.Database.MinConns != 1 {
			t.Fatalf("expected fallback min conns, got %d", cfg.Database.MinConns)
		}
		if cfg.HTTP.ReadTimeout != 5*time.Second {
			t.Fatalf("expected fallback read timeout, got %s", cfg.HTTP.ReadTimeout)
		}
		if cfg.Database.MaxConnIdleTime != 30*time.Minute {
			t.Fatalf("expected fallback max conn idle time, got %s", cfg.Database.MaxConnIdleTime)
		}
	})

	for _, key := range []string{"HTTP_READ_TIMEOUT", "DB_MAX_CONN_IDLE_TIME"} {
		if !strings.Contains(logOutput, "invalid duration for "+key) {
			t.Fatalf("expected log output to mention invalid duration for %s, got %q", key, logOutput)
		}
	}
}

func TestGetEnv(t *testing.T) {
	t.Setenv("CONFIG_TEST_ENV", "configured")
	if value := getEnv("CONFIG_TEST_ENV", "fallback"); value != "configured" {
		t.Fatalf("expected configured value, got %q", value)
	}

	t.Setenv("CONFIG_TEST_ENV", "")
	if value := getEnv("CONFIG_TEST_ENV", "fallback"); value != "fallback" {
		t.Fatalf("expected fallback for empty env, got %q", value)
	}
}

func TestGetInt(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		fallback int
		want     int
	}{
		{name: "empty", value: "", fallback: 10, want: 10},
		{name: "invalid", value: "invalid", fallback: 10, want: 10},
		{name: "valid", value: "42", fallback: 10, want: 42},
		{name: "negative", value: "-1", fallback: 10, want: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CONFIG_TEST_INT", tt.value)

			if got := getInt("CONFIG_TEST_INT", tt.fallback); got != tt.want {
				t.Fatalf("expected %d, got %d", tt.want, got)
			}
		})
	}
}

func TestGetDuration(t *testing.T) {
	tests := []struct {
		name          string
		value         string
		fallback      time.Duration
		want          time.Duration
		wantLogSubstr string
	}{
		{name: "empty", value: "", fallback: time.Second, want: time.Second},
		{name: "valid", value: "2m", fallback: time.Second, want: 2 * time.Minute},
		{
			name:          "invalid",
			value:         "invalid",
			fallback:      time.Second,
			want:          time.Second,
			wantLogSubstr: "invalid duration for CONFIG_TEST_DURATION",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CONFIG_TEST_DURATION", tt.value)

			var got time.Duration
			logOutput := captureLogOutput(t, func() {
				got = getDuration("CONFIG_TEST_DURATION", tt.fallback)
			})

			if got != tt.want {
				t.Fatalf("expected %s, got %s", tt.want, got)
			}
			if tt.wantLogSubstr == "" && logOutput != "" {
				t.Fatalf("expected empty log output, got %q", logOutput)
			}
			if tt.wantLogSubstr != "" && !strings.Contains(logOutput, tt.wantLogSubstr) {
				t.Fatalf("expected log output to contain %q, got %q", tt.wantLogSubstr, logOutput)
			}
		})
	}
}

func clearConfigEnv(t *testing.T) {
	t.Helper()

	for _, key := range configEnvKeys {
		t.Setenv(key, "")
	}
}

func captureLogOutput(t *testing.T, fn func()) string {
	t.Helper()

	var output bytes.Buffer
	oldOutput := log.Writer()
	oldFlags := log.Flags()
	oldPrefix := log.Prefix()
	log.SetOutput(&output)
	log.SetFlags(0)
	log.SetPrefix("")
	defer func() {
		log.SetOutput(oldOutput)
		log.SetFlags(oldFlags)
		log.SetPrefix(oldPrefix)
	}()

	fn()

	return output.String()
}
