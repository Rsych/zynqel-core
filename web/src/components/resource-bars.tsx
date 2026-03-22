"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { ContainerStats } from "@/lib/types";
import { Progress } from "@/components/ui/progress";
import { Cpu, MemoryStick } from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";

export function ResourceBars({ sessionId }: { sessionId: string }) {
  const [stats, setStats] = useState<ContainerStats | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;

    const fetchStats = async () => {
      try {
        const data = await api.getSessionStats(sessionId);
        if (active) {
          setStats(data);
          setError("");
        }
      } catch {
        if (active) setError("Failed to fetch stats");
      }
    };

    fetchStats();
    const interval = setInterval(fetchStats, 5000);
    return () => {
      active = false;
      clearInterval(interval);
    };
  }, [sessionId]);

  if (error) {
    return (
      <div className="text-sm text-muted-foreground py-8 text-center">
        {error}
      </div>
    );
  }

  if (!stats) {
    return (
      <div className="space-y-6 py-4">
        <Skeleton className="h-16 w-full" />
        <Skeleton className="h-16 w-full" />
      </div>
    );
  }

  const cpuPercent = Math.min(stats.cpu_percent, 100);
  const memPercent =
    stats.memory_max_mb > 0
      ? (stats.memory_mb / stats.memory_max_mb) * 100
      : 0;

  return (
    <div className="space-y-6 py-4">
      <div className="space-y-2">
        <div className="flex items-center justify-between text-sm">
          <div className="flex items-center gap-2">
            <Cpu className="h-4 w-4 text-muted-foreground" />
            <span>CPU</span>
          </div>
          <span className="font-mono text-muted-foreground">
            {cpuPercent.toFixed(1)}%
          </span>
        </div>
        <Progress value={cpuPercent} className="h-2" />
      </div>

      <div className="space-y-2">
        <div className="flex items-center justify-between text-sm">
          <div className="flex items-center gap-2">
            <MemoryStick className="h-4 w-4 text-muted-foreground" />
            <span>Memory</span>
          </div>
          <span className="font-mono text-muted-foreground">
            {stats.memory_mb.toFixed(0)} MB / {stats.memory_max_mb.toFixed(0)} MB
          </span>
        </div>
        <Progress value={memPercent} className="h-2" />
      </div>
    </div>
  );
}
