"use client";

import { useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";

export function TerminalView({ sessionId }: { sessionId: string }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<import("@xterm/xterm").Terminal | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const [status, setStatus] = useState<"connecting" | "connected" | "disconnected">(
    "connecting"
  );

  useEffect(() => {
    let disposed = false;

    async function init() {
      const { Terminal } = await import("@xterm/xterm");
      const { FitAddon } = await import("@xterm/addon-fit");
      const { WebLinksAddon } = await import("@xterm/addon-web-links");
      await import("@xterm/xterm/css/xterm.css");

      if (disposed || !containerRef.current) return;

      const term = new Terminal({
        cursorBlink: true,
        fontSize: 13,
        fontFamily: "'Menlo', 'DejaVu Sans Mono', 'Consolas', 'Liberation Mono', monospace",
        theme: {
          background: "#0a0a0a",
          foreground: "#e5e5e5",
          cursor: "#10b981",
          selectionBackground: "#10b98133",
          black: "#171717",
          red: "#ef4444",
          green: "#10b981",
          yellow: "#eab308",
          blue: "#3b82f6",
          magenta: "#a855f7",
          cyan: "#06b6d4",
          white: "#e5e5e5",
          brightBlack: "#404040",
          brightRed: "#f87171",
          brightGreen: "#34d399",
          brightYellow: "#facc15",
          brightBlue: "#60a5fa",
          brightMagenta: "#c084fc",
          brightCyan: "#22d3ee",
          brightWhite: "#fafafa",
        },
      });

      const fitAddon = new FitAddon();
      term.loadAddon(fitAddon);
      term.loadAddon(new WebLinksAddon());
      term.open(containerRef.current);
      fitAddon.fit();
      termRef.current = term;

      // WebSocket connection
      const url = api.streamURL(sessionId);
      const ws = new WebSocket(url);
      wsRef.current = ws;

      ws.onopen = () => {
        if (disposed) return;
        setStatus("connected");
        // Send resize
        ws.send(
          JSON.stringify({
            type: "pty.resize",
            cols: term.cols,
            rows: term.rows,
          })
        );
      };

      ws.onmessage = (evt) => {
        try {
          const msg = JSON.parse(evt.data);
          if (msg.type === "pty.output" && msg.data) {
            term.write(atob(msg.data));
          } else if (msg.type === "session.state") {
            // initial state
          }
        } catch {
          // ignore non-JSON messages
        }
      };

      ws.onclose = () => {
        if (!disposed) setStatus("disconnected");
      };

      ws.onerror = () => {
        if (!disposed) setStatus("disconnected");
      };

      // Terminal input → WebSocket
      term.onData((data) => {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(
            JSON.stringify({
              type: "pty.input",
              data: btoa(data),
            })
          );
        }
      });

      // Resize handling
      const resizeObserver = new ResizeObserver(() => {
        fitAddon.fit();
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(
            JSON.stringify({
              type: "pty.resize",
              cols: term.cols,
              rows: term.rows,
            })
          );
        }
      });
      if (containerRef.current) {
        resizeObserver.observe(containerRef.current);
      }

      return () => {
        resizeObserver.disconnect();
      };
    }

    const cleanup = init();

    return () => {
      disposed = true;
      cleanup.then((fn) => fn?.());
      wsRef.current?.close();
      termRef.current?.dispose();
    };
  }, [sessionId]);

  return (
    <div className="relative h-full">
      {status === "connecting" && (
        <div className="absolute inset-0 flex items-center justify-center bg-background/80 z-10">
          <span className="text-sm text-muted-foreground">Connecting...</span>
        </div>
      )}
      {status === "disconnected" && (
        <div className="absolute inset-0 flex items-center justify-center bg-background/80 z-10">
          <span className="text-sm text-destructive">Disconnected</span>
        </div>
      )}
      <div ref={containerRef} className="h-full w-full" />
    </div>
  );
}
