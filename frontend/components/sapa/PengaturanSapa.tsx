"use client";

import { useState } from "react";
import { Segmented } from "../digitalisasi/controls";
import { PeranPanel } from "./PeranPanel";
import { RefUE1Panel } from "./RefUE1Panel";
import { TemplatePanel } from "./TemplatePanel";

type Tab = "template" | "peran" | "ue1";

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
          { value: "peran", label: "Peran pengguna" },
          { value: "ue1", label: "Referensi UE1" },
        ]}
      />
      {tab === "template" && <TemplatePanel />}
      {tab === "peran" && <PeranPanel />}
      {tab === "ue1" && <RefUE1Panel />}
    </div>
  );
}
