"use client";

import { useState, useEffect } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";

const VALID_WORKSPACE_ID = /^[a-z0-9][a-z0-9_-]*$/;

interface RenameDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentName: string;
  onConfirm: (newName: string) => void;
}

export function RenameDialog({
  open,
  onOpenChange,
  currentName,
  onConfirm,
}: RenameDialogProps) {
  const [name, setName] = useState(currentName);
  const [error, setError] = useState("");

  useEffect(() => {
    if (open) {
      setName(currentName);
      setError("");
    }
  }, [open, currentName]);

  const validate = (value: string) => {
    if (!value) return "Name is required";
    if (!VALID_WORKSPACE_ID.test(value))
      return "Must be lowercase alphanumeric, hyphens, or underscores";
    if (value === currentName) return "Name is unchanged";
    return "";
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const err = validate(name);
    if (err) {
      setError(err);
      return;
    }
    onConfirm(name);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Rename workspace</DialogTitle>
            <DialogDescription>
              Enter a new name for <span className="font-mono">{currentName}</span>.
              Volume data will be preserved.
            </DialogDescription>
          </DialogHeader>
          <div className="py-4 space-y-2">
            <Label htmlFor="workspace-name">Workspace name</Label>
            <Input
              id="workspace-name"
              value={name}
              onChange={(e) => {
                setName(e.target.value.toLowerCase());
                setError("");
              }}
              placeholder="my-workspace"
              autoFocus
            />
            {error && (
              <p className="text-sm text-destructive">{error}</p>
            )}
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={!!validate(name)}>
              Rename
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
