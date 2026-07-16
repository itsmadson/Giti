export function cn(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

export interface JwtPayload {
  sub?: string;
  roles?: string[];
  exp?: number;
}

/** decodeJwt reads the payload of a JWT without verifying its signature. */
export function decodeJwt(token: string): JwtPayload {
  try {
    const part = token.split(".")[1];
    const b64 = part.replace(/-/g, "+").replace(/_/g, "/");
    const json = atob(b64);
    return JSON.parse(json);
  } catch {
    return {};
  }
}
