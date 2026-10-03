"use client";

// Peta Leaflet (imperatif). Hanya boleh dimuat di browser: modul leaflet membaca `window` saat diimpor, jadi file ini
// dipanggil lewat next/dynamic dengan ssr: false (lihat MapPanel.tsx).

import { useEffect, useRef } from "react";
import L from "leaflet";
import "leaflet.markercluster";
import "leaflet/dist/leaflet.css";
import "leaflet.markercluster/dist/MarkerCluster.css";
import { DGDatasetKey, DGMapSet } from "@/lib/api";
import { DATASET_COLOR } from "./digitalisasi";

const TILE_URL = process.env.NEXT_PUBLIC_MAP_TILE_URL || "https://tile.openstreetmap.org/{z}/{x}/{y}.png";
const TILE_ATTRIBUTION = process.env.NEXT_PUBLIC_MAP_TILE_ATTRIBUTION || "&copy; kontributor OpenStreetMap";

// Tampilan awal: seluruh Indonesia.
const INDONESIA_CENTER: L.LatLngTuple = [-2.5, 118];
const INDONESIA_ZOOM = 5;

export interface MapFocus {
  dataset: DGDatasetKey;
  id: number;
  lat: number;
  lng: number;
  nonce: number;
}

interface Props {
  sets: DGMapSet[];
  visible: Set<DGDatasetKey>;
  selected: { dataset: DGDatasetKey; id: number } | null;
  focus: MapFocus | null;
  onSelect: (dataset: DGDatasetKey, id: number) => void;
  onTileStatus: (ok: boolean) => void;
}

type CatMarker = L.CircleMarker & { options: L.CircleMarkerOptions & { cat: DGDatasetKey } };

// Klaster berbentuk donat: irisannya menunjukkan komposisi jenis aset di dalamnya, angka di tengah adalah jumlahnya.
// Satu kelompok klaster untuk semua jenis, supaya gelembung beberapa jenis tidak saling menumpuk di kota yang sama.
function clusterIcon(cluster: L.MarkerCluster) {
  const counts = new Map<DGDatasetKey, number>();
  // Isi klaster adalah CircleMarker buatan kita; tipe pustakanya menyebut Marker.
  for (const m of cluster.getAllChildMarkers() as unknown as CatMarker[]) counts.set(m.options.cat, (counts.get(m.options.cat) ?? 0) + 1);
  const n = cluster.getChildCount();
  let acc = 0;
  const stops: string[] = [];
  [...counts.entries()]
    .sort((a, b) => b[1] - a[1])
    .forEach(([cat, c]) => {
      const from = (acc / n) * 100;
      acc += c;
      stops.push(`${DATASET_COLOR[cat]} ${from.toFixed(2)}% ${((acc / n) * 100).toFixed(2)}%`);
    });
  const size = n < 100 ? 38 : n < 1000 ? 46 : 54;
  return L.divIcon({
    html:
      `<div style="position:relative;width:${size}px;height:${size}px;border-radius:50%;background:conic-gradient(${stops.join(",")});box-shadow:0 1px 4px rgba(0,0,0,.35)">` +
      `<div style="position:absolute;inset:6px;border-radius:50%;background:#fff;display:flex;align-items:center;justify-content:center;font:600 12px system-ui,sans-serif;color:#0b0b0b">${n}</div></div>`,
    className: "dg-cluster",
    iconSize: [size, size],
  });
}

// Marker ditambahkan bertahap supaya halaman tidak membeku saat puluhan ribu titik dimuat. Pemuatan bertahap bawaan
// markercluster (chunkedLoading) tidak bisa dibatalkan: bila grupnya dilepas di tengah jalan, ia tetap menambah marker
// dan melempar galat. Karena itu pemuatan dilakukan di sini, dengan token pembatalan.
const BATCH = 4000;

