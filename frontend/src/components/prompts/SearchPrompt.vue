<template>
  <div class="search-sidebar-panel" id="search-prompt" :style="{ width: searchSidebarWidth }" @click.stop>

    <div class="search-tabs">
      <button :class="['tab-btn', { active: activeTab === 'search' }]" @click="activeTab = 'search'">
        <i class="material-icons">search</i>
        <span>Search</span>
      </button>
      <button :class="['tab-btn', { active: activeTab === 'results' }]" @click="activeTab = 'results'">
        <i class="material-icons">grid_view</i>
        <span>Results ({{ results.length }})</span>
      </button>
    </div>

    <div class="card-content">
      <!-- Search Tab -->
      <div v-show="activeTab === 'search'" class="tab-pane">
        <div class="search-context">
          <label>Search Path:</label>
          <span>{{ getContext }}</span>
        </div>

        <div class="search-input-wrapper">
          <i class="material-icons search-icon">search</i>
          <input
            id="search-input"
            class="input"
            type="text"
            v-model.trim="value"
            @keyup.enter="submit"
            @input="onInput"
            :placeholder="$t('search.search')"
            :aria-label="$t('search.search')"
            ref="input"
            autofocus
          />
          <button v-if="value" class="clear-button" type="button" @click.stop="clearSearch">
            <i class="material-icons">close</i>
          </button>
        </div>

        <!-- Autocomplete Dropdown -->
        <div v-show="autocompleteResults.length > 0" class="autocomplete-dropdown" @click.stop>
          <ul>
            <li
              v-for="(person, k) in autocompleteResults"
              :key="k"
              class="autocomplete-entry"
              @click.stop="selectPerson(person.name)"
            >
              <img v-if="person.avatarUrl" :src="person.avatarUrl" class="autocomplete-avatar" alt="Avatar" />
              <i v-else class="material-icons autocomplete-icon">person</i>
              <span class="text-container">{{ person.name }}</span>
            </li>
          </ul>
        </div>

        <div class="people-section">
          <div class="people-instructions" v-if="people.length > 0">
            <i class="material-icons info-icon">info</i>
            <span>Select multiple to require <b>all</b> (AND). Type <b>OR</b> between tags above to allow <b>any</b>.</span>
          </div>
          <div v-if="people.length > 0" class="people-chips">
            <button 
              v-for="person in people" 
              :key="person.name"
              type="button"
              :class="['filter-chip', { active: value.includes('person:&quot;' + person.name + '&quot;') }]"
              @click.stop="togglePerson(person.name)"
              :title="person.count + ' faces'"
            >
              <img v-if="person.avatarUrl" :src="person.avatarUrl" class="chip-avatar" />
              <div class="chip-label">
                <span class="name">{{ person.name }}</span>
                <span class="count">{{ person.count }}</span>
              </div>
            </button>
          </div>
          <div v-else-if="loadingPeople" class="people-status">
            <i class="material-icons spin">autorenew</i> Loading people...
          </div>
        </div>
      </div>
      
      <!-- Results Grid Tab -->
      <div v-show="activeTab === 'results'" class="tab-pane results-pane">
        <div class="search-results" @click.stop>
          <div v-if="ongoing" class="search-status">
            <i class="material-icons spin">autorenew</i>
            <span>Searching...</span>
          </div>
          <div v-else-if="results.length === 0 && value.length >= 3" class="search-status">
            <span>{{ noneMessage }}</span>
          </div>
          <div v-else-if="results.length === 0" class="search-status">
            <span>No results to display.</span>
          </div>
          
          <ul v-if="results.length > 0" class="results-grid" @click.stop>
              <li
              v-for="(s, k) in results"
              :key="k"
              :class="['grid-item', { active: isCurrent(s) }]"
              @click.stop="openQuickView(s)"
              @contextmenu.prevent="showContextMenu($event, s)"
              @touchstart="handleTouchStart($event, s)"
              @touchend="handleTouchEnd"
              :title="baseName(s.path) + ' (' + humanSize(s.size) + ')'"
            >
              <Icon :mimetype="s.type" :thumbnailUrl="s.thumbnailUrl" :forcePreview="true" />
            </li>
          </ul>
        </div>
      </div>
    </div>

    <!-- Independent Quick View Modal -->
    <div v-if="quickViewFile" class="quick-view-modal" :style="{ right: searchSidebarWidth }" @click="closeQuickView">
      <div class="quick-view-content" @click.stop>
          <!-- Navigation Zones -->
          <div class="nav-zone nav-zone-left" @click="prevItem" title="Previous Item"></div>
          <div class="nav-zone nav-zone-right" @click="nextItem" title="Next Item"></div>

          <!-- Header Controls -->
          <div class="quick-view-header">
              <div class="quick-view-controls">
                  <button @click="navToFile(quickViewFile)" class="qv-btn" title="Go to Image"><i class="material-icons">image</i></button>
                  <button @click="closeQuickView" class="qv-btn close"><i class="material-icons">close</i></button>
              </div>
          </div>
          
          <!-- Main Image -->
          <img :src="quickViewFile.previewUrl" class="quick-view-img" />
          
          <!-- Footer Metadata -->
          <div class="quick-view-footer">
               <div v-if="quickViewFile.metadata" class="metadata-section">
                   <div v-if="quickViewFile.metadata.instructions" class="meta-instructions">
                       {{ quickViewFile.metadata.instructions }}
                   </div>
                   <div class="meta-iptc">
                       <span v-if="quickViewFile.metadata.caption">{{ quickViewFile.metadata.caption }}</span>
                       <span v-if="quickViewFile.metadata.caption && quickViewFile.metadata.byline"> | </span>
                       <span v-if="quickViewFile.metadata.byline">Photo: {{ quickViewFile.metadata.byline }}</span>
                   </div>
               </div>
               <span class="quick-view-path">{{ quickViewFile.path }}</span>
          </div>
      </div>
    </div>

    <!-- Face Thumbnail Context Menu -->
    <div v-if="faceContextMenu.show" class="face-context-menu" :style="{ top: faceContextMenu.top + 'px', left: faceContextMenu.left + 'px', position: 'absolute', zIndex: 1000, background: 'var(--surfacePrimary)', border: '1px solid var(--borderDivider)', padding: '10px', borderRadius: '4px', boxShadow: '0 2px 10px rgba(0,0,0,0.2)' }" @click.stop>
      <div style="display: flex; justify-content: space-between; align-items: center; border-bottom: 1px solid var(--borderDivider); padding-bottom: 5px; margin-bottom: 5px;">
         <b style="color: var(--textPrimary)">Options</b>
         <button @click="closeContextMenu" class="close-icon" style="background: none; border: none; cursor: pointer; color: var(--textSecondary)"><i class="material-icons">close</i></button>
      </div>
      <div v-if="isPersonSearch && getSearchPersonName" style="margin-bottom: 10px;">
         <button @click="setAvatar(faceContextMenu.file, getSearchPersonName); closeContextMenu()" class="button button--flat" style="width: 100%; text-align: left; background: var(--surfaceSecondary); color: var(--textPrimary); border: none; padding: 8px; border-radius: 4px; cursor: pointer; display: flex; align-items: center; gap: 8px;"><i class="material-icons" style="font-size: 18px">person</i> Set as Avatar</button>
      </div>
      <div style="font-size: 0.85em; color: var(--textSecondary); display: flex; flex-direction: column; gap: 4px;">
         <div><b>File:</b> {{ baseName(faceContextMenu.file.path) }}</div>
         <div><b>Size:</b> {{ humanSize(faceContextMenu.file.size) }}</div>
      </div>
    </div>

  </div>
