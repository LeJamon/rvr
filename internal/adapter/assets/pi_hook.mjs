// rvr pi-compatible lifecycle extension.
//
// Loaded with `pi -e <this file>` or `omp -e <this file>`. Both harnesses load
// ESM extensions through a default-exported factory that receives the
// ExtensionAPI. This extension connects to the unix socket named by
// RVR_HOOK_SOCKET and reports lifecycle events as newline-delimited JSON:
//   {"event": "agent_start", "ref": "<session file path>"}
//
// The session file path used for exact native resume is not in the event
// payloads; it is read from ctx.sessionManager.getSessionFile().
import * as net from "node:net";

export default function (api) {
  const socketPath = process.env.RVR_HOOK_SOCKET;
  let sock = null;
  if (socketPath) {
    sock = net.createConnection(socketPath);
    sock.on("error", () => {}); // never let a broken pipe crash the agent
  }

  const emit = (event, extra) => {
    if (!sock) return;
    try {
      sock.write(JSON.stringify({ event, ...(extra || {}) }) + "\n");
    } catch (_) {
      // best-effort; state reporting must never disrupt the agent
    }
  };

  const refOf = (ctx) => {
    try {
      return (ctx && ctx.sessionManager && ctx.sessionManager.getSessionFile()) || "";
    } catch (_) {
      return "";
    }
  };

  api.on("session_start", (_event, ctx) => emit("session_start", { ref: refOf(ctx) }));
  api.on("agent_start", (_event, ctx) => emit("agent_start", { ref: refOf(ctx) }));
  api.on("agent_end", () => emit("agent_end"));
  api.on("session_shutdown", () => emit("session_shutdown"));
}
