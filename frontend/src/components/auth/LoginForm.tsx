"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { motion } from "framer-motion";
import { useT } from "@/i18n/provider";
import { login } from "@/api/auth/api";
import { setSession } from "@/api/auth/store";
import { Button } from "@/components/ui/Button";
import { icons } from "@/components/icons";

export function LoginForm() {
  const { t, locale } = useT();
  const router = useRouter();
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const Brand = icons.brand;

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const res = await login(username, password);
      setSession(res.token);
      router.push(`/${locale}/dashboard`);
    } catch {
      setError(t("login.error"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="graticule flex min-h-screen items-center justify-center px-4">
      <motion.div
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4, ease: "easeOut" }}
        className="w-full max-w-sm"
      >
        <div className="mb-8 flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-[var(--color-primary)] text-[var(--color-primary-fg)]">
            <Brand size={20} />
          </div>
          <div>
            <div className="font-display text-lg font-semibold tracking-tight">{t("app.name")}</div>
            <div className="text-xs text-[var(--color-muted)]">{t("login.subtitle")}</div>
          </div>
        </div>

        <form
          onSubmit={submit}
          className="rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-6"
        >
          <h1 className="font-display mb-5 text-xl font-semibold">{t("login.title")}</h1>

          <label className="mb-1 block text-xs font-medium text-[var(--color-muted)]">
            {t("login.username")}
          </label>
          <input
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            className="mb-4 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-base)] px-3 py-2 text-sm outline-none focus:border-[var(--color-primary)]"
            autoComplete="username"
          />

          <label className="mb-1 block text-xs font-medium text-[var(--color-muted)]">
            {t("login.password")}
          </label>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="mb-4 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-base)] px-3 py-2 text-sm outline-none focus:border-[var(--color-primary)]"
            autoComplete="current-password"
          />

          {error && <p className="mb-3 text-sm text-[var(--color-err)]">{error}</p>}

          <Button type="submit" disabled={busy} className="w-full justify-center">
            {busy ? t("common.loading") : t("action.signIn")}
          </Button>

          <p className="mt-4 text-xs text-[var(--color-muted)]">{t("login.hint")}</p>
        </form>
      </motion.div>
    </div>
  );
}
