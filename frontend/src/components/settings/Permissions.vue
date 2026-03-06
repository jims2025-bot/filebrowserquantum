<template>
  <div class="settings-items">
    <h3>{{ $t("settings.permissions-name") }}</h3>
    <p class="small">{{ $t("settings.permissionsHelp") }}</p>
    <div class="permission-group">
      <ToggleSwitch class="item" v-model="permissions.admin" :name="$t('settings.permissions.admin')" />
      <p class="description">{{ $t("settings.permissions.adminDescription") }}</p>
    </div>
    <div class="permission-group">
      <ToggleSwitch
        class="item"
        v-model="permissions.modify"
        :name="$t('settings.permissions.modify')"
      />
      <p class="description">{{ $t("settings.permissions.modifyDescription") }}</p>
    </div>
    <div class="permission-group">
      <ToggleSwitch class="item" v-model="permissions.share" :name="$t('settings.permissions.share')" />
      <p class="description">{{ $t("settings.permissions.shareDescription") }}</p>
    </div>
    <div class="permission-group">
      <ToggleSwitch class="item" v-model="permissions.api" :name="$t('settings.permissions.api')" />
      <p class="description">{{ $t("settings.permissions.apiDescription") }}</p>
    </div>
    <div class="permission-group">
      <ToggleSwitch
        class="item"
        v-model="permissions.realtime"
        :name="$t('settings.permissions.realtime')"
      />
      <p class="description">{{ $t("settings.permissions.realtimeDescription") }}</p>
    </div>
    <div class="permission-group">
      <ToggleSwitch
        class="item"
        v-model="permissions.updateMap"
        :name="$t('settings.permissions.updateMap')"
      />
      <p class="description">{{ $t("settings.permissions.updateMapDescription") }}</p>
    </div>
    <div class="permission-group">
      <ToggleSwitch
        class="item"
        v-model="permissions.manageServiceShares"
        :name="$t('settings.permissions.manageServiceShares')"
      />
      <p class="description">{{ $t("settings.permissions.manageServiceSharesDescription") }}</p>
    </div>
    <div class="permission-group">
      <ToggleSwitch
        class="item"
        v-model="permissions.manageFaces"
        :name="$t('settings.permissions.manageFaces')"
      />
      <p class="description">{{ $t("settings.permissions.manageFacesDescription") }}</p>
    </div>
    <div class="permission-group">
      <ToggleSwitch
        class="item"
        v-model="permissions.runFaceScan"
        :name="$t('settings.permissions.runFaceScan')"
      />
      <p class="description">{{ $t("settings.permissions.runFaceScanDescription") }}</p>
    </div>
    <div class="permission-group">
      <ToggleSwitch
        class="item"
        v-model="permissions.viewACDSee"
        :name="$t('settings.permissions.viewACDSee')"
      />
      <p class="description">{{ $t("settings.permissions.viewACDSeeDescription") }}</p>
    </div>
    <div class="permission-group">
      <ToggleSwitch
        class="item"
        v-model="permissions.rebuildThumbnails"
        :name="$t('settings.permissions.rebuildThumbnails')"
      />
      <p class="description">{{ $t("settings.permissions.rebuildThumbnailsDescription") }}</p>
    </div>
  </div>
</template>

<script>
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";

export default {
  name: "permissions",
  props: {
    permissions: {
      type: Object,
      required: true,
    },
  },
  components: {
    ToggleSwitch,
  },
};
</script>

<style scoped>
.permission-group {
  margin-bottom: 1.5em;
}

.permission-group .item {
  margin-bottom: 0.25em !important;
}

.description {
  margin: 0 0 0 3.5em;
  font-size: 0.85em;
  color: var(--textSecondary);
  line-height: 1.4;
}
</style>
