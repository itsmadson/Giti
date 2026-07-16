"use client";

import { createContext, useContext } from "react";
import { dictionaries, type Dict, type Locale } from "./config";

type I18nValue = { locale: Locale; dict: Dict };
const I18nContext = createContext<I18nValue>({ locale: "en", dict: dictionaries.en });

export function I18nProvider({
  locale,
  children,
}: {
  locale: Locale;
  children: React.ReactNode;
}) {
  return (
    <I18nContext.Provider value={{ locale, dict: dictionaries[locale] }}>
      {children}
    </I18nContext.Provider>
  );
}

export function useT() {
  const { locale, dict } = useContext(I18nContext);
  const t = (key: string, vars?: Record<string, string>) => {
    let s = dict[key] ?? key;
    if (vars) for (const [k, v] of Object.entries(vars)) s = s.replace(`{${k}}`, v);
    return s;
  };
  return { t, locale };
}
