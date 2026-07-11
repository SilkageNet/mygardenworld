import type { Metadata } from "next";
import Script from "next/script";
import "./globals.css";
import { AuthProvider } from "@/lib/auth/context";
import { ThemeProvider } from "@/lib/theme/context";

const initializeTheme = `
try {
  var theme = localStorage.getItem("garden_theme") === "dark" ? "dark" : "light";
  document.documentElement.dataset.theme = theme;
  document.documentElement.classList.toggle("dark", theme === "dark");
} catch (_) {}
`;

export const metadata: Metadata = {
  applicationName: "小云朵",
  title: "小云朵",
  description: "小云朵本地花园自动化",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html
      lang="zh-CN"
      className="h-full antialiased"
      data-theme="light"
      suppressHydrationWarning
    >
      <body className="min-h-full overflow-y-auto bg-background text-foreground xl:h-full xl:overflow-hidden">
        <ThemeProvider>
          <AuthProvider>{children}</AuthProvider>
        </ThemeProvider>
        <Script
          id="theme-initializer"
          strategy="beforeInteractive"
          dangerouslySetInnerHTML={{ __html: initializeTheme }}
        />
      </body>
    </html>
  );
}