</template>

<script>
import { state, getters, mutations } from "@/store";
import { search } from "@/api";
import { fetchURL } from "@/api/utils";
import { getHumanReadableFilesize } from "@/utils/filesizes";
import { url } from "@/utils/";
import Icon from "@/components/files/Icon.vue";
import router from "@/router";

export default {
  name: "SearchPrompt",
  components: {
    Icon,
  },
  data() {
    return {
      value: "",
      ongoing: false,
      noneMessage: "Type 3 or more characters to search.",
      showOptions: false,
      selectedSource: "",
      autocompleteResults: [],
      autocompleteTimeout: null,
      people: [],
      loadingPeople: false,
      abortController: null,
      quickViewFile: null,
      metadataCache: new Map(),
      activeTab: "search",
      faceContextMenu: {
        show: false,
        top: 0,
        left: 0,
        file: null,
      },
    };
  },
  watch: {
    activeTab(newVal) {
      if (newVal === 'search' && this.people.length === 0) {
        this.fetchPeople();
      }
    },
    getContext() {
      if (this.activeTab === 'search') {
        this.fetchPeople();
      }
    }
  },
  computed: {
    results() {
      return state.searchResults;
    },
    sourceInfo() {
      return state.sources.info;
    },
    multipleSources() {
      return Object.keys(state.sources.info).length > 1;
    },
    getContext() {
      if (!state.route || !state.route.path) return "/";
      let result = url.extractSourceFromPath(decodeURIComponent(state.route.path));
      if (this.selectedSource === "" || result.source === this.selectedSource) {
        return result.path;
      } else {
        return "/";
      }
    },
    searchSidebarWidth() {
      return getters.searchSidebarWidth();
    },
    currentPath() {
      if (this.quickViewFile) return this.quickViewFile.path;
      if (!this.$route || !this.$route.path) return "";
      let path = decodeURIComponent(this.$route.path);
      let parts = path.split("/");
      if (parts.length >= 4 && parts[1] === 'files') {
        return parts.slice(3).join("/");
      }
      return "";
    },
    isPersonSearch() {
      return /person:"([^"]+)"|person:&quot;([^&]+)&quot;/.test(this.value);
    },
    getSearchPersonName() {
      const match = this.value.match(/person:"([^"]+)"|person:&quot;([^&]+)&quot;/);
      return match ? (match[1] || match[2]) : null;
    }
  },
  mounted() {
    if (state.serverHasMultipleSources) {
      this.selectedSource = state.sources.current;
    }
    this.$nextTick(() => {
      this.$refs.input.focus();
    });
    window.addEventListener("keydown", this.keyEvent);
    
    // Initial fetch if starting on search tab
    if (this.activeTab === 'search' && this.people.length === 0) {
      this.fetchPeople();
    }
  },
  beforeUnmount() {
    window.removeEventListener("keydown", this.keyEvent);
  },
  methods: {
    showContextMenu(event, s) {
       this.faceContextMenu.file = s;
       let top = event.clientY;
       let left = event.clientX;
       if (top < 100) top = 100;
       
       this.faceContextMenu.top = top;
       this.faceContextMenu.left = left;
       this.faceContextMenu.show = true;
    },
    closeContextMenu() {
       this.faceContextMenu.show = false;
       this.faceContextMenu.file = null;
    },
    async setAvatar(file, name) {
      try {
        const res = await fetchURL(`/api/facerec/avatar?source=${encodeURIComponent(file.source || state.sources.current)}`, {
          method: 'PUT',
          body: JSON.stringify({
            name: name,
            imagePath: file.path,
            box: file.box || ""
          })
        });
        if (res.ok) {
          // Refresh people list
          this.fetchPeople();
        } else {
          alert("Failed to update avatar.");
        }
      } catch (e) {
        console.error("Avatar update error:", e);
      }
    },
    handleTouchStart(event, s) {
      this.touchTimeout = setTimeout(() => {
        this.showContextMenu(event, s);
      }, 600); // 600ms for long press
    },
    handleTouchEnd() {
      if (this.touchTimeout) {
        clearTimeout(this.touchTimeout);
      }
    },
    close() {
      mutations.closeSearchSidebar();
    },
    clearSearch() {
      this.value = "";
      this.results = [];
      this.autocompleteResults = [];
      this.activeTab = "search";
    },
    onInput() {
      if (this.value.length < 3) {
        this.results = [];
        this.autocompleteResults = [];
        return;
      }
      this.fetchAutocomplete();
      // Optionally auto-submit or just wait for enter. Let's debounce submit too.
      this.debounceSubmit();
    },
    debounceSubmit() {
       if (this.submitTimeout) clearTimeout(this.submitTimeout);
       this.submitTimeout = setTimeout(() => {
          this.submit();
       }, 500);
    },
    async submit() {
      if (this.value.length < 3) {
        this.results = [];
        return;
      }
      
      // Cancel previous search if it's still ongoing
      if (this.abortController) {
        this.abortController.abort();
      }
      this.abortController = new AbortController();
      const signal = this.abortController.signal;

      this.ongoing = true;
      let source = this.selectedSource || state.sources.current;
      
      try {
        // Pass the signal to the search API or fetch call
        const res = await search(this.getContext, source, this.value, signal);
        const safeRes = res || [];
        mutations.setSearchResults(safeRes);
        if (safeRes.length === 0) {
          this.noneMessage = "No results found.";
        }
      } catch (e) {
        if (e.name === 'AbortError') {
           return; // Silent return for aborted requests
        }
        console.error("Search failed:", e);
        this.noneMessage = "Search failed.";
      } finally {
        if (!signal.aborted) {
            this.ongoing = false;
        }
      }
    },
    fetchAutocomplete() {
      if (this.autocompleteTimeout) clearTimeout(this.autocompleteTimeout);
      if (this.value.length < 3) return;

      this.autocompleteTimeout = setTimeout(async () => {
        try {
          const res = await fetchURL(`/api/facerec/people?q=${encodeURIComponent(this.value)}`);
          this.autocompleteResults = (await res.json()) || [];
        } catch (e) {
          console.error("Autocomplete failed:", e);
        }
      }, 300);
    },
    selectPerson(name) {
      this.value = `person:"${name}"`;
      this.autocompleteResults = [];
      this.submit();
    },
    async fetchPeople() {
      if (this.loadingPeople) return;
      this.loadingPeople = true;
      try {
        const scope = encodeURIComponent(this.getContext);
        const source = encodeURIComponent(this.selectedSource || state.sources.current);
        const res = await fetchURL(`/api/facerec/people?scope=${scope}&source=${source}`);
        const data = (await res.json()) || [];
        this.people = data.sort((a, b) => a.name.localeCompare(b.name));
      } catch (e) {
        console.error("Failed to fetch people:", e);
      } finally {
        this.loadingPeople = false;
      }
    },
    togglePerson(name) {
      const tag = `person:"${name}"`;
      // Match all existing person tags regardless of quotes
      const personRegex = /person:"([^"]+)"|person:&quot;([^&]+)&quot;/g;
      
      let currentNames = [];
      let match;
      while ((match = personRegex.exec(this.value)) !== null) {
        currentNames.push(match[1] || match[2]);
      }

      if (currentNames.includes(name)) {
        // Remove it
        const escapedName = name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
        const removeRegex = new RegExp(`(person:"${escapedName}"|person:&quot;${escapedName}&quot;)( |$)`, 'g');
        this.value = this.value.replace(removeRegex, "").trim();
      } else {
        // Add it
        this.value = (tag + " " + this.value).trim();
      }
      this.submit();
    },
    baseName(path) {
      let parts = url.removeTrailingSlash(path).split("/");
      return parts.pop();
    },
    basePath(path, isDir) {
      let result = url.removeLastDir(path);
      return result || "/";
    },
    humanSize(size) {
      return getHumanReadableFilesize(size);
    },
    isCurrent(s) {
      return s.path === this.currentPath;
    },
    getPreviewUrl(path, source, size) {
      if (!path) return "";
      const params = new URLSearchParams();
      params.append('path', path);
      if (source) params.append('source', source);
      params.append('size', size);
      return '/api/preview?' + params.toString();
    },
    async openQuickView(file) {
      this.quickViewFile = {
        ...file,
        previewUrl: this.getPreviewUrl(file.path, file.source || state.sources.current, 'large'),
        metadata: this.metadataCache.get(file.path) || null
      };
      
      mutations.setSearchSidebarWidth('30%');
      
      const index = this.results.findIndex(f => f.path === file.path);
      if (index >= 0) {
        mutations.setCurrentSearchResultIndex(index);
        this.prefetchMetadata(index);
      }
      
      if (!this.quickViewFile.metadata) {
        this.updateQuickViewMetadata();
      }
      
      // Scroll active thumbnail into view
      this.$nextTick(() => {
        const activeEl = document.querySelector('.grid-item.active');
        if (activeEl) {
          activeEl.scrollIntoView({ behavior: 'smooth', block: 'center' });
        }
      });
    },
    closeQuickView() {
      this.quickViewFile = null;
      mutations.setSearchSidebarWidth('100%');
    },
    nextItem(e) {
      if (e) e.stopPropagation();
      const idx = this.results.findIndex(f => f.path === this.quickViewFile.path);
      if (idx < this.results.length - 1) {
        this.openQuickView(this.results[idx + 1]);
      }
    },
    prevItem(e) {
      if (e) e.stopPropagation();
      const idx = this.results.findIndex(f => f.path === this.quickViewFile.path);
      if (idx > 0) {
        this.openQuickView(this.results[idx - 1]);
      }
    },
    async fetchMetadata(file) {
      try {
        const res = await fetchURL(`/api/metadata?path=${encodeURIComponent(file.path)}&source=${encodeURIComponent(file.source || state.sources.current)}`);
        if (res.ok) {
          const meta = await res.json();
          this.metadataCache.set(file.path, meta);
          return meta;
        }
      } catch (e) {
        console.warn("Metadata fetch failed", e);
      }
      return null;
    },
    async updateQuickViewMetadata() {
      if (!this.quickViewFile) return;
      const meta = await this.fetchMetadata(this.quickViewFile);
      if (this.quickViewFile && meta) {
        this.quickViewFile.metadata = meta;
      }
    },
    prefetchMetadata(currentIndex) {
      const list = this.results;
      // Prefetch Next 5
      for (let i = 1; i <= 5; i++) {
        const idx = currentIndex + i;
        if (idx < list.length) {
          const item = list[idx];
          if (!this.metadataCache.has(item.path)) {
            this.fetchMetadata(item);
          }
        }
      }
      // Prefetch Prev 5
      for (let i = 1; i <= 5; i++) {
        const idx = currentIndex - i;
        if (idx >= 0) {
          const item = list[idx];
          if (!this.metadataCache.has(item.path)) {
            this.fetchMetadata(item);
          }
        }
      }
    },
    keyEvent(e) {
      if (!this.quickViewFile) return;
      if (e.key === "Escape") {
        this.closeQuickView();
      } else if (e.key === "ArrowRight") {
        this.nextItem();
      } else if (e.key === "ArrowLeft") {
        this.prevItem();
      }
    },
    navToFile(file) {
      let path = file.path;
      if (path.startsWith("/")) path = path.slice(1);
      const encodedPath = encodeURIComponent(path).replace(/%2F/g, "/");
      let fullpath = "/files/" + (file.source || state.sources.current) + "/" + encodedPath;
      router.push({ path: fullpath });
    }
  }
};
</script>

