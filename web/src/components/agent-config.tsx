"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { AgentConfig } from "@/lib/types";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Skeleton } from "@/components/ui/skeleton";
import { Plus, Pencil, Trash2, Loader2, Terminal, Info } from "lucide-react";

const DEFAULT_DOCKERFILE = `FROM zynqel-base:latest

# Uncomment ONE of these and replace with your agent:
# RUN npm install -g @your-org/your-agent
# RUN pip install your-agent
# RUN apt-get update && apt-get install -y your-agent`;

const DOCKERFILE_EXAMPLES: Record<string, { dockerfile: string; command: string }> = {
  aider: {
    dockerfile: `FROM zynqel-base:latest
RUN pip install aider-chat`,
    command: "aider",
  },
  codex: {
    dockerfile: `FROM zynqel-base:latest
RUN npm install -g @openai/codex`,
    command: "codex",
  },
  qwen: {
    dockerfile: `FROM zynqel-base:latest
RUN npm install -g @qwen-code/qwen-code@latest`,
    command: "qwen",
  },
};

export function AgentConfigList() {
  const [agents, setAgents] = useState<AgentConfig[]>([]);
  const [loading, setLoading] = useState(true);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<AgentConfig | null>(null);

  const fetchAgents = async () => {
    try {
      const data = await api.listAgents();
      setAgents(data || []);
    } catch {
      // retry on next manual refresh
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchAgents();
  }, []);

  const handleDelete = async (name: string) => {
    try {
      await api.deleteAgent(name);
      setAgents((prev) => prev.filter((a) => a.name !== name));
    } catch (err) {
      console.error("Failed to delete agent:", err);
    }
  };

  const handleEdit = (agent: AgentConfig) => {
    setEditing(agent);
    setDialogOpen(true);
  };

  const handleSaved = () => {
    setDialogOpen(false);
    setEditing(null);
    fetchAgents();
  };

  const builtins = agents.filter((a) => a.builtin);
  const custom = agents.filter((a) => !a.builtin);

  if (loading) {
    return (
      <div className="space-y-3">
        {[...Array(3)].map((_, i) => (
          <Skeleton key={i} className="h-12 w-full" />
        ))}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Built-in agents */}
      <div>
        <h3 className="text-sm font-medium text-muted-foreground mb-3">
          Built-in
        </h3>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Image</TableHead>
              <TableHead className="w-[100px]" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {builtins.map((agent) => (
              <TableRow key={agent.name}>
                <TableCell className="font-medium">
                  <div className="flex items-center gap-2">
                    <Terminal className="h-4 w-4 text-muted-foreground" />
                    {agent.name}
                    <Badge variant="secondary" className="text-xs">
                      built-in
                    </Badge>
                  </div>
                </TableCell>
                <TableCell className="font-mono text-sm text-muted-foreground">
                  {agent.image || "zynqel-base:latest"}
                </TableCell>
                <TableCell />
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      {/* Custom agents */}
      <div>
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-sm font-medium text-muted-foreground">Custom</h3>
          <Button
            size="sm"
            onClick={() => {
              setEditing(null);
              setDialogOpen(true);
            }}
          >
            <Plus className="h-4 w-4 mr-1.5" />
            Add Agent
          </Button>
        </div>

        {custom.length === 0 ? (
          <div className="rounded-lg border border-dashed border-border p-8 text-center">
            <Terminal className="h-8 w-8 mx-auto mb-3 text-muted-foreground opacity-50" />
            <p className="text-sm text-muted-foreground mb-1">
              No custom agents yet
            </p>
            <p className="text-xs text-muted-foreground">
              Add one to run any CLI agent tool in your workspaces
            </p>
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Command</TableHead>
                <TableHead>Image</TableHead>
                <TableHead className="w-[100px]" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {custom.map((agent) => (
                <TableRow key={agent.name}>
                  <TableCell className="font-medium">
                    <div className="flex items-center gap-2">
                      <Terminal className="h-4 w-4 text-muted-foreground" />
                      {agent.name}
                    </div>
                  </TableCell>
                  <TableCell className="font-mono text-sm text-muted-foreground">
                    {agent.command?.join(" ")}
                  </TableCell>
                  <TableCell className="font-mono text-sm text-muted-foreground">
                    {agent.image || "zynqel-base:latest"}
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8"
                        onClick={() => handleEdit(agent)}
                      >
                        <Pencil className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 text-destructive hover:text-destructive"
                        onClick={() => handleDelete(agent.name)}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </div>

      <AgentFormDialog
        open={dialogOpen}
        onOpenChange={(v) => {
          setDialogOpen(v);
          if (!v) setEditing(null);
        }}
        editing={editing}
        onSaved={handleSaved}
      />
    </div>
  );
}

function TipBox({ children }: { children: React.ReactNode }) {
  return (
    <div className="rounded-md border border-border bg-muted/30 px-3 py-2.5 text-xs text-muted-foreground">
      <div className="flex gap-2">
        <Info className="h-3.5 w-3.5 mt-0.5 shrink-0" />
        <div>{children}</div>
      </div>
    </div>
  );
}

function AgentFormDialog({
  open,
  onOpenChange,
  editing,
  onSaved,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  editing: AgentConfig | null;
  onSaved: () => void;
}) {
  const [name, setName] = useState("");
  const [command, setCommand] = useState("");
  const [dockerfile, setDockerfile] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [showCommandTip, setShowCommandTip] = useState(false);
  const [showDockerfileTip, setShowDockerfileTip] = useState(false);

  useEffect(() => {
    if (editing) {
      setName(editing.name);
      setCommand(editing.command?.join(" ") || "");
      setDockerfile(editing.dockerfile || "");
    } else {
      setName("");
      setCommand("");
      setDockerfile(DEFAULT_DOCKERFILE);
    }
    setError("");
    setShowCommandTip(false);
    setShowDockerfileTip(false);
  }, [editing, open]);

  // Auto-fill from known examples when name matches.
  const handleNameChange = (val: string) => {
    setName(val);
    if (!editing && val in DOCKERFILE_EXAMPLES) {
      const example = DOCKERFILE_EXAMPLES[val];
      setDockerfile(example.dockerfile);
      setCommand(example.command);
    }
  };

  const handleSave = async () => {
    setSaving(true);
    setError("");
    try {
      const cfg = {
        name,
        command: command.split(/\s+/).filter(Boolean),
        dockerfile: dockerfile.trim() || undefined,
      };
      if (editing) {
        await api.updateAgent(editing.name, cfg);
      } else {
        await api.createAgent(cfg);
      }
      onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save agent");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg max-h-[90vh] overflow-y-auto" onInteractOutside={(e) => e.preventDefault()}>
        <DialogHeader>
          <DialogTitle>{editing ? "Edit Agent" : "Add Agent"}</DialogTitle>
          <DialogDescription>
            {editing
              ? "Update this custom agent configuration."
              : "Install a CLI agent tool and configure how to launch it."}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          {/* Quick start hint */}
          {!editing && (
            <div className="rounded-md border border-primary/20 bg-primary/5 px-3 py-2.5 text-xs">
              <span className="font-medium">Quick start:</span>{" "}
              Type a known agent name ({Object.keys(DOCKERFILE_EXAMPLES).map((k, i) => (
                <span key={k}>
                  {i > 0 && ", "}
                  <button
                    className="text-primary hover:underline font-mono"
                    onClick={() => handleNameChange(k)}
                  >
                    {k}
                  </button>
                </span>
              ))}) to auto-fill the config.
            </div>
          )}

          <div className="space-y-2">
            <Label htmlFor="agent-name">Name</Label>
            <Input
              id="agent-name"
              placeholder="aider"
              value={name}
              onChange={(e) => handleNameChange(e.target.value)}
              disabled={!!editing}
            />
            <p className="text-xs text-muted-foreground">
              Lowercase alphanumeric, hyphens, underscores.
            </p>
          </div>

          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <Label htmlFor="agent-dockerfile">Dockerfile</Label>
              <button
                type="button"
                onClick={() => setShowDockerfileTip(!showDockerfileTip)}
                className="text-xs text-muted-foreground hover:text-foreground flex items-center gap-1"
              >
                <Info className="h-3 w-3" />
                {showDockerfileTip ? "Hide tips" : "How to write this"}
              </button>
            </div>
            {showDockerfileTip && (
              <TipBox>
                <p className="mb-2">
                  This is a standard Dockerfile. Each line that installs something
                  must start with <code className="bg-muted px-1 rounded font-bold">RUN</code>.
                </p>
                <p className="mb-1 font-medium">Examples:</p>
                <pre className="bg-muted/50 rounded p-2 mt-1 overflow-x-auto">
{`FROM zynqel-base:latest
RUN npm install -g @qwen-code/qwen-code@latest

FROM zynqel-base:latest
RUN pip install aider-chat

FROM zynqel-base:latest
RUN apt-get update && apt-get install -y vim`}
                </pre>
                <p className="mt-2">
                  <code className="bg-muted px-1 rounded">zynqel-base</code> includes
                  Node.js, Python, Git, curl, and common dev tools.
                </p>
              </TipBox>
            )}
            <Textarea
              id="agent-dockerfile"
              value={dockerfile}
              onChange={(e) => setDockerfile(e.target.value)}
              className="font-mono text-sm min-h-[120px] resize-y"
              placeholder={DEFAULT_DOCKERFILE}
            />
          </div>

          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <Label htmlFor="agent-command">Start Command</Label>
              <button
                type="button"
                onClick={() => setShowCommandTip(!showCommandTip)}
                className="text-xs text-muted-foreground hover:text-foreground flex items-center gap-1"
              >
                <Info className="h-3 w-3" />
                {showCommandTip ? "Hide tips" : "What is this?"}
              </button>
            </div>
            {showCommandTip && (
              <TipBox>
                <p className="mb-1">
                  The shell command that launches your agent inside the container.
                  This runs when a workspace starts.
                </p>
                <p className="mb-1 font-medium">Examples:</p>
                <ul className="space-y-0.5 ml-3">
                  <li><code className="bg-muted px-1 rounded">aider</code> — Aider CLI</li>
                  <li><code className="bg-muted px-1 rounded">qwen</code> — Qwen Code</li>
                  <li><code className="bg-muted px-1 rounded">codex</code> — OpenAI Codex</li>
                  <li><code className="bg-muted px-1 rounded">aider --model gpt-4o</code> — with flags</li>
                </ul>
              </TipBox>
            )}
            <Input
              id="agent-command"
              placeholder="aider"
              value={command}
              onChange={(e) => setCommand(e.target.value)}
              className="font-mono text-sm"
            />
          </div>

          {error && <p className="text-sm text-destructive">{error}</p>}
          {saving && (
            <p className="text-sm text-muted-foreground animate-pulse">
              Building Docker image... this may take a moment.
            </p>
          )}
        </div>

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={saving}
          >
            Cancel
          </Button>
          <Button onClick={handleSave} disabled={saving || !name || !command}>
            {saving && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
            {saving ? "Building..." : editing ? "Save" : "Add Agent"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