export default function MapView({ sets, visible, selected, focus, onSelect, onTileStatus }: Props) {
  const box = useRef<HTMLDivElement>(null);
  const mapRef = useRef<L.Map | null>(null);
  const group = useRef<L.MarkerClusterGroup | null>(null);
  const markers = useRef<Map<DGDatasetKey, CatMarker[]>>(new Map());
  const shown = useRef<Set<DGDatasetKey>>(new Set());
  const generation = useRef(0); // naik tiap data diganti atau peta dibongkar: membatalkan semua pemuatan yang tertunda
  const pending = useRef<Map<DGDatasetKey, number>>(new Map()); // token pemuatan per jenis: naik saat jenis disembunyikan
  const ring = useRef<L.CircleMarker | null>(null);
  const fitted = useRef<DGMapSet[] | null>(null);
  // Callback terbaru disimpan di ref supaya marker lama tidak memanggil versi usang.
  const selectRef = useRef(onSelect);
  const tileRef = useRef(onTileStatus);
  useEffect(() => {
    selectRef.current = onSelect;
    tileRef.current = onTileStatus;
  });

  const addInBatches = (g: L.MarkerClusterGroup, key: DGDatasetKey, list: CatMarker[]) => {
    const token = (pending.current.get(key) ?? 0) + 1;
    pending.current.set(key, token);
    const gen = generation.current;
    let i = 0;
    const step = () => {
      if (gen !== generation.current || pending.current.get(key) !== token || group.current !== g || !mapRef.current) return;
      g.addLayers(list.slice(i, i + BATCH));
      i += BATCH;
      if (i < list.length) setTimeout(step, 0);
    };
    step();
  };

  // Peta dibuat sekali.
  useEffect(() => {
    const el = box.current;
    if (!el) return;
    const map = L.map(el, { center: INDONESIA_CENTER, zoom: INDONESIA_ZOOM, preferCanvas: true, worldCopyJump: true, minZoom: 3 });
    mapRef.current = map;

    let loaded = 0;
    let failed = 0;
    const tiles = L.tileLayer(TILE_URL, { maxZoom: 19, attribution: TILE_ATTRIBUTION });
    tiles.on("tileload", () => {
      loaded++;
      if (loaded === 1) tileRef.current(true);
    });
    tiles.on("tileerror", () => {
      failed++;
      // Jaringan pengguna bisa memblokir server peta: beri tahu setelah beberapa ubin gagal tanpa satu pun berhasil.
      if (failed >= 3 && loaded === 0) tileRef.current(false);
    });
    tiles.addTo(map);

    // Ukuran wadah berubah (tab dibuka, jendela diubah): Leaflet harus menghitung ulang.
    let ro: ResizeObserver | undefined;
    if (typeof ResizeObserver !== "undefined") {
      ro = new ResizeObserver(() => map.invalidateSize());
      ro.observe(el);
    }
    const markerMap = markers.current;
    const shownSet = shown.current;
    return () => {
      generation.current++;
      fitted.current = null; // peta baru (mis. remount) harus pas ke titik lagi
      ro?.disconnect();
      // Marker dilepas lebih dulu selagi renderer kanvas masih hidup. Bila map.remove() yang melepasnya, renderer sudah
      // dibongkar saat marker menjadwalkan gambar ulang, dan Leaflet melempar "clearRect of undefined".
      if (group.current) map.removeLayer(group.current);
      if (ring.current) map.removeLayer(ring.current);
      markerMap.clear();
      shownSet.clear();
      group.current = null;
      ring.current = null;
      map.remove();
      mapRef.current = null;
    };
  }, []);

  // Titik: satu kelompok klaster untuk semua jenis aset.
  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;
    generation.current++; // batalkan pemuatan bertahap dari data sebelumnya
    if (group.current) map.removeLayer(group.current);
    markers.current.clear();
    shown.current.clear();
    pending.current.clear();

    const g = L.markerClusterGroup({
      showCoverageOnHover: false,
      maxClusterRadius: 45,
      disableClusteringAtZoom: 18,
      iconCreateFunction: clusterIcon,
    });
    group.current = g;
    map.addLayer(g);
    for (const set of sets) {
      const color = DATASET_COLOR[set.key];
      const list = set.titik.map(([id, lat, lng]) => {
        const m = L.circleMarker([lat, lng], { radius: 6, color: "#ffffff", weight: 1.5, fillColor: color, fillOpacity: 0.95, cat: set.key } as L.CircleMarkerOptions) as CatMarker;
        m.on("click", () => selectRef.current(set.key, id));
        return m;
      });
      markers.current.set(set.key, list);
      if (visible.has(set.key)) {
        shown.current.add(set.key);
        addInBatches(g, set.key, list);
      }
    }

    // Pas ke sebaran titik hanya saat data berganti (bukan saat jenis ditampilkan/disembunyikan).
    if (fitted.current !== sets) {
      fitted.current = sets;
      const pts: L.LatLngTuple[] = [];
      for (const set of sets) if (visible.has(set.key)) for (const [, lat, lng] of set.titik) pts.push([lat, lng]);
      if (pts.length > 0) map.fitBounds(L.latLngBounds(pts), { padding: [30, 30], maxZoom: 15 });
    }
    // `visible` sengaja tidak jadi dependensi: perubahan jenis ditangani efek di bawah tanpa membangun ulang marker.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sets]);

  // Menampilkan/menyembunyikan jenis aset: hanya selisihnya yang ditambah atau dibuang dari klaster.
  useEffect(() => {
    const g = group.current;
    if (!g) return;
    markers.current.forEach((list, key) => {
      const on = visible.has(key);
      const was = shown.current.has(key);
      if (on && !was) {
        shown.current.add(key);
        addInBatches(g, key, list);
      } else if (!on && was) {
        pending.current.set(key, (pending.current.get(key) ?? 0) + 1); // batalkan pemuatan jenis ini yang masih berjalan
        g.removeLayers(list);
        shown.current.delete(key);
      }
    });
  }, [visible, sets]);

  // Cincin penanda pada titik terpilih.
  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;
    if (ring.current) {
      map.removeLayer(ring.current);
      ring.current = null;
    }
    if (!selected) return;
    const set = sets.find((s) => s.key === selected.dataset);
    const pt = set?.titik.find((p) => p[0] === selected.id);
    if (!pt) return;
    ring.current = L.circleMarker([pt[1], pt[2]], { radius: 11, color: "#0b0b0b", weight: 2.5, fill: false, interactive: false }).addTo(map);
  }, [selected, sets]);

  useEffect(() => {
    if (!focus || !mapRef.current) return;
    mapRef.current.setView([focus.lat, focus.lng], 17);
  }, [focus]);

  return <div ref={box} className="isolate h-full w-full" role="application" aria-label="Peta aset" />;
}
