import { useState, useEffect, useCallback } from "react";
import { useWs } from "@/hooks/use-ws";
import { Methods } from "@/api/protocol";

export interface ChannelStatus {
  enabled: boolean;
  running: boolean;
}

export function useChannels() {
  const ws = useWs();
  const [channels, setChannels] = useState<Record<string, ChannelStatus>>({});
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    if (!ws.isConnected) return;
    setLoading(true);
    try {
      const res = await ws.call<{ channels: Record<string, ChannelStatus> }>(
        Methods.CHANNELS_STATUS,
      );
      setChannels(res.channels ?? {});
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, [ws]);

  useEffect(() => {
    load();
  }, [load]);

  return { channels, loading, refresh: load };
}
