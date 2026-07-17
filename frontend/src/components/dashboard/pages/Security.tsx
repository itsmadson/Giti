"use client";

import { useEffect, useState } from "react";
import { Plus, Trash2, UserPlus, Shield } from "lucide-react";
import { useT } from "@/i18n/provider";
import {
  listUsers,
  createUser,
  deleteUser,
  listRoles,
  createRole,
  deleteRole,
  listRules,
  createRule,
  deleteRule,
  type User,
  type DataRule,
} from "@/api/dashboard/security/api";
import { DataTable } from "@/components/ui/DataTable";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Input, Select } from "@/components/ui/Field";
import { useToast } from "@/components/ui/Toast";

type Tab = "users" | "roles" | "rules";

export function Security() {
  const { t } = useT();
  const [tab, setTab] = useState<Tab>("users");
  const tabs: [Tab, string][] = [
    ["users", t("sec.users")],
    ["roles", t("sec.roles")],
    ["rules", t("sec.rules")],
  ];
  return (
    <div className="mx-auto max-w-4xl space-y-4">
      <div className="flex gap-1 border-b border-[var(--color-border)]">
        {tabs.map(([id, label]) => (
          <button
            key={id}
            onClick={() => setTab(id)}
            className={
              "px-3 py-2 text-sm " +
              (tab === id ? "border-b-2 border-[var(--color-primary)] text-[var(--color-text)]" : "text-[var(--color-muted)]")
            }
          >
            {label}
          </button>
        ))}
      </div>
      {tab === "users" && <Users />}
      {tab === "roles" && <Roles />}
      {tab === "rules" && <Rules />}
    </div>
  );
}

function Users() {
  const { t } = useT();
  const { toast } = useToast();
  const [items, setItems] = useState<User[]>([]);
  const [u, setU] = useState("");
  const [p, setP] = useState("");
  const load = () => listUsers().then(setItems).catch(() => setItems([]));
  useEffect(() => {
    load();
  }, []);
  async function add() {
    if (!u.trim() || !p) return;
    try {
      await createUser(u.trim(), p);
      toast({ title: t("sec.userAdded") });
      setU("");
      setP("");
      load();
    } catch (e) {
      toast({ title: (e as Error).message, tone: "err" });
    }
  }
  return (
    <div className="space-y-3">
      <div className="grid grid-cols-[1fr_1fr_auto] items-end gap-2">
        <Input label={t("login.username")} value={u} onChange={(e) => setU(e.target.value)} />
        <Input label={t("login.password")} type="password" value={p} onChange={(e) => setP(e.target.value)} />
        <Button onClick={add}><UserPlus size={15} /></Button>
      </div>
      <DataTable
        columns={[t("login.username"), t("stores.status"), ""]}
        rows={items.map((x) => [
          <span key="n" className="font-mono">{x.userName}</span>,
          <Badge key="e" tone={x.enabled ? "ok" : "neutral"}>{x.enabled ? t("stores.enabled") : t("stores.disabled")}</Badge>,
          <button key="d" onClick={() => deleteUser(x.userName).then(load)} className="flex justify-end text-[var(--color-muted)] hover:text-[var(--color-err)]"><Trash2 size={15} /></button>,
        ])}
        empty="—"
      />
    </div>
  );
}

function Roles() {
  const { t } = useT();
  const { toast } = useToast();
  const [items, setItems] = useState<string[]>([]);
  const [r, setR] = useState("");
  const load = () => listRoles().then(setItems).catch(() => setItems([]));
  useEffect(() => {
    load();
  }, []);
  async function add() {
    if (!r.trim()) return;
    try {
      await createRole(r.trim().toUpperCase());
      toast({ title: t("sec.roleAdded") });
      setR("");
      load();
    } catch (e) {
      toast({ title: (e as Error).message, tone: "err" });
    }
  }
  return (
    <div className="space-y-3">
      <div className="flex gap-2">
        <Input value={r} onChange={(e) => setR(e.target.value)} placeholder={t("sec.newRole")} />
        <Button onClick={add}><Plus size={15} /> {t("action.create")}</Button>
      </div>
      <DataTable
        columns={[t("sec.role"), ""]}
        rows={items.map((x) => [
          <span key="n" className="flex items-center gap-2 font-mono"><Shield size={13} className="text-[var(--color-primary)]" />{x}</span>,
          <button key="d" onClick={() => deleteRole(x).then(load)} className="flex justify-end text-[var(--color-muted)] hover:text-[var(--color-err)]"><Trash2 size={15} /></button>,
        ])}
        empty="—"
      />
    </div>
  );
}

function Rules() {
  const { t } = useT();
  const { toast } = useToast();
  const [items, setItems] = useState<DataRule[]>([]);
  const [form, setForm] = useState<DataRule>({ priority: 1, access: "ALLOW", roleName: "", layer: "", cqlFilterRead: "" });
  const load = () => listRules().then(setItems).catch(() => setItems([]));
  useEffect(() => {
    load();
  }, []);
  async function add() {
    try {
      await createRule(form);
      toast({ title: t("sec.ruleAdded") });
      load();
    } catch (e) {
      toast({ title: (e as Error).message, tone: "err" });
    }
  }
  return (
    <div className="space-y-3">
      <div className="grid grid-cols-[80px_1fr_1fr_1fr] items-end gap-2">
        <Input label={t("sec.priority")} type="number" value={String(form.priority)} onChange={(e) => setForm({ ...form, priority: +e.target.value })} />
        <Input label={t("sec.roleField")} value={form.roleName} onChange={(e) => setForm({ ...form, roleName: e.target.value })} placeholder="ADMIN / *" />
        <Input label={t("sec.layerField")} value={form.layer} onChange={(e) => setForm({ ...form, layer: e.target.value })} placeholder="ws:layer / *" />
        <Select label={t("sec.access")} value={form.access} onChange={(e) => setForm({ ...form, access: e.target.value })}>
          <option value="ALLOW">ALLOW</option>
          <option value="DENY">DENY</option>
          <option value="LIMIT">LIMIT</option>
        </Select>
      </div>
      {form.access === "LIMIT" && (
        <Input label={t("sec.cql")} value={form.cqlFilterRead} onChange={(e) => setForm({ ...form, cqlFilterRead: e.target.value })} placeholder="pop > 100000" />
      )}
      <Button onClick={add}><Plus size={15} /> {t("sec.addRule")}</Button>
      <DataTable
        columns={[t("sec.priority"), t("sec.roleField"), t("sec.layerField"), t("sec.access"), t("sec.cql"), ""]}
        rows={items.map((x) => [
          String(x.priority),
          <span key="r" className="font-mono text-xs">{x.roleName || "*"}</span>,
          <span key="l" className="font-mono text-xs">{x.layer || "*"}</span>,
          <Badge key="a" tone={x.access === "DENY" ? "err" : x.access === "LIMIT" ? "warn" : "ok"}>{x.access}</Badge>,
          <span key="c" className="font-mono text-xs text-[var(--color-muted)]">{x.cqlFilterRead}</span>,
          <button key="d" onClick={() => x.id && deleteRule(x.id).then(load)} className="flex justify-end text-[var(--color-muted)] hover:text-[var(--color-err)]"><Trash2 size={15} /></button>,
        ])}
        empty="—"
      />
    </div>
  );
}
