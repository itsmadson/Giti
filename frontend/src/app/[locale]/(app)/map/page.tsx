import { Suspense } from "react";
import { MapWorkspace } from "@/components/map/MapWorkspace";
export default function Page() {
  return (
    <Suspense fallback={null}>
      <MapWorkspace />
    </Suspense>
  );
}
