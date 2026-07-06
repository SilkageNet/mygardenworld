import * as React from "react"
import { Input as InputPrimitive } from "@base-ui/react/input"

import { cn } from "@/lib/utils"

function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <InputPrimitive
      type={type}
      data-slot="input"
      className={cn(
        "h-9 w-full min-w-0 rounded-md border border-input/85 bg-white/66 px-3 py-1 text-base text-foreground shadow-[inset_0_1px_0_rgba(255,255,255,0.78)] transition-colors outline-none file:inline-flex file:h-6 file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-foreground placeholder:text-muted-foreground/72 focus-visible:border-ring focus-visible:bg-white/88 focus-visible:ring-3 focus-visible:ring-ring/24 disabled:pointer-events-none disabled:cursor-not-allowed disabled:bg-input/42 disabled:opacity-55 aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20 md:text-sm dark:bg-input/42 dark:shadow-[inset_0_1px_0_rgba(255,255,255,0.06)] dark:focus-visible:bg-input/58 dark:disabled:bg-input/80 dark:aria-invalid:border-destructive/50 dark:aria-invalid:ring-destructive/40",
        className
      )}
      {...props}
    />
  )
}

export { Input }
