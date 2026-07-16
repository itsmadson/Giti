import { apiFetch } from "../client";
import type { LoginResponse } from "./types";

export function login(username: string, password: string): Promise<LoginResponse> {
  return apiFetch<LoginResponse>("/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify({ username, password }),
  });
}
