CREATE TABLE IF NOT EXISTS workstation_user_grants (
    workstation_id UUID NOT NULL REFERENCES workstations(id) ON DELETE CASCADE,
    tenant_user_id UUID NOT NULL REFERENCES tenant_users(id) ON DELETE CASCADE,
    tenant_id      UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    created_by     VARCHAR(255) NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workstation_id, tenant_user_id)
);

CREATE INDEX IF NOT EXISTS idx_workstation_user_grants_user
    ON workstation_user_grants(tenant_id, tenant_user_id, workstation_id);
CREATE INDEX IF NOT EXISTS idx_workstation_user_grants_workstation
    ON workstation_user_grants(tenant_id, workstation_id);
