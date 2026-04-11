import { useEffect, useState } from "react";

import { APIError, getImageGenerationCapabilities, type ImageGenerationCapabilitiesResponse } from "./api";

export type ImageGenerationBootstrapState =
  | { status: "feature-disabled" }
  | { status: "bootstrap-loading" }
  | { status: "available"; capabilities: ImageGenerationCapabilitiesResponse }
  | { status: "unavailable-503"; errorMessage: string }
  | { status: "generic-error"; errorMessage: string };

export function useImageGenerationBootstrap(isEnabled: boolean): ImageGenerationBootstrapState {
  const [state, setState] = useState<ImageGenerationBootstrapState>(() =>
    isEnabled ? { status: "bootstrap-loading" } : { status: "feature-disabled" },
  );

  useEffect(() => {
    if (!isEnabled) {
      setState({ status: "feature-disabled" });
      return;
    }

    let active = true;
    setState({ status: "bootstrap-loading" });

    void getImageGenerationCapabilities()
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
          errorMessage: toErrorMessage(error, "Failed to load image generation capabilities"),
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
