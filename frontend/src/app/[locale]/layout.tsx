import { notFound } from "next/navigation";
import { I18nProvider } from "@/i18n/provider";
import { dir, isLocale, locales } from "@/i18n/config";

export function generateStaticParams() {
  return locales.map((locale) => ({ locale }));
}

export default async function LocaleLayout({
  children,
  params,
}: {
  children: React.ReactNode;
  params: Promise<{ locale: string }>;
}) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();

  return (
    <div dir={dir(locale)} lang={locale} style={{ minHeight: "100vh" }}>
      <I18nProvider locale={locale}>{children}</I18nProvider>
    </div>
  );
}
