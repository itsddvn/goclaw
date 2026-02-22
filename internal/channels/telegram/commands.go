package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
)

// handleBotCommand checks if the message is a known bot command and handles it.
// Returns true if the message was handled as a command.
func (c *Channel) handleBotCommand(ctx context.Context, chatID int64, chatIDStr, localKey, text, senderID string, isGroup, isForum bool, messageThreadID int) bool {
	if len(text) == 0 || text[0] != '/' {
		return false
	}

	// Extract command (strip @botname suffix if present)
	cmd := strings.SplitN(text, " ", 2)[0]
	cmd = strings.SplitN(cmd, "@", 2)[0]
	cmd = strings.ToLower(cmd)

	chatIDObj := tu.ID(chatID)

	// Helper: set MessageThreadID on outgoing messages for forum topics.
	// TS ref: buildTelegramThreadParams() — General topic (1) must be omitted.
	setThread := func(msg *telego.SendMessageParams) {
		sendThreadID := resolveThreadIDForSend(messageThreadID)
		if sendThreadID > 0 {
			msg.MessageThreadID = sendThreadID
		}
	}

	switch cmd {
	case "/start":
		// Don't intercept /start — let it pass through to agent loop.
		return false

	case "/help":
		helpText := "Available commands:\n" +
			"/start — Start chatting with the bot\n" +
			"/help — Show this help message\n" +
			"/reset — Reset conversation history\n" +
			"/status — Show bot status\n" +
			"\nJust send a message to chat with the AI."
		msg := tu.Message(chatIDObj, helpText)
		setThread(msg)
		c.bot.SendMessage(ctx, msg)
		return true

	case "/reset":
		// Fix: use correct PeerKind so the gateway consumer builds the right session key.
		peerKind := "direct"
		if isGroup {
			peerKind = "group"
		}
		c.Bus().PublishInbound(bus.InboundMessage{
			Channel:  c.Name(),
			SenderID: senderID,
			ChatID:   chatIDStr,
			Content:  "/reset",
			PeerKind: peerKind,
			AgentID:  c.AgentID(),
			UserID:   strings.SplitN(senderID, "|", 2)[0],
			Metadata: map[string]string{
				"command":           "reset",
				"local_key":         localKey,
				"is_forum":          fmt.Sprintf("%t", isForum),
				"message_thread_id": fmt.Sprintf("%d", messageThreadID),
			},
		})
		msg := tu.Message(chatIDObj, "Conversation history has been reset.")
		setThread(msg)
		c.bot.SendMessage(ctx, msg)
		return true

	case "/status":
		statusText := fmt.Sprintf("Bot status: Running\nChannel: Telegram\nBot: @%s", c.bot.Username())
		msg := tu.Message(chatIDObj, statusText)
		setThread(msg)
		c.bot.SendMessage(ctx, msg)
		return true
	}

	return false
}

// --- Pairing UX ---

// buildPairingReply builds the pairing reply message matching TS behavior.
func buildPairingReply(telegramUserID, code string) string {
	return fmt.Sprintf(
		"GoClaw: access not configured.\n\nYour Telegram user id: %s\n\nPairing code: %s\n\nAsk the bot owner to approve with:\n  goclaw pairing approve %s",
		telegramUserID, code, code,
	)
}

// sendPairingReply generates a pairing code and sends the reply to the user.
// Debounces: won't send another reply to the same user within 60 seconds.
func (c *Channel) sendPairingReply(ctx context.Context, chatID int64, userID, username string) {
	if c.pairingService == nil {
		return
	}

	if lastSent, ok := c.pairingReplySent.Load(userID); ok {
		if time.Since(lastSent.(time.Time)) < pairingReplyDebounce {
			slog.Debug("pairing reply debounced", "user_id", userID)
			return
		}
	}

	code, err := c.pairingService.RequestPairing(userID, c.Name(), fmt.Sprintf("%d", chatID), "default")
	if err != nil {
		slog.Debug("pairing request failed", "user_id", userID, "error", err)
		return
	}

	replyText := buildPairingReply(userID, code)
	msg := tu.Message(tu.ID(chatID), replyText)
	if _, err := c.bot.SendMessage(ctx, msg); err != nil {
		slog.Warn("failed to send pairing reply", "chat_id", chatID, "error", err)
	} else {
		c.pairingReplySent.Store(userID, time.Now())
		slog.Info("telegram pairing reply sent",
			"user_id", userID, "username", username, "code", code,
		)
	}
}

// sendGroupPairingReply generates a pairing code for a group and sends the reply.
// Debounces: won't send another reply to the same group within 60 seconds.
func (c *Channel) sendGroupPairingReply(ctx context.Context, chatID int64, chatIDStr, groupSenderID string) {
	if lastSent, ok := c.pairingReplySent.Load(chatIDStr); ok {
		if time.Since(lastSent.(time.Time)) < pairingReplyDebounce {
			return
		}
	}

	code, err := c.pairingService.RequestPairing(groupSenderID, c.Name(), chatIDStr, "default")
	if err != nil {
		slog.Debug("group pairing request failed", "chat_id", chatIDStr, "error", err)
		return
	}

	replyText := fmt.Sprintf(
		"This group is not approved yet.\n\nPairing code: %s\n\nAsk the bot owner to approve with:\n  goclaw pairing approve %s",
		code, code,
	)
	msg := tu.Message(tu.ID(chatID), replyText)
	if _, err := c.bot.SendMessage(ctx, msg); err != nil {
		slog.Warn("failed to send group pairing reply", "chat_id", chatIDStr, "error", err)
	} else {
		c.pairingReplySent.Store(chatIDStr, time.Now())
		slog.Info("telegram group pairing reply sent", "chat_id", chatIDStr, "code", code)
	}
}

// SendPairingApproved sends the approval notification to a user.
func (c *Channel) SendPairingApproved(ctx context.Context, chatID, botName string) error {
	id, err := parseChatID(chatID)
	if err != nil {
		return fmt.Errorf("invalid chat ID: %w", err)
	}
	if botName == "" {
		botName = "GoClaw"
	}

	msg := tu.Message(tu.ID(id), fmt.Sprintf("✅ %s access approved. Send a message to start chatting.", botName))
	_, err = c.bot.SendMessage(ctx, msg)
	return err
}

// SyncMenuCommands registers bot commands with Telegram via setMyCommands.
func (c *Channel) SyncMenuCommands(ctx context.Context, commands []telego.BotCommand) error {
	if err := c.bot.DeleteMyCommands(ctx, nil); err != nil {
		slog.Debug("deleteMyCommands failed (may not exist)", "error", err)
	}

	if len(commands) == 0 {
		return nil
	}

	if len(commands) > 100 {
		commands = commands[:100]
	}

	return c.bot.SetMyCommands(ctx, &telego.SetMyCommandsParams{
		Commands: commands,
	})
}

// DefaultMenuCommands returns the default bot menu commands.
func DefaultMenuCommands() []telego.BotCommand {
	return []telego.BotCommand{
		{Command: "start", Description: "Start chatting with the bot"},
		{Command: "help", Description: "Show available commands"},
		{Command: "reset", Description: "Reset conversation history"},
		{Command: "status", Description: "Show bot status"},
	}
}
