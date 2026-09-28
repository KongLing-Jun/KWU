// @refresh reset
import { useCallback, useEffect, useMemo, useRef, useState, type ChangeEvent } from "react";

import { BRAND_SLUG } from "../../../lib/brand";
import { onCloudAssetsChanged } from "../../../lib/bridge";
import { reportClientError } from "../../../lib/clientError";
import { useT } from "../../../lib/i18n";
import { useToast } from "../../../lib/toast";
import { accountBridge } from "../bridge/accountBridge";
import {
  hasRecoverableCloudIdentity,
  useAccountStore,
  type CloudAccountView,
  type CloudCachedAssetsView,
} from "../store/accountStore";

const bootstrapByContext = new Map<string, Promise<void>>();
const cachedAssetsByContext = new Map<string, Promise<CloudCachedAssetsView | null>>();

function cachedAssetsMatchAccount(view: CloudCachedAssetsView, account: CloudAccountView | null): boolean {
  return account !== null
    && hasRecoverableCloudIdentity(account)
    && account.profile_id === view.profile_id
    && account.cloud_user_id === view.cloud_user_id
    && account.binding_generation === view.binding_generation;
}

export function useAccountSidebar(contextKey: string) {
  const t = useT();
  const { showToast } = useToast();
  const [menuOpen, setMenuOpen] = useState(false);
  const [loadedAvatar, setLoadedAvatar] = useState({ storageKey: "", src: "" });
  const [cachedAssets, setCachedAssets] = useState<{ contextKey: string; view: CloudCachedAssetsView } | null>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const avatarInputRef = useRef<HTMLInputElement>(null);
  const account = useAccountStore((state) => state.account);
  const scopeKey = useAccountStore((state) => state.scopeKey);
  const initialized = useAccountStore((state) => state.initialized);
  const balance = useAccountStore((state) => state.balance?.total_balance);
  const available = useAccountStore((state) => state.available);
  const plan = useAccountStore((state) => state.subscription?.current.plan);
  const status = useAccountStore((state) => state.subscription?.current.status);
  const subscriptionVersion = useAccountStore((state) => state.subscription?.current.version);

  const closeMenu = useCallback(() => setMenuOpen(false), []);
  const ensureBootstrap = useCallback((): Promise<void> => {
    let current = useAccountStore.getState();
    if (current.contextKey !== contextKey) {
      current.activateContext(contextKey);
      current = useAccountStore.getState();
    }
    if (current.initialized) return Promise.resolve();

    const pending = bootstrapByContext.get(contextKey);
    if (pending) return pending;

    let request!: Promise<void>;
    request = accountBridge.CloudBootstrap()
      .then((view) => {
        const latest = useAccountStore.getState();
        if (latest.contextKey === contextKey) latest.applyBootstrap(view);
      })
      .catch(() => {
        // The compact menu keeps its safe fallback label; AccountCenter owns detailed errors.
      })
      .finally(() => {
        const latest = useAccountStore.getState();
        if (latest.contextKey === contextKey) latest.finishBootstrap();
        if (bootstrapByContext.get(contextKey) === request) bootstrapByContext.delete(contextKey);
      });
    bootstrapByContext.set(contextKey, request);
    return request;
  }, [contextKey]);

  useEffect(() => {
    const readCachedAssets = accountBridge.CloudCachedAssets;
    if (readCachedAssets === undefined) return;
    let disposed = false;
    let request = cachedAssetsByContext.get(contextKey);
    if (request === undefined) {
      request = readCachedAssets()
        .catch(() => null)
        .finally(() => {
          cachedAssetsByContext.delete(contextKey);
        });
      cachedAssetsByContext.set(contextKey, request);
    }
    void request.then((view) => {
      if (!disposed && view?.balance) setCachedAssets({ contextKey, view });
    });
    return () => {
      disposed = true;
    };
  }, [contextKey]);

  useEffect(() => {
    void ensureBootstrap();
  }, [ensureBootstrap]);

  useEffect(() => {
    let disposed = false;
    let inFlight = false;
    let dirty = false;

    const refreshRemoteAssets = () => {
      if (disposed) return;
      if (inFlight) {
        dirty = true;
        return;
      }
      const request = useAccountStore.getState().beginAssetRequest(contextKey);
      if (request === null) return;
      inFlight = true;
      void accountBridge.CloudRefreshAssets()
        .then((view) => {
          const latest = useAccountStore.getState();
          if (!disposed && latest.contextKey === contextKey) latest.applyBootstrap(view);
        })
        .catch((cause: unknown) => {
          if (disposed) return;
          const message = reportClientError("account.assets.refresh", cause, {
            fallbackMessage: "同步云端账户信息失败，请稍后重试。",
          }).userMessage;
          useAccountStore.getState().recordAssetSyncFailure(request, message);
        })
        .finally(() => {
          inFlight = false;
          if (disposed || !dirty) return;
          dirty = false;
          refreshRemoteAssets();
        });
    };

    const unsubscribe = onCloudAssetsChanged((changes) => {
      if (!changes.balanceChanged && !changes.subscriptionChanged) return;
      void ensureBootstrap().then(refreshRemoteAssets);
    });
    return () => {
      disposed = true;
      unsubscribe();
    };
  }, [contextKey, ensureBootstrap]);

  const toggleMenu = useCallback(() => {
    if (menuOpen) {
      setMenuOpen(false);
      return;
    }
    setMenuOpen(true);
    void ensureBootstrap();
  }, [ensureBootstrap, menuOpen]);

  useEffect(() => {
    if (!menuOpen) return;
    const closeOnOutsidePointer = (event: globalThis.PointerEvent) => {
      if (!menuRef.current?.contains(event.target as Node)) setMenuOpen(false);
    };
    const closeOnEscape = (event: globalThis.KeyboardEvent) => {
      if (event.key === "Escape") setMenuOpen(false);
    };
    document.addEventListener("pointerdown", closeOnOutsidePointer);
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("pointerdown", closeOnOutsidePointer);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [menuOpen]);

  const avatarStorageKey = useMemo(() => {
    const accountID = hasRecoverableCloudIdentity(account)
      ? account?.cloud_user_id?.trim() || "local"
      : "local";
    return `${BRAND_SLUG}.sidebar-avatar.${accountID}`;
  }, [account]);

  useEffect(() => {
    try {
      setLoadedAvatar({
        storageKey: avatarStorageKey,
        src: window.localStorage.getItem(avatarStorageKey) ?? "",
      });
    } catch {
      setLoadedAvatar({ storageKey: avatarStorageKey, src: "" });
    }
  }, [avatarStorageKey]);

  const avatarSrc = loadedAvatar.storageKey === avatarStorageKey ? loadedAvatar.src : "";
  const currentCachedAssets = cachedAssets?.contextKey === contextKey ? cachedAssets.view : null;
  const displayBalance = balance ?? (
    currentCachedAssets !== null
    && (!initialized || cachedAssetsMatchAccount(currentCachedAssets, account))
      ? currentCachedAssets.balance?.total_balance
      : undefined
  );

  const handleAvatarChange = useCallback((event: ChangeEvent<HTMLInputElement>) => {
    const input = event.currentTarget;
    const file = input.files?.[0];
    input.value = "";
    if (!file) return;
    if (!["image/png", "image/jpeg", "image/webp"].includes(file.type)) {
      showToast(t("account.avatarInvalid"), "error");
      return;
    }
    if (file.size > 2 * 1024 * 1024) {
      showToast(t("account.avatarTooLarge"), "error");
      return;
    }
    const reader = new FileReader();
    reader.onerror = () => showToast(t("account.avatarInvalid"), "error");
    reader.onload = () => {
      if (typeof reader.result !== "string") return;
      try {
        window.localStorage.setItem(avatarStorageKey, reader.result);
        setLoadedAvatar({ storageKey: avatarStorageKey, src: reader.result });
      } catch {
        showToast(t("account.avatarTooLarge"), "error");
      }
    };
    reader.readAsDataURL(file);
  }, [avatarStorageKey, showToast, t]);

  return {
    menuOpen,
    menuRef,
    avatarInputRef,
    avatarSrc,
    initialized,
    account,
    scopeKey,
    available,
    balance,
    displayBalance,
    plan,
    status,
    subscriptionVersion,
    closeMenu,
    toggleMenu,
    handleAvatarChange,
  };
}
