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
import { Plus, Pencil, Trash2, Loader2, Terminal } from "lucide-react";

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
          <div className="text-center py-8 text-muted-foreground text-sm">
            No custom agents yet. Add one to get started.
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
                    {agent.image || "default"}
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
  const [image, setImage] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (editing) {
      setName(editing.name);
      setCommand(editing.command?.join(" ") || "");
      setImage(editing.image || "");
    } else {
      setName("");
      setCommand("");
      setImage("");
    }
    setError("");
  }, [editing, open]);

  const handleSave = async () => {
    setSaving(true);
    setError("");
    try {
      const cfg = {
        name,
        command: command.split(/\s+/).filter(Boolean),
        image: image || undefined,
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
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{editing ? "Edit Agent" : "Add Agent"}</DialogTitle>
          <DialogDescription>
            {editing
              ? "Update this custom agent configuration."
              : "Define a custom agent to run in workspaces."}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor="agent-name">Name</Label>
            <Input
              id="agent-name"
              placeholder="aider"
              value={name}
              onChange={(e) => setName(e.target.value)}
              disabled={!!editing}
            />
            <p className="text-xs text-muted-foreground">
              Lowercase letters, numbers, hyphens, underscores
            </p>
          </div>

          <div className="space-y-2">
            <Label htmlFor="agent-command">Start Command</Label>
            <Input
              id="agent-command"
              placeholder="/usr/local/bin/aider"
              value={command}
              onChange={(e) => setCommand(e.target.value)}
              className="font-mono"
            />
            <p className="text-xs text-muted-foreground">
              The command to launch the agent CLI inside the container
            </p>
          </div>

          <div className="space-y-2">
            <Label htmlFor="agent-image">
              Docker Image{" "}
              <span className="text-muted-foreground font-normal">(optional)</span>
            </Label>
            <Input
              id="agent-image"
              placeholder="zynqel-base:latest"
              value={image}
              onChange={(e) => setImage(e.target.value)}
              className="font-mono"
            />
            <p className="text-xs text-muted-foreground">
              Leave empty to use zynqel-base:latest
            </p>
          </div>

          {error && <p className="text-sm text-destructive">{error}</p>}
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
            {editing ? "Save" : "Add Agent"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