<style scoped>
.search-sidebar-panel {
  position: fixed;
  top: 4em;
  right: 0;
  bottom: 0;
  height: calc(100vh - 4em);
  background: var(--surfacePrimary, #fff);
  z-index: 1000;
  box-shadow: -4px 0 16px rgba(0,0,0,0.2);
  display: flex;
  flex-direction: column;
  transition: width 0.3s ease;
  overflow: hidden;
}

.search-sidebar-panel.dark-mode {
  background: var(--surfacePrimary, #1e1e2e);
}

.card-title {
  display: flex;
  align-items: center;
  padding: 16px;
  border-bottom: 1px solid var(--divider);
}

.card-title h2 {
  flex: 1;
  margin: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.card-title .action {
  background: none;
  border: none;
  cursor: pointer;
  color: var(--textSecondary);
  padding: 4px;
  margin-left: 8px;
  display: flex;
  align-items: center;
  border-radius: 50%;
  transition: background 0.2s;
}

.card-title .action:hover {
  background: rgba(128, 128, 128, 0.1);
  color: var(--textPrimary);
}

.search-input-wrapper {
  position: relative;
  display: flex;
  align-items: center;
  background: var(--surfaceSecondary);
  border-radius: 8px;
  padding: 0 12px;
  margin-bottom: 16px;
  border: 1px solid var(--divider);
}

.search-icon {
  color: var(--textSecondary);
  margin-right: 8px;
}

.search-input-wrapper input {
  flex: 1;
  border: none;
  background: transparent;
  padding: 12px 0;
  font-size: 1.1em;
  color: var(--textPrimary);
  outline: none;
}

.clear-button {
  background: none;
  border: none;
  cursor: pointer;
  color: var(--textSecondary);
  display: flex;
  align-items: center;
}

/* Autocomplete */
.autocomplete-dropdown {
  position: absolute;
  top: 100px; /* Adjust based on input position */
  left: 24px;
  right: 24px;
  background: var(--surfacePrimary);
  border: 1px solid var(--divider);
  border-radius: 8px;
  box-shadow: 0 4px 12px rgba(0,0,0,0.2);
  z-index: 100;
  max-height: 200px;
  overflow-y: auto;
}

.autocomplete-entry {
  display: flex;
  align-items: center;
  padding: 8px 16px;
  cursor: pointer;
}

.autocomplete-entry:hover {
  background: var(--surfaceSecondary);
}

.autocomplete-avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  margin-right: 8px;
  object-fit: cover;
}

.autocomplete-icon {
  font-size: 24px;
  margin-right: 8px;
}

/* Options */
.search-options {
  margin-bottom: 16px;
  border: 1px solid var(--divider);
  border-radius: 8px;
  overflow: hidden;
}

.options-header {
  padding: 8px 16px;
  background: var(--surfaceSecondary);
  cursor: pointer;
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.9em;
  font-weight: 600;
}

.options-content {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.people-filters {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.people-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0; /* Important for scrollable flex child */
  margin-top: 16px;
}

.people-instructions {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  background: var(--surfaceSecondary);
  border-radius: 8px;
  margin-bottom: 12px;
  font-size: 0.85em;
  color: var(--textSecondary);
}

.info-icon {
  font-size: 16px;
  color: var(--primaryColor, #2196F3);
}

.people-chips {
  flex: 1;
  display: flex;
  flex-wrap: wrap;
  align-content: flex-start;
  gap: 8px;
  overflow-y: auto;
  padding: 4px;
  padding-bottom: 24px;
}

.filter-chip {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 3px 14px 3px 3px;
  border-radius: 100px;
  border: 1px solid var(--divider);
  background: var(--surfaceSecondary);
  cursor: pointer;
  font-size: 0.95em;
  font-weight: 500;
  color: var(--textSecondary);
  transition: all 0.2s;
  height: 56px;
  max-width: 100%;
}

.filter-chip.active {
  background: var(--primaryColor);
  color: white;
  border-color: var(--primaryColor);
}

.filter-chip:hover:not(.active) {
  background: var(--background);
  border-color: var(--textSecondary);
}

.chip-avatar {
  width: 50px;
  height: 50px;
  border-radius: 50%;
  object-fit: cover;
  background: var(--background);
  flex-shrink: 0;
}

.chip-label {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  overflow: hidden;
}

.chip-label .name {
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 100%;
}

.chip-label .count {
  font-size: 0.85em;
  opacity: 0.8;
}

.header-slim {
  justify-content: flex-end;
  padding: 8px 16px;
  min-height: auto;
}

.people-status {
  padding: 8px;
  font-size: 0.9em;
  color: var(--textSecondary);
  display: flex;
  align-items: center;
  gap: 8px;
}


/* Results Grid */
.search-results {
  flex: 1;
  overflow-y: auto;
  min-height: 200px;
  padding: 10px;
}

.card-content {
  display: flex;
  flex-direction: column;
  flex: 1;
  overflow: hidden;
}

.results-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(100px, 1fr));
  gap: 12px;
  list-style: none;
  padding: 0;
  margin: 0;
}

.grid-item {
  aspect-ratio: 1 / 1;
  border-radius: 8px;
  overflow: hidden;
  cursor: pointer;
  transition: transform 0.2s, box-shadow 0.2s;
  background: var(--surfaceSecondary);
  border: 1px solid var(--divider);
  display: flex;
  align-items: center;
  justify-content: center;
}

.grid-item.active {
  border-color: var(--primaryColor);
  border-width: 3px;
  box-shadow: 0 0 12px var(--primaryColor);
}

.grid-item:hover {
  transform: scale(1.05);
  box-shadow: 0 4px 12px rgba(0,0,0,0.3);
  z-index: 10;
}

.grid-item :deep(.icon) {
  width: 100% !important;
  height: 100% !important;
  margin: 0 !important;
}

.grid-item :deep(img) {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

/* Quick View Modal Styles */
.quick-view-modal {
  position: fixed;
  top: 0;
  left: 0;
  bottom: 0;
  background: rgba(0,0,0,0.85);
  z-index: 2000;
  display: flex;
  justify-content: center;
  align-items: center;
}

.quick-view-content {
  position: relative;
  background: rgba(30,30,45, 0.95);
  padding: 8px;
  border-radius: 8px;
  box-shadow: 0 12px 36px rgba(0,0,0,0.6);
  max-width: 90vw;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.quick-view-header {
  width: 100%;
  display: flex;
  justify-content: flex-end;
  padding: 4px;
  z-index: 10;
}

.quick-view-controls {
  display: flex;
  gap: 12px;
}

.qv-btn {
  background: none;
  border: none;
  color: white;
  cursor: pointer;
  padding: 4px;
  border-radius: 50%;
  transition: background 0.2s;
}

.qv-btn:hover {
  background: rgba(255,255,255,0.1);
}

.quick-view-img {
  max-width: 100%;
  max-height: calc(90vh - 120px);
  object-fit: contain;
  background: #000;
  border-radius: 4px;
}

.nav-zone {
  position: absolute;
  top: 0;
  height: 100%;
  width: 25%;
  z-index: 5;
  cursor: pointer;
}

.nav-zone-left { left: 0; }
.nav-zone-right { right: 0; }

.nav-zone:hover {
  background: rgba(255,255,255,0.03);
}

.quick-view-footer {
  width: 100%;
  padding: 12px 8px 4px;
  color: white;
}

.metadata-section {
  margin-bottom: 8px;
}

.meta-instructions {
  color: #ffca28;
  font-weight: bold;
  font-size: 1.1em;
  margin-bottom: 4px;
}

.meta-iptc {
  color: #ccc;
  font-size: 0.9em;
}

.quick-view-path {
  font-size: 0.8em;
  opacity: 0.6;
  word-break: break-all;
  display: block;
}

.search-status {
  padding: 32px;
  text-align: center;
  color: var(--textSecondary);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

/* Tab Styles */
.search-tabs {
  display: flex;
  width: 100%;
  background: var(--background);
  border-bottom: 2px solid var(--surface-hover);
  position: sticky;
  top: 0;
  z-index: 10;
}

.tab-btn {
  flex: 1;
  padding: 12px 0;
  background: transparent;
  border: none;
  border-bottom: 3px solid transparent;
  color: var(--textSecondary);
  font-size: 1rem;
  font-weight: 500;
  cursor: pointer;
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 8px;
  transition: all 0.2s ease;
}

.tab-btn:hover {
  background: var(--surface-hover);
  color: var(--textPrimary);
}

.tab-btn.active {
  color: var(--primaryColor, #2196F3);
  border-bottom-color: var(--primaryColor, #2196F3);
}

.tab-pane {
  flex: 1;
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow-y: auto;
}

.results-pane {
  padding: 0; /* Let the grid handle the padding */
}
</style>

