"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { ArrowLeft, Check, AlertCircle, Wand2, ShieldCheck } from "lucide-react";
import { useT } from "@/i18n/provider";
import { Button } from "@/components/ui/Button";
import { Input, Select } from "@/components/ui/Field";
import { useToast } from "@/components/ui/Toast";
import {
  getStyle,
  createStyle,
  updateStyle,
  validateStyle,
  generateStyle,
} from "@/api/dashboard/styles/api";
import type { ValidationError } from "@/api/dashboard/styles/types";

export function StyleEditor() {
  const { t } = useT();
  const { toast } = useToast();
  const router = useRouter();
  const params = useParams();
  const locale = (params?.locale as string) ?? "en";
  const routeName = decodeURIComponent((params?.name as string) ?? "new");
  const isNew = routeName === "new";

  const [name, setName] = useState(isNew ? "" : routeName);
  const [format, setFormat] = useState("sld");
  const [content, setContent] = useState("");
  const [errors, setErrors] = useState<ValidationError[] | null>(null);
  const [ok, setOk] = useState(false);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!isNew) {
      getStyle(routeName)
        .then((s) => {
          setContent(s.content);
          setFormat(s.format || "sld");
        })
        .catch(() => setContent(""));
    }
  }, [isNew, routeName]);

  async function validate() {
    const r = await validateStyle(format, content);
    setOk(r.ok);
    setErrors(r.errors ?? []);
    toast(r.ok ? { title: t("styleEdit.valid") } : { title: t("styleEdit.invalid"), tone: "err" });
  }

  async function generate() {
    const geom = prompt(t("styleEdit.genPrompt"), "POINT") || "POINT";
    const r = await generateStyle(geom.toUpperCase());
    setContent(r.sld);
    setFormat("sld");
  }

  async function save() {
    if (!name.trim()) {
      toast({ title: t("styleEdit.nameRequired"), tone: "err" });
      return;
    }
    setBusy(true);
    try {
      if (isNew) await createStyle(name.trim(), format, content);
      else await updateStyle(name, format, content);
      toast({ title: t("styleEdit.saved") });
      router.push(`/${locale}/dashboard/styles`);
    } catch (e) {
      toast({ title: (e as Error).message, tone: "err" });
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mx-auto max-w-5xl space-y-4">
      <div className="flex items-center gap-3">
        <Link href={`/${locale}/dashboard/styles`} className="rounded-md p-1.5 text-[var(--color-muted)] hover:bg-[var(--color-surface-2)]">
          <ArrowLeft size={18} />
        </Link>
        <h1 className="font-display text-xl font-semibold">{isNew ? t("styles.add") : name}</h1>
        <div className="ms-auto flex gap-2">
          <Button variant="ghost" onClick={generate}>
            <Wand2 size={15} /> {t("styleEdit.generate")}
          </Button>
          <Button variant="ghost" onClick={validate}>
            <ShieldCheck size={15} /> {t("styleEdit.validate")}
          </Button>
          <Button onClick={save} disabled={busy}>
            {busy ? t("common.loading") : t("action.save")}
          </Button>
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-[1fr_200px]">
        <Input label={t("styles.name")} value={name} onChange={(e) => setName(e.target.value)} disabled={!isNew} />
        <Select label={t("styles.format")} value={format} onChange={(e) => setFormat(e.target.value)}>
          <option value="sld">SLD</option>
          <option value="css">CSS</option>
          <option value="ysld">YSLD</option>
          <option value="mbstyle">MBStyle</option>
        </Select>
      </div>

      <textarea
        value={content}
        onChange={(e) => {
          setContent(e.target.value);
          setErrors(null);
        }}
        spellCheck={false}
        className="h-[420px] w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] p-3 font-mono text-xs leading-relaxed outline-none focus:border-[var(--color-primary)]"
        placeholder="<StyledLayerDescriptor …>"
      />

      {errors !== null && (
        <div className="rounded-lg border border-[var(--color-border)] p-3 text-sm">
          {ok ? (
            <div className="flex items-center gap-2 text-[var(--color-ok)]">
              <Check size={15} /> {t("styleEdit.valid")}
            </div>
          ) : (
            <div className="space-y-1">
              {errors.map((e, i) => (
                <div key={i} className="flex items-center gap-2 text-[var(--color-err)]">
                  <AlertCircle size={14} /> {t("styleEdit.line")} {e.line}: {e.message}
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
