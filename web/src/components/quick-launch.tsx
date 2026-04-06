"use client";

import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Terminal, Play, FolderGit2 } from "lucide-react";

interface QuickLaunchProps {
  agent: string;
  repoUrl?: string;
  workspaceId?: string;
  onRunCommand: (command: string) => void;
}

const agentCommands: Record<string, { cmd: string; label: string }> = {
  claude: { cmd: "claude", label: "Claude Code" },
  opencode: { cmd: "opencode", label: "OpenCode" },
  codex: { cmd: "codex", label: "Codex CLI" },
  qwen: { cmd: "qwen", label: "Qwen Code" },
};

export function QuickLaunch({
  agent,
  repoUrl,
  workspaceId,
  onRunCommand,
}: QuickLaunchProps) {
  if (agent === "shell") return null;

  const known = agentCommands[agent];
  const cmd = known?.cmd || agent;
  const label = known?.label || agent;

  return (
    <div className="border-b border-border bg-card/50 px-4 py-2.5">
      <div className="flex items-center justify-between max-w-7xl mx-auto">
        <div className="flex items-center gap-3 text-sm">
          <div className="flex items-center gap-2">
            <Terminal className="h-3.5 w-3.5 text-muted-foreground" />
            <span className="text-muted-foreground">Agent:</span>
            <Badge variant="secondary" className="font-mono text-xs">
              {label}
            </Badge>
          </div>

          {repoUrl && (
            <div className="flex items-center gap-1.5 text-muted-foreground">
              <FolderGit2 className="h-3.5 w-3.5" />
              <span className="text-xs truncate max-w-[200px]">
                {repoUrl.split("/").pop()?.replace(".git", "")}
              </span>
            </div>
          )}
        </div>

        <Button
          size="sm"
          variant="outline"
          className="h-7 text-xs gap-1.5"
          onClick={() => onRunCommand(cmd + "\n")}
        >
          <Play className="h-3 w-3" />
          Run {cmd}
        </Button>
      </div>
    </div>
  );
}
