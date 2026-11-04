package runner

import "testing"

func TestValidateEnvAssignments(t *testing.T) {
	t.Parallel()

	valid := []string{
		"NAME=value",
		"_NAME=",
		"NAME=value=with=equals",
		"NAME=value with spaces",
		"NAME=_unset_",
	}
	if err := ValidateEnvAssignments(valid); err != nil {
		t.Fatalf("ValidateEnvAssignments() error = %v", err)
	}

	for _, assignment := range []string{"NAME", "=value", "1NAME=value", "BAD-NAME=value", "NAME value=x"} {
		if err := ValidateEnvAssignments([]string{assignment}); err == nil {
			t.Errorf("ValidateEnvAssignments(%q) returned nil error", assignment)
		}
	}
}

func TestBuildRemoteCommandWithEnv(t *testing.T) {
	t.Parallel()

	got := buildRemoteCommand(
		"/remote/path with spaces",
		[]string{"printf", "%s", "hello world"},
		[]string{"TOKEN=it's secret", "EMPTY=", "COMPLEX=a=b", "REMOVE=_unset_"},
	)
	want := `bash -l -c 'cd '\''/remote/path with spaces'\'' && env -u '\''REMOVE'\'' '\''TOKEN=it'\''\'\'''\''s secret'\'' '\''EMPTY='\'' '\''COMPLEX=a=b'\'' '\''printf'\'' '\''%s'\'' '\''hello world'\'''`

	if got != want {
		t.Fatalf("buildRemoteCommand() =\n%s\nwant:\n%s", got, want)
	}
}

func TestBuildRemoteCommandUsesLastEnvAssignment(t *testing.T) {
	t.Parallel()

	got := buildRemoteCommand(
		"/remote",
		[]string{"command"},
		[]string{"REMOVE=first", "KEEP=old", "REMOVE=_unset_", "KEEP=new", "RESTORE=_unset_", "RESTORE=last"},
	)
	want := `bash -l -c 'cd '\''/remote'\'' && env -u '\''REMOVE'\'' '\''KEEP=new'\'' '\''RESTORE=last'\'' '\''command'\'''`

	if got != want {
		t.Fatalf("buildRemoteCommand() =\n%s\nwant:\n%s", got, want)
	}
}
