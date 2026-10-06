"use client";

import { useState } from "react";
import { Segmented } from "../digitalisasi/controls";
import { BMNPanel } from "./BMNPanel";
import { RefUE1Panel } from "./RefUE1Panel";
import { TemplatePanel } from "./TemplatePanel";

type Tab = "template" | "bmn" | "ue1";

export function PengaturanSapa() {
  const [tab, setTab] = useState<Tab>("template");
  return (
    <div className="space-y-5">
      <Segmented<Tab>
        label="Bagian pengaturan"
        value={tab}
        onChange={setTab}
        options={[
          { value: "template", label: "Template dokumen" },
          { value: "bmn", label: "Jenis & satuan BMN" },
          { value: "ue1", label: "Referensi UE1" },
        ]}
      />
      {tab === "template" && <TemplatePanel />}
      {tab === "bmn" && <BMNPanel />}
      {tab === "ue1" && <RefUE1Panel />}
    </div>
  );
}
