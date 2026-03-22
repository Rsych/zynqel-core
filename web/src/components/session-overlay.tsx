"use client";

import { Button } from "@/components/ui/button";
import { Play, ArrowLeft, Loader2, CircleStop, AlertTriangle } from "lucide-react";

interface SessionOverlayProps {
  status: "stopped" | "error";
  error?: string;
  restarting: boolean;
  onRestart: () => void;
  onBack: () => void;
}

export function SessionOverlay({
  status,
  error,
  restarting,
  onRestart,
  onBack,
}: SessionOverlayProps) {
  const isError = status === "error";

  return (
    <div className="absolute inset-0 z-20 flex items-center justify-center">
      {/* Blurred backdrop */}
      <div className="absolute inset-0 bg-background/70 backdrop-blur-sm" />

      {/* Card */}
      <div className="relative w-full max-w-sm mx-4">
        <div className="rounded-xl border border-border bg-card/95 backdrop-blur-md shadow-2xl shadow-black/40 overflow-hidden">
          {/* Accent strip */}
          <div
            className={`h-1 ${isError ? "bg-destructive" : "bg-emerald-500/60"}`}
          />

          <div className="px-6 py-8 text-center">
            {/* Icon */}
            <div
              className={`mx-auto mb-4 h-12 w-12 rounded-full flex items-center justify-center ${
                isError
                  ? "bg-destructive/10 text-destructive"
                  : "bg-emerald-500/10 text-emerald-400"
              }`}
            >
              {isError ? (
                <AlertTriangle className="h-6 w-6" />
              ) : (
                <CircleStop className="h-6 w-6" />
              )}
            </div>

            {/* Title */}
            <h3 className="text-lg font-semibold mb-2">
              {isError ? "Session Error" : "Session Stopped"}
            </h3>

            {/* Message */}
            <p className="text-sm text-muted-foreground mb-1">
              {isError
                ? "Something went wrong with this session."
                : "This workspace has been stopped."}
            </p>
            {error && (
              <p className="text-xs text-destructive font-mono bg-destructive/5 rounded-md px-3 py-2 mt-2 mb-1 break-all">
                {error}
              </p>
            )}
            <p className="text-xs text-muted-foreground mt-2">
              Your workspace data is saved and can be resumed.
            </p>

            {/* Actions */}
            <div className="flex items-center justify-center gap-3 mt-6">
              <Button variant="outline" size="sm" onClick={onBack}>
                <ArrowLeft className="h-3.5 w-3.5 mr-1.5" />
                Back
              </Button>
              <Button size="sm" onClick={onRestart} disabled={restarting}>
                {restarting ? (
                  <Loader2 className="h-3.5 w-3.5 mr-1.5 animate-spin" />
                ) : (
                  <Play className="h-3.5 w-3.5 mr-1.5" />
                )}
                {restarting ? "Starting..." : "Start"}
              </Button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
