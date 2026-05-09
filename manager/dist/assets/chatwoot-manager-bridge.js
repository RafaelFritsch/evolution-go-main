(function () {
  const STYLE_ID = "chatwoot-manager-bridge-style";
  const HEADER_BUTTON_ID = "chatwoot-manager-link";
  const RAIL_ID = "chatwoot-manager-rail";

  installStyle();
  syncBridge();

  const observer = new MutationObserver(syncBridge);
  observer.observe(document.body, { childList: true, subtree: true });

  window.addEventListener("popstate", syncBridge);
  document.addEventListener("click", function () {
    window.setTimeout(syncBridge, 50);
  });

  function syncBridge() {
    const match = window.location.pathname.match(/^\/manager\/instances\/([^/]+)\/settings$/);
    if (!match) {
      removeIfExists(HEADER_BUTTON_ID);
      removeIfExists(RAIL_ID);
      return;
    }

    const instanceId = match[1];
    const instanceName = findInstanceName();
    if (!instanceName) {
      return;
    }

    const href = `/manager/chatwoot?instance=${encodeURIComponent(instanceName)}&instanceId=${encodeURIComponent(instanceId)}`;
    mountHeaderButton(href);
    mountRail(href, instanceName);
  }

  function findInstanceName() {
    const paragraphs = Array.from(document.querySelectorAll("p"));
    const match = paragraphs.find(function (node) {
      return node.textContent && node.textContent.trim() && node.closest(".border-b");
    });
    return match ? match.textContent.trim() : "";
  }

  function mountHeaderButton(href) {
    if (document.getElementById(HEADER_BUTTON_ID)) {
      document.getElementById(HEADER_BUTTON_ID).href = href;
      return;
    }

    const headerRow = Array.from(document.querySelectorAll("div"))
      .find(function (node) {
        return node.className && String(node.className).includes("items-center justify-between");
      });

    if (!headerRow) {
      return;
    }

    const action = document.createElement("a");
    action.id = HEADER_BUTTON_ID;
    action.href = href;
    action.className = "inline-flex items-center gap-2 rounded-md border border-input bg-background px-3 py-2 text-sm font-medium text-foreground transition hover:bg-accent";
    action.textContent = "Chatwoot";
    headerRow.appendChild(action);
  }

  function mountRail(href, instanceName) {
    let rail = document.getElementById(RAIL_ID);
    if (!rail) {
      rail = document.createElement("aside");
      rail.id = RAIL_ID;
      rail.className = "chatwoot-manager-rail";
      rail.innerHTML = [
        '<p class="chatwoot-manager-rail-kicker">Instancia</p>',
        '<h3 class="chatwoot-manager-rail-title">Integracoes</h3>',
        '<p class="chatwoot-manager-rail-copy"></p>',
        '<a class="chatwoot-manager-rail-link" data-role="chatwoot-link">Abrir Chatwoot</a>',
      ].join("");
      document.body.appendChild(rail);
    }

    rail.querySelector(".chatwoot-manager-rail-copy").textContent = instanceName;
    rail.querySelector('[data-role="chatwoot-link"]').href = href;
  }

  function installStyle() {
    if (document.getElementById(STYLE_ID)) {
      return;
    }

    const style = document.createElement("style");
    style.id = STYLE_ID;
    style.textContent = [
      ".chatwoot-manager-rail{position:fixed;right:24px;top:96px;z-index:30;display:none;width:220px;border:1px solid rgba(148,163,184,.28);border-radius:20px;padding:16px;background:rgba(255,255,255,.88);backdrop-filter:blur(14px);box-shadow:0 20px 45px rgba(15,23,42,.12)}",
      ".chatwoot-manager-rail-kicker{margin:0 0 6px;font-size:11px;font-weight:700;letter-spacing:.18em;text-transform:uppercase;color:rgb(13 148 136)}",
      ".chatwoot-manager-rail-title{margin:0;font-size:16px;font-weight:700;color:rgb(15 23 42)}",
      ".chatwoot-manager-rail-copy{margin:8px 0 14px;font-size:13px;color:rgb(71 85 105)}",
      ".chatwoot-manager-rail-link{display:inline-flex;width:100%;justify-content:center;border-radius:12px;background:rgb(15 118 110);padding:10px 14px;font-size:14px;font-weight:600;color:white;text-decoration:none}",
      ".chatwoot-manager-rail-link:hover{background:rgb(17 94 89)}",
      "@media (min-width: 1200px){.chatwoot-manager-rail{display:block}}",
    ].join("");
    document.head.appendChild(style);
  }

  function removeIfExists(id) {
    const element = document.getElementById(id);
    if (element) {
      element.remove();
    }
  }
})();
