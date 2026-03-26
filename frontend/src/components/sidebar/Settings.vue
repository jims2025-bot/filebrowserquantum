<template>
  <div
    v-for="setting in settings"
    :key="setting.id + '-sidebar'"
    :id="setting.id + '-sidebar'"
    class="card clickable"
    @click="setView(setting.id + '-main')"
    :class="{
      hidden: !shouldShow(setting),
      'active-settings': active(setting.id + '-main'),
    }"
  >
    <div v-if="shouldShow(setting)" class="settings-card">{{ setting.label }}</div>
  </div>
</template>

<script>
import { state, getters, mutations } from "@/store";
import { settings } from "@/utils/constants";
import { router } from "@/router";

export default {
  name: "SidebarSettings",
  data() {
    return {
      settings, // Initialize the settings array in data
    };
  },
  computed: {
    currentHash: () => getters.currentHash(),
  },
  methods: {
    shouldShow(setting) {
      if (!state.user) return false;
      const permObj = state.user.permissions || state.user.perm || {};
      const requiredPerms = setting?.permissions || {};
      if (Object.keys(requiredPerms).length === 0) return true;
      
      const checkPerm = (key) => {
        const lowerKey = key.toLowerCase();
        return permObj[key] === true || 
               permObj[lowerKey] === true || 
               permObj[key.charAt(0).toUpperCase() + key.slice(1)] === true;
      };

      if (setting.anyPermission) {
        return Object.keys(requiredPerms).some(checkPerm);
      }
      return Object.keys(requiredPerms).every(checkPerm);
    },
    active: (view) => state.activeSettingsView === view,
    setView(view) {
      if (view === "usage-main") {
        mutations.showHover("usageLog");
        return;
      }
      const currentPath = state.route.path;
      if (currentPath !== "/settings" && currentPath !== "/settings/") {
        router.push({ path: "/settings", hash: "#" + view }, () => {
          // Callback after push is completed
          mutations.setActiveSettingsView(view);
        });
      } else {
        mutations.setActiveSettingsView(view);
      }
    },
  },
};
</script>
<style>
.active-settings {
  font-weight: bold;
  /* border-color: white; */
  border-style: solid;
}
.settings-card {
  padding: 1em;
  text-align: center;
}
</style>
