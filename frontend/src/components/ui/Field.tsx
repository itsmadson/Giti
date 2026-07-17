import { cn } from "@/lib/utils";

const inputClass =
  "w-full rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--color-primary)]";

export function Input({
  label,
  className,
  ...props
}: React.InputHTMLAttributes<HTMLInputElement> & { label?: string }) {
  return (
    <label className="block space-y-1">
      {label && (
        <span className="text-xs font-medium text-[var(--color-muted)]">{label}</span>
      )}
      <input className={cn(inputClass, className)} {...props} />
    </label>
  );
}

export function Select({
  label,
  className,
  children,
  ...props
}: React.SelectHTMLAttributes<HTMLSelectElement> & { label?: string }) {
  return (
    <label className="block space-y-1">
      {label && (
        <span className="text-xs font-medium text-[var(--color-muted)]">{label}</span>
      )}
      <select className={cn(inputClass, className)} {...props}>
        {children}
      </select>
    </label>
  );
}
