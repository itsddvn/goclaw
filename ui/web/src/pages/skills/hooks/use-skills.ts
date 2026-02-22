import { useState, useEffect, useCallback } from "react";
import { useWs, useHttp } from "@/hooks/use-ws";
import { Methods } from "@/api/protocol";

export interface SkillInfo {
  id?: string;
  name: string;
  description: string;
  source: string;
}

export function useSkills() {
  const ws = useWs();
  const http = useHttp();
  const [skills, setSkills] = useState<SkillInfo[]>([]);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    if (!ws.isConnected) return;
    setLoading(true);
    try {
      const res = await ws.call<{ skills: SkillInfo[] }>(Methods.SKILLS_LIST);
      setSkills(res.skills ?? []);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, [ws]);

  useEffect(() => {
    load();
  }, [load]);

  const getSkill = useCallback(
    async (name: string) => {
      if (!ws.isConnected) return null;
      return ws.call<SkillInfo & { content: string }>(Methods.SKILLS_GET, { name });
    },
    [ws],
  );

  const uploadSkill = useCallback(
    async (file: File) => {
      const formData = new FormData();
      formData.append("file", file);
      return http.upload<{ id: string; slug: string; version: number; name: string }>(
        "/v1/skills/upload",
        formData,
      );
    },
    [http],
  );

  const deleteSkill = useCallback(
    async (id: string) => {
      return http.delete<{ ok: string }>(`/v1/skills/${id}`);
    },
    [http],
  );

  return { skills, loading, refresh: load, getSkill, uploadSkill, deleteSkill };
}
