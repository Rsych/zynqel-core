"use client";

import Link from "next/link";
import { Button } from "@/components/ui/button";
import { AgentConfigList } from "@/components/agent-config";
import { ArrowLeft } from "lucide-react";

export default function AgentsPage() {
  return (
    <div className="min-h-screen">
      <header className="border-b border-border bg-background/80 backdrop-blur-sm sticky top-0 z-50">
        <div className="mx-auto max-w-4xl flex items-center gap-3 px-6 h-14">
          <Button variant="ghost" size="icon" className="h-8 w-8" asChild>
            <Link href="/console">
              <ArrowLeft className="h-4 w-4" />
            </Link>
          </Button>
          <span className="font-medium">Agent Configuration</span>
        </div>
      </header>
      <main className="mx-auto max-w-4xl px-6 py-8">
        <div className="mb-6">
          <h1 className="text-2xl font-bold tracking-tight">Agents</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Manage built-in and custom agent configurations
          </p>
        </div>
        <AgentConfigList />
      </main>
    </div>
  );
}
