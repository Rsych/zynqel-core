"use client";

import { useEffect, useRef, useState, useImperativeHandle, forwardRef } from "react";
import { api } from "@/lib/api";

// UTF-8 safe base64 encode (btoa only handles Latin-1).
function utf8ToBase64(str: string): string {
  const bytes = new TextEncoder().encode(str);
  let binary = "";
  for (let i = 0; i < bytes.length; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary);
}

export interface TerminalViewHandle {
  sendInput: (text: string) => void;
}

export const TerminalView = forwardRef<TerminalViewHandle, { sessionId: string }>(function TerminalView({ sessionId }, ref) {
  const containerRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<import("@xterm/xterm").Terminal | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const reconnectAttempts = useRef(0);
  const maxReconnectAttempts = 3;
  const [status, setStatus] = useState<"connecting" | "connected" | "disconnected">(
    "connecting"
  );

  useImperativeHandle(ref, () => ({
    sendInput: (text: string) => {
      const ws = wsRef.current;
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(
          JSON.stringify({
            type: "pty.input",
            data: utf8ToBase64(text),
          })
        );
      }
      termRef.current?.focus();
    },
  }));

  useEffect(() => {
    let disposed = false;
    let fitAddon: import("@xterm/addon-fit").FitAddon | null = null;
    let resizeObserver: ResizeObserver | null = null;

    async function init() {
      const { Terminal } = await import("@xterm/xterm");
      const { FitAddon } = await import("@xterm/addon-fit");
      const { WebLinksAddon } = await import("@xterm/addon-web-links");
      await import("@xterm/xterm/css/xterm.css");

      if (disposed || !containerRef.current) return;

      const term = new Terminal({
        cursorBlink: true,
        fontSize: 13,
        scrollback: 5000,
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

      fitAddon = new FitAddon();
      term.loadAddon(fitAddon);
      term.loadAddon(new WebLinksAddon());
      term.open(containerRef.current);
      fitAddon.fit();
      termRef.current = term;

      // Terminal input → WebSocket
      term.onData((data) => {
        const ws = wsRef.current;
        if (ws && ws.readyState === WebSocket.OPEN) {
          ws.send(
            JSON.stringify({
              type: "pty.input",
              data: utf8ToBase64(data),
            })
          );
        }
      });

      // Resize handling — use rAF to let xterm measure after layout settles.
      resizeObserver = new ResizeObserver(() => {
        requestAnimationFrame(() => {
          fitAddon?.fit();
          const ws = wsRef.current;
          if (ws && ws.readyState === WebSocket.OPEN) {
            ws.send(
              JSON.stringify({
                type: "pty.resize",
                data: { cols: term.cols, rows: term.rows },
              })
            );
          }
        });
      });
      if (containerRef.current) {
        resizeObserver.observe(containerRef.current);
      }

      // Start WebSocket connection (with auto-reconnect).
      connect(term);
    }

    function connect(term: import("@xterm/xterm").Terminal) {
      if (disposed) return;

      const url = api.streamURL(sessionId);
      const ws = new WebSocket(url);
      wsRef.current = ws;

      const utf8Decoder = new TextDecoder();

      ws.onopen = () => {
        if (disposed) return;
        reconnectAttempts.current = 0;
        setStatus("connected");
        fitAddon?.fit();
        ws.send(
          JSON.stringify({
            type: "pty.resize",
            data: { cols: term.cols, rows: term.rows },
          })
        );
      };

      ws.onmessage = (evt) => {
        try {
          const msg = JSON.parse(evt.data);
          if (msg.type === "pty.output" && msg.data) {
            const bytes = Uint8Array.from(atob(msg.data), (c) => c.charCodeAt(0));
            term.write(utf8Decoder.decode(bytes));
          }
        } catch {
          // ignore non-JSON
        }
      };

      ws.onclose = () => {
        if (disposed) return;
        reconnectAttempts.current++;
        if (reconnectAttempts.current > maxReconnectAttempts) {
          setStatus("disconnected");
          return;
        }
        // Exponential backoff: 1s, 2s, 4s.
        const delay = Math.min(1000 * Math.pow(2, reconnectAttempts.current - 1), 4000);
        setStatus("connecting");
        reconnectTimer.current = setTimeout(() => {
          if (!disposed) connect(term);
        }, delay);
      };

      ws.onerror = (e) => {
        console.error("WebSocket error:", e);
      };
    }

    init();

    return () => {
      disposed = true;
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current);
      resizeObserver?.disconnect();
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
      <div ref={containerRef} className="h-full w-full overflow-hidden p-2" />
    </div>
  );
});
