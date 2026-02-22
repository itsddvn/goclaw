package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nextlevelbuilder/goclaw/internal/config"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

func doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check system environment and configuration health",
		Run: func(cmd *cobra.Command, args []string) {
			runDoctor()
		},
	}
}

func runDoctor() {
	fmt.Println("goclaw doctor")
	fmt.Printf("  Version:  0.2.0 (protocol %d)\n", protocol.ProtocolVersion)
	fmt.Printf("  OS:       %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  Go:       %s\n", runtime.Version())
	fmt.Println()

	// Config
	cfgPath := resolveConfigPath()
	fmt.Printf("  Config:   %s", cfgPath)
	if _, err := os.Stat(cfgPath); err != nil {
		fmt.Println(" (NOT FOUND)")
	} else {
		fmt.Println(" (OK)")
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Printf("  Config load error: %s\n", err)
		return
	}

	// Providers
	fmt.Println()
	fmt.Println("  Providers:")
	checkProvider("Anthropic", cfg.Providers.Anthropic.APIKey)
	checkProvider("OpenAI", cfg.Providers.OpenAI.APIKey)
	checkProvider("OpenRouter", cfg.Providers.OpenRouter.APIKey)
	checkProvider("Gemini", cfg.Providers.Gemini.APIKey)
	checkProvider("Groq", cfg.Providers.Groq.APIKey)
	checkProvider("DeepSeek", cfg.Providers.DeepSeek.APIKey)
	checkProvider("Mistral", cfg.Providers.Mistral.APIKey)
	checkProvider("XAI", cfg.Providers.XAI.APIKey)

	// Channels
	fmt.Println()
	fmt.Println("  Channels:")
	checkChannel("Telegram", cfg.Channels.Telegram.Enabled, cfg.Channels.Telegram.Token != "")
	checkChannel("Discord", cfg.Channels.Discord.Enabled, cfg.Channels.Discord.Token != "")
	checkChannel("Zalo", cfg.Channels.Zalo.Enabled, cfg.Channels.Zalo.Token != "")
	checkChannel("Feishu", cfg.Channels.Feishu.Enabled, cfg.Channels.Feishu.AppID != "")
	checkChannel("WhatsApp", cfg.Channels.WhatsApp.Enabled, cfg.Channels.WhatsApp.BridgeURL != "")

	// External tools
	fmt.Println()
	fmt.Println("  External Tools:")
	checkBinary("docker")
	checkBinary("curl")
	checkBinary("git")

	// Workspace
	fmt.Println()
	ws := config.ExpandHome(cfg.Agents.Defaults.Workspace)
	fmt.Printf("  Workspace: %s", ws)
	if _, err := os.Stat(ws); err != nil {
		fmt.Println(" (NOT FOUND)")
	} else {
		fmt.Println(" (OK)")
	}

	fmt.Println()
	fmt.Println("Doctor check complete.")
}

func checkProvider(name, apiKey string) {
	if apiKey != "" {
		maskedKey := apiKey[:4] + strings.Repeat("*", len(apiKey)-8) + apiKey[len(apiKey)-4:]
		fmt.Printf("    %-12s %s\n", name+":", maskedKey)
	} else {
		fmt.Printf("    %-12s (not configured)\n", name+":")
	}
}

func checkChannel(name string, enabled, hasCredentials bool) {
	status := "disabled"
	if enabled && hasCredentials {
		status = "enabled"
	} else if enabled {
		status = "enabled (missing credentials)"
	}
	fmt.Printf("    %-12s %s\n", name+":", status)
}

func checkBinary(name string) {
	path, err := exec.LookPath(name)
	if err != nil {
		fmt.Printf("    %-12s NOT FOUND\n", name+":")
	} else {
		fmt.Printf("    %-12s %s\n", name+":", path)
	}
}
