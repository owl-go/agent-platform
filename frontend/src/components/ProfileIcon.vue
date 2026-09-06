<script setup lang="ts">
import { Compass, Sparkles, UserRound, Users } from "@lucide/vue";
defineProps<{ icon?: string; background?: string; team?: boolean }>();
</script>
<template><span class="profile-icon" :class="['profile-icon-' + (background || 'sage')]" aria-hidden="true"><Users v-if="team || icon === 'users' || icon === 'team'" :size="21" /><Sparkles v-else-if="icon === 'sparkles'" :size="21" /><Compass v-else-if="icon === 'compass'" :size="21" /><UserRound v-else :size="21" /></span></template>
