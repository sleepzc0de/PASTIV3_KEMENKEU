"use client";

import { useEffect, useRef, useCallback } from "react";

interface UseIdleTimerOptions {
  idleTimeout: number;
  warningBeforeMs?: number;
  onIdle: () => void;
  onWarning?: () => void;
  onActive?: () => void;
  enabled?: boolean;
}

const ACTIVITY_EVENTS = [
  "mousedown",
  "mousemove",
  "keydown",
  "scroll",
  "touchstart",
  "click",
] as const;

const CHECK_INTERVAL_MS = 1000; // cek setiap 1 detik, ringan dan akurat

export function useIdleTimer({
  idleTimeout,
  warningBeforeMs = 0,
  onIdle,
  onWarning,
  onActive,
  enabled = true,
}: UseIdleTimerOptions) {
  const lastActivityRef = useRef<number>(Date.now());
  const hasWarnedRef = useRef(false);
  const hasIdledRef = useRef(false);

  // Simpan callback terbaru di ref supaya interval selalu pakai versi terkini,
  // tidak terjebak stale closure meskipun identity function berubah tiap render.
  const onIdleRef = useRef(onIdle);
  const onWarningRef = useRef(onWarning);
  const onActiveRef = useRef(onActive);

  useEffect(() => {
    onIdleRef.current = onIdle;
    onWarningRef.current = onWarning;
    onActiveRef.current = onActive;
  }, [onIdle, onWarning, onActive]);

  const registerActivity = useCallback(() => {
    lastActivityRef.current = Date.now();

    if (hasWarnedRef.current || hasIdledRef.current) {
      hasWarnedRef.current = false;
      hasIdledRef.current = false;
      onActiveRef.current?.();
    }
  }, []);

  const resetTimer = useCallback(() => {
    registerActivity();
  }, [registerActivity]);

  useEffect(() => {
    if (!enabled) return;

    registerActivity();

    ACTIVITY_EVENTS.forEach((event) => {
      window.addEventListener(event, registerActivity, { passive: true });
    });

    const intervalId = setInterval(() => {
      const elapsed = Date.now() - lastActivityRef.current;

      if (
        warningBeforeMs > 0 &&
        !hasWarnedRef.current &&
        !hasIdledRef.current &&
        elapsed >= idleTimeout - warningBeforeMs
      ) {
        hasWarnedRef.current = true;
        onWarningRef.current?.();
      }

      if (!hasIdledRef.current && elapsed >= idleTimeout) {
        hasIdledRef.current = true;
        onIdleRef.current();
      }
    }, CHECK_INTERVAL_MS);

    return () => {
      clearInterval(intervalId);
      ACTIVITY_EVENTS.forEach((event) => {
        window.removeEventListener(event, registerActivity);
      });
    };
  }, [enabled, idleTimeout, warningBeforeMs, registerActivity]);

  return { resetTimer };
}