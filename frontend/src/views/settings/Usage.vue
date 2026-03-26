<template>
  <errors v-if="error" :errorCode="error.status" />
  <div class="card" :class="{ active: active }">
    <div class="card-title">
      <h2>Usage Logs</h2>
    </div>

    <div class="card-content">
      <div v-if="loading">Loading logs...</div>
      <table v-else style="width: 100%; text-align: left; border-collapse: collapse;">
        <thead>
          <tr style="border-bottom: 2px solid var(--surfaceSecondary);">
            <th style="padding: 10px;">Datetime</th>
            <th style="padding: 10px;">Username</th>
            <th style="padding: 10px;">Map Used</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(log, idx) in logs" :key="idx" style="border-bottom: 1px solid var(--surfaceSecondary);">
            <td style="padding: 10px;">{{ formatDate(log.datetime) }}</td>
            <td style="padding: 10px;">{{ log.username }}</td>
            <td style="padding: 10px; display:flex; align-items:center;">
              <i class="material-icons" :style="{ color: log.mapUsed ? '#4caf50' : '#f44336' }">
                {{ log.mapUsed ? 'check_circle' : 'cancel' }}
              </i>
            </td>
          </tr>
          <tr v-if="logs.length === 0">
            <td colspan="3" style="padding: 10px; text-align: center;">No logs found.</td>
          </tr>
        </tbody>
      </table>
      
      <!-- Pagination Controls -->
      <div v-if="!loading && total > 0" style="margin-top: 20px; display: flex; justify-content: space-between; align-items: center; padding: 0 10px;">
        <div style="font-size: 13px; opacity: 0.7;">
          Showing {{ (page - 1) * limit + 1 }} - {{ Math.min(page * limit, total) }} of {{ total }} logs
        </div>
        <div style="display: flex; gap: 10px;">
          <button @click="changePage(page - 1)" :disabled="page <= 1" class="button button--flat" style="padding: 4px 12px; font-size: 13px;">
            <i class="material-icons" style="font-size: 18px;">chevron_left</i> Previous
          </button>
          <button @click="changePage(page + 1)" :disabled="page * limit >= total" class="button button--flat" style="padding: 4px 12px; font-size: 13px;">
            Next <i class="material-icons" style="font-size: 18px;">chevron_right</i>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { state } from "@/store";
import { fetchJSON } from "@/api/utils";
import Errors from "@/views/Errors.vue";

export default {
  name: "UsageSettings",
  components: {
    Errors,
  },
  data() {
    return {
      error: null,
      loading: true,
      logs: [],
      total: 0,
      page: 1,
      limit: 10,
    };
  },
  computed: {
    active() {
      // The settings component loops over constants and sets view to `{id}-main`
      return state.activeSettingsView === "usage-main";
    },
  },
  async mounted() {
    this.fetchLogs();
  },
  methods: {
    async fetchLogs() {
      this.loading = true;
      try {
        const res = await fetchJSON(`/api/usage?page=${this.page}`);
        this.logs = res.logs || [];
        this.total = res.total || 0;
        this.limit = res.limit || 10;
      } catch (e) {
        this.error = e;
      } finally {
        this.loading = false;
      }
    },
    changePage(p) {
      this.page = p;
      this.fetchLogs();
    },
    formatDate(ds) {
      if (!ds) return "Unknown";
      return new Date(ds).toLocaleString();
    }
  }
};
</script>
