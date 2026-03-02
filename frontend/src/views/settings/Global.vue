<template>
  <errors v-if="error" :errorCode="error.status" />
  <form class="card" @submit.prevent="save">
    <div class="card-title">
      <h2>{{ $t("settings.globalSettings") }}</h2>
    </div>

    <div class="card-content">
      <p>
        <input type="checkbox" v-model="showDebugInfo" @change="toggleDebugInfo" />
        {{ $t('settings.showDebugInfo') }}
      </p>

      <h3 style="margin-top: 2rem;">Facial Recognition Storage & API</h3>
      <p class="small">Configure the connection to the standalone Python facial recognition microservice.</p>
      
      <p v-if="selectedSettings && selectedSettings.integrations">
        <input type="checkbox" id="facerec-enabled" v-model="selectedSettings.integrations.facerec.enabled" />
        <label for="facerec-enabled">Enable System-Wide Facial Recognition Scans</label>
      </p>

      <p v-if="selectedSettings && selectedSettings.integrations">
        <label for="facerec-server">Python Microservice URL:</label>
        <input
          class="input input--block"
          type="text"
          id="facerec-server"
          v-model="selectedSettings.integrations.facerec.serverAddress"
          placeholder="http://localhost:8000"
        />
      </p>

    </div>

    <div class="card-action">
      <input class="button button--flat" type="submit" :value="$t('buttons.update')" />
    </div>
  </form>
</template>

<script>
import { notify } from "@/notify";
import { state, mutations, getters } from "@/store";
import { settingsApi } from "@/api";
import Errors from "@/views/Errors.vue";

export default {
  name: "settings",
  components: {
    Errors,
  },
  data: function () {
    return {
      error: null,
      originalSettings: null,
      selectedSettings: state.settings,
    };
  },
  computed: {
    loading() {
      return getters.isLoading();
    },
    user() {
      return state.user;
    },
    showDebugInfo: {
      get() { return state.showDebugInfo; },
      set(val) { /* mutations handle toggle */ }
    }
  },
  async created() {
    mutations.setLoading("settings", true);
    const original = await settingsApi.get();
    mutations.setSettings(original);
    mutations.setLoading("settings", false);
  },
  methods: {
    updateRules(updatedRules) {
      this.selectedSettings = { ...this.selectedSettings, rules: updatedRules };
    },
    capitalize(name, where = "_") {
      if (where === "caps") where = /(?=[A-Z])/;
      let splitted = name.split(where);
      name = "";

      for (let i = 0; i < splitted.length; i++) {
        name += splitted[i].charAt(0).toUpperCase() + splitted[i].slice(1) + " ";
      }

      return name.slice(0, -1);
    },
    async save() {
      try {
        mutations.setSettings(this.selectedSettings);
        await settingsApi.update(state.settings);
        notify.showSuccess(this.$t("settings.settingsUpdated"));
      } catch (e) {
        notify.showError(e);
      }
    },
    toggleDebugInfo() {
      mutations.toggleDebugInfo();
    }
  },
};
</script>
