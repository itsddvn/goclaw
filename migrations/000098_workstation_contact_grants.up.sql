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

-- The prior tenant-user model is intentionally not migrated because expanding
-- one user grant to every merged Contact would silently broaden access.
DROP TABLE IF EXISTS workstation_user_grants;
