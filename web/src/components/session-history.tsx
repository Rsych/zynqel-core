"use client";

import { useEffect, useState, useCallback } from "react";
import { api } from "@/lib/api";
import type { SessionRecord } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { LogViewer } from "./log-viewer";
import { Clock, FileText, Trash2, History } from "lucide-react";
import { toast } from "sonner";

export function SessionHistory() {
  const [records, setRecords] = useState<SessionRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [logTarget, setLogTarget] = useState<string | null>(null);

  const fetchHistory = useCallback(async () => {
    try {
      const data = await api.listSessionHistory();
      setRecords(data || []);
    } catch {
      // Session history endpoint may not exist on older servers.
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchHistory();
  }, [fetchHistory]);

  const handleDelete = async (id: string) => {
    try {
      await api.deleteSessionHistory(id);
      setRecords((prev) => prev.filter((r) => r.id !== id));
      toast.success("History entry removed");
    } catch {
      toast.error("Failed to remove history entry");
    }
  };

  if (loading) {
    return (
      <div className="space-y-2">
        {[...Array(2)].map((_, i) => (
          <Skeleton key={i} className="h-12 w-full rounded-lg" />
        ))}
      </div>
    );
  }

  if (records.length === 0) return null;

  return (
    <div>
      <h3 className="text-sm font-medium text-muted-foreground mb-3 flex items-center gap-2">
        <History className="h-3.5 w-3.5" />
        Session History
      </h3>
      <div className="space-y-2">
        {records.map((r) => (
          <div
            key={r.id}
            className="flex items-center justify-between gap-3 p-3 rounded-lg border border-border bg-card"
          >
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2 mb-0.5">
                <span className="font-medium text-sm truncate">
                  {r.workspace_id || r.id.slice(0, 8)}
                </span>
                <Badge variant="outline" className="text-xs">
                  {r.status}
                </Badge>
                {r.error && (
                  <Badge variant="destructive" className="text-xs">
                    error
                  </Badge>
                )}
              </div>
              <div className="flex items-center gap-3 text-xs text-muted-foreground">
                <span className="font-mono bg-muted/50 px-1.5 py-0.5 rounded">
                  {r.agent}
                </span>
                {r.duration && (
                  <span className="flex items-center gap-1">
                    <Clock className="h-3 w-3" />
                    {r.duration}
                  </span>
                )}
                <span>
                  {new Date(r.created_at).toLocaleDateString()}{" "}
                  {new Date(r.created_at).toLocaleTimeString([], {
                    hour: "2-digit",
                    minute: "2-digit",
                  })}
                </span>
              </div>
            </div>
            <div className="flex items-center gap-1 shrink-0">
              {r.has_log && (
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8"
                  onClick={() => setLogTarget(r.id)}
                  title="View log"
                >
                  <FileText className="h-3.5 w-3.5" />
                </Button>
              )}
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8 text-destructive hover:text-destructive"
                onClick={() => handleDelete(r.id)}
                title="Delete"
              >
                <Trash2 className="h-3.5 w-3.5" />
              </Button>
            </div>
          </div>
        ))}
      </div>
      <LogViewer
        open={logTarget !== null}
        onOpenChange={(open) => !open && setLogTarget(null)}
        sessionId={logTarget || ""}
      />
    </div>
  );
}
