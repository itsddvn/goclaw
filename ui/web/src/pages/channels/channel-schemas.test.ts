import { describe, it, expect } from "vitest";
import {
  configSchema,
  credentialsSchema,
  fieldTranslationKey,
  requiredScopes,
  resolveRequiredScopes,
} from "./channel-schemas";
import { deliveryModelKey, isDeliveryModelKey, isDeliveryProviderKey } from "./channel-delivery-provider-fields";
import { normalizeReasoningDeliveryConfig, resolveReasoningDeliveryValue } from "./reasoning-delivery-config";

describe("telegram configSchema", () => {
  const telegramConfig = configSchema["telegram"]!;

  it("uses reasoning_delivery mode instead of legacy reasoning_stream", () => {
    const reasoningDelivery = telegramConfig.find((f) => f.key === "reasoning_delivery");
    expect(reasoningDelivery).toBeDefined();
    expect(reasoningDelivery!.type).toBe("select");
    expect(reasoningDelivery!.defaultValue).toBe("streaming_only");
    expect(reasoningDelivery!.help).not.toMatch(/requires streaming/i);
    expect(reasoningDelivery!.options!.map((o) => o.value)).toEqual(["streaming_only", "always_bubbles", "off"]);
    expect(telegramConfig.find((f) => f.key === "reasoning_stream")).toBeUndefined();
  });

  it("normalizes legacy reasoning_stream into explicit reasoning_delivery", () => {
    expect(resolveReasoningDeliveryValue({ reasoning_stream: false })).toBe("off");
    expect(resolveReasoningDeliveryValue({ reasoning_stream: true })).toBe("streaming_only");
    expect(normalizeReasoningDeliveryConfig({ reasoning_delivery: "always_bubbles", reasoning_stream: false })).toEqual({
      reasoning_delivery: "always_bubbles",
    });
  });

  it("exposes independent sidecar delivery overrides", () => {
    const keys = new Set(telegramConfig.map((field) => field.key));
    expect(keys.has("chat_behavior.intermediate_replies.enabled")).toBe(true);
    expect(keys.has("chat_behavior.intermediate_replies.provider")).toBe(true);
    expect(keys.has("chat_behavior.intermediate_replies.model")).toBe(true);
    expect(keys.has("chat_behavior.quick_ack.provider")).toBe(true);
    expect(keys.has("chat_behavior.quick_ack.model")).toBe(true);
    const quickMode = telegramConfig.find((field) => field.key === "chat_behavior.quick_ack.mode")!;
    expect(quickMode.help).not.toMatch(/main LLM block reply/i);
    expect(quickMode.help).not.toMatch(/fallback/i);
    expect(quickMode.options!.map((option) => option.value)).toContain("sidecar_generated");
    expect(quickMode.options!.map((option) => option.label).join(" ")).not.toMatch(/fallback/i);
  });

  it("groups delivery provider and model fields for dropdown rendering", () => {
    expect(isDeliveryProviderKey("chat_behavior.quick_ack.provider")).toBe(true);
    expect(isDeliveryProviderKey("chat_behavior.intermediate_replies.provider")).toBe(true);
    expect(isDeliveryModelKey("chat_behavior.quick_ack.model")).toBe(true);
    expect(isDeliveryModelKey("chat_behavior.intermediate_replies.model")).toBe(true);
    expect(deliveryModelKey("chat_behavior.intermediate_replies.provider")).toBe("chat_behavior.intermediate_replies.model");
  });

  it("documents every channel behavior field for tooltip rendering", () => {
    const behaviorFields = telegramConfig.filter((field) => field.key.startsWith("chat_behavior."));
    expect(behaviorFields.length).toBeGreaterThan(0);
    for (const field of behaviorFields) {
      expect(field.help?.trim(), `${field.key} should have tooltip help`).toBeTruthy();
    }
  });

  it("exposes Telegram manager permissions as fixed multi-select actions", () => {
    const enabled = telegramConfig.find((field) => field.key === "telegram_manager.enabled");
    const allowedActions = telegramConfig.find((field) => field.key === "telegram_manager.allowed_actions");

    expect(enabled).toBeDefined();
    expect(enabled!.type).toBe("boolean");
    expect(allowedActions).toBeDefined();
    expect(allowedActions!.type).toBe("multi-select");
    expect(allowedActions!.options!.map((option) => option.value)).toEqual([
      "topic",
      "message",
      "member",
      "invite",
      "chat",
      "join_request",
    ]);
  });
});

