export const locales = ["en", "fa"] as const;
export type Locale = (typeof locales)[number];
export const defaultLocale: Locale = "en";

export const dir = (l: Locale): "ltr" | "rtl" => (l === "fa" ? "rtl" : "ltr");

export function isLocale(v: string): v is Locale {
  return (locales as readonly string[]).includes(v);
}

export type Dict = Record<string, string>;

import { en } from "./dictionaries/en";
import { fa } from "./dictionaries/fa";

export const dictionaries: Record<Locale, Dict> = { en, fa };
