import { installWindowErrorTracking, recordClientError } from "./runtime/client-error-tracker";

installWindowErrorTracking();

let settled = false;
const timer = window.setTimeout(() => {
  showStartupFailure(new Error("主界面启动超时，请重试或查看日志"));
}, 30000);

function finishStartup(): void {
  settled = true;
  window.clearTimeout(timer);
  window.removeEventListener("amitia:mounted", finishStartup);
}

function showStartupFailure(error: unknown): void {
  if (settled) return;
  finishStartup();
  const message = error instanceof Error ? error.message : String(error);
  recordClientError({
    message,
    stack: error instanceof Error ? error.stack : "",
    source: "bootstrap",
  });
  console.error("[Amitia] 主界面启动失败:", error);
  const root = document.getElementById("app");
  if (!root) return;
  const panel = document.createElement("main");
  panel.setAttribute("role", "alert");
  panel.style.cssText = "max-width:560px;margin:15vh auto;padding:24px;font-family:system-ui;line-height:1.6;color:#202124;background:#fff";
  const title = document.createElement("h1");
  title.textContent = "Amitia 主界面启动失败";
  const detail = document.createElement("p");
  detail.textContent = message;
  const retry = document.createElement("button");
  retry.textContent = "重新加载";
  retry.addEventListener("click", () => window.location.reload());
  panel.append(title, detail, retry);
  root.replaceChildren(panel);
}

window.addEventListener("amitia:mounted", finishStartup);
void import("./main")
  .then((module) => module.applicationReady)
  .then(finishStartup)
  .catch(showStartupFailure);
