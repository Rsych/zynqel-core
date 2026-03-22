"use client";

import { Suspense, useEffect, useState } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import Link from "next/link";
import { api } from "@/lib/api";
import type { Session } from "@/lib/types";
import { TerminalView } from "@/components/terminal-view";
import { ResourceBars } from "@/components/resource-bars";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import {
  ArrowLeft,
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

  useEffect(() => {
    if (!id) {
      setLoading(false);
      return;
    }
    api
      .getSession(id)
      .then(setSession)
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [id]);

  const handleDelete = async () => {
    if (!id) return;
    try {
      await api.deleteSession(id);
      router.push("/");
    } catch (err) {
      console.error("Failed to delete session:", err);
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
    <div className="min-h-screen flex flex-col">
      {/* Header */}
      <header className="border-b border-border bg-background/80 backdrop-blur-sm sticky top-0 z-50">
        <div className="mx-auto max-w-7xl flex items-center justify-between px-6 h-14">
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
            <Button variant="destructive" size="sm" onClick={handleDelete}>
              {isRunning ? (
                <>
                  <Square className="h-3.5 w-3.5 mr-1.5" />
                  Stop
                </>
              ) : (
                <>
                  <Trash2 className="h-3.5 w-3.5 mr-1.5" />
                  Delete
                </>
              )}
            </Button>
          </div>
        </div>
      </header>

      {/* Content */}
      <div className="flex-1 flex flex-col">
        <Tabs defaultValue="terminal" className="flex-1 flex flex-col">
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

          <TabsContent value="terminal" className="flex-1 m-0 p-0">
            {isRunning ? (
              <div className="h-[calc(100vh-7.5rem)]">
                <TerminalView sessionId={id} />
              </div>
            ) : (
              <div className="flex items-center justify-center h-64 text-muted-foreground">
                Session is not running
              </div>
            )}
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
