"use client";

import { useEffect, useRef, useState } from "react";
import { Check, Copy } from "lucide-react";

async function copyText(text: string): Promise<boolean> {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text);
      return true;
    }
  } catch {
    // jatuh ke cara lama di bawah
  }
  // Halaman yang dibuka lewat http://<IP> (bukan localhost/https) bukan secure context, sehingga
  // Clipboard API tidak tersedia; server development dibuka seperti itu.
  try {
    const area = document.createElement("textarea");
    area.value = text;
    area.setAttribute("readonly", "");
    area.style.position = "fixed";
    area.style.top = "0";
    area.style.opacity = "0";
    document.body.appendChild(area);
    area.select();
    area.setSelectionRange(0, text.length);
    const ok = document.execCommand("copy");
    document.body.removeChild(area);
    return ok;
  } catch {
    return false;
  }
}

export function CopyButton({ text, label }: { text: string; label: string }) {
  const [state, setState] = useState<"idle" | "done" | "failed">("idle");
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  useEffect(() => () => clearTimeout(timer.current), []);

  const handleClick = async () => {
    const ok = await copyText(text);
    setState(ok ? "done" : "failed");
    clearTimeout(timer.current);
    timer.current = setTimeout(() => setState("idle"), 1800);
  };

  return (
    <button
      type="button"
      onClick={handleClick}
      aria-label={label}
      className="inline-flex items-center gap-1 rounded-md px-1.5 py-1 text-xs font-medium text-slate-500 hover:bg-slate-100 hover:text-slate-700"
    >
      {state === "done" ? <Check className="h-3.5 w-3.5 text-green-600" /> : <Copy className="h-3.5 w-3.5" />}
      <span aria-live="polite">{state === "done" ? "Tersalin" : state === "failed" ? "Gagal menyalin" : "Salin"}</span>
    </button>
  );
}
