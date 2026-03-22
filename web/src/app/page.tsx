"use client";

import { useState } from "react";
import { NavBar } from "@/components/nav-bar";
import { WorkspaceList } from "@/components/workspace-list";
import { CreateDialog } from "@/components/create-dialog";
import { Button } from "@/components/ui/button";
import { Plus } from "lucide-react";

export default function Home() {
  const [createOpen, setCreateOpen] = useState(false);

  return (
    <div className="min-h-screen">
      <NavBar />
      <main className="mx-auto max-w-4xl px-6 py-8">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-2xl font-bold tracking-tight">Workspaces</h1>
            <p className="text-sm text-muted-foreground mt-1">
              Manage your agent sessions
            </p>
          </div>
          <Button onClick={() => setCreateOpen(true)}>
            <Plus className="h-4 w-4 mr-2" />
            New Workspace
          </Button>
        </div>
        <WorkspaceList onCreateClick={() => setCreateOpen(true)} />
        <CreateDialog open={createOpen} onOpenChange={setCreateOpen} />
      </main>
    </div>
  );
}
