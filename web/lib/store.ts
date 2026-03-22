"use client";

import { create } from "zustand";
import { api } from "./api";
import type { Organization, OrgListItem } from "./types";

interface TaskHubStore {
  // State
  orgs: OrgListItem[];
  currentOrg: Organization | null;
  isLoading: boolean;

  // Actions
  loadOrgs: () => Promise<void>;
  selectOrg: (orgId: string) => Promise<void>;
  reset: () => void;
}

export const useTaskHubStore = create<TaskHubStore>((set) => ({
  orgs: [],
  currentOrg: null,
  isLoading: false,

  loadOrgs: async () => {
    try {
      set({ isLoading: true });
      const orgs = await api.orgs.list();
      set({ orgs, isLoading: false });
    } catch (e) {
      console.error("loadOrgs:", e);
      set({ isLoading: false });
    }
  },

  selectOrg: async (orgId: string) => {
    try {
      set({ isLoading: true });
      const org = await api.orgs.get(orgId);
      set({ currentOrg: org, isLoading: false });
    } catch (e) {
      console.error("selectOrg:", e);
      set({ isLoading: false });
    }
  },

  reset: () => {
    set({
      currentOrg: null,
      orgs: [],
      isLoading: false,
    });
  },
}));
