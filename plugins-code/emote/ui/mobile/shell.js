(() => {
  const button = document.getElementById("mobile-back");
  if (!button) return;
  button.addEventListener("click", async () => {
    const toggle = document.getElementById("toggle");
    if (toggle) {
      toggle.click();
      return;
    }
    let target = "/extensions";
    try {
      const context = await window.amitiaUI.getContext();
      const route = String(context?.route || "");
      const characterId = String(context?.characterId || context?.character?.id || "");
      const match = route.match(/^\/characters\/([^/?]+)/);
      if (match) target = `/characters/${match[1]}`;
      else if (characterId) target = `/characters/${characterId}`;
      else if (route.startsWith("/channels/wechat-personal/messages")) target = "/channels/wechat-personal";
      else if (route === "/channels/wechat-personal") target = "/channels";
      else if (route === "/emotes") target = "/chat";
      await window.amitiaUI.navigate(target, "back");
    } catch {
      window.location.href = target;
    }
  });
})();
