"use client";
import { createContext, useContext, useState, useCallback } from "react";
import { Check, AlertCircle } from "lucide-react";

type Tone = "ok" | "err";
type Item = { id: number; title: string; tone: Tone };
const Ctx = createContext<{ toast: (t: { title: string; tone?: Tone }) => void }>({ toast: () => {} });

export function useToast() {
  return useContext(Ctx);
}

export function Toaster({ children }: { children: React.ReactNode }) {
  const [items, setItems] = useState<Item[]>([]);
  const toast = useCallback((t: { title: string; tone?: Tone }) => {
    const id = Date.now() + Math.random();
    setItems((x) => [...x, { id, title: t.title, tone: t.tone ?? "ok" }]);
    setTimeout(() => setItems((x) => x.filter((i) => i.id !== id)), 3500);
  }, []);
  return (
    <Ctx.Provider value={{ toast }}>
      {children}
      <div className="fixed bottom-4 end-4 z-[60] flex flex-col gap-2">
        {items.map((i) => (
          <div
            key={i.id}
            className="flex items-center gap-2 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2 text-sm shadow-lg"
          >
            {i.tone === "ok" ? (
              <Check size={15} className="text-[var(--color-ok)]" />
            ) : (
              <AlertCircle size={15} className="text-[var(--color-err)]" />
            )}
            {i.title}
          </div>
        ))}
      </div>
    </Ctx.Provider>
  );
}
