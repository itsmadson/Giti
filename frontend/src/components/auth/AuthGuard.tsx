"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useSession } from "@/api/auth/store";

export function AuthGuard({ children }: { children: React.ReactNode }) {
  const session = useSession();
  const router = useRouter();
  const params = useParams();
  const locale = (params?.locale as string) ?? "en";
  const [checked, setChecked] = useState(false);

  useEffect(() => {
    if (!session) router.replace(`/${locale}/login`);
    else setChecked(true);
  }, [session, locale, router]);

  if (!session || !checked) return null;
  return <>{children}</>;
}
