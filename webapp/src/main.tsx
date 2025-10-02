import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { FliptProvider } from "@flipt-io/flipt-client-react";
import "./index.css";
import App from "./App.tsx";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <FliptProvider
      options={{
        environment: "onoffinc",
        namespace: "default",
        url: "http://localhost:8080",
        updateInterval: 10, // Fetch flag updates every 10 seconds
        // Add other configuration options as needed
      }}
    >
      <App />
    </FliptProvider>
  </StrictMode>,
);
