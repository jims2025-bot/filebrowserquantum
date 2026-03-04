<template>
  <div class="card floating folder-details-modal" id="folder-details">
    <div class="card-title">
      <h2><i class="material-icons" style="vertical-align:middle;margin-right:6px">folder_open</i>Folder Info</h2>
    </div>

    <div class="card-content">
      <!-- Loading state -->
      <div v-if="loading" class="fd-loading">
        <i class="material-icons spinning">sync</i> Loading...
      </div>

      <template v-else>
        <!-- Read-only metadata -->
        <div class="fd-meta-row" v-if="details.createDate">
          <span class="fd-label">Created</span>
          <span class="fd-value">{{ formatTimestamp(details.createDate) }}</span>
        </div>
        <div class="fd-meta-row" v-if="details.folderAddedDate">
          <span class="fd-label">Folder Added</span>
          <span class="fd-value">{{ formatTimestamp(details.folderAddedDate) }}</span>
        </div>
        <div class="fd-meta-row" v-if="details.fileCount">
          <span class="fd-label">File Count</span>
          <span class="fd-value">{{ details.fileCount }}</span>
        </div>

        <!-- Divider -->
        <hr class="fd-divider" />

        <!-- Folder Notes -->
        <div class="fd-section">
          <label class="fd-section-label">Folder Notes</label>
          <textarea
            v-if="canEdit"
            v-model="editNotes"
            class="fd-textarea"
            placeholder="Add notes about this folder..."
            rows="4"
          ></textarea>
          <pre v-else class="fd-pre">{{ details.folderNotes || '(none)' }}</pre>
        </div>

        <!-- Date range -->
        <div class="fd-date-row">
          <div class="fd-section fd-date-field">
            <label class="fd-section-label">Oldest Date</label>
            <input
              v-if="canEdit"
              type="date"
              v-model="editOldestDate"
              class="fd-date-input"
            />
            <span v-else class="fd-value">{{ details.oldestDate || '(none)' }}</span>
          </div>
          <div class="fd-section fd-date-field">
            <label class="fd-section-label">Most Recent Date</label>
            <input
              v-if="canEdit"
              type="date"
              v-model="editMostRecentDate"
              class="fd-date-input"
            />
            <span v-else class="fd-value">{{ details.mostRecentDate || '(none)' }}</span>
          </div>
        </div>

        <!-- People -->
        <div class="fd-section">
          <label class="fd-section-label">People</label>

          <!-- Restriction Toggle -->
          <div class="fd-toggle-row" v-if="canEdit" style="margin-bottom: 12px; display: flex; align-items: center; gap: 8px;">
            <label class="switch">
              <input type="checkbox" v-model="editRestrictFaces" />
              <span class="slider round"></span>
            </label>
            <span style="font-size: 0.9em; color: var(--textSecondary, #666);">Restrict Machine Learning to only find these faces</span>
          </div>
          <div v-else-if="details.restrictFaces" style="margin-bottom: 12px; font-size: 0.9em; color: var(--textSecondary, #666);">
            <i class="material-icons" style="font-size: 14px; vertical-align: middle;">lock</i> <em>Restricted to the following people</em>
          </div>

          <!-- People chips -->
          <div class="fd-chips">
            <span
              v-for="(person, idx) in editPeople"
              :key="idx"
              class="fd-chip"
            >
              {{ person }}
              <button
                v-if="canEdit"
                class="fd-chip-remove"
                @click="removePerson(idx)"
                title="Remove"
              >&times;</button>
            </span>
            <span v-if="editPeople.length === 0 && !canEdit" class="fd-empty">(none)</span>
          </div>

          <!-- Autocomplete input (edit mode only) -->
          <div v-if="canEdit" class="fd-autocomplete-wrap">
            <input
              ref="personInput"
              v-model="personQuery"
              class="fd-person-input"
              placeholder="Add person name..."
              @input="onPersonInput"
              @keydown.enter.prevent="addPersonFromInput"
              @keydown.comma.prevent="addPersonFromInput"
              @keydown.esc="closeSuggestions"
              autocomplete="off"
            />
            <ul v-if="filteredSuggestions.length > 0" class="fd-suggestions">
              <li
                v-for="(s, i) in filteredSuggestions"
                :key="i"
                @mousedown.prevent="selectSuggestion(s)"
                class="fd-suggestion-item"
              >{{ s }}</li>
            </ul>
          </div>
        </div>

        <!-- File Notes (read-only, populated by jobs) -->
        <div class="fd-section" v-if="details.fileNotes && details.fileNotes.length > 0">
          <label class="fd-section-label">Files with Notes</label>
          <ul class="fd-file-notes">
            <li v-for="(fn, i) in details.fileNotes" :key="i">{{ fn }}</li>
          </ul>
        </div>

      </template>
    </div>

    <div class="card-action">
      <button
        class="button button--flat button--grey"
        @click="close"
      >Close</button>
      <button
        v-if="canEdit"
        class="button button--flat button--blue"
        :disabled="saving"
        @click="save"
      >{{ saving ? 'Saving...' : 'Save' }}</button>
    </div>
  </div>
