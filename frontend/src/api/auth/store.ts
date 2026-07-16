"use client";

import { useSyncExternalStore } from "react";
import { decodeJwt } from "@/lib/utils";
import type { Session } from "./types";

const KEY = "giti.token";
let current: Session | null = null;
const listeners = new Set<() => void>();

function load(): Session | null {
  if (typeof window === "undefined") return null;
  const token = window.localStorage.getItem(KEY);
  if (!token) return null;
  const p = decodeJwt(token);
  return { token, user: p.sub ?? "", roles: p.roles ?? [] };
}

// hydrate on module load (client only)
if (typeof window !== "undefined") current = load();

function emit() {
  for (const l of listeners) l();
}

export function setSession(token: string) {
  window.localStorage.setItem(KEY, token);
  const p = decodeJwt(token);
  current = { token, user: p.sub ?? "", roles: p.roles ?? [] };
  emit();
}

export function clearSession() {
  window.localStorage.removeItem(KEY);
  current = null;
  emit();
}

/** getToken is a non-hook accessor used by the API client. */
export function getToken(): string | null {
  if (current) return current.token;
  if (typeof window !== "undefined") return window.localStorage.getItem(KEY);
  return null;
}

function subscribe(cb: () => void) {
  listeners.add(cb);
  return () => listeners.delete(cb);
}

/** useSession is the reactive session hook. */
export function useSession(): Session | null {
  return useSyncExternalStore(
    subscribe,
    () => current,
    () => null,
  );
}
