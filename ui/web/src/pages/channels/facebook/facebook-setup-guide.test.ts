import { describe, expect, it, vi } from "vitest";
import {
  VERIFY_TOKEN_BYTE_LENGTH,
  generateVerifyToken,
  getFacebookCallbackUrl,
  isPublicHttpsOrigin,
} from "./facebook-setup";

describe("Facebook callback origin", () => {
  it("builds the fixed callback path from the UI origin", () => {
    expect(getFacebookCallbackUrl("https://goclaw.ai")).toBe(
      "https://goclaw.ai/v1/channels/facebook/webhook",
    );
  });

  it("accepts only public HTTPS origins", () => {
    expect(isPublicHttpsOrigin("https://goclaw.ai")).toBe(true);
    expect(isPublicHttpsOrigin("https://8.8.8.8:9443")).toBe(true);

    for (const origin of [
      "https://goclaw.example",
      "https://example.com",
      "https://example.net",
      "https://example.org",
      "https://admin.example.com",
      "http://admin.example.com",
      "https://localhost:3000",
      "https://goclaw.internal",
      "https://10.0.0.4",
      "https://172.20.1.4",
      "https://192.168.1.4",
      "https://127.0.0.1",
      "https://[::1]",
      "https://[fd00::1]",
    ]) {
      expect(isPublicHttpsOrigin(origin), origin).toBe(false);
    }
  });
});

describe("Facebook Verify Token generation", () => {
  it("uses 32 Web Crypto bytes and emits URL-safe unpadded base64", () => {
    let requestedBytes = 0;
    const getRandomValues = vi.fn((array: Uint8Array) => {
      requestedBytes = array.byteLength;
      for (let index = 0; index < array.length; index += 1) array[index] = index;
      return array;
    });

    const token = generateVerifyToken({ getRandomValues } as unknown as Pick<Crypto, "getRandomValues">);

    expect(requestedBytes).toBe(VERIFY_TOKEN_BYTE_LENGTH);
    expect(getRandomValues).toHaveBeenCalledTimes(1);
    expect(token).toHaveLength(43);
    expect(token).toMatch(/^[A-Za-z0-9_-]+$/);
    expect(token).not.toMatch(/[+/=]/);
  });
});
