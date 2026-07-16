import type { Metadata } from "next";
import { ThemeProvider } from "next-themes";
import "@/styles/globals.css";

export const metadata: Metadata = {
  title: "Geoson",
  description: "High-performance OGC geo engine — admin console",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html suppressHydrationWarning>
      <head>
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossOrigin="" />
        <link
          href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&family=Space+Grotesk:wght@500;600;700&family=JetBrains+Mono:wght@400;500&family=Vazirmatn:wght@400;500;600&display=swap"
          rel="stylesheet"
        />
      </head>
      <body>
        <ThemeProvider attribute="class" defaultTheme="dark" enableSystem={false}>
          {children}
        </ThemeProvider>
      </body>
    </html>
  );
}
