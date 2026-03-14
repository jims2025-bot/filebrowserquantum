<template>
  <div class="card floating" id="overlay-editor">
    <div class="card-title">
      <h2>Overlay Management</h2>
    </div>

    <div class="card-content">
      <div v-if="loading" class="loading-state">
        <i class="material-icons spin">autorenew</i>
        <p>Fetching overlays...</p>
      </div>
      
      <div v-else-if="overlays.length === 0" class="empty-state">
         <i class="material-icons">layers_clear</i>
         <p>No overlays found in this folder.</p>
         <p class="small">Upload .geojson or .pmtiles files to see them here.</p>
      </div>

      <div v-else class="overlay-list">
        <!-- Overlay Selection Dropdown -->
        <div class="input-group">
          <label>Select Overlay File</label>
          <select class="overlay-select" v-model="selectedOverlayPath">
            <option v-for="overlay in overlays" :key="overlay.path" :value="overlay.path">
              {{ getFileName(overlay.path) }}
            </option>
          </select>
        </div>

        <!-- Single Selected Overlay Editor -->
        <div v-if="currentOverlay" class="overlay-item">
           <div class="overlay-header">
              <i class="material-icons">{{ currentOverlay.type === 'pmtiles' ? 'layers' : 'map' }}</i>
              <span class="file-path">{{ getFileName(currentOverlay.path) }}</span>
           </div>
           
           <div class="input-group">
              <label>Name</label>
              <input type="text" v-model="currentOverlay.name" placeholder="Friendly name for the map layer" />
           </div>
           
           <div class="input-group">
              <label>Description</label>
              <textarea v-model="currentOverlay.description" placeholder="Brief description of this overlay"></textarea>
           </div>
           
           <div class="item-actions">
              <button class="button button--flat" @click="saveOverlay(currentOverlay)" :disabled="saving === currentOverlay.path">
                <i class="material-icons" v-if="saving !== currentOverlay.path">save</i>
                <i class="material-icons spin" v-else>autorenew</i>
                {{ saving === currentOverlay.path ? 'Saving...' : 'Save Changes' }}
              </button>
           </div>
        </div>
      </div>
    </div>

    <div class="card-action">
      <button
        class="button button--flat button--grey"
        @click="closeHovers"
        aria-label="Close"
        title="Close"
      >
        {{ $t("buttons.close") }}
      </button>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, computed, watch } from 'vue';
import { fetchJSON } from "@/api/utils";
import { notify } from "@/notify";
import { state, mutations } from "@/store";

const props = defineProps({
  path: String,
  source: String
});

const overlays = ref([]);
const loading = ref(true);
const saving = ref(null);
const selectedOverlayPath = ref(null);

const currentOverlay = computed(() => {
  if (!selectedOverlayPath.value || overlays.value.length === 0) return null;
  return overlays.value.find(o => o.path === selectedOverlayPath.value) || null;
});

const closeHovers = () => {
  mutations.closeHovers();
};

const getFileName = (path) => {
  return path.split('/').pop();
};

const fetchOverlays = async () => {
  loading.value = true;
  try {
    // We use the context mode to get overlays JUST for this folder
    const url = `/api/heatmap/overlays?source=${encodeURIComponent(props.source)}&path=${encodeURIComponent(props.path)}&mode=context`;
    const data = await fetchJSON(url);
    overlays.value = data.overlays || [];
    
    // Auto-select the first overlay if available
    if (overlays.value.length > 0 && !selectedOverlayPath.value) {
      selectedOverlayPath.value = overlays.value[0].path;
    }
  } catch (e) {
    console.error("Failed to fetch overlays:", e);
    notify.showError("Failed to fetch overlays list");
  } finally {
    loading.value = false;
  }
};

const saveOverlay = async (overlay) => {
  saving.value = overlay.path;
  try {
    const res = await fetch('/api/heatmap/overlay/metadata', {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        'X-Auth': state.jwt
      },
      body: JSON.stringify({
        source: overlay.source,
        path: overlay.path,
        name: overlay.name,
        description: overlay.description
      })
    });

    if (res.ok) {
      notify.showSuccess(`Updated metadata for ${getFileName(overlay.path)}`);
    } else {
      const err = await res.text();
      throw new Error(err || res.statusText);
    }
  } catch (e) {
    console.error("Failed to save overlay metadata:", e);
    notify.showError("Failed to save metadata: " + e.message);
  } finally {
    saving.value = null;
  }
};

onMounted(() => {
  fetchOverlays();
});
</script>

<style scoped>
#overlay-editor {
  width: 500px;
  max-width: 90vw;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
}

.overlay-list {
  display: flex;
  flex-direction: column;
  gap: 1em;
  padding-bottom: 1em;
}

.overlay-select {
  width: 100%;
  background: var(--background);
  color: var(--textPrimary);
  border: 1px solid var(--divider);
  padding: 0.8em;
  border-radius: 4px;
  font-size: 1em;
  cursor: pointer;
  appearance: auto; /* Uses native dropdown arrow, looks better natively */
}
.overlay-select option {
  background: var(--background);
  color: var(--textPrimary);
}

.overlay-item {
  padding: 1em;
  background: var(--surfaceSecondary);
  border-radius: 8px;
  border: 1px solid var(--divider);
}

.overlay-header {
  display: flex;
  align-items: center;
  gap: 0.5em;
  margin-bottom: 1em;
  font-weight: bold;
  color: var(--textSecondary);
}

.file-path {
  font-family: monospace;
  font-size: 0.9em;
  word-break: break-all;
}

.input-group {
  margin-bottom: 1em;
}

.input-group label {
  display: block;
  font-size: 0.8em;
  text-transform: uppercase;
  color: var(--textSecondary);
  margin-bottom: 0.3em;
}

.input-group input, .input-group textarea {
  width: 100%;
  background: var(--background);
  color: var(--textPrimary);
  border: 1px solid var(--divider);
  padding: 0.6em;
  border-radius: 4px;
}

.input-group textarea {
  height: 60px;
  resize: vertical;
}

.item-actions {
  display: flex;
  justify-content: flex-end;
}

.loading-state, .empty-state {
  text-align: center;
  padding: 3em 1em;
  color: var(--textSecondary);
}

.empty-state .material-icons {
  font-size: 3em;
  margin-bottom: 0.3em;
}

.spin {
  animation: rotation 2s infinite linear;
}

@keyframes rotation {
  from { transform: rotate(0deg); }
  to { transform: rotate(359deg); }
}
</style>
