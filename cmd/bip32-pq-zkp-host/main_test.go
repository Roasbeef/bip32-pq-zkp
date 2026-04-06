package main

import "testing"

func TestParseExecuteArgsDefaultsToBIP86(t *testing.T) {
	args, err := parseExecuteArgs(nil)
	if err != nil {
		t.Fatalf("parseExecuteArgs failed: %v", err)
	}

	if !args.requireBIP86 {
		t.Fatal("expected execute default require-bip86=true")
	}
}

func TestParseExecuteArgsAllowsBIP86OptOut(t *testing.T) {
	args, err := parseExecuteArgs([]string{"--require-bip86=false"})
	if err != nil {
		t.Fatalf("parseExecuteArgs failed: %v", err)
	}

	if args.requireBIP86 {
		t.Fatal("expected execute require-bip86=false override")
	}
}

func TestParseProveArgsDefaultsToBIP86(t *testing.T) {
	args, err := parseProveArgs(nil)
	if err != nil {
		t.Fatalf("parseProveArgs failed: %v", err)
	}

	if !args.witness.requireBIP86 {
		t.Fatal("expected prove default require-bip86=true")
	}
}

func TestParseProveArgsAllowsBIP86OptOut(t *testing.T) {
	args, err := parseProveArgs([]string{"--require-bip86=false"})
	if err != nil {
		t.Fatalf("parseProveArgs failed: %v", err)
	}

	if args.witness.requireBIP86 {
		t.Fatal("expected prove require-bip86=false override")
	}
}

func TestParseVerifyArgsDefaultsToBIP86(t *testing.T) {
	args, err := parseVerifyArgs(nil)
	if err != nil {
		t.Fatalf("parseVerifyArgs failed: %v", err)
	}

	if !args.requireBIP86.set || !args.requireBIP86.value {
		t.Fatal("expected verify default require-bip86=true")
	}
}

func TestParseVerifyArgsAllowsBIP86OptOut(t *testing.T) {
	args, err := parseVerifyArgs([]string{"--require-bip86=false"})
	if err != nil {
		t.Fatalf("parseVerifyArgs failed: %v", err)
	}

	if !args.requireBIP86.set {
		t.Fatal("expected verify require-bip86 override to be recorded")
	}
	if args.requireBIP86.value {
		t.Fatal("expected verify require-bip86=false override")
	}
}
