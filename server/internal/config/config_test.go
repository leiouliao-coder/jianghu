package config

import "testing"

func TestLoad(t *testing.T) {
	tests := []struct {
		name, env, want string
		invalid         bool
	}{
		{"default", "", DefaultListenAddr, false},
		{"whitespace default", "  ", DefaultListenAddr, false},
		{"explicit LAN", "0.0.0.0:8081", "0.0.0.0:8081", false},
		{"IPv6", "[::1]:8080", "[::1]:8080", false},
		{"trim", " 127.0.0.1:9090 ", "127.0.0.1:9090", false},
		{"missing port", "localhost", "", true},
		{"named port", "localhost:http", "", true},
		{"zero port", "localhost:0", "", true},
		{"large port", "localhost:65536", "", true},
		{"negative port", "localhost:-1", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("JIANGHU_LISTEN_ADDR", tt.env)
			cfg, err := Load()
			if (err != nil) != tt.invalid {
				t.Fatalf("error=%v, invalid=%v", err, tt.invalid)
			}
			if err == nil && cfg.ListenAddr != tt.want {
				t.Fatalf("address=%q want=%q", cfg.ListenAddr, tt.want)
			}
		})
	}
}

func TestLoadDatabaseAndData(t *testing.T) {
	t.Setenv("JIANGHU_LISTEN_ADDR", "")
	t.Setenv("JIANGHU_DATABASE_URL", "")
	t.Setenv("JIANGHU_DATA_DIR", "")
	cfg, err := Load()
	if err != nil || cfg.DatabaseURL != "" || cfg.DataDir != DefaultDataDir {
		t.Fatalf("defaults: %+v %v", cfg, err)
	}
	t.Setenv("JIANGHU_DATABASE_URL", " postgres://u@h/db ")
	t.Setenv("JIANGHU_DATA_DIR", "/srv/data")
	cfg, err = Load()
	if err != nil || cfg.DatabaseURL != "postgres://u@h/db" || cfg.DataDir != "/srv/data" {
		t.Fatalf("explicit: %+v %v", cfg, err)
	}
}
