import type {
  UpdateWorkstationParams,
  Workstation,
  WorkstationBackendType,
} from "./hooks/use-workstations";

export interface WorkstationEditFormState {
  name: string;
  active: boolean;
  defaultCwd: string;
  backend: WorkstationBackendType;
  host: string;
  port: string;
  user: string;
  privateKey: string;
  password: string;
  container: string;
  image: string;
  socketPath: string;
}

export type BuildEditPayloadResult =
  | { kind: "ok"; payload: UpdateWorkstationParams }
  | { kind: "error"; errorKey: string };

export function editFormFromWorkstation(workstation: Workstation): WorkstationEditFormState {
  const summary = workstation.metadataSummary ?? {};
  return {
    name: workstation.name,
    active: workstation.active,
    defaultCwd: workstation.defaultCwd ?? "",
    backend: workstation.backendType,
    host: summary.host ?? "",
    port: String(summary.port ?? 22),
    user: summary.user ?? "",
    privateKey: "",
    password: "",
    container: summary.containerName ?? summary.host ?? "",
    image: summary.image ?? "",
    socketPath: summary.socketPath ?? "",
  };
}

export function buildWorkstationEditPayload(
  form: WorkstationEditFormState,
): BuildEditPayloadResult {
  const name = form.name.trim();
  if (!name) return { kind: "error", errorKey: "nameRequired" };

  let metadata: Record<string, unknown>;
  if (form.backend === "ssh") {
    const host = form.host.trim();
    const user = form.user.trim();
    if (!host || !user) return { kind: "error", errorKey: "sshHostUserRequired" };
    const port = Number(form.port);
    if (!Number.isInteger(port) || port < 1 || port > 65535) {
      return { kind: "error", errorKey: "sshPortRange" };
    }
    metadata = {
      host,
      port,
      user,
      ...(form.privateKey.trim() ? { privateKey: form.privateKey.trim() } : {}),
      ...(form.password ? { password: form.password } : {}),
    };
  } else {
    const container = form.container.trim();
    const image = form.image.trim();
    if (!container) return { kind: "error", errorKey: "dockerContainerRequired" };
    if (!image) return { kind: "error", errorKey: "dockerImageRequired" };
    metadata = {
      host: container,
      image,
      ...(form.socketPath.trim() ? { socketPath: form.socketPath.trim() } : {}),
    };
  }

  return {
    kind: "ok",
    payload: {
      name,
      active: form.active,
      defaultCwd: form.defaultCwd.trim(),
      metadata,
    },
  };
}

export function isValidCommandPattern(pattern: string): boolean {
  const normalized = pattern.trim();
  if (!normalized || normalized === "*") return false;
  return /^[A-Za-z0-9][A-Za-z0-9._+-]*\*?$/.test(normalized);
}
