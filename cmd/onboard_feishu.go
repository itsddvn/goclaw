package cmd

import "fmt"

// promptFeishuConfig runs the Feishu/Lark configuration prompts sequentially.
func promptFeishuConfig(appID, appSecret, domain, connMode *string) error {
	fmt.Println("\n  ── Feishu / Lark Configuration ──")

	val, err := promptString("Feishu App ID", "From the Feishu/Lark Open Platform console", *appID)
	if err != nil {
		return err
	}
	*appID = val

	val, err = promptPassword("Feishu App Secret", "Keep this secret safe")
	if err != nil {
		return err
	}
	if val != "" {
		*appSecret = val
	}

	// API Domain
	d, err := promptSelect("API Domain", []SelectOption[string]{
		{"Lark Global (open.larksuite.com)", "lark"},
		{"Feishu China (open.feishu.cn)", "feishu"},
	}, 0)
	if err != nil {
		return err
	}
	*domain = d

	// Connection Mode
	cm, err := promptSelect("Connection Mode", []SelectOption[string]{
		{"WebSocket (recommended, no public URL needed)", "websocket"},
		{"Webhook (requires public URL)", "webhook"},
	}, 0)
	if err != nil {
		return err
	}
	*connMode = cm

	return nil
}
