import { describe, expect, it, vi } from "vitest";
import { grantContactsInBatch } from "../workstation-contact-access-helpers";

describe("workstation Contact batch grants", () => {
  it("grants every distinct selected Contact", async () => {
    const grant = vi.fn(async () => undefined);

    const result = await grantContactsInBatch(["contact-a", "contact-b", "contact-a"], grant);

    expect(grant).toHaveBeenCalledTimes(2);
    expect(grant).toHaveBeenCalledWith("contact-a");
    expect(grant).toHaveBeenCalledWith("contact-b");
    expect(result).toEqual({
      succeededIds: ["contact-a", "contact-b"],
      failedIds: [],
    });
  });

  it("reports individual failures without cancelling successful grants", async () => {
    const grant = vi.fn(async (contactId: string) => {
      if (contactId === "contact-b") throw new Error("rejected");
    });

    const result = await grantContactsInBatch(["contact-a", "contact-b", "contact-c"], grant);

    expect(grant).toHaveBeenCalledTimes(3);
    expect(result).toEqual({
      succeededIds: ["contact-a", "contact-c"],
      failedIds: ["contact-b"],
    });
  });
});
