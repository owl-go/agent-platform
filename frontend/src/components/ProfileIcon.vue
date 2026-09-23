<script setup lang="ts">
import { Code2, Compass, Sparkles, Terminal, UserRound, Users } from "@lucide/vue";
defineProps<{ icon?: string; background?: string; team?: boolean }>();
</script>
<template><span class="profile-icon" :class="['profile-icon-' + (background || 'sage')]" aria-hidden="true"><img v-if="icon?.startsWith('data:image/') || icon?.startsWith('blob:')" :src="icon" alt="" /><Users v-else-if="team || icon === 'users' || icon === 'team'" :size="21" /><Sparkles v-else-if="icon === 'sparkles'" :size="21" /><Compass v-else-if="icon === 'compass'" :size="21" /><Code2 v-else-if="icon === 'code'" :size="21" /><Terminal v-else-if="icon === 'terminal'" :size="21" /><UserRound v-else :size="21" /></span></template>
