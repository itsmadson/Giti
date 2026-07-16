"use client";

import { useEffect, useState } from "react";
import { useT } from "@/i18n/provider";
import { listWorkspaces, createWorkspace } from "@/api/dashboard/workspaces/api";
import type { Workspace } from "@/api/dashboard/workspaces/types";
import { DataTable } from "@/components/ui/DataTable";
import { Button } from "@/components/ui/Button";
import { Plus } from "lucide-react";

export function Workspaces() {
  const { t } = useT();
  const [items, setItems] = useState<Workspace[]>([]);
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);

  const load = () => listWorkspaces().then(setItems).catch(() => setItems([]));
  useEffect(() => {
    load();
  }, []);

  async function create(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    setBusy(true);
    try {
      await createWorkspace(name.trim());
      setName("");
      await load();
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mx-auto max-w-4xl space-y-4">
      <form onSubmit={create} className="flex gap-2">
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder={t("workspaces.new")}
          className="flex-1 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--color-primary)]"
        />
        <Button type="submit" disabled={busy}>
          <Plus size={15} /> {t("action.create")}
        </Button>
      </form>

      <DataTable
        columns={[t("workspaces.name")]}
        rows={items.map((w) => [<span key={w.name} className="font-mono">{w.name}</span>])}
        empty={t("workspaces.empty")}
      />
    </div>
  );
}
