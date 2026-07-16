import type { Metadata } from "next";
import { Inter, Space_Grotesk, JetBrains_Mono, Vazirmatn } from "next/font/google";
import { ThemeProvider } from "next-themes";
import "@/styles/globals.css";

const body = Inter({ subsets: ["latin"], variable: "--font-body", display: "swap" });
const display = Space_Grotesk({ subsets: ["latin"], variable: "--font-display", display: "swap" });
const mono = JetBrains_Mono({ subsets: ["latin"], variable: "--font-mono", display: "swap" });
const fa = Vazirmatn({ subsets: ["arabic"], variable: "--font-fa", display: "swap" });

export const metadata: Metadata = {
  title: "Geoson",
  description: "High-performance OGC geo engine — admin console",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html suppressHydrationWarning className={`${body.variable} ${display.variable} ${mono.variable} ${fa.variable}`}>
      <body>
        <ThemeProvider attribute="class" defaultTheme="dark" enableSystem={false}>
          {children}
        </ThemeProvider>
      </body>
    </html>
  );
}
