"use client";

import { Sidebar } from "./Sidebar";
import { Header } from "./Header";

export function Shell({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex h-screen overflow-hidden">
      <Sidebar />
      <div className="flex flex-1 flex-col overflow-hidden">
        <Header />
        <main className="graticule flex-1 overflow-auto p-6">{children}</main>
      </div>
    </div>
  );
}
