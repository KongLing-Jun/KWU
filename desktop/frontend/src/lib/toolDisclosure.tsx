import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from "react";

type ToolDisclosureContextValue = {
  openToolIds: ReadonlySet<string>;
  setToolOpen: (toolId: string, open: boolean) => void;
};

const ToolDisclosureContext = createContext<ToolDisclosureContextValue | null>(null);
const EMPTY_OPEN_TOOL_IDS: ReadonlySet<string> = new Set();

export function ToolDisclosureProvider({ children }: { children: ReactNode }) {
  const [openToolIds, setOpenToolIds] = useState<ReadonlySet<string>>(() => new Set());
  const setToolOpen = useCallback((toolId: string, open: boolean) => {
    setOpenToolIds((current) => {
      if (current.has(toolId) === open) return current;
      const next = new Set(current);
      if (open) next.add(toolId);
      else next.delete(toolId);
      return next;
    });
  }, []);
  const value = useMemo(() => ({ openToolIds, setToolOpen }), [openToolIds, setToolOpen]);
  return <ToolDisclosureContext.Provider value={value}>{children}</ToolDisclosureContext.Provider>;
}

export function useToolDisclosureContext() {
  return useContext(ToolDisclosureContext);
}

export function useOpenToolDisclosureIds(): ReadonlySet<string> {
  return useContext(ToolDisclosureContext)?.openToolIds ?? EMPTY_OPEN_TOOL_IDS;
}
