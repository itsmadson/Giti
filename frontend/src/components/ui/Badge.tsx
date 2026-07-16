import { cn } from "@/lib/utils";

type Tone = "neutral" | "ok" | "warn" | "err" | "primary";

export function Badge({
  tone = "neutral",
  className,
  children,
}: {
  tone?: Tone;
  className?: string;
  children: React.ReactNode;
}) {
  const tones: Record<Tone, string> = {
    neutral: "text-[var(--color-muted)] border-[var(--color-border)]",
    ok: "text-[var(--color-ok)] border-[color-mix(in_oklab,var(--color-ok),transparent_70%)]",
    warn: "text-[var(--color-amber)] border-[color-mix(in_oklab,var(--color-amber),transparent_70%)]",
    err: "text-[var(--color-err)] border-[color-mix(in_oklab,var(--color-err),transparent_70%)]",
    primary: "text-[var(--color-primary)] border-[color-mix(in_oklab,var(--color-primary),transparent_70%)]",
  };
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-medium",
        tones[tone],
        className,
      )}
    >
      {children}
    </span>
  );
}
