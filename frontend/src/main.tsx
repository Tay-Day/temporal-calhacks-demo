import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "@radix-ui/themes/styles.css";
import App from "./App.tsx";
import { Theme, ThemePanel } from "@radix-ui/themes";
import "./main.scss";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <Theme appearance="dark">
      <App />
      <ThemePanel />
    </Theme>
  </StrictMode>
);
