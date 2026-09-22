export interface ContactGrantBatchResult {
  succeededIds: string[];
  failedIds: string[];
}

/** Grant every distinct Contact without letting one failure cancel the rest. */
export async function grantContactsInBatch(
  contactIds: string[],
  grant: (contactId: string) => Promise<void>,
): Promise<ContactGrantBatchResult> {
  const distinctIds = [...new Set(contactIds)];
  const results = await Promise.allSettled(
    distinctIds.map(async (contactId) => grant(contactId)),
  );
  const succeededIds: string[] = [];
  const failedIds: string[] = [];

  results.forEach((result, index) => {
    const contactId = distinctIds[index];
    if (!contactId) return;
    if (result.status === "fulfilled") succeededIds.push(contactId);
    else failedIds.push(contactId);
  });

  return { succeededIds, failedIds };
}
