"use client";

import { useT } from "@/i18n/provider";
import { Card } from "@/components/ui/Card";
import { icons } from "@/components/icons";

export function Placeholder({ titleKey, iconKey, service }: { titleKey: string; iconKey: string; service: string }) {
  const { t } = useT();
  const Icon = icons[iconKey] ?? icons.overview;
  return (
    <div className="mx-auto max-w-4xl">
      <Card className="flex flex-col items-center justify-center gap-3 p-16 text-center">
        <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-[var(--color-surface-2)] text-[var(--color-primary)]">
          <Icon size={22} />
        </div>
        <div className="font-display text-lg font-semibold">{t(titleKey)}</div>
        <p className="max-w-sm text-sm text-[var(--color-muted)]">
          {t("common.comingFrom", { service })}
        </p>
      </Card>
    </div>
  );
}
