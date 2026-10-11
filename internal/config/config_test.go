package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("ADDR", "")
	t.Setenv("BASE_URL", "")
	t.Setenv("DB_PATH", "")

	got := Load()
	want := Config{Addr: ":8080", BaseURL: "http://localhost:8080", DBPath: "shortener.db"}
	if got != want {
		t.Fatalf("Load() = %+v, want %+v", got, want)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("ADDR", "127.0.0.1:9000")
	t.Setenv("BASE_URL", "https://sho.rt")
	t.Setenv("DB_PATH", "/tmp/links.db")

	got := Load()
	want := Config{Addr: "127.0.0.1:9000", BaseURL: "https://sho.rt", DBPath: "/tmp/links.db"}
	if got != want {
		t.Fatalf("Load() = %+v, want %+v", got, want)
	}
}

func TestLoadTrimsTrailingSlash(t *testing.T) {
	cases := map[string]string{
		"https://sho.rt/":   "https://sho.rt",
		"https://sho.rt//":  "https://sho.rt",
		"https://sho.rt/s/": "https://sho.rt/s",
		"/":                 DefaultBaseURL,
	}
	for in, want := range cases {
		t.Setenv("BASE_URL", in)
		if got := Load().BaseURL; got != want {
			t.Errorf("BASE_URL=%q: BaseURL = %q, want %q", in, got, want)
		}
	}
}
