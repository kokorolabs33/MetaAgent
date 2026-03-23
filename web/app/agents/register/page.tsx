"use client";

import { useEffect, useMemo, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import { ArrowLeft } from "lucide-react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { AdapterForm } from "@/components/agent/AdapterForm";
import { useOrgStore, useAgentStore } from "@/lib/store";
import type { Agent } from "@/lib/types";

export default function RegisterAgentPage() {
  const router = useRouter();
  const { orgs, currentOrg, loadOrgs, selectOrg } = useOrgStore();
  const { registerAgent } = useAgentStore();

  const orgId = useMemo(
    () => (orgs.length > 0 ? orgs[0].id : null),
    [orgs],
  );

  const [formData, setFormData] = useState<Partial<Agent>>({
    name: "",
    description: "",
    endpoint: "",
    adapter_type: "native",
    auth_type: "none",
    capabilities: [],
  });
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Load orgs and select the first one if needed
  useEffect(() => {
    if (orgs.length === 0) {
      loadOrgs();
    } else if (!currentOrg && orgs.length > 0) {
      void selectOrg(orgs[0].id);
    }
  }, [orgs, currentOrg, loadOrgs, selectOrg]);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();

      if (!orgId) return;
      if (!formData.name?.trim() || !formData.endpoint?.trim()) return;

      setIsSubmitting(true);
      setError(null);

      try {
        await registerAgent(orgId, formData);
        router.push("/agents");
      } catch (err) {
        setError(
          err instanceof Error ? err.message : "Failed to register agent",
        );
      } finally {
        setIsSubmitting(false);
      }
    },
    [orgId, formData, registerAgent, router],
  );

  const isValid =
    (formData.name?.trim().length ?? 0) > 0 &&
    (formData.endpoint?.trim().length ?? 0) > 0;

  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="flex items-center gap-3 border-b border-border px-4 py-3">
        <Link
          href="/agents"
          className="rounded-md p-1 text-muted-foreground transition-colors hover:text-foreground"
        >
          <ArrowLeft className="size-5" />
        </Link>
        <h1 className="text-lg font-semibold text-foreground">
          Register Agent
        </h1>
      </div>

      {/* Form */}
      <div className="flex-1 overflow-auto p-6">
        <form
          onSubmit={(e) => void handleSubmit(e)}
          className="mx-auto max-w-lg space-y-6"
        >
          <AdapterForm value={formData} onChange={setFormData} />

          {error && (
            <p className="text-sm text-destructive">{error}</p>
          )}

          <div className="flex justify-end gap-2 border-t border-border pt-4">
            <Link href="/agents">
              <Button type="button" variant="ghost">
                Cancel
              </Button>
            </Link>
            <Button type="submit" disabled={isSubmitting || !isValid}>
              {isSubmitting ? "Registering..." : "Register Agent"}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
}
