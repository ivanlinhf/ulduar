import { useEffect, useState } from "react";

import {
  APIError,
  getPresentationGenerationCapabilities,
  type PresentationGenerationCapabilitiesResponse,
} from "./api";

export type PresentationGenerationBootstrapState =
  | { status: "feature-disabled" }
  | { status: "bootstrap-loading" }
  | { status: "available"; capabilities: PresentationGenerationCapabilitiesResponse }
  | { status: "unavailable-503"; errorMessage: string }
  | { status: "generic-error"; errorMessage: string };

export function usePresentationGenerationBootstrap(
  isEnabled: boolean,
): PresentationGenerationBootstrapState {
  const [state, setState] = useState<PresentationGenerationBootstrapState>(() =>
    isEnabled ? { status: "bootstrap-loading" } : { status: "feature-disabled" },
  );

  useEffect(() => {
    if (!isEnabled) {
      setState({ status: "feature-disabled" });
      return;
    }

    let active = true;
    setState({ status: "bootstrap-loading" });

    void getPresentationGenerationCapabilities()
      .then((capabilities) => {
        if (!active) {
          return;
        }

        setState({ status: "available", capabilities });
      })
      .catch((error: unknown) => {
        if (!active) {
          return;
        }

        if (error instanceof APIError && error.status === 503) {
          setState({ status: "unavailable-503", errorMessage: error.message });
          return;
        }

        setState({
          status: "generic-error",
          errorMessage: toErrorMessage(error, "Failed to load presentation generation capabilities"),
        });
      });

    return () => {
      active = false;
    };
  }, [isEnabled]);

  return state;
}

function toErrorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim() !== "") {
    return error.message;
  }

  return fallback;
}
