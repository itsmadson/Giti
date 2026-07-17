# Sprint E6 — Security (Users, Roles, Rules) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development or superpowers:executing-plans. `- [ ]` steps.

**Goal:** GeoServer-grade security admin: Users/Groups/Roles + user-group & role services (XML + JDBC), authentication chain, password policies, data rules (GeoFence), admin rules, service rules, WPS security, URL checks, CSP, REST security.

**Architecture:** `auth` (Go) already has users/groups/roles + argon2id + JWT/Basic + GeoFence data rules (S4). E6 adds JDBC user/group + role service backends, admin-rule + service-rule engines, password-policy model, URL-check list, CSP config, and the admin UI. Rules enforced in gateway/auth on every OWS/REST request (existing X-Giti-User/Roles/CQL-Read headers).

**Tech Stack:** Go (auth), Next.js UI, PostGIS (rule store), JWT.

## Global Constraints

Same as prior. Never log secrets. Password hashing stays argon2id. Data-rule semantics ALLOW/DENY/LIMIT(+CQL) match S4. Default admin `admin/geoserver` documented "change it" — enforce password-policy on change.

## File Structure

- `services/auth/internal/ugsvc/{xml,jdbc}.go` — user/group service backends behind an interface.
- `services/auth/internal/rolesvc/{xml,jdbc}.go` — role service backends.
- `services/auth/internal/rules/{data,admin,service}.go` — rule engines.
- `services/auth/internal/policy/password.go` — password policies.
- `services/auth/internal/rest/apiv1_security.go` — all security API v1.
- `services/gateway/internal/authz/*` — enforce admin/service rules per request.
- `frontend/src/api/dashboard/security/{types,api}.ts`.
- `frontend/src/components/dashboard/pages/Security.tsx` (replace stub) + tabs.

## Task 1: User/Group + Role service interfaces + JDBC (backend)

**Interfaces produced:** `type UserGroupService interface { ListUsers/CreateUser/…; ListGroups/… }`; `type RoleService interface { ListRoles/AssignRole/… }`; XML-backed (existing) + JDBC-backed (config: connection + table/column mapping).

- [ ] **Test:** register a JDBC user-group service over a test table; list users returns the JDBC rows; auth login validates against it.
- [ ] Implement interfaces; move existing default XML impl behind `UserGroupService`; add `jdbc.go` (pgx to a configured DB, configurable user/password/role tables).
- [ ] Config store: `security_services(name, kind, type, config jsonb)`.
- [ ] Build; commit `feat(auth): user-group + role services (XML + JDBC) (E6)`.

## Task 2: Password policies (backend)

- [ ] `policy/password.go`: policy `{name, minLength, requireDigit, requireUpper, requireSpecial, expireDays}`; enforce on create/change; default policy seeded.
- [ ] **Test:** weak password rejected by policy; strong accepted.
- [ ] API: `GET/POST/PUT/DELETE /api/v1/security/password-policies`.
- [ ] Commit `feat(auth): password policies (E6)`.

## Task 3: Admin rules + service rules (backend)

**Interfaces produced:** admin rule `{role, workspace, access:ADMIN|USER}`; service rule `{service:wms|wfs|…, role, allow bool}`. Engines evaluate per request alongside data rules.

- [ ] **Test:** a workspace-admin role can admin only its workspace; a service rule disabling WFS for anonymous blocks WFS.
- [ ] Implement engines + stores (`admin_rules`, `service_rules`); enforce in gateway authz middleware (read role from JWT/headers).
- [ ] API: `GET/POST/PUT/DELETE /api/v1/security/admin-rules`, `/service-rules`; existing data-rules endpoints surfaced under `/api/v1/security/data-rules`.
- [ ] Build; live verify a service rule blocks a service.
- [ ] Commit `feat(auth): admin + service rules (E6)`.

## Task 4: URL checks + CSP + REST/file security (backend)

- [ ] `url_checks(pattern, allow bool)` enforced on proxy/cascade fetches; CSP config `{policy}` emitted as `Content-Security-Policy` header by gateway/frontend; filesystem sandbox root for file stores (`GITI_FILE_SANDBOX`), reject paths outside it.
- [ ] **Test:** a file store path outside sandbox is rejected; a blocked URL check denies a cascade fetch.
- [ ] API: `GET/PUT /api/v1/security/settings` (CSP, sandbox, REST security toggles), `/url-checks` CRUD.
- [ ] Commit `feat(auth): url checks + CSP + file sandbox (E6)`.

## Task 5: Security API client + UI (frontend)

- [ ] Client for users/groups/roles, services, password-policies, data/admin/service rules, url-checks, settings.
- [ ] `Security.tsx` tabs: **Users/Groups/Roles** (CRUD + assign), **Services** (user-group + role services incl. add JDBC), **Authentication** (chain view + providers read-only for now), **Passwords** (policies), **Data rules**, **Admin rules**, **Service rules**, **WPS security**, **URL checks**, **Settings** (CSP/sandbox/REST). Each tab a CRUD table + drawer editor.
- [ ] i18n both dicts; live verify each tab 200.
- [ ] Commit `feat(frontend): security admin (users/roles/rules/services) (E6)`.

## Task 6: E6 acceptance

- [ ] Create user+group+role via UI; back a service with JDBC; login validates against JDBC.
- [ ] Author a data rule limiting a layer by CQL → enforced on WFS/WMS.
- [ ] Scope a workspace admin; disable a service via service rule.
- [ ] Weak password rejected by policy.
- [ ] Commit `chore: E6 security complete`.

## Self-Review

Spec E6 coverage: UGR + services(XML/JDBC) ✅(T1), passwords ✅(T2), data/admin/service rules ✅(T3), url-checks/CSP/sandbox/REST-sec ✅(T4), WPS security tab ✅(T5). LDAP/OIDC/AD/CAS explicitly v2 per spec. Interfaces `UserGroupService`/`RoleService` consistent T1↔T5.
