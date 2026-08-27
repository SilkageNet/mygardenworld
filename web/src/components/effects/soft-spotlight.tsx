"use client";

import type { CSSProperties, FocusEvent, HTMLAttributes, PointerEvent } from "react";
import { cn } from "@/lib/utils";

// Adapted from React Bits SpotlightCard:
// https://github.com/DavidHDev/react-bits
// Copyright (c) 2026 David Haz; MIT License with Commons Clause.

type SpotlightStyle = CSSProperties & {
  "--soft-spotlight-x"?: string;
  "--soft-spotlight-y"?: string;
  "--soft-spotlight-opacity"?: string;
};

function setSpotlightOpacity(element: HTMLDivElement, opacity: "0" | "1") {
  element.style.setProperty("--soft-spotlight-opacity", opacity);
}

function canTrackPointer(event: PointerEvent<HTMLDivElement>) {
  return event.pointerType === "mouse"
    && window.matchMedia("(hover: hover) and (pointer: fine) and (prefers-reduced-motion: no-preference)").matches;
}

function updateSpotlightPosition(event: PointerEvent<HTMLDivElement>) {
  const bounds = event.currentTarget.getBoundingClientRect();
  event.currentTarget.style.setProperty("--soft-spotlight-x", `${event.clientX - bounds.left}px`);
  event.currentTarget.style.setProperty("--soft-spotlight-y", `${event.clientY - bounds.top}px`);
}

export function SoftSpotlight({
  className,
  children,
  style,
  onPointerEnter,
  onPointerMove,
  onPointerLeave,
  onFocus,
  onBlur,
  ...props
}: HTMLAttributes<HTMLDivElement>) {
  const spotlightStyle: SpotlightStyle = {
    "--soft-spotlight-x": "50%",
    "--soft-spotlight-y": "50%",
    "--soft-spotlight-opacity": "0",
    ...style,
  };

  const handlePointerEnter = (event: PointerEvent<HTMLDivElement>) => {
    onPointerEnter?.(event);
    const tracking = canTrackPointer(event);
    event.currentTarget.dataset.softSpotlightTracking = tracking ? "true" : "false";
    if (!tracking) return;
    updateSpotlightPosition(event);
    setSpotlightOpacity(event.currentTarget, "1");
  };

  const handlePointerMove = (event: PointerEvent<HTMLDivElement>) => {
    onPointerMove?.(event);
    if (event.currentTarget.dataset.softSpotlightTracking !== "true") return;
    updateSpotlightPosition(event);
  };

  const handlePointerLeave = (event: PointerEvent<HTMLDivElement>) => {
    onPointerLeave?.(event);
    delete event.currentTarget.dataset.softSpotlightTracking;
    setSpotlightOpacity(event.currentTarget, "0");
  };

  const handleFocus = (event: FocusEvent<HTMLDivElement>) => {
    onFocus?.(event);
    event.currentTarget.style.setProperty("--soft-spotlight-x", "50%");
    event.currentTarget.style.setProperty("--soft-spotlight-y", "50%");
    setSpotlightOpacity(event.currentTarget, "1");
  };

  const handleBlur = (event: FocusEvent<HTMLDivElement>) => {
    onBlur?.(event);
    if (!event.currentTarget.contains(event.relatedTarget)) setSpotlightOpacity(event.currentTarget, "0");
  };

  return (
    <div
      className={cn("soft-spotlight", className)}
      style={spotlightStyle}
      onPointerEnter={handlePointerEnter}
      onPointerMove={handlePointerMove}
      onPointerLeave={handlePointerLeave}
      onFocus={handleFocus}
      onBlur={handleBlur}
      {...props}
    >
      <span aria-hidden="true" className="soft-spotlight__glow" />
      <div className="soft-spotlight__content">{children}</div>
    </div>
  );
}
