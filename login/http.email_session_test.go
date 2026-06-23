package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	xconfig "github.com/75912001/xlib/config"
)

func TestNormalizeEmail(t *testing.T) {
	if got, want := normalizeEmail(" User@Example.COM "), "user@example.com"; got != want {
		t.Fatalf("normalizeEmail = %q, want %q", got, want)
	}
}

func TestLoadEmailPasswordUsers(t *testing.T) {
	restoreConfigPath := setTestLoginConfig(t, `
custom:
  emailPasswordUsers:
    - email: " User@Example.COM "
      password: "secret"
`)
	defer restoreConfigPath()

	users, err := loadEmailPasswordUsers()
	if err != nil {
		t.Fatalf("loadEmailPasswordUsers: %v", err)
	}
	if got, want := users["user@example.com"], "secret"; got != want {
		t.Fatalf("password = %q, want %q", got, want)
	}
}

func TestLoadEmailPasswordUsersRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "duplicate email",
			body: `
custom:
  emailPasswordUsers:
    - email: "user@example.com"
      password: "secret1"
    - email: " USER@example.com "
      password: "secret2"
`,
		},
		{
			name: "empty password",
			body: `
custom:
  emailPasswordUsers:
    - email: "user@example.com"
      password: ""
`,
		},
		{
			name: "empty email",
			body: `
custom:
  emailPasswordUsers:
    - email: " "
      password: "secret"
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restoreConfigPath := setTestLoginConfig(t, tt.body)
			defer restoreConfigPath()

			if _, err := loadEmailPasswordUsers(); err == nil {
				t.Fatal("loadEmailPasswordUsers error is nil, want error")
			}
		})
	}
}

func TestHandleLoginEmailSessionWrongPassword(t *testing.T) {
	restoreConfigPath := setTestLoginConfig(t, `
custom:
  emailPasswordUsers:
    - email: "user@example.com"
      password: "secret"
`)
	defer restoreConfigPath()
	oldMaxBodyBytes := GCfgCustomMaxBodyBytes
	GCfgCustomMaxBodyBytes = 4096
	defer func() { GCfgCustomMaxBodyBytes = oldMaxBodyBytes }()

	req := httptest.NewRequest(http.MethodPost, "/api/login/emailSession", strings.NewReader(`{"email":"USER@example.com","password":"bad"}`))
	recorder := httptest.NewRecorder()

	handleLoginEmailSession(recorder, req)

	if got, want := recorder.Code, http.StatusUnauthorized; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}

func setTestLoginConfig(t *testing.T, body string) func() {
	t.Helper()

	file, err := os.CreateTemp(t.TempDir(), "login-*.yaml")
	if err != nil {
		t.Fatalf("create temp config: %v", err)
	}
	if _, err = file.WriteString(body); err != nil {
		_ = file.Close()
		t.Fatalf("write temp config: %v", err)
	}
	if err = file.Close(); err != nil {
		t.Fatalf("close temp config: %v", err)
	}

	oldPath := xconfig.GConfigMgr.ExecutablePath
	xconfig.GConfigMgr.ExecutablePath = file.Name()
	return func() {
		xconfig.GConfigMgr.ExecutablePath = oldPath
	}
}
