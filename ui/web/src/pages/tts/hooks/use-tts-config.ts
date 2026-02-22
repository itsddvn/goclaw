import { useState, useEffect, useCallback } from "react";
import { useWs } from "@/hooks/use-ws";
import { Methods } from "@/api/protocol";

export interface TtsProviderConfig {
  api_key?: string;
  api_base?: string;
  base_url?: string;
  model?: string;
  voice?: string;
  voice_id?: string;
  model_id?: string;
  enabled?: boolean;
  rate?: string;
  group_id?: string;
}

export interface TtsConfig {
  provider: string;
  auto: string;
  mode: string;
  max_length: number;
  timeout_ms: number;
  openai: TtsProviderConfig;
  elevenlabs: TtsProviderConfig;
  edge: TtsProviderConfig;
  minimax: TtsProviderConfig;
}

const DEFAULT_TTS: TtsConfig = {
  provider: "",
  auto: "off",
  mode: "final",
  max_length: 1500,
  timeout_ms: 30000,
  openai: {},
  elevenlabs: {},
  edge: {},
  minimax: {},
};

export function useTtsConfig() {
  const ws = useWs();
  const [tts, setTts] = useState<TtsConfig>(DEFAULT_TTS);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!ws.isConnected) return;
    setLoading(true);
    try {
      const res = await ws.call<{ config: Record<string, unknown> }>(Methods.CONFIG_GET);
      const ttsConfig = (res.config?.tts as TtsConfig) ?? DEFAULT_TTS;
      setTts({ ...DEFAULT_TTS, ...ttsConfig });
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, [ws]);

  useEffect(() => {
    load();
  }, [load]);

  const save = useCallback(
    async (updates: Partial<TtsConfig>) => {
      setSaving(true);
      setError(null);
      try {
        await ws.call(Methods.CONFIG_PATCH, { tts: updates });
        await load();
      } catch (err) {
        const msg = err instanceof Error ? err.message : "Failed to save TTS config";
        setError(msg);
        throw err;
      } finally {
        setSaving(false);
      }
    },
    [ws, load],
  );

  return { tts, loading, saving, error, refresh: load, save };
}
