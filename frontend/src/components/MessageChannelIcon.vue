<script setup lang="ts">
import telegram from "../assets/message-channels/telegram.svg";
import discord from "../assets/message-channels/discord.svg";
import slack from "../assets/message-channels/slack.svg";
import dingtalk from "../assets/dingtalk.png";
import feishu from "../assets/feishu.png";
import matrix from "../assets/message-channels/matrix.svg";
import whatsapp from "../assets/message-channels/whatsapp.svg";
import signal from "../assets/message-channels/signal.svg";
import wecom from "../assets/message-channels/wecom.png";
import wechat from "../assets/message-channels/wechat.svg";
import qqbot from "../assets/message-channels/qqbot.svg";
import bluebubbles from "../assets/message-channels/bluebubbles.png";
import yuanbao from "../assets/message-channels/yuanbao.png";

withDefaults(defineProps<{ provider: string; size?: number }>(), { size: 32 });
const icons: Record<string, string> = { telegram, discord, slack, dingtalk, feishu, matrix, whatsapp, signal, wecom, wechat, qqbot, bluebubbles, yuanbao };
</script>

<template>
  <img v-if="icons[provider]" class="message-channel-icon" :src="icons[provider]" :width="size" :height="size" alt="" aria-hidden="true" />
</template>

<style scoped>
.message-channel-icon { display:block; flex:0 0 auto; object-fit:contain; }
</style>
