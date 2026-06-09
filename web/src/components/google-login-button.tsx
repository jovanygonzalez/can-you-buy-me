import { useEffect, useRef, useState } from "react";

import { authClient } from "@/lib/grpc";
import { setToken } from "@/lib/auth";

// Tipos mínimos de Google Identity Services (https://accounts.google.com/gsi/client).
// Solo declaramos lo que usamos, para no depender de @types/google.accounts.
interface GoogleCredentialResponse {
  credential: string; // el id_token (JWT) de Google
}
interface GoogleIdApi {
  initialize(config: {
    client_id: string;
    callback: (res: GoogleCredentialResponse) => void;
  }): void;
  renderButton(
    parent: HTMLElement,
    options: { theme?: string; size?: string; width?: number; text?: string },
  ): void;
  prompt(): void;
}
declare global {
  interface Window {
    google?: { accounts: { id: GoogleIdApi } };
  }
}

const GIS_SRC = "https://accounts.google.com/gsi/client";
const CLIENT_ID = import.meta.env.PUBLIC_GOOGLE_CLIENT_ID as string | undefined;

// Carga el script de GIS una sola vez (idempotente entre montajes).
function loadGisScript(): Promise<void> {
  return new Promise((resolve, reject) => {
    if (window.google?.accounts?.id) {
      resolve();
      return;
    }
    const existing = document.querySelector<HTMLScriptElement>(
      `script[src="${GIS_SRC}"]`,
    );
    if (existing) {
      existing.addEventListener("load", () => resolve());
      existing.addEventListener("error", () => reject(new Error("GIS load failed")));
      return;
    }
    const script = document.createElement("script");
    script.src = GIS_SRC;
    script.async = true;
    script.defer = true;
    script.onload = () => resolve();
    script.onerror = () => reject(new Error("GIS load failed"));
    document.head.appendChild(script);
  });
}

type State =
  | { kind: "loading" }
  | { kind: "ready" }
  | { kind: "exchanging" }
  | { kind: "error"; message: string };

/**
 * Botón de login con Google. Obtiene el id_token vía Google Identity Services,
 * lo intercambia por el JWT propio con AuthService.LoginWithProvider (gRPC-Web)
 * y deja la sesión iniciada en localStorage, redirigiendo a /account.
 */
export default function GoogleLoginButton() {
  const buttonRef = useRef<HTMLDivElement>(null);
  const [state, setState] = useState<State>({ kind: "loading" });

  // El callback de GIS: recibe el id_token y lo canjea por nuestro JWT.
  async function handleCredential(res: GoogleCredentialResponse) {
    setState({ kind: "exchanging" });
    try {
      const out = await authClient.loginWithProvider({
        provider: "google",
        idToken: res.credential,
      });
      setToken(out.jwtToken);
      window.location.href = "/account";
    } catch (err) {
      setState({
        kind: "error",
        message: err instanceof Error ? err.message : String(err),
      });
    }
  }

  useEffect(() => {
    if (!CLIENT_ID) {
      setState({
        kind: "error",
        message: "Falta PUBLIC_GOOGLE_CLIENT_ID (configúralo en web/.env).",
      });
      return;
    }
    let cancelled = false;
    loadGisScript()
      .then(() => {
        if (cancelled || !buttonRef.current || !window.google) return;
        window.google.accounts.id.initialize({
          client_id: CLIENT_ID,
          callback: (res) => void handleCredential(res),
        });
        window.google.accounts.id.renderButton(buttonRef.current, {
          theme: "outline",
          size: "large",
          text: "continue_with",
          width: 320,
        });
        setState({ kind: "ready" });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setState({
          kind: "error",
          message: err instanceof Error ? err.message : String(err),
        });
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <div className="flex flex-col items-center gap-3">
      {/* GIS renderiza su botón dentro de este contenedor */}
      <div ref={buttonRef} />

      {state.kind === "loading" && (
        <p className="text-sm text-muted-foreground">Cargando Google…</p>
      )}
      {state.kind === "exchanging" && (
        <p className="text-sm text-muted-foreground">Iniciando sesión…</p>
      )}
      {state.kind === "error" && (
        <p className="text-sm text-cybm-danger">❌ {state.message}</p>
      )}
    </div>
  );
}
