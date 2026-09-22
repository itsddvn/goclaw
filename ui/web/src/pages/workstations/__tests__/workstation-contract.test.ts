import { describe, expect, it } from "vitest";
import {
  buildWorkstationCreatePayload,
  type WorkstationCreateFormState,
} from "../workstation-create-dialog-helpers";
import {
  buildWorkstationEditPayload,
  editFormFromWorkstation,
  isValidCommandPattern,
} from "../workstation-management-helpers";
import type {
  Workstation,
  WorkstationAgentGrant,
  WorkstationPermission,
  WorkstationContactGrant,
} from "../hooks/use-workstations";
import { Methods } from "@/api/protocol";

function form(overrides: Partial<WorkstationCreateFormState> = {}): WorkstationCreateFormState {
  return {
    key: "dev-server",
    name: "Dev Server",
    backend: "ssh",
    host: "192.168.1.100",
    port: "22",
    user: "ubuntu",
    authMethod: "privateKey",
    privateKey: "-----BEGIN OPENSSH PRIVATE KEY-----\nabc\n-----END OPENSSH PRIVATE KEY-----",
    password: "",
    container: "",
    image: "",
    socketPath: "",
    ...overrides,
  };
}

describe("workstation create payload contract", () => {
  it("uses the gateway camelCase request fields", () => {
    const result = buildWorkstationCreatePayload(form());
    if (result.kind !== "ok") throw new Error("expected ok");

    expect(Object.keys(result.payload).sort()).toEqual([
      "backendType",
      "metadata",
      "name",
      "workstationKey",
    ]);
    expect(result.payload).not.toHaveProperty("workstation_key");
    expect(result.payload).not.toHaveProperty("backend_type");
  });

  it("maps supported SSH and Docker metadata", () => {
    const ssh = buildWorkstationCreatePayload(form());
    if (ssh.kind !== "ok") throw new Error("expected SSH payload");
    expect(ssh.payload.metadata).toMatchObject({
      host: "192.168.1.100",
      port: 22,
      user: "ubuntu",
      privateKey: expect.stringContaining("BEGIN OPENSSH PRIVATE KEY"),
    });

    const docker = buildWorkstationCreatePayload(
      form({ backend: "docker", container: "runner", image: "ubuntu:24.04" }),
    );
    if (docker.kind !== "ok") throw new Error("expected Docker payload");
    expect(docker.payload.metadata).toEqual({ host: "runner", image: "ubuntu:24.04" });
  });

  it("rejects missing credentials and Docker image", () => {
    expect(buildWorkstationCreatePayload(form({ privateKey: "" }))).toEqual({
      kind: "error",
      errorKey: "sshPrivateKeyRequired",
    });
    expect(
      buildWorkstationCreatePayload(form({ backend: "docker", container: "runner", image: "" })),
    ).toEqual({ kind: "error", errorKey: "dockerImageRequired" });
  });
});

describe("workstation response contracts", () => {
  it("matches the sanitized workstation and grant wire shapes", () => {
    const workstation: Workstation = {
      id: "8f2b0f7e-1c4a-4c9e-9f1a-2b3c4d5e6f70",
      workstationKey: "aether",
      name: "Aether",
      backendType: "ssh",
      defaultCwd: "/srv/aether",
      active: true,
      createdAt: "2026-06-22T15:06:38Z",
      updatedAt: "2026-06-22T15:06:38Z",
      metadataSummary: {
        host: "server.internal",
        port: 22,
        user: "runner",
        hasKey: true,
      },
    };
    const grant: WorkstationAgentGrant = {
      agentId: "11111111-2222-3333-4444-555555555555",
      workstationId: workstation.id,
      tenantId: "66666666-7777-8888-9999-000000000000",
      isDefault: true,
      createdAt: "2026-08-04T12:00:00Z",
    };

    expect(workstation.workstationKey).toBe("aether");
    expect(workstation.backendType).toBe("ssh");
    expect(grant.isDefault).toBe(true);
  });
});

