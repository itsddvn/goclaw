import { useEffect } from "react";
import { useWs } from "./use-ws";

/**
 * Subscribe to a WebSocket event. Automatically unsubscribes on unmount.
 */
export function useWsEvent(
  event: string,
  handler: (payload: unknown) => void,
): void {
  const ws = useWs();

  useEffect(() => {
    const unsubscribe = ws.on(event, handler);
    return unsubscribe;
  }, [ws, event, handler]);
}
