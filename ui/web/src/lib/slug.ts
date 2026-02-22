/**
 * Convert any string to a valid slug: lowercase, [a-z0-9-], no leading/trailing dashes.
 */
export function slugify(input: string): string {
  return input
    .toLowerCase()
    .replace(/[^a-z0-9-]/g, "-")
    .replace(/-{2,}/g, "-")
    .replace(/^-+|-+$/g, "");
}

/**
 * Validate slug format: lowercase alphanumeric + hyphens, cannot start/end with hyphen.
 */
export function isValidSlug(slug: string): boolean {
  return /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/.test(slug);
}
