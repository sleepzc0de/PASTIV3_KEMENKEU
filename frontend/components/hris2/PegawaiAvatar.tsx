"use client";

import { useState } from "react";
import { initials } from "@/components/hris2/pegawai";

const SIZES = {
  sm: "h-10 w-10 text-sm",
  lg: "h-16 w-16 text-xl sm:h-20 sm:w-20 sm:text-2xl",
} as const;

// Inisial nama selalu terlihat lebih dulu. Foto pegawai (bila ada) menutupinya hanya setelah
// selesai dimuat, jadi foto yang lambat atau gagal dimuat tidak meninggalkan lingkaran kosong.
export function PegawaiAvatar({ nama, src, size = "sm" }: { nama: string; src?: string; size?: keyof typeof SIZES }) {
  const [loadedSrc, setLoadedSrc] = useState<string | null>(null);
  const [failedSrc, setFailedSrc] = useState<string | null>(null);
  const loaded = Boolean(src) && loadedSrc === src;

  return (
    <div
      aria-hidden="true"
      className={`relative flex shrink-0 select-none items-center justify-center overflow-hidden rounded-full bg-blue-100 font-bold text-blue-600 ${SIZES[size]}`}
    >
      {!loaded && initials(nama)}
      {src && failedSrc !== src && (
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src={src}
          alt=""
          loading="lazy"
          referrerPolicy="no-referrer"
          onLoad={() => setLoadedSrc(src)}
          onError={() => setFailedSrc(src)}
          className={`absolute inset-0 h-full w-full object-cover transition-opacity ${loaded ? "opacity-100" : "opacity-0"}`}
        />
      )}
    </div>
  );
}