describe("discord configSchema", () => {
  const discordConfig = configSchema["discord"]!;

  it("matches the backend pending group history default", () => {
    const historyLimit = discordConfig.find((field) => field.key === "history_limit");
    expect(historyLimit).toBeDefined();
    expect(historyLimit!.defaultValue).toBe(200);
    expect(historyLimit!.help).toMatch(/0 = disabled/i);
  });
});

describe("facebook schemas", () => {
  const facebookCredentials = credentialsSchema["facebook"]!;
  const facebookConfig = configSchema["facebook"]!;

  it("preserves the Facebook credential and Page ID payload keys", () => {
    expect(facebookCredentials.map((field) => field.key)).toEqual([
      "page_access_token",
      "app_secret",
      "verify_token",
    ]);
    expect(facebookConfig.some((field) => field.key === "page_id")).toBe(true);
  });

  it("uses channel-specific translations without changing shared-channel lookups", () => {
    const facebookAppSecret = facebookCredentials.find((field) => field.key === "app_secret")!;
    const feishuAppSecret = credentialsSchema["feishu"]!.find((field) => field.key === "app_secret")!;
    const facebookPageToken = facebookCredentials.find((field) => field.key === "page_access_token")!;
    const pancakePageToken = credentialsSchema["pancake"]!.find((field) => field.key === "page_access_token")!;

    expect(fieldTranslationKey(facebookAppSecret, "help")).toBe(
      "fieldConfig.facebook.app_secret.help",
    );
    expect(fieldTranslationKey(feishuAppSecret, "help")).toBe("fieldConfig.app_secret.help");
    expect(fieldTranslationKey(facebookPageToken, "help")).toBe(
      "fieldConfig.facebook.page_access_token.help",
    );
    expect(fieldTranslationKey(pancakePageToken, "help")).toBe(
      "fieldConfig.page_access_token.help",
    );
  });

  it("removes the unused session timeout and documents feature dependencies", () => {
    expect(facebookConfig.find((field) => field.key === "messenger_options.session_timeout")).toBeUndefined();

    const messengerAutoReply = facebookConfig.find(
      (field) => field.key === "features.messenger_auto_reply",
    )!;
    const firstInbox = facebookConfig.find((field) => field.key === "features.first_inbox")!;

    expect(messengerAutoReply.help).toMatch(/must be enabled/i);
    expect(firstInbox.disabledWhen).toEqual({
      key: "features.comment_reply",
      value: "false",
      hint: "fieldConfig.facebook.features.first_inbox.disabledHint",
    });
    expect(firstInbox.help).toMatch(/running GoClaw process/i);
  });

  it("lists the exact Messenger, comment, and use-case permissions", () => {
    expect(requiredScopes.facebook?.map((entry) => entry.scope)).toEqual([
      "pages_show_list",
      "pages_manage_metadata",
      "pages_messaging",
      "pages_manage_engagement",
      "pages_read_user_content",
      "pages_read_engagement",
      "business_management",
      "public_profile",
    ]);
  });

  it("shows comment permissions only when Comment Auto-Reply is enabled", () => {
    const alwaysRequired = [
      "pages_show_list",
      "pages_manage_metadata",
      "pages_messaging",
      "business_management",
      "public_profile",
    ];

    expect(resolveRequiredScopes("facebook").map((entry) => entry.scope)).toEqual(alwaysRequired);
    expect(resolveRequiredScopes("facebook", {
      "features.comment_reply": false,
    }).map((entry) => entry.scope)).toEqual(alwaysRequired);
    expect(resolveRequiredScopes("facebook", {
      "features.comment_reply": true,
    }).map((entry) => entry.scope)).toEqual(
      requiredScopes.facebook?.map((entry) => entry.scope),
    );
  });
});

