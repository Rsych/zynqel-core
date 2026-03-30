"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
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
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectSeparator,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { api } from "@/lib/api";
import type { AgentConfig } from "@/lib/types";
import { Loader2, Plus, X } from "lucide-react";
import { toast } from "sonner";

interface CreateDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function CreateDialog({ open, onOpenChange }: CreateDialogProps) {
  const router = useRouter();
  const [agents, setAgents] = useState<AgentConfig[]>([]);
  const [agent, setAgent] = useState("shell");
  const [workspaceId, setWorkspaceId] = useState("");
  const [repoUrl, setRepoUrl] = useState("");
  const [branch, setBranch] = useState("");
  const [authMethod, setAuthMethod] = useState("none");
  const [gitToken, setGitToken] = useState("");
  const [sshKeyPath, setSshKeyPath] = useState("~/.ssh");
  const [envVars, setEnvVars] = useState<Record<string, string>>({});
  const [envKey, setEnvKey] = useState("");
  const [envValue, setEnvValue] = useState("");
  const envValueRef = useRef<HTMLInputElement>(null);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (open) {
      api.listAgents().then(setAgents).catch(() => {});
    }
  }, [open]);

  const builtinAgents = agents.filter((a) => a.builtin);
  const customAgents = agents.filter((a) => !a.builtin);

  const isValidEnvKey = (key: string) => /^[A-Za-z_][A-Za-z0-9_]*$/.test(key);

  const addEnvVar = () => {
    const key = envKey.trim();
    if (!key || !isValidEnvKey(key)) return;
    setEnvVars((prev) => ({ ...prev, [key]: envValue }));
    setEnvKey("");
    setEnvValue("");
  };

  const removeEnvVar = (key: string) => {
    setEnvVars((prev) => {
      const next = { ...prev };
      delete next[key];
      return next;
    });
  };

  const fillPreset = (key: string) => {
    setEnvKey(key);
    setEnvValue("");
    setTimeout(() => envValueRef.current?.focus(), 0);
  };

  const handleCreate = async () => {
    setCreating(true);
    setError("");
    try {
      const session = await api.createSession({
        agent,
        workspace_id: workspaceId || undefined,
        repo_url: repoUrl || undefined,
        branch: branch || undefined,
        git_token: authMethod === "token" ? gitToken : undefined,
        ssh_key_path: authMethod === "ssh" ? sshKeyPath : undefined,
        env: Object.keys(envVars).length > 0 ? envVars : undefined,
      });
      resetForm();
      onOpenChange(false);
      router.push(`/workspace?id=${session.id}`);
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Failed to create workspace";
      setError(msg);
      toast.error(msg);
    } finally {
      setCreating(false);
    }
  };

  const resetForm = () => {
    setAgent("shell");
    setWorkspaceId("");
    setRepoUrl("");
    setBranch("");
    setAuthMethod("none");
    setGitToken("");
    setSshKeyPath("~/.ssh");
    setEnvVars({});
    setEnvKey("");
    setEnvValue("");
    setError("");
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        onOpenChange(v);
        if (!v) resetForm();
      }}
    >
      <DialogContent className="sm:max-w-lg" onInteractOutside={(e) => e.preventDefault()}>
        <DialogHeader>
          <DialogTitle>New Workspace</DialogTitle>
          <DialogDescription>
            Create a new agent workspace session.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor="agent">Agent</Label>
            <Select value={agent} onValueChange={setAgent}>
              <SelectTrigger id="agent">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  <SelectLabel>Built-in</SelectLabel>
                  {builtinAgents.map((a) => (
                    <SelectItem key={a.name} value={a.name}>
                      {a.name.charAt(0).toUpperCase() + a.name.slice(1)}
                    </SelectItem>
                  ))}
                  {builtinAgents.length === 0 && (
                    <>
                      <SelectItem value="shell">Shell</SelectItem>
                      <SelectItem value="claude">Claude</SelectItem>
                    </>
                  )}
                </SelectGroup>
                {customAgents.length > 0 && (
                  <>
                    <SelectSeparator />
                    <SelectGroup>
                      <SelectLabel>Custom</SelectLabel>
                      {customAgents.map((a) => (
                        <SelectItem key={a.name} value={a.name}>
                          {a.name}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </>
                )}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label htmlFor="workspace-id">
              Workspace ID{" "}
              <span className="text-muted-foreground font-normal">(optional)</span>
            </Label>
            <Input
              id="workspace-id"
              placeholder="my-project"
              value={workspaceId}
              onChange={(e) => setWorkspaceId(e.target.value)}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="repo-url">
              Repository URL{" "}
              <span className="text-muted-foreground font-normal">(optional)</span>
            </Label>
            <Input
              id="repo-url"
              placeholder="https://github.com/user/repo.git"
              value={repoUrl}
              onChange={(e) => setRepoUrl(e.target.value)}
            />
          </div>

          {repoUrl && (
            <>
              <div className="space-y-2">
                <Label htmlFor="branch">
                  Branch{" "}
                  <span className="text-muted-foreground font-normal">
                    (optional)
                  </span>
                </Label>
                <Input
                  id="branch"
                  placeholder="main"
                  value={branch}
                  onChange={(e) => setBranch(e.target.value)}
                />
              </div>

              <div className="space-y-2">
                <Label>Authentication</Label>
                <Select value={authMethod} onValueChange={setAuthMethod}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="none">None (public repo)</SelectItem>
                    <SelectItem value="token">Git Token (PAT)</SelectItem>
                    <SelectItem value="ssh">SSH Key</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              {authMethod === "token" && (
                <div className="space-y-2">
                  <Label htmlFor="git-token">Token</Label>
                  <Input
                    id="git-token"
                    type="password"
                    placeholder="ghp_... or glpat-..."
                    value={gitToken}
                    onChange={(e) => setGitToken(e.target.value)}
                  />
                  <p className="text-xs text-muted-foreground">
                    GitHub PAT, GitLab token, etc. Also set as GITHUB_TOKEN env var.
                  </p>
                </div>
              )}

              {authMethod === "ssh" && (
                <div className="space-y-2">
                  <Label htmlFor="ssh-path">SSH Key Directory</Label>
                  <Input
                    id="ssh-path"
                    placeholder="~/.ssh"
                    value={sshKeyPath}
                    onChange={(e) => setSshKeyPath(e.target.value)}
                    className="font-mono text-sm"
                  />
                  <p className="text-xs text-muted-foreground">
                    Host path to mount into the container. Use SSH repo URL
                    (git@github.com:user/repo.git).
                  </p>
                </div>
              )}
            </>
          )}

          <div className="space-y-2">
            <Label>
              Environment Variables{" "}
              <span className="text-muted-foreground font-normal">(optional)</span>
            </Label>
            <div className="flex gap-2">
              <Input
                placeholder="KEY"
                value={envKey}
                onChange={(e) => setEnvKey(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    addEnvVar();
                  }
                }}
                className="font-mono text-sm flex-1"
              />
              <Input
                ref={envValueRef}
                type="password"
                placeholder="value"
                value={envValue}
                onChange={(e) => setEnvValue(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    addEnvVar();
                  }
                }}
                className="font-mono text-sm flex-1"
              />
              <Button
                type="button"
                variant="outline"
                size="icon"
                onClick={addEnvVar}
                disabled={!envKey.trim() || !isValidEnvKey(envKey.trim())}
                className="shrink-0"
              >
                <Plus className="h-4 w-4" />
              </Button>
            </div>
            <div className="flex flex-wrap gap-1.5">
              {["ANTHROPIC_API_KEY", "OPENAI_API_KEY"].map((preset) => (
                <Button
                  key={preset}
                  type="button"
                  variant="outline"
                  size="sm"
                  className="h-6 text-xs font-mono"
                  onClick={() => fillPreset(preset)}
                  disabled={preset in envVars}
                >
                  +{preset.replace("_API_KEY", "")}
                </Button>
              ))}
            </div>
            {Object.keys(envVars).length > 0 && (
              <div className="flex flex-wrap gap-1.5 mt-1.5">
                {Object.entries(envVars).map(([key, value]) => (
                  <span
                    key={key}
                    className="inline-flex items-center gap-1 rounded-md bg-muted px-2 py-1 text-xs font-mono"
                  >
                    {key}=
                    {value ? (
                      "••••"
                    ) : (
                      <span className="text-muted-foreground italic">empty</span>
                    )}
                    <button
                      type="button"
                      onClick={() => removeEnvVar(key)}
                      className="text-muted-foreground hover:text-foreground ml-0.5"
                    >
                      <X className="h-3 w-3" />
                    </button>
                  </span>
                ))}
              </div>
            )}
          </div>

          {error && (
            <p className="text-sm text-destructive">{error}</p>
          )}
        </div>

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={creating}
          >
            Cancel
          </Button>
          <Button onClick={handleCreate} disabled={creating}>
            {creating && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
            Create
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
