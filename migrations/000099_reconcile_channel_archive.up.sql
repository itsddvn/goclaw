-- Version 98 denotes either CPPAI's Contact grants or upstream's archive and
-- Zalo retype. Reconcile structures idempotently without replaying a blanket
-- Zalo retype: OAuth zalo_oa rows may already exist in either history.
CREATE TABLE IF NOT EXISTS channel_message_archive (
    id                 UUID PRIMARY KEY,
    channel_name       VARCHAR(100) NOT NULL,
    history_key        VARCHAR(200) NOT NULL,
    parent_history_key VARCHAR(200) NOT NULL DEFAULT '',
    sender             VARCHAR(255) NOT NULL,
    sender_id          VARCHAR(255) NOT NULL DEFAULT '',
    body               TEXT NOT NULL,
    platform_msg_id    VARCHAR(100) NOT NULL DEFAULT '',
    is_summary         BOOLEAN NOT NULL DEFAULT false,
    created_at         TIMESTAMPTZ NOT NULL,
    updated_at         TIMESTAMPTZ NOT NULL,
    tenant_id          UUID NOT NULL REFERENCES tenants(id),
    archived_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archive_reason     VARCHAR(20) NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_channel_message_archive_lookup
    ON channel_message_archive (tenant_id, channel_name, history_key, created_at);
CREATE INDEX IF NOT EXISTS idx_channel_message_archive_archived_at
    ON channel_message_archive (tenant_id, archived_at);

CREATE TABLE IF NOT EXISTS workstation_contact_grants (
    workstation_id UUID NOT NULL REFERENCES workstations(id) ON DELETE CASCADE,
    contact_id     UUID NOT NULL REFERENCES channel_contacts(id) ON DELETE CASCADE,
    tenant_id      UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    created_by     VARCHAR(255) NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workstation_id, contact_id)
);
CREATE INDEX IF NOT EXISTS idx_workstation_contact_grants_contact
    ON workstation_contact_grants(tenant_id, contact_id, workstation_id);
CREATE INDEX IF NOT EXISTS idx_workstation_contact_grants_workstation
    ON workstation_contact_grants(tenant_id, workstation_id);
-- Expanding one tenant-user grant to all their Contacts would widen access.
DROP TABLE IF EXISTS workstation_user_grants;

-- Only an explicit Bot-only plaintext config token proves a legacy Bot row.
-- Most credentials are encrypted, so ambiguous zalo_oa rows need operator
-- classification rather than a potentially destructive automatic conversion.
UPDATE channel_instances SET channel_type = 'zalo_bot'
WHERE channel_type = 'zalo_oa'
  AND jsonb_typeof(config->'token') = 'string'
  AND config->>'token' <> '';
