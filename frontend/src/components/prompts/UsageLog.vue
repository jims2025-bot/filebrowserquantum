<template>
  <div class="card floating">
    <div class="card-title">
      <h2>Usage Logs</h2>
    </div>

    <div class="card-content">
      <errors v-if="error" :errorCode="error.status" />
      <div v-if="loading" class="spinner-container">
        <div class="spinner">
          <div class="bounce1"></div>
          <div class="bounce2"></div>
          <div class="bounce3"></div>
        </div>
      </div>
      <template v-else-if="logs && logs.length >= 0">
        <div v-if="logs.length > 0" class="table-container">
          <table style="width: 100%; text-align: left; border-collapse: collapse;">
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
            </tbody>
          </table>
        </div>
        
        <div v-if="logs && logs.length === 0" style="padding: 20px; text-align: center; opacity: 0.5;">
           No usage logs found.
        </div>

        <!-- Pagination Controls -->
        <div v-if="total > 0 && !loading" style="margin-top: 20px; display: flex; justify-content: space-between; align-items: center; padding: 0 10px;">
          <div style="font-size: 13px; opacity: 0.7;">
            {{ Math.max(0, (page - 1) * limit + 1) }}-{{ Math.min(page * limit, total) }} of {{ total }}
          </div>
          <div style="display: flex; gap: 10px;">
            <button @click="changePage(page - 1)" :disabled="page <= 1" class="button button--flat" style="padding: 4px 8px;">
              <i class="material-icons">chevron_left</i>
            </button>
            <button @click="changePage(page + 1)" :disabled="page * limit >= total" class="button button--flat" style="padding: 4px 8px;">
              <i class="material-icons">chevron_right</i>
            </button>
          </div>
        </div>
      </template>
    </div>

    <div class="card-action">
      <button
        class="button button--flat"
        @click="close"
        :aria-label="$t('buttons.close')"
        :title="$t('buttons.close')"
      >
        {{ $t("buttons.close") }}
      </button>
    </div>
  </div>
</template>

<script>
import { fetchJSON } from "@/api/utils";
import { mutations } from "@/store";
import Errors from "@/views/Errors.vue";

export default {
  name: "UsageLog",
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
  async mounted() {
    await this.fetchLogs();
  },
  methods: {
    close() {
      mutations.closeHovers();
    },
    async fetchLogs() {
      this.loading = true;
      this.error = null;
      try {
        const res = await fetchJSON(`/api/usage?page=${this.page}`);
        if (res && typeof res === 'object') {
          this.logs = Array.isArray(res.logs) ? res.logs : [];
          this.total = typeof res.total === 'number' ? res.total : 0;
          this.limit = typeof res.limit === 'number' ? res.limit : 10;
        } else {
           this.logs = [];
           this.total = 0;
        }
      } catch (e) {
        this.error = e;
        console.error("Failed to fetch logs:", e);
      } finally {
        this.loading = false;
      }
    },
    changePage(p) {
      if (typeof p !== 'number' || p < 1) return;
      this.page = p;
      this.fetchLogs();
    },
    formatDate(ds) {
      if (!ds) return "---";
      try {
        const d = new Date(ds);
        if (isNaN(d.getTime())) return "Invalid Date";
        return d.toLocaleString();
      } catch (e) {
        return "---";
      }
    }
  }
};
</script>

<style scoped>
.floating {
  max-width: 50em !important;
}
.spinner-container {
  display: flex;
  justify-content: center;
  padding: 2em;
}
</style>
