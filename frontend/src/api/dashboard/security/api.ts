import { apiFetch, apiPost, apiDelete } from "@/api/client";

export interface User {
  userName: string;
  enabled: boolean;
}
export interface DataRule {
  id?: number;
  priority: number;
  roleName?: string;
  userName?: string;
  service?: string;
  workspace?: string;
  layer?: string;
  access: string; // ALLOW | DENY | LIMIT
  cqlFilterRead?: string;
}

const J = { headers: { Accept: "application/json" } };

export async function listUsers(): Promise<User[]> {
  const r = await apiFetch<{ users: User[] }>("/giti/rest/security/usergroup/users.json", J);
  return r.users ?? [];
}
export function createUser(userName: string, password: string): Promise<void> {
  return apiPost("/giti/rest/security/usergroup/users", { user: { userName, password, enabled: true } });
}
export function deleteUser(userName: string): Promise<void> {
  return apiDelete(`/giti/rest/security/usergroup/user/${encodeURIComponent(userName)}`);
}

export async function listRoles(): Promise<string[]> {
  const r = await apiFetch<{ roles: string[] }>("/giti/rest/security/roles.json", J);
  return r.roles ?? [];
}
export function createRole(role: string): Promise<void> {
  return apiPost(`/giti/rest/security/roles/role/${encodeURIComponent(role)}`, {});
}
export function deleteRole(role: string): Promise<void> {
  return apiDelete(`/giti/rest/security/roles/role/${encodeURIComponent(role)}`);
}
export function assignRole(role: string, user: string): Promise<void> {
  return apiPost(`/giti/rest/security/roles/role/${encodeURIComponent(role)}/user/${encodeURIComponent(user)}`, {});
}

export async function listRules(): Promise<DataRule[]> {
  const r = await apiFetch<{ rules: DataRule[] }>("/giti/rest/geofence/rules", J);
  return r.rules ?? [];
}
export function createRule(rule: DataRule): Promise<void> {
  return apiPost("/giti/rest/geofence/rules", { rule });
}
export function deleteRule(id: number): Promise<void> {
  return apiDelete(`/giti/rest/geofence/rules/id/${id}`);
}
