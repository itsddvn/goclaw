import { useState, useEffect, useRef } from "react";

/**
 * Returns a boolean that stays `true` for at least `minMs` after `loading` goes from true→false.
 * Useful for showing a brief spin animation on refresh buttons.
 */
export function useMinLoading(loading: boolean, minMs = 2000): boolean {
  const [visible, setVisible] = useState(loading);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (loading) {
      // Clear any pending "turn off" timer
      if (timerRef.current) clearTimeout(timerRef.current);
      setVisible(true);
    } else if (visible) {
      // loading ended — keep visible for at least minMs total
      timerRef.current = setTimeout(() => setVisible(false), minMs);
    }
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, [loading, minMs, visible]);

  return visible;
}
