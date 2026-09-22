export const FACEBOOK_WEBHOOK_PATH = "/v1/channels/facebook/webhook";
export const VERIFY_TOKEN_BYTE_LENGTH = 32;

const BASE64_URL_ALPHABET = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_";
const PRIVATE_HOST_SUFFIXES = [".local", ".internal", ".home", ".lan", ".test", ".invalid", ".example", ".onion"];
const RESERVED_DOCUMENTATION_DOMAINS = ["example.com", "example.net", "example.org"];

export function getFacebookCallbackUrl(origin: string): string {
  return `${origin.replace(/\/$/, "")}${FACEBOOK_WEBHOOK_PATH}`;
}

function isReservedIPv4(hostname: string): boolean {
  const parts = hostname.split(".");
  if (parts.length !== 4) return false;

  const octets = parts.map(Number);
  if (octets.some((part) => !Number.isInteger(part) || part < 0 || part > 255)) return false;

  const a = octets[0] ?? 0;
  const b = octets[1] ?? 0;
  const c = octets[2] ?? 0;
  return (
    a === 0 ||
    a === 10 ||
    a === 127 ||
    (a === 100 && b >= 64 && b <= 127) ||
    (a === 169 && b === 254) ||
    (a === 172 && b >= 16 && b <= 31) ||
    (a === 192 && b === 168) ||
    (a === 192 && b === 0 && (c === 0 || c === 2)) ||
    (a === 198 && (b === 18 || b === 19 || (b === 51 && c === 100))) ||
    (a === 203 && b === 0 && c === 113) ||
    a >= 224
  );
}

function isReservedHostname(hostname: string): boolean {
  const normalized = hostname.toLowerCase().replace(/^\[|\]$/g, "").replace(/\.$/, "");
  const mappedIPv4 = normalized.match(/(?:^|:)ffff:(\d+\.\d+\.\d+\.\d+)$/)?.[1];

  if (mappedIPv4) return isReservedIPv4(mappedIPv4);
  if (isReservedIPv4(normalized)) return true;
  if (normalized === "localhost" || normalized.endsWith(".localhost")) return true;
  if (PRIVATE_HOST_SUFFIXES.some((suffix) => normalized.endsWith(suffix))) return true;
  if (RESERVED_DOCUMENTATION_DOMAINS.some(
    (domain) => normalized === domain || normalized.endsWith(`.${domain}`),
  )) return true;

  if (normalized.includes(":")) {
    return (
      normalized === "::" ||
      normalized === "::1" ||
      normalized.startsWith("fc") ||
      normalized.startsWith("fd") ||
      /^fe[89ab]/.test(normalized) ||
      normalized.startsWith("2001:db8:")
    );
  }

  return !normalized.includes(".");
}

export function isPublicHttpsOrigin(origin: string): boolean {
  try {
    const url = new URL(origin);
    return url.protocol === "https:" && !isReservedHostname(url.hostname);
  } catch {
    return false;
  }
}

export function encodeBase64Url(bytes: Uint8Array): string {
  let encoded = "";

  for (let index = 0; index < bytes.length; index += 3) {
    const first = bytes[index] ?? 0;
    const second = bytes[index + 1];
    const third = bytes[index + 2];
    const value = (first << 16) | ((second ?? 0) << 8) | (third ?? 0);

    encoded += BASE64_URL_ALPHABET.charAt((value >>> 18) & 63);
    encoded += BASE64_URL_ALPHABET.charAt((value >>> 12) & 63);
    if (second !== undefined) encoded += BASE64_URL_ALPHABET.charAt((value >>> 6) & 63);
    if (third !== undefined) encoded += BASE64_URL_ALPHABET.charAt(value & 63);
  }

  return encoded;
}

export function generateVerifyToken(
  randomSource: Pick<Crypto, "getRandomValues"> = globalThis.crypto,
): string {
  const bytes = new Uint8Array(VERIFY_TOKEN_BYTE_LENGTH);
  randomSource.getRandomValues(bytes);
  return encodeBase64Url(bytes);
}
