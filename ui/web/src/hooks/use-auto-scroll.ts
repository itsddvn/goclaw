import { useEffect, useRef, useCallback } from "react";

/**
 * Auto-scroll to bottom of a container when content changes.
 * Only auto-scrolls if user is near the bottom (within threshold).
 */
export function useAutoScroll<T extends HTMLElement>(
  deps: unknown[],
  threshold = 100,
) {
  const ref = useRef<T>(null);
  const isNearBottom = useRef(true);

  const checkScroll = useCallback(() => {
    const el = ref.current;
    if (!el) return;
    const { scrollTop, scrollHeight, clientHeight } = el;
    isNearBottom.current = scrollHeight - scrollTop - clientHeight < threshold;
  }, [threshold]);

  const scrollToBottom = useCallback(() => {
    const el = ref.current;
    if (!el) return;
    el.scrollTop = el.scrollHeight;
  }, []);

  useEffect(() => {
    if (isNearBottom.current) {
      scrollToBottom();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  return { ref, onScroll: checkScroll, scrollToBottom };
}
