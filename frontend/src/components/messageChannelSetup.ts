// Each entry describes the transport implemented by this platform. Auth modes
// are explicit: no OAuth or QR button is offered without a server adapter.
export interface ChannelSetup {
  fields: string[];
  qr?: boolean;
  audience: "both" | "direct" | "rooms";
  receive: "webhook" | "connection" | "polling";
  docs: string;
  sender: string;
  group?: string;
}
export const channelSetups: Record<string, ChannelSetup> = {
  telegram: { fields: ["bot_token"], audience: "both", receive: "webhook", docs: "https://core.telegram.org/bots/tutorial", sender: "Telegram User ID", group: "Telegram Chat ID" },
  discord: { fields: ["bot_token"], audience: "both", receive: "connection", docs: "https://docs.discord.com/developers/quick-start/getting-started", sender: "Discord User ID", group: "Channel / Thread ID" },
  slack: { fields: ["bot_token", "signing_secret"], audience: "both", receive: "webhook", docs: "https://docs.slack.dev/apis/events-api/", sender: "Slack User ID", group: "Slack Channel ID" },
  dingtalk: { fields: ["client_id", "client_secret", "corp_id"], audience: "both", receive: "connection", docs: "https://open-dingtalk.github.io/developerpedia/docs/explore/tutorials/stream/overview/", sender: "Staff ID", group: "Conversation ID" },
  feishu: { fields: ["app_id", "app_secret", "tenant_key"], audience: "both", receive: "connection", docs: "https://open.feishu.cn/document/server-docs/im-v1/message/events/receive", sender: "Open ID", group: "Chat ID" },
  matrix: { fields: ["endpoint", "access_token"], audience: "rooms", receive: "polling", docs: "https://spec.matrix.org/latest/client-server-api/", sender: "@user:homeserver", group: "!room:homeserver" },
  whatsapp: { fields: ["access_token", "app_secret", "verify_token", "phone_number_id", "business_account_id", "graph_version"], audience: "direct", receive: "webhook", docs: "https://developers.facebook.com/docs/whatsapp/cloud-api/", sender: "WhatsApp User ID" },
  signal: { fields: ["endpoint", "bridge_token", "account_id"], audience: "both", receive: "connection", docs: "https://github.com/AsamK/signal-cli", sender: "Signal ACI", group: "Signal Group ID" },
  wecom: { fields: ["bot_id", "bot_secret"], audience: "both", receive: "connection", docs: "https://cloud.tencent.com/document/product/1831/137051", sender: "User ID", group: "Chat ID" },
  wechat: { fields: [], qr: true, audience: "direct", receive: "polling", docs: "https://github.com/Tencent/openclaw-weixin/blob/main/docs/protocol.md", sender: "iLink User ID" },
  qqbot: { fields: ["app_id", "app_secret"], qr: true, audience: "both", receive: "connection", docs: "https://bot.q.qq.com/wiki/develop/api-v2/", sender: "user_openid / member_openid", group: "group_openid" },
  bluebubbles: { fields: ["endpoint", "password"], audience: "direct", receive: "polling", docs: "https://docs.bluebubbles.app/server/developer-guides/rest-api-and-webhooks", sender: "iMessage handle address" },
  yuanbao: { fields: ["app_key", "app_secret"], audience: "both", receive: "connection", docs: "https://github.com/Tencent/yuanbao-openclaw-plugin", sender: "from_account", group: "group_code" },
};
