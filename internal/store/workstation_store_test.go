package store

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWorkstationSanitizedViewSummarizesCredentialsWithoutSecrets(t *testing.T) {
	sshMetadata, err := json.Marshal(SSHMetadata{
		Host:       "server.example",
		Port:       22,
		User:       "deployer",
		PrivateKey: "private-key-secret",
		Password:   "password-secret",
	})
	if err != nil {
		t.Fatalf("marshal SSH metadata: %v", err)
	}
	sshView := (&Workstation{BackendType: BackendSSH, Metadata: sshMetadata}).SanitizedView()
	if sshView.MetadataSummary["hasKey"] != true || sshView.MetadataSummary["hasPassword"] != true {
		t.Fatalf("SSH credential summary = %#v", sshView.MetadataSummary)
	}
	encodedSSH, err := json.Marshal(sshView)
	if err != nil {
		t.Fatalf("marshal SSH view: %v", err)
	}
	if string(encodedSSH) == "" || strings.Contains(string(encodedSSH), "private-key-secret") || strings.Contains(string(encodedSSH), "password-secret") {
		t.Fatalf("sanitized SSH view leaked a secret: %s", encodedSSH)
	}

	dockerMetadata, err := json.Marshal(DockerMetadata{
		SocketPath: "/var/run/docker.sock",
		Image:      "alpine:latest",
	})
	if err != nil {
		t.Fatalf("marshal Docker metadata: %v", err)
	}
	dockerView := (&Workstation{BackendType: BackendDocker, Metadata: dockerMetadata}).SanitizedView()
	if dockerView.MetadataSummary["socketPath"] != "/var/run/docker.sock" {
		t.Fatalf("Docker metadata summary = %#v", dockerView.MetadataSummary)
	}
}
