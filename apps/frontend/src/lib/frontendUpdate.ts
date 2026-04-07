import { useCallback, useEffect, useRef, useState } from "react";

import { reloadWindow } from "./browser";

export const currentFrontendVersion = __APP_VERSION__;
export const versionMetadataPath = "/version.json";
export const versionCheckIntervalMs = 5 * 60 * 1000;

type FrontendVersionMetadata = {
  version?: string;
};

export function useFrontendUpdate(userTurnCount: number) {
  const [updateAvailable, setUpdateAvailable] = useState(false);
  const updateAvailableRef = useRef(false);

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

  const reloadToUpdate = useCallback(() => {
    if (
      userTurnCount > 0 &&
      !window.confirm("Reloading will start a new chat and lose this session. Reload now?")
    ) {
      return;
    }

    reloadWindow();
  }, [userTurnCount]);

  return {
    reloadToUpdate,
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
