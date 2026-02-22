package tools

import "testing"

func TestScrubCredentials_OpenAI(t *testing.T) {
	input := "Found key: sk-abcdefghijklmnopqrstuvwxyz1234567890 in env"
	got := ScrubCredentials(input)
	if got != "Found key: [REDACTED] in env" {
		t.Errorf("OpenAI key not scrubbed: %s", got)
	}
}

func TestScrubCredentials_Anthropic(t *testing.T) {
	input := "key=sk-ant-abc123-def456-ghi789-jkl012"
	got := ScrubCredentials(input)
	if got != "key=[REDACTED]" {
		t.Errorf("Anthropic key not scrubbed: %s", got)
	}
}

func TestScrubCredentials_GitHub(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"ghp", "token ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij done"},
		{"gho", "token gho_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij done"},
		{"ghu", "token ghu_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij done"},
		{"ghs", "token ghs_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij done"},
		{"ghr", "token ghr_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij done"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScrubCredentials(tt.input)
			want := "token [REDACTED] done"
			if got != want {
				t.Errorf("GitHub %s not scrubbed: got %q, want %q", tt.name, got, want)
			}
		})
	}
}

func TestScrubCredentials_AWS(t *testing.T) {
	input := "aws_key: AKIAIOSFODNN7EXAMPLE"
	got := ScrubCredentials(input)
	if got == input {
		t.Errorf("AWS key not scrubbed: %s", got)
	}
}

func TestScrubCredentials_GenericKeyValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"api_key", "api_key=supersecretvalue123"},
		{"token", "token: mysecrettoken12345"},
		{"password", "password=MyStr0ngP@ssword!"},
		{"bearer", "bearer: eyJhbGciOiJIUzI1NiJ9.abc"},
		{"authorization", "authorization=eyJhbGciOiJIUzI1NiJ9abcdef"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScrubCredentials(tt.input)
			if got == tt.input {
				t.Errorf("generic pattern %q not scrubbed: %s", tt.name, got)
			}
		})
	}
}

func TestScrubCredentials_NoFalsePositive(t *testing.T) {
	inputs := []string{
		"hello world",
		"sk-short",       // too short for OpenAI pattern
		"ghp_tooshort",   // too short for GitHub pattern
		"normal text with no secrets",
		"AKIA1234",       // too short for AWS (needs 16 chars after AKIA)
	}
	for _, input := range inputs {
		got := ScrubCredentials(input)
		if got != input {
			t.Errorf("false positive on %q: got %q", input, got)
		}
	}
}

func TestScrubCredentials_MultiplePatterns(t *testing.T) {
	input := "openai=sk-abcdefghijklmnopqrstuvwxyz, github=ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij"
	got := ScrubCredentials(input)
	if got == input {
		t.Errorf("multiple patterns not scrubbed: %s", got)
	}
}

func TestScrubCredentials_EmptyString(t *testing.T) {
	got := ScrubCredentials("")
	if got != "" {
		t.Errorf("empty string changed: %q", got)
	}
}
