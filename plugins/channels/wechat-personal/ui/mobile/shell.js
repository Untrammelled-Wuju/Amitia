(() => {
  const button = document.getElementById("mobile-back");
  if (!button) return;
  button.addEventListener("click", async () => {
    let target = "/channels";
    try {
      const context = await window.amitiaUI.getContext();
      const route = String(context?.route || "");
      if (route.startsWith("/channels/wechat-personal/messages")) target = "/channels/wechat-personal";
      await window.amitiaUI.navigate(target, "back");
    } catch {
      window.location.href = target;
    }
  });
})();
