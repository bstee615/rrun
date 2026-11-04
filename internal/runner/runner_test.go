package runner

import (
	"strings"
	"testing"
)

func TestValidateEnv(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		env     []string
		wantErr string
	}{
		{name: "valid", env: []string{"MODE=train", "_TOKEN=", "PATH=/opt/bin:$PATH", "CONFIG=a=b"}},
		{name: "missing equals", env: []string{"MODE"}, wantErr: "expected NAME=VALUE"},
		{name: "empty name", env: []string{"=value"}, wantErr: "invalid environment variable name"},
		{name: "invalid name", env: []string{"BAD-NAME=value"}, wantErr: "invalid environment variable name"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateEnv(tt.env)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateEnv() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateEnv() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestRemoteCommandIncludesEscapedEnvironment(t *testing.T) {
	t.Parallel()

	got, err := remoteCommand(
		"/remote/project dir",
		[]string{"python", "train.py", "--label", "it's ready"},
		[]string{"HOME=_unset_", "MODE=fast run", "CONFIG=a=b"},
	)
	if err != nil {
		t.Fatalf("remoteCommand() error = %v", err)
	}

	for _, want := range []string{
		"cd '\\''/remote/project dir'\\''",
		"unset '\\''HOME'\\''",
		"export '\\''MODE=fast run'\\''",
		"export '\\''CONFIG=a=b'\\''",
		"'\\''python'\\'' '\\''train.py'\\''",
		"'\\''it'\\''\\'\\'''\\''s ready'\\''",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("remoteCommand() = %q, want substring %q", got, want)
		}
	}
}
