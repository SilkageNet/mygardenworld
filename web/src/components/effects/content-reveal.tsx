import type { HTMLAttributes } from "react";
import { cn } from "@/lib/utils";

export function ContentReveal({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("content-reveal min-w-0", className)} {...props} />;
}
