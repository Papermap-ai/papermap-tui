package workspace

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/papermap/papermap-tui/internal/api"
)

func TestRequiredString(t *testing.T) {
	t.Parallel()

	check := requiredString("name", 5)
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"", true},
		{"   ", true},
		{"abc", false},
		{"abcdef", true},
	}
	for _, c := range cases {
		err := check(c.in)
		if (err != nil) != c.wantErr {
			t.Fatalf("requiredString(%q) err=%v wantErr=%v", c.in, err, c.wantErr)
		}
	}
}

func TestOptionalPort(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in      string
		wantErr bool
	}{
		{"", false},
		{"5432", false},
		{"0", true},
		{"-1", true},
		{"99999", true},
		{"abc", true},
	}
	for _, c := range cases {
		err := optionalPort(c.in)
		if (err != nil) != c.wantErr {
			t.Fatalf("optionalPort(%q) err=%v wantErr=%v", c.in, err, c.wantErr)
		}
	}
}

func TestResolvePort(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		enum  string
		want  int
		err   bool
	}{
		{"", "POSTGRES", 5432, false},
		{"", "MYSQL", 3306, false},
		{"", "MONGODB", 27017, false},
		{"", "SUPABASE", 5432, false},
		{"6543", "SUPABASE", 6543, false},
		{"abc", "POSTGRES", 0, true},
		{"", "ORACLE", 0, true},
	}
	for _, c := range cases {
		got, err := resolvePort(c.input, c.enum)
		if (err != nil) != c.err {
			t.Fatalf("resolvePort(%q, %q) err=%v wantErr=%v", c.input, c.enum, err, c.err)
		}
		if got != c.want {
			t.Fatalf("resolvePort(%q, %q) = %d, want %d", c.input, c.enum, got, c.want)
		}
	}
}

func TestRunListWith_Empty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var buf bytes.Buffer
	err := runListWith(context.Background(), &buf, ListDeps{
		List: func(context.Context) ([]api.WorkspaceSummary, error) { return nil, nil },
	})
	if err != nil {
		t.Fatalf("runListWith err: %v", err)
	}
	if !strings.Contains(buf.String(), "No workspaces yet") {
		t.Fatalf("expected empty message, got %q", buf.String())
	}
}

func TestRunListWith_Rows(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var buf bytes.Buffer
	err := runListWith(context.Background(), &buf, ListDeps{
		List: func(context.Context) ([]api.WorkspaceSummary, error) {
			return []api.WorkspaceSummary{
				{WorkspaceID: "ws_1", Name: "Acme", WorkspaceType: "POSTGRES"},
				{WorkspaceID: "ws_2", Name: "", WorkspaceType: "", IsUnified: true},
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("runListWith err: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"ID", "NAME", "ws_1", "Acme", "POSTGRES", "ws_2", "(unnamed)", "yes"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRunListWith_Error(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var buf bytes.Buffer
	err := runListWith(context.Background(), &buf, ListDeps{
		List: func(context.Context) ([]api.WorkspaceSummary, error) {
			return nil, errors.New("boom")
		},
	})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestDatabaseInputFromForm(t *testing.T) {
	t.Parallel()

	got := DatabaseInputFromForm("POSTGRES", "  db.example.com ", 5432, " app ", " user ", " s3cret ")
	if got.DatabaseType != "POSTGRES" {
		t.Fatalf("type = %q", got.DatabaseType)
	}
	if got.Host != "db.example.com" {
		t.Fatalf("host = %q", got.Host)
	}
	if got.Port != 5432 {
		t.Fatalf("port = %d", got.Port)
	}
	if got.Name != "app" || got.UserName != "user" {
		t.Fatalf("name/user trimmed wrong: %+v", got)
	}
	// Password must NOT be trimmed; users may have leading/trailing spaces.
	if got.Password != " s3cret " {
		t.Fatalf("password trimmed unexpectedly: %q", got.Password)
	}
}
