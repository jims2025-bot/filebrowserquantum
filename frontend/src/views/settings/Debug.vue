<template>
  <div class="card active" id="debug-page">
    <div class="card-title">
      <h2>System Debug & Configuration</h2>
    </div>

    <div class="card-content">
      <p>
        This page dumps the active backend configuration, user scopes, and system environment info. 
        It is extremely helpful for debugging why Docker container paths (e.g. <code>/data/...</code>) may not be mounting or mapping to Filebrowser sources correctly.
      </p>

      <div v-if="loading" class="loading-state">
        <p>Loading debug information...</p>
      </div>
      
      <div v-else-if="error" class="error-state">
        <p class="wrong-login">Failed to load debug output: {{ error }}</p>
      </div>

      <div v-else class="debug-data-container">
        <div style="display: flex; gap: 1em; margin-bottom: 1em;">
          <button class="button button--flat" @click="fetchDebugData">
            Refresh Data
          </button>
          <button class="button button--flat" @click="copyData">
            Copy JSON
          </button>
        </div>
        <pre class="debug-output">{{ formattedDebugData }}</pre>
      </div>
    </div>
  </div>
</template>

<script>
import { notify } from "@/notify";
import { getApiPath } from "@/utils/url.js";
import { state } from "@/store";

export default {
  name: "Debug",
  data() {
    return {
      debugData: null,
      loading: true,
      error: null,
    };
  },
  computed: {
    formattedDebugData() {
      if (!this.debugData) return "";
      return JSON.stringify(this.debugData, null, 2);
    }
  },
  created() {
    this.fetchDebugData();
  },
  methods: {
    async fetchDebugData() {
      this.loading = true;
      this.error = null;
      try {
        let basePath = getApiPath("api/debug");
        const res = await fetch(basePath, {
          method: "GET",
          headers: {
            "X-Auth": state.jwt
          }
        });
        
        if (!res.ok) {
          throw new Error(`HTTP ${res.status} - ${res.statusText}`);
        }
        
        const data = await res.json();
        this.debugData = data;
      } catch (err) {
        console.error("Failed to fetch debug data:", err);
        this.error = err.message || "Unknown error occurred";
        notify.showError("Failed to fetch debug info.");
      } finally {
        this.loading = false;
      }
    },
    async copyData() {
      if (!this.formattedDebugData) return;
      try {
        if (navigator.clipboard && window.isSecureContext) {
          await navigator.clipboard.writeText(this.formattedDebugData);
        } else {
          // Fallback for HTTP environments like Portainer
          const textArea = document.createElement("textarea");
          textArea.value = this.formattedDebugData;
          textArea.style.position = "absolute";
          textArea.style.left = "-999999px";
          document.body.appendChild(textArea);
          textArea.focus();
          textArea.select();
          document.execCommand("copy");
          textArea.remove();
        }
        notify.showSuccess("Debug JSON copied to clipboard!");
      } catch (err) {
        console.error("Failed to copy data:", err);
        notify.showError("Failed to copy data to clipboard.");
      }
    }
  }
};
</script>

<style scoped>
#debug-page {
  max-width: 1200px;
  width: 100%;
}

.debug-data-container {
  margin-top: 1em;
}

.debug-output {
  background: var(--surfacePrimary);
  color: var(--textSecondary);
  padding: 1.5em;
  border-radius: 6px;
  overflow-x: auto;
  font-family: monospace;
  font-size: 0.9em;
  line-height: 1.4;
  white-space: pre-wrap;
  word-wrap: break-word;
  border: 1px solid var(--surfaceSecondary);
}

.loading-state, .error-state {
  margin: 2em 0;
  text-align: center;
}

.wrong-login {
  color: #f44336;
  font-weight: bold;
}
</style>