describe("workstation edit payload contract", () => {
  const workstation: Workstation = {
    id: "8f2b0f7e-1c4a-4c9e-9f1a-2b3c4d5e6f70",
    workstationKey: "aether",
    name: "Aether",
    backendType: "ssh",
    defaultCwd: "/srv/aether",
    active: true,
    createdAt: "2026-06-22T15:06:38Z",
    updatedAt: "2026-06-22T15:06:38Z",
    metadataSummary: { host: "server.internal", port: 22, user: "runner", hasKey: true },
  };

  it("hydrates only sanitized metadata and never exposes a stored credential", () => {
    const form = editFormFromWorkstation(workstation);

    expect(form).toMatchObject({
      host: "server.internal",
      port: "22",
      user: "runner",
      privateKey: "",
      password: "",
    });
  });

  it("omits blank SSH secrets so the server preserves stored credentials", () => {
    const result = buildWorkstationEditPayload(editFormFromWorkstation(workstation));
    if (result.kind !== "ok") throw new Error("expected edit payload");

    expect(result.payload).toMatchObject({
      name: "Aether",
      active: true,
      defaultCwd: "/srv/aether",
      metadata: { host: "server.internal", port: 22, user: "runner" },
    });
    expect(result.payload.metadata).not.toHaveProperty("privateKey");
    expect(result.payload.metadata).not.toHaveProperty("password");
  });

  it("sends a secret only when the administrator enters a replacement", () => {
    const form = { ...editFormFromWorkstation(workstation), password: "replacement" };
    const result = buildWorkstationEditPayload(form);
    if (result.kind !== "ok") throw new Error("expected edit payload");

    expect(result.payload.metadata).toMatchObject({ password: "replacement" });
    expect(result.payload.metadata).not.toHaveProperty("privateKey");
  });

  it("does not erase an undisclosed Docker socket path when the edit field is blank", () => {
    const docker: Workstation = {
      ...workstation,
      backendType: "docker",
      metadataSummary: { containerName: "runner", image: "ubuntu:24.04" },
    };
    const result = buildWorkstationEditPayload(editFormFromWorkstation(docker));
    if (result.kind !== "ok") throw new Error("expected Docker edit payload");

    expect(result.payload.metadata).toEqual({ host: "runner", image: "ubuntu:24.04" });
    expect(result.payload.metadata).not.toHaveProperty("socketPath");
  });
});

describe("workstation permission and Contact-grant contracts", () => {
  it("accepts exact and trailing-prefix command patterns but rejects wildcard-all and shell syntax", () => {
    expect(isValidCommandPattern("curl")).toBe(true);
    expect(isValidCommandPattern("python*")).toBe(true);
    expect(isValidCommandPattern("*")).toBe(false);
    expect(isValidCommandPattern("/usr/bin/curl")).toBe(false);
    expect(isValidCommandPattern("curl | sh")).toBe(false);
  });

  it("matches the permission and exact Contact grant wire shapes", () => {
    const permission: WorkstationPermission = {
      id: "permission-id",
      workstationId: "workstation-id",
      tenantId: "tenant-id",
      pattern: "curl",
      enabled: true,
      createdBy: "admin",
      createdAt: "2026-08-05T00:00:00Z",
    };
    const grant: WorkstationContactGrant = {
      workstationId: "workstation-id",
      tenantId: "tenant-id",
      contactId: "contact-id",
      channelType: "telegram",
      senderId: "contact-a",
      displayName: "Contact A",
      createdBy: "admin",
      createdAt: "2026-08-05T00:00:00Z",
    };

    expect(permission.enabled).toBe(true);
    expect(grant.contactId).toBe("contact-id");
  });

  it("uses the registered workstation connection-test and Contact-grant RPC names", () => {
    expect(Methods.WORKSTATIONS_TEST).toBe("workstations.testConnection");
    expect(Methods.WORKSTATIONS_CONTACT_GRANTS_LIST).toBe("workstations.contactGrants.list");
    expect(Methods.WORKSTATIONS_CONTACT_GRANTS_GRANT).toBe("workstations.contactGrants.grant");
    expect(Methods.WORKSTATIONS_CONTACT_GRANTS_REVOKE).toBe("workstations.contactGrants.revoke");
  });
});