</template>

<script>
import { state, mutations } from "@/store";
import { getFolderDetails, saveFolderDetails, getPeopleList, savePeopleList } from "@/api/files";
import { notify } from "@/notify";

export default {
  name: "FolderDetails",
  data() {
    return {
      loading: true,
      saving: false,
      details: {
        createDate: "",
        folderAddedDate: "",
        folderNotes: "",
        people: [],
        oldestDate: "",
        mostRecentDate: "",
        fileCount: 0,
        fileNotes: [],
      },
      // Editable copies
      editNotes: "",
      editRestrictFaces: false,
      editPeople: [],
      editOldestDate: "",      // YYYY-MM-DD
      editMostRecentDate: "",  // YYYY-MM-DD
      // Autocomplete
      personQuery: "",
      allPeople: [],       // global PeopleList.json names
      showSuggestions: false,
    };
  },
  computed: {
    canEdit() {
      return state.user.permissions.modify;
    },
    req() {
      return state.req;
    },
    filteredSuggestions() {
      if (!this.personQuery.trim()) return [];
      const q = this.personQuery.toLowerCase();
      return this.allPeople
        .filter(p => p.toLowerCase().includes(q) && !this.editPeople.includes(p))
        .slice(0, 8);
    },
  },
  async mounted() {
    await this.load();
  },
  methods: {
    async load() {
      this.loading = true;
      try {
        const source = this.req.source;
        const path = this.req.path;

        const [det, pl] = await Promise.all([
          getFolderDetails(source, path),
          getPeopleList(source),
        ]);

        if (det) {
          this.details = det;
          this.editNotes = det.folderNotes || "";
          this.editRestrictFaces = det.restrictFaces || false;
          this.editPeople = det.people ? [...det.people] : [];
          // date inputs expect YYYY-MM-DD; trim any time portion just in case
          this.editOldestDate = (det.oldestDate || "").substring(0, 10);
          this.editMostRecentDate = (det.mostRecentDate || "").substring(0, 10);
        }
        if (pl && pl.people) {
          this.allPeople = pl.people;
        }
      } catch (e) {
        notify.showError("Failed to load folder details");
      } finally {
        this.loading = false;
      }
    },

    async save() {
      this.saving = true;
      try {
        const source = this.req.source;
        const path = this.req.path;

        // Finish typing any partially entered person
        if (this.personQuery.trim()) {
          this.addPersonFromInput();
        }

        const payload = {
          ...this.details,
          folderNotes: this.editNotes,
          restrictFaces: this.editRestrictFaces,
          people: this.editPeople,
          oldestDate: this.editOldestDate,         // YYYY-MM-DD
          mostRecentDate: this.editMostRecentDate, // YYYY-MM-DD
        };

        await saveFolderDetails(source, path, payload);

        // Merge any new names into the global PeopleList
        const newNames = this.editPeople.filter(p => !this.allPeople.includes(p));
        if (newNames.length > 0) {
          await savePeopleList(source, [...this.allPeople, ...newNames]);
          this.allPeople = [...new Set([...this.allPeople, ...newNames])].sort();
        }

        this.details = { ...payload };
        notify.showSuccess("Folder details saved");
        this.close();
      } catch (e) {
        notify.showError("Failed to save folder details");
      } finally {
        this.saving = false;
      }
    },

    close() {
      mutations.closeHovers();
    },

    // ── People helpers ──────────────────────────────────────────────────────

    addPersonFromInput() {
      const name = this.personQuery.trim().replace(/,+$/, "").trim();
      if (!name) return;
      if (!this.editPeople.includes(name)) {
        this.editPeople.push(name);
      }
      this.personQuery = "";
    },

    selectSuggestion(name) {
      if (!this.editPeople.includes(name)) {
        this.editPeople.push(name);
      }
      this.personQuery = "";
    },

    removePerson(idx) {
      this.editPeople.splice(idx, 1);
    },

    onPersonInput() {
      this.showSuggestions = true;
    },

    closeSuggestions() {
      this.personQuery = "";
      this.showSuggestions = false;
    },

    // ── Utilities ───────────────────────────────────────────────────────────

    // For system timestamps (RFC3339) — show full date + time
    formatTimestamp(dateStr) {
      if (!dateStr) return "";
      try {
        return new Date(dateStr).toLocaleString();
      } catch {
        return dateStr;
      }
    },

    // For user-entered dates (YYYY-MM-DD) — display as locale date only, no time
    formatDate(dateStr) {
      if (!dateStr) return "";
      try {
        // Parse as UTC noon to avoid timezone shifts flipping the day
        const d = new Date(dateStr + (dateStr.length === 10 ? 'T12:00:00Z' : ''));
        return isNaN(d.getTime()) ? dateStr : d.toLocaleDateString();
      } catch {
        return dateStr;
      }
    },
  },
};
</script>

