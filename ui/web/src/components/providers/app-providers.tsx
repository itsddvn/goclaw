import { ThemeProvider } from "./theme-provider";
import { WsProvider } from "./ws-provider";

export function AppProviders({ children }: { children: React.ReactNode }) {
  return (
    <ThemeProvider>
      <WsProvider>{children}</WsProvider>
    </ThemeProvider>
  );
}
