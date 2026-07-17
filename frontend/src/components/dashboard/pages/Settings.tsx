"use client";

import { useEffect, useState } from "react";
import { Save } from "lucide-react";
import { useT } from "@/i18n/provider";
import { getSettings, saveSettings, type GlobalSettings } from "@/api/dashboard/settings/api";
import { Card } from "@/components/ui/Card";
import { Input } from "@/components/ui/Field";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";

export function Settings() {
  const { t } = useT();
  const { toast } = useToast();
  const [s, setS] = useState<GlobalSettings>({});
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    getSettings().then(setS).catch(() => setS({}));
  }, []);
  const set = (p: Partial<GlobalSettings>) => setS((x) => ({ ...x, ...p }));

  async function save() {
    setBusy(true);
    try {
      await saveSettings(s);
      toast({ title: t("settings.saved") });
    } catch (e) {
      toast({ title: (e as Error).message, tone: "err" });
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mx-auto max-w-2xl space-y-4">
      <Card className="space-y-4 p-5">
        <div className="text-xs font-medium uppercase tracking-wide text-[var(--color-muted)]">{t("settings.service")}</div>
        <Input label={t("settings.serviceTitle")} value={s.serviceTitle ?? ""} onChange={(e) => set({ serviceTitle: e.target.value })} />
        <label className="block space-y-1">
          <span className="text-xs font-medium text-[var(--color-muted)]">{t("settings.serviceAbstract")}</span>
          <textarea
            value={s.serviceAbstract ?? ""}
            onChange={(e) => set({ serviceAbstract: e.target.value })}
            className="min-h-20 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--color-primary)]"
          />
        </label>
        <Input label={t("settings.proxyBase")} value={s.proxyBaseUrl ?? ""} onChange={(e) => set({ proxyBaseUrl: e.target.value })} placeholder="http://localhost" />
      </Card>

      <Card className="space-y-4 p-5">
        <div className="text-xs font-medium uppercase tracking-wide text-[var(--color-muted)]">{t("settings.contact")}</div>
        <div className="grid grid-cols-2 gap-3">
          <Input label={t("settings.organization")} value={s.organization ?? ""} onChange={(e) => set({ organization: e.target.value })} />
          <Input label={t("settings.contactName")} value={s.contactName ?? ""} onChange={(e) => set({ contactName: e.target.value })} />
        </div>
        <Input label={t("settings.contactEmail")} type="email" value={s.contactEmail ?? ""} onChange={(e) => set({ contactEmail: e.target.value })} />
      </Card>

      <div className="flex justify-end">
        <Button onClick={save} disabled={busy}>
          <Save size={15} /> {busy ? t("common.loading") : t("action.save")}
        </Button>
      </div>
    </div>
  );
}
