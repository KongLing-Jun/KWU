import { useCallback, useRef, useState } from "react";

import type { CloudBootstrapView } from "../store/accountStore";
import { useAccountStore } from "../store/accountStore";
import type { AccountCenterRequest } from "../ui/AccountCenter";

interface AccountNavigationOptions {
  beforeLogout?: () => void;
  logout: () => Promise<CloudBootstrapView>;
  onLogoutError: (cause: unknown) => void;
}

export function useAccountNavigation(
  beforeOpen: () => void,
  openSurface: () => void,
  { beforeLogout, logout, onLogoutError }: AccountNavigationOptions,
) {
  const [request, setRequest] = useState<AccountCenterRequest>({ id: 0, page: "overview" });
  const [accountLogoutPending, setAccountLogoutPending] = useState(false);
  const logoutRequestRef = useRef<Promise<void> | null>(null);

  const openDestination = useCallback((page: AccountCenterRequest["page"], action?: AccountCenterRequest["action"]) => {
    beforeOpen();
    setRequest((current) => ({ id: current.id + 1, page, action }));
    openSurface();
  }, [beforeOpen, openSurface]);

  const openAccountCenter = useCallback(() => openDestination("overview"), [openDestination]);
  const openAccountPayments = useCallback(() => openDestination("payments"), [openDestination]);
  const requestAccountLogout = useCallback(() => {
    if (logoutRequestRef.current !== null) return logoutRequestRef.current;

    beforeLogout?.();
    setAccountLogoutPending(true);
    const request = Promise.resolve()
      .then(logout)
      .then((view) => {
        useAccountStore.getState().applyBootstrap(view);
        openDestination("overview");
      })
      .catch(onLogoutError)
      .finally(() => {
        if (logoutRequestRef.current === request) logoutRequestRef.current = null;
        setAccountLogoutPending(false);
      });
    logoutRequestRef.current = request;
    return request;
  }, [beforeLogout, logout, onLogoutError, openDestination]);

  return {
    accountCenterRequest: request,
    accountLogoutPending,
    openAccountCenter,
    openAccountPayments,
    requestAccountLogout,
  };
}
