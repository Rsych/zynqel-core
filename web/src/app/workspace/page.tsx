"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import Link from "next/link";
import { api, isAPIError } from "@/lib/api";
import type { Session } from "@/lib/types";
import { toast } from "sonner";
import { TerminalView, type TerminalViewHandle } from "@/components/terminal-view";
import { ResourceBars } from "@/components/resource-bars";
import { SessionOverlay } from "@/components/session-overlay";
import { QuickLaunch } from "@/components/quick-launch";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import {
  ArrowLeft,
  Loader2,
  Play,
  Square,
  Trash2,
  Terminal,
  BarChart3,
  Info,
} from "lucide-react";

const statusConfig: Record<
  string,
  { label: string; variant: "default" | "secondary" | "destructive" | "outline" }
> = {
  running: { label: "Running", variant: "default" },
  pending: { label: "Pending", variant: "secondary" },
  stopped: { label: "Stopped", variant: "outline" },
  error: { label: "Error", variant: "destructive" },
};

function WorkspaceDetail() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const id = searchParams.get("id");
  const [session, setSession] = useState<Session | null>(null);
  const [loading, setLoading] = useState(true);
  const [confirmStop, setConfirmStop] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [restarting, setRestarting] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const terminalRef = useRef<TerminalViewHandle>(null);
  const deletingRef = useRef(false);
  const mountedRef = useRef(true);

  useEffect(() => {
    return () => {
      mountedRef.current = false;
    };
  }, []);

  useEffect(() => {
    if (!id) {
      setLoading(false);
      return;
    }

    let active = true;

    api
      .getSession(id)
      .then((s) => {
        if (active) setSession(s);
      })
      .catch(() => {
        // Session may have been deleted in another tab/process.
        if (active) setSession(null);
      })
      .finally(() => {
        if (active) setLoading(false);
      });

    // Poll session status to detect stop/error from outside (agent exit, timeout).
    // Intentionally read deletingRef.current in the interval callback so we don't
    // need to recreate the timer when deleting state changes.
    const POLL_INTERVAL = 5000;
    const interval = setInterval(() => {
      if (deletingRef.current) return;
      api
        .getSession(id)
        .then((s) => {
          if (active) setSession(s);
        })
        .catch((err: unknown) => {
          // 404 after delete/restart is expected; avoid noisy dev-console errors.
          if (isAPIError(err) && err.status === 404) {
            if (active) setSession(null);
            return;
          }
          if (!isAPIError(err) || err.status !== 404) {
            console.warn("Failed to poll session:", err);
          }
        });
    }, POLL_INTERVAL);
    return () => {
      active = false;
      clearInterval(interval);
    };
  }, [id]);

  useEffect(() => {
    const name = session?.spec.workspace_id || session?.id?.slice(0, 8);
    document.title = name ? `${name} — Zynqel` : "Zynqel Console";
  }, [session?.spec.workspace_id, session?.id]);

  const handleStop = async () => {
    if (!id || !session) return;
    setSession({ ...session, status: "stopped" });
    try {
      const updated = await api.stopSession(id);
      setSession(updated);
      toast.success("Workspace stopped");
    } catch (err) {
      setSession(session);
      toast.error("Failed to stop workspace");
    }
  };

  const handleRestart = async () => {
    if (!id) return;
    setRestarting(true);
    toast.loading("Starting workspace...", { id: "restart" });
    try {
      const newSession = await api.restartSession(id);
      toast.success("Workspace started", { id: "restart" });
      router.push(`/workspace?id=${newSession.id}`);
    } catch (err) {
      toast.error("Failed to start workspace", { id: "restart" });
    } finally {
      setRestarting(false);
    }
  };

  const handleDelete = async () => {
    if (!id) return;
    setDeleting(true);
    deletingRef.current = true;
    toast.loading("Removing workspace...", { id: "delete" });
    try {
      await api.deleteSession(id);
      toast.success("Workspace removed", { id: "delete" });
      router.push("/");
    } catch (err) {
      try {
        const refreshed = await api.getSession(id);
        setSession(refreshed);
      } catch {
        // Keep current fallback behavior if refetch fails.
      }
      toast.error("Failed to remove workspace", { id: "delete" });
    } finally {
      if (mountedRef.current) {
        setDeleting(false);
      }
      deletingRef.current = false;
    }
  };

  if (loading) {
    return (
      <div className="min-h-screen p-6">
        <Skeleton className="h-8 w-48 mb-4" />
        <Skeleton className="h-[600px] w-full" />
      </div>
    );
  }

  if (!id || !session) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="text-center">
          <p className="text-lg font-medium mb-2">Session not found</p>
          <Link href="/" className="text-primary hover:underline text-sm">
            Back to workspaces
          </Link>
        </div>
      </div>
    );
  }

  const status = statusConfig[session.status] || statusConfig.stopped;
  const isRunning = session.status === "running";

  return (
    <div className="h-screen flex flex-col overflow-hidden">
      {/* Header */}
      <header className="border-b border-border bg-background/80 backdrop-blur-sm sticky top-0 z-50">
        <div className="mx-auto max-w-7xl flex items-center justify-between px-3 sm:px-6 h-14">
          <div className="flex items-center gap-3">
            <Button variant="ghost" size="icon" className="h-8 w-8" asChild>
              <Link href="/">
                <ArrowLeft className="h-4 w-4" />
              </Link>
            </Button>
            <div className="flex items-center gap-2.5">
              {isRunning && (
                <span className="relative flex h-2 w-2">
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
                  <span className="relative inline-flex h-2 w-2 rounded-full bg-emerald-500" />
                </span>
              )}
              <span className="font-medium">
                {session.spec.workspace_id || session.id.slice(0, 8)}
              </span>
              <Badge variant={status.variant} className="text-xs">
                {status.label}
              </Badge>
            </div>
          </div>

          <div className="flex items-center gap-2">
            {!isRunning && (
              <Button size="sm" onClick={handleRestart} disabled={restarting}>
                {restarting ? (
                  <Loader2 className="h-3.5 w-3.5 sm:mr-1.5 animate-spin" />
                ) : (
                  <Play className="h-3.5 w-3.5 sm:mr-1.5" />
                )}
                <span className="hidden sm:inline">{restarting ? "Starting..." : "Start"}</span>
              </Button>
            )}
            {isRunning && (
              <Button variant="secondary" size="sm" onClick={() => setConfirmStop(true)}>
                <Square className="h-3.5 w-3.5 sm:mr-1.5" />
                <span className="hidden sm:inline">Stop</span>
              </Button>
            )}
            <Button variant="destructive" size="sm" onClick={() => setConfirmDelete(true)}>
              <Trash2 className="h-3.5 w-3.5 sm:mr-1.5" />
              <span className="hidden sm:inline">Remove</span>
            </Button>
          </div>
        </div>
      </header>

      {/* Content */}
      <div className="flex-1 flex flex-col min-h-0 overflow-hidden">
        <Tabs defaultValue="terminal" className="flex-1 flex flex-col min-h-0">
          <div className="border-b border-border px-6">
            <TabsList className="bg-transparent h-10 p-0 gap-4">
              <TabsTrigger
                value="terminal"
                className="bg-transparent data-[state=active]:bg-transparent data-[state=active]:shadow-none data-[state=active]:border-b-2 data-[state=active]:border-primary rounded-none px-1 pb-2.5"
              >
                <Terminal className="h-4 w-4 mr-1.5" />
                Terminal
              </TabsTrigger>
              <TabsTrigger
                value="resources"
                className="bg-transparent data-[state=active]:bg-transparent data-[state=active]:shadow-none data-[state=active]:border-b-2 data-[state=active]:border-primary rounded-none px-1 pb-2.5"
              >
                <BarChart3 className="h-4 w-4 mr-1.5" />
                Resources
              </TabsTrigger>
              <TabsTrigger
                value="info"
                className="bg-transparent data-[state=active]:bg-transparent data-[state=active]:shadow-none data-[state=active]:border-b-2 data-[state=active]:border-primary rounded-none px-1 pb-2.5"
              >
                <Info className="h-4 w-4 mr-1.5" />
                Info
              </TabsTrigger>
            </TabsList>
          </div>

          <TabsContent value="terminal" className="flex-1 m-0 p-0 flex flex-col overflow-hidden">
            {isRunning && session.spec.agent !== "shell" && (
              <QuickLaunch
                agent={session.spec.agent}
                repoUrl={session.spec.repo_url}
                workspaceId={session.spec.workspace_id}
                onRunCommand={(cmd) => terminalRef.current?.sendInput(cmd)}
              />
            )}
            <div className="relative flex-1 min-h-0">
              <TerminalView ref={terminalRef} sessionId={id} />
              {!isRunning && (
                <SessionOverlay
                  status={session.status as "stopped" | "error"}
                  error={session.error}
                  restarting={restarting}
                  onRestart={handleRestart}
                  onRemove={() => setConfirmDelete(true)}
                  onBack={() => router.push("/")}
                />
              )}
            </div>
          </TabsContent>

          <TabsContent value="resources" className="m-0 px-6 py-4">
            <div className="max-w-lg">
              <h2 className="text-lg font-medium mb-4">Resource Usage</h2>
              {isRunning ? (
                <ResourceBars sessionId={id} />
              ) : (
                <p className="text-sm text-muted-foreground">
                  Stats are only available for running sessions.
                </p>
              )}
            </div>
          </TabsContent>

          <TabsContent value="info" className="m-0 px-6 py-4">
            <div className="max-w-lg space-y-4">
              <h2 className="text-lg font-medium mb-4">Session Info</h2>
              <InfoRow label="Session ID" value={session.id} mono />
              <Separator />
              <InfoRow
                label="Workspace"
                value={session.spec.workspace_id || "—"}
              />
              <Separator />
              <InfoRow label="Agent" value={session.spec.agent} />
              <Separator />
              {session.container_id && (
                <>
                  <InfoRow
                    label="Container"
                    value={session.container_id.slice(0, 12)}
                    mono
                  />
                  <Separator />
                </>
              )}
              {session.spec.repo_url && (
                <>
                  <InfoRow label="Repository" value={session.spec.repo_url} />
                  <Separator />
                </>
              )}
              {session.spec.branch && (
                <>
                  <InfoRow label="Branch" value={session.spec.branch} />
                  <Separator />
                </>
              )}
              <InfoRow
                label="Created"
                value={new Date(session.created_at).toLocaleString()}
              />
              {session.stopped_at && (
                <>
                  <Separator />
                  <InfoRow
                    label="Stopped"
                    value={new Date(session.stopped_at).toLocaleString()}
                  />
                </>
              )}
              {session.error && (
                <>
                  <Separator />
                  <InfoRow label="Error" value={session.error} />
                </>
              )}
            </div>
          </TabsContent>
        </Tabs>
      </div>

      <ConfirmDialog
        open={confirmStop}
        onOpenChange={setConfirmStop}
        title="Stop workspace?"
        description="The workspace will be stopped and its state saved. You can resume it later by creating a new session with the same workspace ID."
        confirmLabel="Stop"
        onConfirm={handleStop}
      />
      <ConfirmDialog
        open={confirmDelete}
        onOpenChange={setConfirmDelete}
        title="Remove workspace?"
        description="This will stop the session and remove the container. Workspace data on the volume is preserved, but the container and any uncommitted state will be lost."
        confirmLabel="Remove"
        confirmDisabled={deleting}
        variant="destructive"
        onConfirm={handleDelete}
      />
    </div>
  );
}

function InfoRow({
  label,
  value,
  mono,
}: {
  label: string;
  value: string;
  mono?: boolean;
}) {
  return (
    <div className="flex items-center justify-between py-1">
      <span className="text-sm text-muted-foreground">{label}</span>
      <span
        className={`text-sm ${mono ? "font-mono bg-muted/50 px-2 py-0.5 rounded" : ""}`}
      >
        {value}
      </span>
    </div>
  );
}

export default function WorkspacePage() {
  return (
    <Suspense
      fallback={
        <div className="min-h-screen p-6">
          <Skeleton className="h-8 w-48 mb-4" />
          <Skeleton className="h-[600px] w-full" />
        </div>
      }
    >
      <WorkspaceDetail />
    </Suspense>
  );
}
