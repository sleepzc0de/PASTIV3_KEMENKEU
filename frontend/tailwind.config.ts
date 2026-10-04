import type { Config } from "tailwindcss";

// Sistem desain PASTI V3. Palet `blue` dan `slate` di bawah menggantikan bawaan Tailwind, sehingga seluruh kelas
// yang sudah dipakai di aplikasi (bg-blue-600, text-slate-500, dst.) ikut berubah tampilan tanpa menyunting
// setiap berkas. Radius, bayangan, dan animasi juga didefinisikan di sini supaya konsisten.
const config: Config = {
  content: [
    "./app/**/*.{js,ts,jsx,tsx,mdx}",
    "./components/**/*.{js,ts,jsx,tsx,mdx}",
    "./lib/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      fontFamily: {
        // Tanpa unduhan font: tumpukan font sistem modern (Segoe UI Variable di Windows 11, SF di Apple).
        sans: [
          '"Inter"',
          '"Segoe UI Variable Text"',
          '"Segoe UI"',
          "system-ui",
          "-apple-system",
          "BlinkMacSystemFont",
          "Roboto",
          '"Helvetica Neue"',
          "Arial",
          "sans-serif",
        ],
        mono: ['"JetBrains Mono"', '"Cascadia Mono"', "ui-monospace", "SFMono-Regular", "Menlo", "Consolas", "monospace"],
      },
      colors: {
        // Biru utama: lebih dalam dan tenang dari biru bawaan, tetap terbaca sebagai identitas lembaga.
        blue: {
          50: "#f1f5ff",
          100: "#e3ebff",
          200: "#c9d9ff",
          300: "#a3bdff",
          400: "#7898fb",
          500: "#4f73f2",
          600: "#3358e0",
          700: "#2a47c2",
          800: "#273d9c",
          900: "#253779",
          950: "#18224d",
        },
        // Abu-abu netral kebiruan: permukaan terasa bersih, teks tetap kontras.
        slate: {
          50: "#f7f9fc",
          100: "#eef2f7",
          200: "#e1e7ef",
          300: "#cbd5e1",
          400: "#94a3b8",
          500: "#64748b",
          600: "#475569",
          700: "#334155",
          800: "#1e293b",
          900: "#0f172a",
          950: "#0a1020",
        },
      },
      borderRadius: {
        lg: "0.625rem",
        xl: "0.875rem",
        "2xl": "1.125rem",
        "3xl": "1.5rem",
      },
      boxShadow: {
        // Bayangan berlapis dan lembut: kartu terangkat tanpa garis tebal.
        sm: "0 1px 2px rgb(15 23 42 / 0.04), 0 1px 3px rgb(15 23 42 / 0.06)",
        DEFAULT: "0 1px 3px rgb(15 23 42 / 0.06), 0 2px 8px rgb(15 23 42 / 0.05)",
        md: "0 2px 4px rgb(15 23 42 / 0.04), 0 6px 16px rgb(15 23 42 / 0.07)",
        lg: "0 4px 8px rgb(15 23 42 / 0.04), 0 12px 28px rgb(15 23 42 / 0.1)",
        xl: "0 8px 16px rgb(15 23 42 / 0.06), 0 24px 48px rgb(15 23 42 / 0.14)",
        card: "0 0 0 1px rgb(15 23 42 / 0.05), 0 1px 2px rgb(15 23 42 / 0.04), 0 4px 12px rgb(15 23 42 / 0.04)",
        glow: "0 0 0 4px rgb(51 88 224 / 0.12)",
      },
      keyframes: {
        // Animasi drawer menu di tampilan mobile.
        "drawer-in": {
          from: { transform: "translateX(-100%)" },
          to: { transform: "translateX(0)" },
        },
        "fade-in": {
          from: { opacity: "0" },
          to: { opacity: "1" },
        },
        "fade-up": {
          from: { opacity: "0", transform: "translateY(8px)" },
          to: { opacity: "1", transform: "translateY(0)" },
        },
        "scale-in": {
          from: { opacity: "0", transform: "scale(0.96) translateY(4px)" },
          to: { opacity: "1", transform: "scale(1) translateY(0)" },
        },
        "slide-in-right": {
          from: { opacity: "0", transform: "translateX(16px)" },
          to: { opacity: "1", transform: "translateX(0)" },
        },
        shimmer: {
          "100%": { transform: "translateX(100%)" },
        },
        // Batang grafik tumbuh dari kiri saat pertama tampil.
        "grow-x": {
          from: { transform: "scaleX(0)" },
          to: { transform: "scaleX(1)" },
        },
        // Kolom grafik tumbuh dari dasar saat pertama tampil.
        "grow-y": {
          from: { transform: "scaleY(0)" },
          to: { transform: "scaleY(1)" },
        },
        "pop-in": {
          "0%": { opacity: "0", transform: "scale(0.9)" },
          "60%": { transform: "scale(1.03)" },
          "100%": { opacity: "1", transform: "scale(1)" },
        },
      },
      animation: {
        "drawer-in": "drawer-in 240ms cubic-bezier(0.22, 1, 0.36, 1)",
        "fade-in": "fade-in 200ms ease-out",
        "fade-up": "fade-up 320ms cubic-bezier(0.22, 1, 0.36, 1) backwards",
        "scale-in": "scale-in 200ms cubic-bezier(0.22, 1, 0.36, 1) backwards",
        "slide-in-right": "slide-in-right 260ms cubic-bezier(0.22, 1, 0.36, 1) backwards",
        shimmer: "shimmer 1.6s infinite",
        "grow-x": "grow-x 700ms cubic-bezier(0.22, 1, 0.36, 1) backwards",
        "grow-y": "grow-y 700ms cubic-bezier(0.22, 1, 0.36, 1) backwards",
        "pop-in": "pop-in 300ms cubic-bezier(0.22, 1, 0.36, 1) backwards",
      },
      transitionTimingFunction: {
        smooth: "cubic-bezier(0.22, 1, 0.36, 1)",
      },
    },
  },
  plugins: [],
};

export default config;
