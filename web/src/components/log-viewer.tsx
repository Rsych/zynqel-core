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
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!open || !sessionId) return;

    let mounted = true;
    setLoading(true);

    async function loadLog() {
      const [logData, { Terminal }] = await Promise.all([
        api.getSessionLog(sessionId),
        import("@xterm/xterm"),
      ]);

      if (!mounted || !containerRef.current) return;

      // Clean up any existing terminal.
      if (termRef.current) {
        termRef.current.dispose();
      }

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
      termRef.current = term;
      term.open(containerRef.current);

      if (logData) {
        term.write(logData);
      } else {
        term.write("\x1b[2mNo log data available\x1b[0m");
      }

      setLoading(false);
    }

    loadLog().catch(() => setLoading(false));

    return () => {
      mounted = false;
      if (termRef.current) {
        termRef.current.dispose();
        termRef.current = null;
      }
    };
  }, [open, sessionId]);

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
          <div ref={containerRef} className="h-full w-full" />
        </div>
      </DialogContent>
    </Dialog>
  );
}
