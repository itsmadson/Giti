import { AuthGuard } from "@/components/auth/AuthGuard";
import { Shell } from "@/components/layout/Shell";
import { Toaster } from "@/components/ui/Toast";
import { CommandPalette } from "@/components/layout/CommandPalette";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <AuthGuard>
      <Toaster>
        <Shell>{children}</Shell>
        <CommandPalette />
      </Toaster>
    </AuthGuard>
  );
}
