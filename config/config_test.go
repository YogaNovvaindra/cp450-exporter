package config

import "testing"

func TestParseTargets(t *testing.T) {
	t.Parallel()

	targets, err := ParseTargets("bridge=http://10.1.1.240,client=http://10.1.1.241")
	if err != nil {
		t.Fatalf("ParseTargets() error = %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("len(targets) = %d, want 2", len(targets))
	}
	if targets[0].Name != "bridge" || targets[1].URL != "http://10.1.1.241" {
		t.Fatalf("unexpected targets: %#v", targets)
	}
}

func TestParseTargetsUsesHostname(t *testing.T) {
	t.Parallel()

	targets, err := ParseTargets("http://10.1.1.240")
	if err != nil {
		t.Fatalf("ParseTargets() error = %v", err)
	}
	if targets[0].Name != "10.1.1.240" {
		t.Fatalf("target name = %q, want 10.1.1.240", targets[0].Name)
	}
}

func TestParseTargetsRejectsDuplicates(t *testing.T) {
	t.Parallel()

	if _, err := ParseTargets("cp=http://10.1.1.240,cp=http://10.1.1.241"); err == nil {
		t.Fatal("ParseTargets() error = nil, want duplicate-name error")
	}
}
