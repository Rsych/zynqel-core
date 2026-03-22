"use client";

import { useEffect, useState } from "react";
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
import { Loader2 } from "lucide-react";

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
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (open) {
      api.listAgents().then(setAgents).catch(() => {});
    }
  }, [open]);

  const builtinAgents = agents.filter((a) => a.builtin);
  const customAgents = agents.filter((a) => !a.builtin);

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
      });
      onOpenChange(false);
      resetForm();
      router.push(`/workspace?id=${session.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create workspace");
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
      <DialogContent className="sm:max-w-md" onInteractOutside={(e) => e.preventDefault()}>
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
