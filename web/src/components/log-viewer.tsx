"use client";

import { useEffect, useRef, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { api } from "@/lib/api";
import { Loader2 } from "lucide-react";

interface LogViewerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  sessionId: string;
}

export function LogViewer({ open, onOpenChange, sessionId }: LogViewerProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<import("@xterm/xterm").Terminal | null>(null);
  const fitRef = useRef<import("@xterm/addon-fit").FitAddon | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open || !sessionId) return;

    let mounted = true;
    setLoading(true);
    setError(null);

    async function loadLog() {
      const [logData, { Terminal }, { FitAddon }] = await Promise.all([
        api.getSessionLog(sessionId),
        import("@xterm/xterm"),
        import("@xterm/addon-fit"),
        import("@xterm/xterm/css/xterm.css"), // side-effect: loads xterm styles
      ]);

      if (!mounted || !containerRef.current) return;

      // Clean up any existing terminal.
      if (termRef.current) {
        termRef.current.dispose();
      }

      const fitAddon = new FitAddon();
      fitRef.current = fitAddon;

      const term = new Terminal({
        fontSize: 13,
        fontFamily: '"JetBrains Mono", "Fira Code", monospace',
        theme: {
          background: "#09090b",
          foreground: "#fafafa",
          cursor: "#09090b", // hide cursor
        },
        disableStdin: true,
        cursorBlink: false,
        cursorStyle: "bar",
        scrollback: 50000,
        convertEol: true,
      });
      term.loadAddon(fitAddon);
      termRef.current = term;
      term.open(containerRef.current);
      fitAddon.fit();

      if (logData) {
        term.write(logData);
      } else {
        term.write("\x1b[2mNo log data available\x1b[0m");
      }

      setLoading(false);
    }

    loadLog().catch((err) => {
      if (mounted) {
        setError("Failed to load session log");
        setLoading(false);
        console.error("Log viewer error:", err);
      }
    });

    return () => {
      mounted = false;
      if (termRef.current) {
        termRef.current.dispose();
        termRef.current = null;
      }
      fitRef.current = null;
    };
  }, [open, sessionId]);

  // Re-fit terminal when dialog resizes.
  useEffect(() => {
    if (!open) return;
    const observer = new ResizeObserver(() => {
      fitRef.current?.fit();
    });
    if (containerRef.current) {
      observer.observe(containerRef.current);
    }
    return () => observer.disconnect();
  }, [open]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-4xl h-[80vh] flex flex-col">
        <DialogHeader>
          <DialogTitle className="font-mono text-sm">
            Session Log — {sessionId.slice(0, 8)}
          </DialogTitle>
        </DialogHeader>
        <div className="flex-1 relative rounded-md overflow-hidden bg-[#09090b]">
          {loading && (
            <div className="absolute inset-0 flex items-center justify-center z-10">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          )}
          {error && (
            <div className="absolute inset-0 flex items-center justify-center z-10 text-sm text-muted-foreground">
              {error}
            </div>
          )}
          <div ref={containerRef} className="h-full w-full" />
        </div>
      </DialogContent>
    </Dialog>
  );
}
