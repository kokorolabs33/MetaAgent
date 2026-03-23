"use client";

import { useEffect } from "react";
import { useOrgStore } from "@/lib/store";

/**
 * OrgProvider ensures orgs are loaded and the first org is selected.
 * Place this in the root layout so all pages have currentOrg available.
 */
export function OrgProvider({ children }: { children: React.ReactNode }) {
  const { orgs, currentOrg, loadOrgs, selectOrg } = useOrgStore();

  useEffect(() => {
    if (orgs.length === 0) {
      void loadOrgs();
    }
  }, [orgs.length, loadOrgs]);

  useEffect(() => {
    if (orgs.length > 0 && !currentOrg) {
      void selectOrg(orgs[0].id);
    }
  }, [orgs, currentOrg, selectOrg]);

  return <>{children}</>;
}
