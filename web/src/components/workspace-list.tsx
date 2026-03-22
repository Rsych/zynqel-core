"use client";

import { useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";
import type { Session, Workspace } from "@/lib/types";
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
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "./confirm-dialog";
import { Search, Inbox, Play, Trash2, HardDrive, Loader2 } from "lucide-react";
import { toast } from "sonner";

export function WorkspaceList({
  onCreateClick,
}: {
  onCreateClick: () => void;
}) {
  const router = useRouter();
  const [sessions, setSessions] = useState<Session[]>([]);
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [confirmAction, setConfirmAction] = useState<{
    type: "stop" | "delete";
    id: string;
  } | null>(null);

  const fetchData = useCallback(async () => {
    try {
      const [sessData, wsData] = await Promise.all([
        api.listSessions(),
        api.listWorkspaces(),
      ]);
      setSessions(sessData || []);
      setWorkspaces(wsData || []);
    } catch {
      // silently retry on next interval
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
    const interval = setInterval(fetchData, 10000);
    return () => clearInterval(interval);
  }, [fetchData]);

  const [resumingId, setResumingId] = useState<string | null>(null);

  const handleStop = async (id: string) => {
    setSessions((prev) =>
      prev.map((s) => (s.id === id ? { ...s, status: "stopped" as const } : s))
    );
    toast.success("Workspace stopped");
    try {
      const updated = await api.stopSession(id);
      setSessions((prev) =>
        prev.map((s) => (s.id === id ? updated : s))
      );
    } catch (err) {
      fetchData();
      toast.error("Failed to stop workspace");
    }
  };

  const handleRestart = async (id: string) => {
    toast.loading("Starting workspace...", { id: "restart" });
    try {
      const newSession = await api.restartSession(id);
      setSessions((prev) =>
        prev.map((s) => (s.id === id ? newSession : s))
      );
      toast.success("Workspace started", { id: "restart" });
    } catch (err) {
      toast.error("Failed to start workspace", { id: "restart" });
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await api.deleteSession(id);
      setSessions((prev) => prev.filter((s) => s.id !== id));
      toast.success("Workspace removed");
    } catch (err) {
      toast.error("Failed to remove workspace");
    }
  };

  const handleResume = async (ws: Workspace) => {
    setResumingId(ws.id);
    toast.loading("Resuming workspace...", { id: "resume" });
    try {
      const session = await api.createSession({
        agent: ws.agent || "shell",
        workspace_id: ws.id,
      });
      toast.success("Workspace resumed", { id: "resume" });
      router.push(`/workspace?id=${session.id}`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to resume workspace", { id: "resume" });
    } finally {
      setResumingId(null);
    }
  };

  const handleDeleteWorkspace = async (id: string) => {
    try {
      await api.deleteWorkspace(id);
      setWorkspaces((prev) => prev.filter((w) => w.id !== id));
      toast.success("Workspace deleted");
    } catch (err) {
      toast.error("Failed to delete workspace");
    }
  };

  // Workspace IDs that have active sessions.
  const activeWorkspaceIds = new Set(
    sessions.map((s) => s.spec.workspace_id).filter(Boolean)
  );

  // Saved workspaces without an active session.
  const savedWorkspaces = workspaces.filter(
    (w) => !activeWorkspaceIds.has(w.id)
  );

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

  const filteredSaved = savedWorkspaces.filter(
    (w) =>
      search === "" ||
      w.id.toLowerCase().includes(search.toLowerCase()) ||
      (w.agent || "").toLowerCase().includes(search.toLowerCase())
  );

  if (loading) {
    return (
      <div className="space-y-3">
        {[...Array(3)].map((_, i) => (
          <Skeleton key={i} className="h-20 w-full rounded-lg" />
        ))}
      </div>
    );
  }

  const hasAnything = filtered.length > 0 || filteredSaved.length > 0;

  return (
    <div className="space-y-6">
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

      {!hasAnything ? (
        <div className="flex flex-col items-center justify-center py-16 text-muted-foreground">
          <Inbox className="h-12 w-12 mb-4 opacity-50" />
          <p className="text-lg font-medium mb-1">
            {sessions.length === 0 && workspaces.length === 0
              ? "No workspaces yet"
              : "No matching workspaces"}
          </p>
          <p className="text-sm">
            {sessions.length === 0 && workspaces.length === 0 ? (
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
        <>
          {/* Active sessions */}
          {filtered.length > 0 && (
            <div className="grid gap-3">
              {filtered.map((s) => (
                <WorkspaceCard
                  key={s.id}
                  session={s}
                  onStop={(id) => setConfirmAction({ type: "stop", id })}
                  onRestart={handleRestart}
                  onDelete={(id) => setConfirmAction({ type: "delete", id })}
                />
              ))}
            </div>
          )}

          {/* Saved workspaces (no active session) */}
          {filteredSaved.length > 0 && statusFilter === "all" && (
            <div>
              <h3 className="text-sm font-medium text-muted-foreground mb-3 flex items-center gap-2">
                <HardDrive className="h-3.5 w-3.5" />
                Saved Workspaces
              </h3>
              <div className="grid gap-3">
                {filteredSaved.map((ws) => (
                  <Card
                    key={ws.id}
                    className="bg-card border-border border-dashed hover:border-primary/40 transition-colors"
                  >
                    <CardContent className="p-5">
                      <div className="flex items-center justify-between gap-3">
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-2.5 mb-1.5">
                            <span className="font-medium">{ws.id}</span>
                            <Badge variant="outline" className="text-xs">
                              Saved
                            </Badge>
                          </div>
                          <div className="flex items-center gap-3 text-xs text-muted-foreground">
                            {ws.agent && (
                              <span className="font-mono bg-muted/50 px-1.5 py-0.5 rounded">
                                {ws.agent}
                              </span>
                            )}
                            <span>
                              {new Date(ws.created_at).toLocaleDateString()}
                            </span>
                          </div>
                        </div>
                        <div className="flex items-center gap-1.5 shrink-0">
                          <Button
                            size="sm"
                            onClick={() => handleResume(ws)}
                            disabled={resumingId === ws.id}
                          >
                            {resumingId === ws.id ? (
                              <Loader2 className="h-3.5 w-3.5 mr-1.5 animate-spin" />
                            ) : (
                              <Play className="h-3.5 w-3.5 mr-1.5" />
                            )}
                            {resumingId === ws.id ? "Starting..." : "Resume"}
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-8 w-8 text-destructive hover:text-destructive"
                            onClick={() => handleDeleteWorkspace(ws.id)}
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
                        </div>
                      </div>
                    </CardContent>
                  </Card>
                ))}
              </div>
            </div>
          )}
        </>
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
