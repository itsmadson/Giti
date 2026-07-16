"use client";

import { cn } from "@/lib/utils";

type Variant = "primary" | "ghost" | "danger";

export function Button({
  variant = "primary",
  className,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant }) {
  const styles: Record<Variant, string> = {
    primary: "bg-[var(--color-primary)] text-[var(--color-primary-fg)] hover:opacity-90",
    ghost: "bg-transparent text-[var(--color-text)] hover:bg-[var(--color-surface-2)] border border-[var(--color-border)]",
    danger: "bg-transparent text-[var(--color-err)] hover:bg-[var(--color-surface-2)] border border-[var(--color-border)]",
  };
  return (
    <button
      className={cn(
        "inline-flex items-center gap-2 rounded-md px-3.5 py-2 text-sm font-medium transition-all disabled:opacity-50 focus-visible:outline focus-visible:outline-2 focus-visible:outline-[var(--color-primary)]",
        styles[variant],
        className,
      )}
      {...props}
    />
  );
}
