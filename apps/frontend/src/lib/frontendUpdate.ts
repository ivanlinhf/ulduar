import { useCallback, useEffect, useRef, useState } from "react";

import { reloadWindow } from "./browser";

export const currentFrontendVersion = __APP_VERSION__;
export const versionMetadataPath = "/version.json";
export const versionCheckIntervalMs = 5 * 60 * 1000;
export const reloadLosesSessionMessage = "Reloading will start a new chat and lose this session.";

type FrontendVersionMetadata = {
  version?: string;
};

export function useFrontendUpdate(userTurnCount: number) {
  const [updateAvailable, setUpdateAvailable] = useState(false);
  const [isReloadConfirmationOpen, setIsReloadConfirmationOpen] = useState(false);
  const updateAvailableRef = useRef(false);
  const reloadTriggerRef = useRef<HTMLElement | null>(null);
  const shouldRestoreTriggerFocusRef = useRef(false);

  useEffect(() => {
    updateAvailableRef.current = updateAvailable;
  }, [updateAvailable]);

  const checkForUpdate = useCallback(async () => {
    if (updateAvailableRef.current) {
      return;
    }

    try {
      const latestVersion = await fetchLatestFrontendVersion();
      if (latestVersion && latestVersion !== currentFrontendVersion) {
        updateAvailableRef.current = true;
        setUpdateAvailable(true);
      }
    } catch {
      // Ignore transient version check failures and try again on the next trigger.
    }
  }, []);

  useEffect(() => {
    void checkForUpdate();

    const intervalId = window.setInterval(() => {
      void checkForUpdate();
    }, versionCheckIntervalMs);

    const handleVisibilityChange = () => {
      if (document.visibilityState === "visible") {
        void checkForUpdate();
      }
    };

    const handleOnline = () => {
      void checkForUpdate();
    };

    document.addEventListener("visibilitychange", handleVisibilityChange);
    window.addEventListener("online", handleOnline);

    return () => {
      window.clearInterval(intervalId);
      document.removeEventListener("visibilitychange", handleVisibilityChange);
      window.removeEventListener("online", handleOnline);
    };
  }, [checkForUpdate]);

  const requestReloadToUpdate = useCallback(() => {
    if (userTurnCount > 0) {
      const reloadTrigger = document.activeElement instanceof HTMLElement ? document.activeElement : null;
      reloadTriggerRef.current = reloadTrigger;
      shouldRestoreTriggerFocusRef.current = reloadTrigger !== null;
      setIsReloadConfirmationOpen(true);
      return;
    }

    reloadWindow();
  }, [userTurnCount]);

  const cancelReloadConfirmation = useCallback(() => {
    setIsReloadConfirmationOpen(false);
    const reloadTrigger = reloadTriggerRef.current;

    if (!shouldRestoreTriggerFocusRef.current) {
      reloadTriggerRef.current = null;
      return;
    }

    const restoreFocus = () => {
      reloadTrigger?.focus();
      reloadTriggerRef.current = null;
      shouldRestoreTriggerFocusRef.current = false;
    };

    // Wait until the dialog unmounts before moving focus back to the toast action.
    if (typeof window.requestAnimationFrame === "function") {
      window.requestAnimationFrame(() => {
        restoreFocus();
      });
      return;
    }

    window.setTimeout(() => {
      restoreFocus();
    }, 0);
  }, []);

  const confirmReloadToUpdate = useCallback(() => {
    shouldRestoreTriggerFocusRef.current = false;
    reloadTriggerRef.current = null;
    setIsReloadConfirmationOpen(false);
    reloadWindow();
  }, []);

  return {
    cancelReloadConfirmation,
    confirmReloadToUpdate,
    isReloadConfirmationOpen,
    requestReloadToUpdate,
    updateAvailable,
  };
}

export async function fetchLatestFrontendVersion(fetchImpl: typeof fetch = fetch): Promise<string | null> {
  const url = new URL(versionMetadataPath, window.location.origin);
  url.searchParams.set("t", String(Date.now()));

  const response = await fetchImpl(url.toString(), {
    cache: "no-store",
    headers: {
      "Cache-Control": "no-cache",
      Pragma: "no-cache",
    },
  });

  if (!response.ok) {
    return null;
  }

  const data = (await response.json()) as FrontendVersionMetadata;
  const version = data.version?.trim();
  return version ? version : null;
}
