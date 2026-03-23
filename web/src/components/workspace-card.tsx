"use client";

import Link from "next/link";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { MoreVertical, ExternalLink, Square, Trash2, Play } from "lucide-react";
import type { Session } from "@/lib/types";

const statusConfig: Record<
  string,
  { label: string; variant: "default" | "secondary" | "destructive" | "outline" }
> = {
  running: { label: "Running", variant: "default" },
  pending: { label: "Pending", variant: "secondary" },
  stopped: { label: "Stopped", variant: "outline" },
  error: { label: "Error", variant: "destructive" },
};

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

interface WorkspaceCardProps {
  session: Session;
  onStop: (id: string) => void;
  onRestart: (id: string) => void;
  onDelete: (id: string) => void;
}

export function WorkspaceCard({ session, onStop, onRestart, onDelete }: WorkspaceCardProps) {
  const status = statusConfig[session.status] || statusConfig.stopped;
  const isRunning = session.status === "running";
  const isStopped = session.status === "stopped";

  return (
    <Card className="group relative bg-card border-border hover:border-primary/40 transition-colors">
      <CardContent className="p-5">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2.5 mb-1.5">
              {isRunning && (
                <span className="relative flex h-2.5 w-2.5">
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
                  <span className="relative inline-flex h-2.5 w-2.5 rounded-full bg-emerald-500" />
                </span>
              )}
              <Link
                href={`/workspace?id=${session.id}`}
                className="font-medium truncate hover:underline"
              >
                {session.spec.workspace_id || session.id.slice(0, 8)}
              </Link>
              <Badge variant={status.variant} className="text-xs shrink-0">
                {status.label}
              </Badge>
            </div>

            <div className="flex items-center gap-3 text-xs text-muted-foreground">
              <span className="font-mono bg-muted/50 px-1.5 py-0.5 rounded">
                {session.spec.agent}
              </span>
              {session.spec.repo_url && (
                <span className="truncate max-w-[200px]">
                  {session.spec.repo_url.split("/").pop()?.replace(".git", "")}
                </span>
              )}
              <span>{timeAgo(session.created_at)}</span>
            </div>
          </div>

          <div className="flex items-center gap-1.5 shrink-0">
            {isRunning && (
              <Button variant="ghost" size="icon" className="h-8 w-8" asChild>
                <Link href={`/workspace?id=${session.id}`}>
                  <ExternalLink className="h-4 w-4" />
                </Link>
              </Button>
            )}
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon" className="h-8 w-8">
                  <MoreVertical className="h-4 w-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                {isRunning && (
                  <DropdownMenuItem asChild>
                    <Link href={`/workspace?id=${session.id}`}>
                      <ExternalLink className="h-4 w-4 mr-2" />
                      Open Terminal
                    </Link>
                  </DropdownMenuItem>
                )}
                {isStopped && (
                  <DropdownMenuItem onClick={() => onRestart(session.id)}>
                    <Play className="h-4 w-4 mr-2" />
                    Start
                  </DropdownMenuItem>
                )}
                {isRunning && (
                  <DropdownMenuItem onClick={() => onStop(session.id)}>
                    <Square className="h-4 w-4 mr-2" />
                    Stop
                  </DropdownMenuItem>
                )}
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  onClick={() => onDelete(session.id)}
                  className="text-destructive focus:text-destructive"
                >
                  <Trash2 className="h-4 w-4 mr-2" />
                  Remove
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
