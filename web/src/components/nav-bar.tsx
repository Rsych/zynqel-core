"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { api } from "@/lib/api";
import type { SystemInfo } from "@/lib/types";
import { Activity, Settings } from "lucide-react";
import { Button } from "@/components/ui/button";

export function NavBar() {
  const [info, setInfo] = useState<SystemInfo | null>(null);

  useEffect(() => {
    api.getSystemInfo().then(setInfo).catch(() => {});
    const interval = setInterval(() => {
      api.getSystemInfo().then(setInfo).catch(() => {});
    }, 10000);
    return () => clearInterval(interval);
  }, []);

  return (
    <header className="border-b border-border bg-background/80 backdrop-blur-sm sticky top-0 z-50">
      <div className="mx-auto max-w-7xl flex items-center justify-between px-6 h-14">
        <div className="flex items-center gap-3">
          <div className="h-7 w-7 rounded-md bg-primary flex items-center justify-center">
            <span className="text-sm font-bold text-primary-foreground">Z</span>
          </div>
          <span className="text-lg font-semibold tracking-tight">Zynqel</span>
        </div>

        <div className="flex items-center gap-3">
          {info && (
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <Activity className="h-3.5 w-3.5" />
              <span>
                {info.active_count}/{info.max_sessions || "∞"} sessions
              </span>
            </div>
          )}
          <Button variant="ghost" size="icon" className="h-8 w-8" asChild>
            <Link href="/agents">
              <Settings className="h-4 w-4" />
            </Link>
          </Button>
        </div>
      </div>
    </header>
  );
}
