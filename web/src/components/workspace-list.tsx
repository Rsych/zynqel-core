"use client";

import { useEffect, useState, useCallback } from "react";
import { api } from "@/lib/api";
import type { Session } from "@/lib/types";
import { WorkspaceCard } from "./workspace-card";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { ConfirmDialog } from "./confirm-dialog";
import { Search, Inbox } from "lucide-react";

export function WorkspaceList({
  onCreateClick,
}: {
  onCreateClick: () => void;
}) {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [confirmAction, setConfirmAction] = useState<{
    type: "stop" | "delete";
    id: string;
  } | null>(null);

  const fetchSessions = useCallback(async () => {
    try {
      const data = await api.listSessions();
      setSessions(data || []);
    } catch {
      // silently retry on next interval
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchSessions();
    const interval = setInterval(fetchSessions, 10000);
    return () => clearInterval(interval);
  }, [fetchSessions]);

  const handleStop = async (id: string) => {
    // Instant feedback — mark as stopped before waiting for backend.
    setSessions((prev) =>
      prev.map((s) => (s.id === id ? { ...s, status: "stopped" as const } : s))
    );
    try {
      const updated = await api.stopSession(id);
      setSessions((prev) =>
        prev.map((s) => (s.id === id ? updated : s))
      );
    } catch (err) {
      // Revert on failure — refetch to get real state.
      fetchSessions();
      console.error("Failed to stop session:", err);
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await api.deleteSession(id);
      setSessions((prev) => prev.filter((s) => s.id !== id));
    } catch (err) {
      console.error("Failed to delete session:", err);
    }
  };

  const filtered = sessions.filter((s) => {
    const matchesSearch =
      search === "" ||
      s.id.toLowerCase().includes(search.toLowerCase()) ||
      (s.spec.workspace_id || "").toLowerCase().includes(search.toLowerCase()) ||
      s.spec.agent.toLowerCase().includes(search.toLowerCase()) ||
      (s.spec.repo_url || "").toLowerCase().includes(search.toLowerCase());
    const matchesStatus =
      statusFilter === "all" || s.status === statusFilter;
    return matchesSearch && matchesStatus;
  });

  if (loading) {
    return (
      <div className="space-y-3">
        {[...Array(3)].map((_, i) => (
          <Skeleton key={i} className="h-20 w-full rounded-lg" />
        ))}
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search workspaces..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9"
          />
        </div>
        <Select value={statusFilter} onValueChange={setStatusFilter}>
          <SelectTrigger className="w-[140px]">
            <SelectValue placeholder="Status" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All</SelectItem>
            <SelectItem value="running">Running</SelectItem>
            <SelectItem value="stopped">Stopped</SelectItem>
            <SelectItem value="error">Error</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {filtered.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 text-muted-foreground">
          <Inbox className="h-12 w-12 mb-4 opacity-50" />
          <p className="text-lg font-medium mb-1">
            {sessions.length === 0
              ? "No workspaces yet"
              : "No matching workspaces"}
          </p>
          <p className="text-sm">
            {sessions.length === 0 ? (
              <button
                onClick={onCreateClick}
                className="text-primary hover:underline"
              >
                Create your first workspace
              </button>
            ) : (
              "Try adjusting your search or filter"
            )}
          </p>
        </div>
      ) : (
        <div className="grid gap-3">
          {filtered.map((s) => (
            <WorkspaceCard
              key={s.id}
              session={s}
              onStop={(id) => setConfirmAction({ type: "stop", id })}
              onDelete={(id) => setConfirmAction({ type: "delete", id })}
            />
          ))}
        </div>
      )}

      <ConfirmDialog
        open={confirmAction?.type === "stop"}
        onOpenChange={(open) => !open && setConfirmAction(null)}
        title="Stop workspace?"
        description="The workspace will be stopped and its state saved. You can resume it later."
        confirmLabel="Stop"
        onConfirm={() => {
          if (confirmAction) handleStop(confirmAction.id);
          setConfirmAction(null);
        }}
      />
      <ConfirmDialog
        open={confirmAction?.type === "delete"}
        onOpenChange={(open) => !open && setConfirmAction(null)}
        title="Remove workspace?"
        description="This will stop the session and remove the container. Workspace volume data is preserved."
        confirmLabel="Remove"
        variant="destructive"
        onConfirm={() => {
          if (confirmAction) handleDelete(confirmAction.id);
          setConfirmAction(null);
        }}
      />
    </div>
  );
}