<style scoped>
.folder-details-modal {
  min-width: 420px;
  max-width: 600px;
  width: 95vw;
}

/* Loading */
.fd-loading {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--textSecondary, #888);
}
.spinning {
  animation: spin 1.2s linear infinite;
}
@keyframes spin {
  from { transform: rotate(0deg); }
  to   { transform: rotate(360deg); }
}

/* Meta row (read-only pairs) */
.fd-meta-row {
  display: flex;
  gap: 8px;
  margin-bottom: 4px;
  font-size: 0.92em;
}
.fd-label {
  font-weight: 600;
  flex: 0 0 140px;
  color: var(--textSecondary, #666);
}
.fd-value {
  flex: 1;
  word-break: break-word;
}

.fd-divider {
  border: none;
  border-top: 1px solid var(--divider, #ddd);
  margin: 12px 0;
}

/* Sections */
.fd-section {
  margin-bottom: 14px;
}
.fd-section-label {
  display: block;
  font-weight: 600;
  font-size: 0.9em;
  color: var(--textSecondary, #666);
  margin-bottom: 6px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

/* Notes */
.fd-textarea {
  width: 100%;
  box-sizing: border-box;
  border: 1px solid var(--divider, #ccc);
  border-radius: 4px;
  padding: 8px;
  font-family: inherit;
  font-size: 0.95em;
  resize: vertical;
  background: var(--surfaceSecondary, #fafafa);
  color: inherit;
}
.fd-pre {
  white-space: pre-wrap;
  word-break: break-word;
  font-family: inherit;
  font-size: 0.95em;
  margin: 0;
  padding: 6px 0;
}

/* People chips */
.fd-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 8px;
}
.fd-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: var(--blue, #2196f3);
  color: #fff;
  border-radius: 14px;
  padding: 3px 10px 3px 12px;
  font-size: 0.88em;
  font-weight: 500;
}
.fd-chip-remove {
  background: none;
  border: none;
  color: rgba(255,255,255,0.8);
  cursor: pointer;
  font-size: 1.1em;
  line-height: 1;
  padding: 0 0 0 2px;
}
.fd-chip-remove:hover {
  color: #fff;
}
.fd-empty {
  color: var(--textSecondary, #999);
  font-size: 0.9em;
}

/* Autocomplete */
.fd-autocomplete-wrap {
  position: relative;
}
.fd-person-input {
  width: 100%;
  box-sizing: border-box;
  border: 1px solid var(--divider, #ccc);
  border-radius: 4px;
  padding: 6px 10px;
  font-size: 0.95em;
  font-family: inherit;
  background: var(--surfaceSecondary, #fafafa);
  color: var(--textPrimary, #333);
}
.fd-person-input:focus {
  outline: none;
  border-color: var(--blue, #2196f3);
}
.fd-suggestions {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  z-index: 200;
  background: var(--surfacePrimary, #fff);
  border: 1px solid var(--divider, #ccc);
  border-top: none;
  border-radius: 0 0 4px 4px;
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 200px;
  overflow-y: auto;
  box-shadow: 0 4px 12px rgba(0,0,0,.15);
}
.fd-suggestion-item {
  padding: 7px 12px;
  cursor: pointer;
  font-size: 0.92em;
  color: var(--textPrimary, #333);
}
.fd-suggestion-item:hover {
  background: var(--blue, #2196f3);
  color: #fff;
}

/* File notes list */
.fd-file-notes {
  list-style: disc;
  padding-left: 20px;
  font-size: 0.9em;
  color: var(--textSecondary, #555);
}

/* Date row — two pickers side by side */
.fd-date-row {
  display: flex;
  gap: 16px;
  margin-bottom: 14px;
}
.fd-date-field {
  flex: 1;
  margin-bottom: 0 !important;
}
.fd-date-input {
  width: 100%;
  box-sizing: border-box;
  border: 1px solid var(--divider, #ccc);
  border-radius: 4px;
  padding: 6px 10px;
  font-size: 0.95em;
  font-family: inherit;
  background: var(--surfaceSecondary, #fafafa);
  color: inherit;
}
.fd-date-input:focus {
  outline: none;
  border-color: var(--blue, #2196f3);
}

/* Switch styling */
.switch {
  position: relative;
  display: inline-block;
  width: 36px;
  height: 20px;
}
.switch input {
  opacity: 0;
  width: 0;
  height: 0;
}
.slider {
  position: absolute;
  cursor: pointer;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: #ccc;
  transition: .4s;
}
.slider:before {
  position: absolute;
  content: "";
  height: 14px;
  width: 14px;
  left: 3px;
  bottom: 3px;
  background-color: white;
  transition: .4s;
}
input:checked + .slider {
  background-color: #2196F3;
}
input:checked + .slider:before {
  transform: translateX(16px);
}
.slider.round {
  border-radius: 20px;
}
.slider.round:before {
  border-radius: 50%;
}
</style>
