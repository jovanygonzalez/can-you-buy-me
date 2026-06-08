/**
 * Manejo de sesión JWT en localStorage.
 *
 * El token se comparte same-origin entre Astro y Flutter (ver CLAUDE.md):
 * ambos clientes leen/escriben la MISMA clave. No cambiar TOKEN_KEY sin
 * actualizar también el lado Flutter.
 *
 * Todas las funciones son SSR-safe (Astro pre-renderiza en el server, donde
 * no existe `window`): devuelven null / no-op fuera del navegador.
 */

const TOKEN_KEY = "cybm_jwt";

/** Devuelve el JWT actual, o null si no hay sesión / estamos en SSR. */
export function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return window.localStorage.getItem(TOKEN_KEY);
}

/** Persiste el JWT tras un login/registro exitoso. */
export function setToken(token: string): void {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(TOKEN_KEY, token);
}

/** Borra la sesión (logout). */
export function clearToken(): void {
  if (typeof window === "undefined") return;
  window.localStorage.removeItem(TOKEN_KEY);
}

/** True si hay un token presente (no valida expiración: eso lo hace el backend). */
export function isAuthenticated(): boolean {
  return getToken() !== null;
}
