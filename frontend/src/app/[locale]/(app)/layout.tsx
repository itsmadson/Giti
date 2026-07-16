import { AuthGuard } from "@/components/auth/AuthGuard";
import { Shell } from "@/components/layout/Shell";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <AuthGuard>
      <Shell>{children}</Shell>
    </AuthGuard>
  );
}
