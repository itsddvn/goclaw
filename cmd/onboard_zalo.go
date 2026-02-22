package cmd

import "fmt"

// promptZaloConfig runs the Zalo OA Bot configuration prompts sequentially.
func promptZaloConfig(zaloToken, zaloDMPolicy *string) error {
	fmt.Println("\n  ── Zalo OA Configuration ──")

	val, err := promptPassword("Zalo Bot API Token", "From the Zalo OA developer dashboard")
	if err != nil {
		return err
	}
	if val != "" {
		*zaloToken = val
	}

	// DM Policy
	policy, err := promptSelect("Zalo DM Policy — How to handle direct messages", []SelectOption[string]{
		{"Pairing (require approval code)", "pairing"},
		{"Open (anyone can chat)", "open"},
		{"Allowlist (only allow_from IDs)", "allowlist"},
		{"Disabled (reject all DMs)", "disabled"},
	}, 0)
	if err != nil {
		return err
	}
	*zaloDMPolicy = policy

	return nil
}
