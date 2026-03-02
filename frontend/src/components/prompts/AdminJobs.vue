<template>
  <div class="overlay" @click.self="close">
    <div class="jobs-modal">
      <div class="jobs-header">
        <span class="material-icons">schedule</span>
        <h2>System Jobs</h2>
        <button class="close-btn" @click="close" title="Close">
          <span class="material-icons">close</span>
        </button>
      </div>

      <div class="jobs-body">
        <p class="jobs-note">
          Jobs run automatically on their scheduled day. Use <strong>Run Now</strong> to trigger manually.
        </p>

        <div v-if="loading" class="jobs-loading">
          <span class="material-icons spinning">sync</span>
          Loading…
        </div>

        <div v-else class="jobs-list">
          <div
            v-for="job in jobs"
            :key="job.name"
            class="job-card"
            :class="{ running: job.isRunning }"
          >
            <div class="job-info">
              <div class="job-name">
                <span v-if="job.isRunning" class="material-icons spinning job-running-icon">sync</span>
                <span v-else class="material-icons job-idle-icon">check_circle</span>
                {{ job.name }}
                <span v-if="job.isRunning" class="badge running-badge">Running…</span>
              </div>
              <p class="job-desc">{{ job.description }}</p>
              <div class="job-meta">
                <span><strong>Schedule:</strong> {{ job.schedule }}</span>
                <span><strong>Next run:</strong> {{ formatDate(job.nextRun) }}</span>
                <span v-if="job.lastRun && job.lastRun !== '0001-01-01T00:00:00Z'">
                  <strong>Last run:</strong> {{ formatDate(job.lastRun) }}
                </span>
                <span v-else><strong>Last run:</strong> Never</span>
              </div>
              <div v-if="job.lastError" class="job-error">
                <span class="material-icons">error</span> {{ job.lastError }}
              </div>
            </div>
            <div class="job-actions">
              <button
                class="run-btn"
                :disabled="job.isRunning"
                :title="job.isRunning ? 'Job is already running' : 'Run this job now'"
                @click="runJob(job.name)"
              >
                <span class="material-icons">play_arrow</span>
                Run Now
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { mutations } from '@/store';
import { getJobsStatus, runJobNow } from '@/api/files';

export default {
  name: 'AdminJobs',
  data() {
    return {
      jobs: [],
      loading: true,
      pollTimer: null,
    };
  },
  async mounted() {
    await this.fetchJobs();
    // Poll every 3 seconds while modal is open
    this.pollTimer = setInterval(this.fetchJobs, 3000);
  },
  beforeUnmount() {
    clearInterval(this.pollTimer);
  },
  methods: {
    close() {
      clearInterval(this.pollTimer);
      mutations.closeHovers();
    },
    async fetchJobs() {
      try {
        const data = await getJobsStatus();
        if (data) this.jobs = data;
      } catch (e) {
        console.error('Error fetching jobs:', e);
      } finally {
        this.loading = false;
      }
    },
    async runJob(name) {
      try {
        await runJobNow(name);
        // Immediately refresh to show running state
        await this.fetchJobs();
      } catch (e) {
        console.error('Error starting job:', e);
      }
    },
    formatDate(iso) {
      if (!iso || iso === '0001-01-01T00:00:00Z') return '—';
      const d = new Date(iso);
      return d.toLocaleDateString(undefined, {
        weekday: 'short', month: 'short', day: 'numeric',
        hour: '2-digit', minute: '2-digit'
      });
    },
  },
};
</script>

<style scoped>
.overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.55);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9000;
}

.jobs-modal {
  background: var(--background, #1e1e2e);
  border-radius: 12px;
  width: min(680px, 95vw);
  max-height: 85vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.5);
  overflow: hidden;
}

.jobs-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 18px 20px;
  border-bottom: 1px solid rgba(255,255,255,0.08);
  background: rgba(255,255,255,0.03);
}

.jobs-header .material-icons {
  color: #7c8cf8;
  font-size: 22px;
}

.jobs-header h2 {
  margin: 0;
  font-size: 1.15rem;
  font-weight: 600;
  flex: 1;
  color: var(--textPrimary, #e0e0e0);
}

.close-btn {
  background: none;
  border: none;
  cursor: pointer;
  color: var(--textSecondary, #888);
  padding: 4px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  transition: background 0.15s;
}
.close-btn:hover { background: rgba(255,255,255,0.08); }

.jobs-body {
  overflow-y: auto;
  padding: 16px 20px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.jobs-note {
  font-size: 0.84rem;
  color: var(--textSecondary, #888);
  margin: 0 0 4px;
}

.jobs-loading {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--textSecondary, #888);
  padding: 20px 0;
}

.jobs-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.job-card {
  background: rgba(255,255,255,0.04);
  border: 1px solid rgba(255,255,255,0.08);
  border-radius: 10px;
  padding: 16px;
  display: flex;
  align-items: flex-start;
  gap: 16px;
  transition: border-color 0.2s;
}

.job-card.running {
  border-color: #7c8cf8;
  background: rgba(124, 140, 248, 0.07);
}

.job-info {
  flex: 1;
}

.job-name {
  font-weight: 600;
  font-size: 0.95rem;
  color: var(--textPrimary, #e0e0e0);
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 4px;
  text-transform: capitalize;
}

.job-idle-icon { color: #4caf50; font-size: 18px; }
.job-running-icon { color: #7c8cf8; font-size: 18px; }

.badge {
  font-size: 0.72rem;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 99px;
  letter-spacing: 0.03em;
}
.running-badge { background: rgba(124,140,248,0.2); color: #7c8cf8; }

.job-desc {
  font-size: 0.84rem;
  color: var(--textSecondary, #888);
  margin: 0 0 8px;
}

.job-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 20px;
  font-size: 0.8rem;
  color: var(--textSecondary, #aaa);
}

.job-error {
  margin-top: 8px;
  font-size: 0.8rem;
  color: #ef5350;
  display: flex;
  align-items: center;
  gap: 4px;
}
.job-error .material-icons { font-size: 16px; }

.job-actions {
  flex-shrink: 0;
  padding-top: 2px;
}

.run-btn {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 8px 14px;
  border-radius: 8px;
  border: none;
  cursor: pointer;
  font-size: 0.875rem;
  font-weight: 600;
  background: #7c8cf8;
  color: #fff;
  transition: background 0.15s, opacity 0.15s;
  white-space: nowrap;
}
.run-btn:hover:not(:disabled) { background: #6270e0; }
.run-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}
.run-btn .material-icons { font-size: 18px; }

.spinning {
  animation: spin 1.2s linear infinite;
}
@keyframes spin {
  from { transform: rotate(0deg); }
  to   { transform: rotate(360deg); }
}
</style>
