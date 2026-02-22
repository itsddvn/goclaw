package cmd

import "fmt"

// promptTTSConfig runs the TTS configuration prompts sequentially.
// Returns early on any error (e.g. user pressed Ctrl+C).
func promptTTSConfig(ttsProvider, ttsAPIKey, ttsGroupID, ttsAutoMode *string) error {
	// TTS provider
	provider, err := promptSelect("TTS Provider (Text-to-Speech for agent replies)", []SelectOption[string]{
		{"None (disabled)", "none"},
		{"OpenAI     (gpt-4o-mini-tts, alloy voice)", "openai"},
		{"ElevenLabs (high quality, multilingual)", "elevenlabs"},
		{"MiniMax    (speech-02-hd, 300+ voices)", "minimax"},
		{"Edge       (free Microsoft Edge TTS, no API key)", "edge"},
	}, 0)
	if err != nil {
		return err
	}
	*ttsProvider = provider

	if *ttsProvider == "none" {
		return nil
	}

	// TTS auto-apply mode
	autoMode, err := promptSelect("TTS auto-apply mode", []SelectOption[string]{
		{"Off         (agent can use tts tool manually)", "off"},
		{"Always      (all replies get audio)", "always"},
		{"Inbound     (only when user sends voice/audio)", "inbound"},
		{"Tagged      (only when reply has [[tts]] tag)", "tagged"},
	}, 0)
	if err != nil {
		return err
	}
	*ttsAutoMode = autoMode

	// API key (not needed for edge)
	if *ttsProvider != "edge" {
		key, err := promptPassword("TTS API Key", "Leave empty to reuse your chat provider's API key (if same provider)")
		if err != nil {
			return err
		}
		*ttsAPIKey = key
	}

	// MiniMax Group ID
	if *ttsProvider == "minimax" {
		groupID, err := promptString("MiniMax Group ID", "Only needed for MiniMax TTS", *ttsGroupID)
		if err != nil {
			return err
		}
		*ttsGroupID = groupID
	}

	fmt.Println()
	return nil
}