describe("pancake configSchema", () => {
  const pancakeConfig = configSchema["pancake"]!;

  it("has a platform field", () => {
    expect(pancakeConfig).toBeDefined();
    const platformField = pancakeConfig.find((f) => f.key === "platform");
    expect(platformField).toBeDefined();
  });

  it("platform field is type select", () => {
    const platformField = pancakeConfig.find((f) => f.key === "platform")!;
    expect(platformField.type).toBe("select");
  });

  it("platform field is required", () => {
    const platformField = pancakeConfig.find((f) => f.key === "platform")!;
    expect(platformField.required).toBe(true);
  });

  it("platform options include all expected platforms", () => {
    const platformField = pancakeConfig.find((f) => f.key === "platform")!;
    const values = platformField.options!.map((o) => o.value);
    expect(values).toContain("facebook");
    expect(values).toContain("instagram");
    expect(values).toContain("tiktok");
    expect(values).toContain("line");
    expect(values).toContain("shopee");
    expect(values).toContain("lazada");
    expect(values).toContain("tokopedia");
  });

  it("platform options do NOT include natively-supported channels", () => {
    const platformField = pancakeConfig.find((f) => f.key === "platform")!;
    const values = platformField.options!.map((o) => o.value);
    expect(values).not.toContain("telegram");
    expect(values).not.toContain("zalo");
    expect(values).not.toContain("whatsapp");
    expect(values).not.toContain("zalo_oa");
  });

  it("exposes private_reply feature toggle gated on fb/ig only", () => {
    const feat = pancakeConfig.find((f) => f.key === "features.private_reply");
    expect(feat).toBeDefined();
    expect(feat!.type).toBe("boolean");
    expect(feat!.defaultValue).toBe(false);
    expect(feat!.showWhen).toMatchObject({
      key: "platform",
      value: ["facebook", "instagram"],
    });
  });

  it("exposes private_reply_message gated by the feature toggle", () => {
    const msg = pancakeConfig.find((f) => f.key === "private_reply_message");
    expect(msg).toBeDefined();
    expect(msg!.type).toBe("textarea");
    expect(msg!.showWhen).toEqual({ key: "features.private_reply", value: "true" });
  });

  it("does NOT expose removed private_reply config fields", () => {
    const removed = [
      "private_reply_mode",
      "private_reply_only",
      "private_reply_ttl_days",
      "private_reply_options.allow_post_ids",
      "private_reply_options.deny_post_ids",
    ];
    for (const key of removed) {
      expect(pancakeConfig.find((f) => f.key === key), `field ${key} should be removed`).toBeUndefined();
    }
  });

  it("has features.auto_react boolean toggle gated on platform=facebook", () => {
    const f = pancakeConfig.find((x) => x.key === "features.auto_react");
    expect(f).toBeDefined();
    expect(f!.type).toBe("boolean");
    expect(f!.defaultValue).toBe(false);
    expect(f!.showWhen).toEqual({ key: "platform", value: "facebook" });
  });

  it.each([
    "auto_react_options.allow_post_ids",
    "auto_react_options.deny_post_ids",
    "auto_react_options.allow_user_ids",
    "auto_react_options.deny_user_ids",
  ])("has %s as tags field gated by features.auto_react", (key) => {
    const f = pancakeConfig.find((x) => x.key === key);
    expect(f).toBeDefined();
    expect(f!.type).toBe("tags");
    expect(f!.showWhen).toEqual({ key: "features.auto_react", value: "true" });
  });
});
