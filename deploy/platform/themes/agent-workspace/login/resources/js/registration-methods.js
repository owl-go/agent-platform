// Keep the inherited Keycloak form, broker URLs and accessibility behavior.
// This script only localizes labels of the two platform-owned scan providers.
document.addEventListener("DOMContentLoaded", () => {
  const english = document.documentElement.lang.startsWith("en");
  const labels = {
    "aw-feishu": english ? "Feishu sign in / register" : "飞书扫码登录 / 注册",
    "aw-wechat_official": english ? "WeChat Official Account sign in / register" : "微信公众号登录 / 注册",
  };
  for (const [alias, label] of Object.entries(labels)) {
    const link = document.getElementById(`social-${alias}`);
    if (!link) continue;
    const text = link.querySelector("span");
    if (text) text.textContent = label;
    else link.textContent = label;
    link.setAttribute("aria-label", label);
  }
});
