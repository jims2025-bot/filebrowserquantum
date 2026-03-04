<template>
  <header v-if="!isOnlyOffice" :class="['flexbar', { 'dark-mode-header': isDarkMode, 'header-hidden': !showHeader }]">
    <action
      v-if="!isShare"
      icon="close_back"
      :label="$t('buttons.close')"
      @action="multiAction"
    />
    <action
      v-if="isListingView"
      :icon="fullscreenIcon"
      :label="fullscreenLabel"
      :class="{ 'flash-on-load': !isFullscreen }"
      @action="toggleFullscreen"
    />
    <div class="header-middle">
        <title v-if="isSettings" class="topTitle">{{ $t("sidebar.settings") }}</title>
        <!-- Sticky breadcrumb: shown when page breadcrumb scrolls off screen -->
        <nav v-else-if="isListingView && !breadcrumbVisible" class="sticky-breadcrumb" aria-label="breadcrumb">
          <router-link to="/files/" class="sticky-crumb" :title="$t('files.home')">
            <i class="material-icons">home</i>
          </router-link>
          <template v-for="(crumb, i) in stickyBreadcrumbs" :key="i">
            <span class="sticky-sep">›</span>
            <router-link :to="crumb.url" class="sticky-crumb" :title="crumb.name">{{ crumb.name }}</router-link>
          </template>
        </nav>
        <title v-else class="topTitle">{{ req.name }}</title>

        <!-- Map Overlays Dropdown REMOVED -->
    </div>
    
    <action
      v-if="isListingView"
      icon="map"
      :label="$t('sidebar.heatmap')"
      @action="openHeatmap"
    />
    <action
      v-if="isListingView"
      icon="folder_open"
      label="Folder Info"
      @action="openFolderDetails"
    />
    <action
      v-if="isListingView && canRunFaceScan"
      icon="face"
      :label="isScanningFaces ? 'Scanning Faces...' : 'Scan Folder for Faces'"
      :disabled="isScanningFaces"
      :class="{ 'spin-action': isScanningFaces }"
      @action="scanFolderFaces"
    />
    <action
      v-if="isListingView && canRegenerate"
      icon="sync"
      :label="regenerationLabel"
      :disabled="isRegenerating"
      @action="handleRegenerateHeatmap"
    />
    <action
      v-if="showIntegrityCheck"
      icon="security"
      label="Integrity Check"
      @action="handleIntegrityCheck"
    />
    <action
      v-if="isAdmin"
      icon="schedule"
      label="Jobs"
      @action="openAdminJobs"
    />

     <action
      v-if="showThumbnailFix"
      icon="build"
      :label="$t('files.fixThumbnails')"
      @action="confirmThumbnailFix"
    />
    <action
      v-if="isListingView"
      :icon="viewIcon"
      :label="$t('buttons.switchView')"
      @action="switchView"
    />
    <action
      v-if="isPreviewView"
      icon="file_download"
      :label="$t('buttons.download')"
      @action="download"
    />
    <action
      v-if="isPreviewView && isImage"
      icon="rotate_right"
      label="Rotate"
      @action="rotate"
    />
    <action
      v-if="isPreviewView && canShare"
      icon="share"
      :label="$t('buttons.share')"
      @action="share"
    />
    <action
      v-if="isMetadataToggleVisible"
      icon="info"
      label="Metadata"
      @action="toggleMetadata"
    />
    <action
      v-if="showSearch"
      icon="search"
      :label="$t('search.search')"
      @action="openSearch"
    />

    <action
      v-if="showFileIssueButton"
      icon="warning"
      label="View Issue"
      @action="showFileIssue"
    />

	<!--
	<action
      v-if="!isShare && isPreviewView"
      :icon="iconName"	  
      :disabled="noItems"
      @click="toggleOverflow"
    />
	-->

  </header>
  
  <!-- Visible progress indicator for heatmap regeneration -->
  <div v-if="isRegenerating && isListingView" class="heatmap-progress-banner">
    <i class="material-icons">sync</i>
    <span>{{ regenerationLabel }}</span>
  </div>
</template>

<script>
import router from "@/router";
import { getters, state, mutations } from "@/store";
import Action from "@/components/Action.vue";
import * as filesApi from "@/api/files";
import { notify } from "@/notify";
import { fixThumbnails } from "@/api/files";

export default {
  name: "UnifiedHeader",
  components: {
    Action,
  },
  data() {
    return {
      viewModes: ["list", "compact", "normal", "gallery"],
      showHeader: true,
      headerTimeout: null,
      isRegenerating: false,
      isScanningFaces: false,
      scanInterval: null,
      regenerationLabel: "Regenerate Heatmap",
      regenInterval: null,
      currentFileIssue: null,
      isFullscreen: false,
      selectedMapOverlay: "",
      isFlashing: false,
      breadcrumbVisible: true,
    };
  },
  computed: {
    fullscreenIcon() {
      return this.isFullscreen ? "fullscreen_exit" : "fullscreen";
    },
    fullscreenLabel() {
      return this.isFullscreen ? "Exit Fullscreen" : "Fullscreen";
    },
    canRegenerate() {
      return state.user.permissions.updateMap;
    },
    canRunFaceScan() {
      return state.user && state.user.permissions && state.user.permissions.runFaceScan;
    },
    isOnlyOffice() {
      return getters.currentView() === "onlyOfficeEditor";
    },
    isMobile() {
      return state.isMobile;
    },
    isListingView() {
      return getters.currentView() == "listingView";
    },
    isPreviewView() {
      return getters.currentView() == "preview";
    },
    iconName() {
      return getters.currentPromptName() === "OverflowMenu"
        ? "keyboard_arrow_up"
        : "more_vert";
    },
    viewIcon() {
      const icons = {
        list: "view_list",
        compact: "table_rows_narrow",
        normal: "view_module",
        gallery: "grid_view",
      };
      return icons[state.user.viewMode] || "grid_view";
    },
    isShare() {
      return getters.currentView() == "share";
    },
    noItems() {
      return !this.showEdit && !this.showSave && !this.showDelete;
    },
    showEdit() {
      return window.location.hash != "#edit" && state.user.permissions.modify;
    },
    showDelete() {
      // SECURITY: Disabled by User Request
      return false;
      // if (
      //   this.req.type == "directory" ||
      //   this.view == "editor" ||
      //   !this.user.permissions.modify
      // ) {
      //   return false;
      // }
      // return this.selectedCount > 0;
    },
    showSave() {
      return getters.currentView() == "editor" && state.user.permissions.modify;
    },
    showSearch() {
      return getters.isLoggedIn() && getters.currentView() === "listingView";
    },
    isSearchActive() {
      return state.isSearchActive;
    },
    showSwitchView() {
      return getters.currentView() === "listingView";
    },
    showSidebarToggle() {
      return getters.currentView() === "listingView";
    },
    req() {
      return state.req;
    },
    isDarkMode() {
      return getters.isDarkMode();
    },
    isSettings() {
      return getters.isSettings();
    },
    canShare() {
       return typeof navigator.share === 'function';
    },
    isImage() {
      return getters.previewType() === 'image';
    },
    isMetadataToggleVisible() {
      return getters.currentView() === 'preview';
    },
    showIntegrityCheck() {
      // Show for admins in listing view, but not for root if we want to be safe, or just folder level
      // FIX: Use permissions, not perm.
      return getters.currentView() === 'listingView' && state.user.permissions.admin;
    },
    showThumbnailFix() {
       return getters.currentView() === 'listingView' && state.user.permissions.admin;
    },
    isAdmin() {
      return !!state.user.permissions.admin;
    },
    showFileIssueButton() {
      // Show for admins only in preview mode if current file has integrity issues
      return this.isPreviewView && this.currentFileIssue !== null && state.user.permissions.admin;
    },
    availableMaps() {
        if (!this.req || !this.req.items) return [];
        const maps = this.req.items
            .filter(f => !f.isDir && f.name.toLowerCase().endsWith('.geojson'))
            .map(f => ({
                name: f.name,
                path: f.path || (this.req.path === '/' ? '/' + f.name : this.req.path + '/' + f.name)
            }));
        console.log('[Default] Available maps:', maps);
        return maps;
    },
    stickyBreadcrumbs() {
      const req = state.req;
      if (!req || !req.path) return [];
      let path = req.path.replace(/#/g, "%23");
      let parts = path.split("/").filter((p) => p !== "");
      let base = "/files/";
      if (state.serverHasMultipleSources && req.source) {
        base = `/files/${req.source}/`;
      }
      let crumbs = [];
      let buildRef = base;
      parts.forEach((part) => {
        buildRef = buildRef + encodeURIComponent(part) + "/";
        crumbs.push({ name: part, url: buildRef });
      });
      // Keep last 3 segments max to fit in header
      if (crumbs.length > 3) {
        crumbs = crumbs.slice(-3);
        crumbs[0].name = "...";
      }
      return crumbs;
    },
  },

  watch: {
    selectedMapOverlay(newVal) {
        console.log('[Default] selectedMapOverlay changed to:', newVal);
        if (newVal) {
            this.isFlashing = true;
            // Remove class after animation completes (e.g. 2s)
            setTimeout(() => {
                this.isFlashing = false;
            }, 2000);
        }
    },
    req: {
      handler() {
        this.checkRegenerationStatus();
        this.checkFileIntegrity();
      },
      deep: true
    }
  },
  mounted() {
    this.checkRegenerationStatus();
    this.checkScanStatus();
    this.checkFileIntegrity();
    this.showHeader = true;
    
    // Check initial state
    this.updateFullscreenState();
    // Listen for changes
    document.addEventListener("fullscreenchange", this.updateFullscreenState);
    // Listen for breadcrumb scroll-off-screen events from Breadcrumbs.vue
    this._onBreadcrumbVisibility = (e) => {
      this.breadcrumbVisible = e.detail.visible;
    };
    window.addEventListener("breadcrumb-visibility", this._onBreadcrumbVisibility);
  },
  beforeDestroy() {
    if (this.regenInterval) clearInterval(this.regenInterval);
    if (this.scanInterval) clearInterval(this.scanInterval);
    document.removeEventListener("fullscreenchange", this.updateFullscreenState);
    window.removeEventListener("breadcrumb-visibility", this._onBreadcrumbVisibility);
  },
  methods: {
    async checkScanStatus() {
        try {
            const { getFaceScanStatus } = await import('@/api/files');
            const status = await getFaceScanStatus();
            this.isScanningFaces = status.isScanning;
            
            if (this.isScanningFaces && !this.scanInterval) {
                this.scanInterval = setInterval(async () => {
                    const s = await getFaceScanStatus();
                    this.isScanningFaces = s.isScanning;
                    if (!this.isScanningFaces) {
                        clearInterval(this.scanInterval);
                        this.scanInterval = null;
                        notify.showSuccess(`Finished scanning faces`);
                    }
                }, 1000);
            }
        } catch (e) {
            this.isScanningFaces = false;
            if (this.scanInterval) {
                clearInterval(this.scanInterval);
                this.scanInterval = null;
            }
        }
    },
    viewMapOverlay(source) {
        if (!this.selectedMapOverlay) return;

        const overlaySource = this.req.source || "";
        const targetPath = '/heatmap';
        const query = { 
            source: overlaySource, 
            path: this.req.path, 
            overlay: this.selectedMapOverlay,
            fit: 'true'
        };
        
        router.push({ 
            path: targetPath, 
            query: query 
        }).catch(err => {
            console.error('[Default] Router push failed:', err);
        });
    },
    updateFullscreenState() {
      this.isFullscreen = !!document.fullscreenElement;
    },
    toggleFullscreen() {
      if (!document.fullscreenElement) {
          document.documentElement.requestFullscreen().catch(err => {
            console.error(`Error attempting to enable full-screen mode: ${err.message} (${err.name})`);
          });
      } else {
        if (document.exitFullscreen) {
          document.exitFullscreen();
        }
      }
    },
    async handleIntegrityCheck() {
        notify.showSuccess("Starting Integrity Check...");
        try {
            await filesApi.scanIntegrity(this.req.source, this.req.path);
            notify.showSuccess("Integrity Check Complete. Check logs/files.");
            mutations.setReload(true);
        } catch (e) {
            notify.showError(e.message);
        }
    },
    openAdminJobs() {
      mutations.showHover({ name: 'AdminJobs', props: {} });
    },
    confirmThumbnailFix() {
       if (confirm("Are you sure you want to run this action on all files in this folder and subfolders?\n\nThis will repair IPTCDigest issues and fix Thumbnail Tags in the EXIF.")) {
           this.handleThumbnailFix();
       }
    },
    async handleThumbnailFix() {
       notify.showSuccess("Thumbnail fix job started...");
       try {
         await fixThumbnails(this.req.source, this.req.path);
         notify.showSuccess("Thumbnail fix job done!");
       } catch (e) {
          notify.showError("Thumbnail fix failed: " + e.message);
       }
    },
    async checkFileIntegrity() {
      if (!this.isPreviewView || !this.req || !this.req.source || !this.req.path) {
        this.currentFileIssue = null;
        return;
      }
      
      const issue = await filesApi.getIntegrityIssues(this.req.source, this.req.path);
      this.currentFileIssue = issue;
    },
    showFileIssue() {
      if (!this.currentFileIssue) return;
      
      let message = 'Integrity Issues Detected:\n\n';
      
      if (this.currentFileIssue.Error) {
        message += `Error: ${this.currentFileIssue.Error}\n`;
      }
      if (this.currentFileIssue.Warning) {
        message += `Warning: ${this.currentFileIssue.Warning}\n`;
        // User Request: Indicate that SceneType warning is not concerning
        if (this.currentFileIssue.Warning.includes("Non-standard format (int16u) for EXIFIFD 0xa301 SceneType")) {
            message += "\n(NOTE: The SceneType warning is not an error that should be concerning.)\n";
        }
        // User Request: Indicate that Wrong IFD for ImageWidth is not concerning
        if (this.currentFileIssue.Warning.includes("Wrong IFD for 0x0100 ImageWidth")) {
            message += "\n(NOTE: This warning is about metadata placement, not image data integrity, and NOT critical. Actual pixel dimensions live in the JPEG SOF marker, not EXIF.)\n";
        }
      }
      if (this.currentFileIssue.FileSize && this.currentFileIssue.FileSize < 20000) {
        message += `File Size: ${this.currentFileIssue.FileSize} bytes (below 20KB threshold)\n`;
      }
      
      alert(message);
    },
    async checkRegenerationStatus() {
        if (!this.req || !this.isListingView) {
            this.isRegenerating = false;
            return;
        }
        try {
            const status = await filesApi.getHeatmapStatus(this.req.source, this.req.path);
            console.log('[HeatmapDebug] Status response:', status);
            this.isRegenerating = status.isScanning || false;
            
            if (this.isRegenerating) {
                 if (status.progress && status.progress.total > 0) {
                     this.regenerationLabel = `Scanning ${status.progress.current} of ${status.progress.total}`;
                     console.log('[HeatmapDebug] Progress:', status.progress.current, '/', status.progress.total);
                 } else {
                     this.regenerationLabel = "Regenerating...";
                     console.log('[HeatmapDebug] No progress data, showing generic message');
                 }
            } else {
                 this.regenerationLabel = "Regenerate Heatmap";
            }
            
            // If scanning, start polling if not already
            if (this.isRegenerating && !this.regenInterval) {
                console.log('[HeatmapDebug] Starting polling interval');
                this.regenInterval = setInterval(async () => {
                   if (!this.req) return;
                   const s = await filesApi.getHeatmapStatus(this.req.source, this.req.path);
                   console.log('[HeatmapDebug] Poll response:', s);
                   this.isRegenerating = s.isScanning || false;
                   
                   if (this.isRegenerating) {
                        if (s.progress && s.progress.total > 0) {
                             this.regenerationLabel = `Scanning ${s.progress.current} of ${s.progress.total}`;
                        } else {
                             this.regenerationLabel = "Regenerating...";
                        }
                   }
                   
                   if (!this.isRegenerating) {
                       clearInterval(this.regenInterval);
                       this.regenInterval = null;
                       this.regenerationLabel = "Regenerate Heatmap";
                       notify.showSuccess("Heatmap regeneration finished");
                   }
                }, 500); // Poll every 500ms for more responsive updates
            }
        } catch (e) {
            this.isRegenerating = false;
            if (this.regenInterval) {
                 clearInterval(this.regenInterval);
                 this.regenInterval = null;
            }
        }
    },
    toggleMetadata() {
      mutations.toggleMetadataVisibility();
    },
    toggleOverflow() {
      if (getters.currentPromptName() === "OverflowMenu") {
        mutations.closeHovers();
      } else {
        mutations.showHover({ name: "OverflowMenu" });
      }
    },
    switchView() {
      mutations.closeHovers();
      const index = this.viewModes.indexOf(state.user.viewMode);
      const next = (index + 1) % this.viewModes.length;
      mutations.updateCurrentUser({ viewMode: this.viewModes[next] });
    },
    openHeatmap() {
      // Navigate to heatmap for current folder
      // Allow root path (empty string)
      if (this.req.source !== undefined && this.req.path !== undefined) {
          const targetPath = this.req.path || "/";
          router.push({ path: '/heatmap', query: { source: this.req.source, path: targetPath } });
      }
    },
    openFolderDetails() {
      mutations.showHover({ name: "FolderDetails" });
    },
    async scanFolderFaces() {
      if (this.isScanningFaces) return;
      this.isScanningFaces = true;
      try {
        notify.showSuccess(`Scanning folder for faces...`);
        const { scanFacesFolder } = await import('@/api/files');
        await scanFacesFolder(this.req.url);
        // Start polling the global status
        this.checkScanStatus();
      } catch (e) {
        notify.showError(`Error scanning folder: ${e.message}`);
        this.isScanningFaces = false;
      }
    },
    openSearch() {
      mutations.toggleSearchSidebar();
    },
    async handleRegenerateHeatmap() {
        if (this.isRegenerating) return;
        this.isRegenerating = true;
        this.regenerationLabel = "Starting scan...";
        notify.showSuccess("Regenerating heatmap...");
        try {
            await filesApi.regenerateHeatmap(this.req.source, this.req.path);
            // API returns immediately now, start polling for progress
            this.checkRegenerationStatus();
        } catch (e) {
            notify.showError(e.message);
            this.regenerationLabel = "Regenerate Heatmap";
            this.isRegenerating = false;
        }
    },
    multiAction() {
      const listingView = getters.currentView();
      if (listingView == "listingView") {
        mutations.toggleSidebar();
      } else {
        mutations.closeHovers();
        if (listingView === "settings") {
          router.push({ path: "/files" });
          return;
        }
        mutations.replaceRequest({});
        router.go(-1);
      }
    },
    showHeaderTemporarily() {
      this.showHeader = true;
    },

    download() {
      const url = filesApi.getDownloadURL(state.req.source, state.req.path);
      window.open(url);
    },
    rotate() {
      mutations.rotatePreview();
    },
    async share() {
      const url = filesApi.getDownloadURL(state.req.source, state.req.path, true);
      try {
        const response = await fetch(url);
        const blob = await response.blob();
        const file = new File([blob], state.req.name, { type: blob.type });

        if (navigator.canShare && navigator.canShare({ files: [file] })) {
          await navigator.share({
            files: [file],
            title: state.req.name,
            text: 'Check out this file!',
          });
        } else {
             alert('Sharing is not supported on this device/browser.');
        }
      } catch (err) {
        if (err.name !== 'AbortError' && err.name !== 'NotAllowedError') {
             console.error('Share failed:', err);
        }
      }
    },
  },
  beforeUnmount() {
    // Clean up
  },
};
</script>



<style scoped>
.header-hidden {
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.3s ease;
}

header {
  transition: opacity 0.3s ease;
  display: flex !important;
  align-items: center;
  gap: 0.5em; /* Add some spacing between items */
}

/* Ensure flexbar items don't shrink too much */
.flexbar > * {
    flex-shrink: 0;
}
.header-middle {
    flex-grow: 1;
    /* Allow shrinking below content size if needed */
    flex-shrink: 1; 
    min-width: 0;
    
    display: flex;
    justify-content: center; /* Center search bar */
    align-items: center;
}
.header-middle > * {
    /* Don't force width 100% on search, let it size itself */
    width: auto;
    max-width: 100%;
}
.flexbar .topTitle {
    /* flex-shrink handled by wrapper */
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

/* Sticky breadcrumb in header */
.sticky-breadcrumb {
    display: flex;
    align-items: center;
    gap: 0.25em;
    overflow: hidden;
    white-space: nowrap;
    max-width: 100%;
    animation: fadeIn 0.2s ease;
}

@keyframes fadeIn {
    from { opacity: 0; transform: translateY(-4px); }
    to   { opacity: 1; transform: translateY(0); }
}

.sticky-crumb {
    color: var(--textPrimary);
    text-decoration: none;
    font-size: 0.9em;
    padding: 0.2em 0.4em;
    border-radius: 4px;
    max-width: 14ch;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    display: inline-flex;
    align-items: center;
    transition: background 0.15s;
}

.sticky-crumb:hover {
    background: var(--alt-background);
    color: var(--primaryColor);
}

.sticky-crumb .material-icons {
    font-size: 1.1em;
}

.sticky-sep {
    color: var(--textSecondary, #888);
    font-size: 1em;
    flex-shrink: 0;
    user-select: none;
}

/* Map Overlays Dropdown */
.map-overlays-select-container {
    margin-left: 10px;
    display: flex;
    align-items: center;
}

.map-overlays-select {
    background-color: rgba(0, 0, 0, 0.1);
    color: inherit;
    border: 1px solid rgba(0, 0, 0, 0.2);
    border-radius: 4px;
    padding: 4px 8px;
    font-size: 0.9em;
    cursor: pointer;
    outline: none;
    max-width: 150px;
}

.dark-mode-header .map-overlays-select {
    background-color: rgba(255, 255, 255, 0.1);
    border-color: rgba(255, 255, 255, 0.2);
    color: white;
}

.map-overlays-select option {
    background-color: white;
    color: black;
}

.spin-action :deep(i), .spin-action i {
    animation: 2s rotate linear infinite;
}

@keyframes flash-highlight {
    0% { background-color: transparent; box-shadow: none; }
    25% { background-color: rgba(255, 235, 59, 0.8); box-shadow: 0 0 10px rgba(255, 235, 59, 0.8); transform: scale(1.1); }
    50% { background-color: transparent; box-shadow: none; transform: scale(1.0); }
    75% { background-color: rgba(255, 235, 59, 0.8); box-shadow: 0 0 10px rgba(255, 235, 59, 0.8); transform: scale(1.1); }
    100% { background-color: transparent; box-shadow: none; transform: scale(1.0); }
}

.flash-animation {
    animation: flash-highlight 1.5s ease-in-out;
}


.dark-mode-header .map-overlays-select option {
    background-color: #333;
    color: white;
}

.flash-on-load {
    animation: flash-highlight 1s ease-in-out 3;
}

/* Heatmap progress banner */
.heatmap-progress-banner {
    position: fixed;
    top: 4rem;
    left: 50%;
    transform: translateX(-50%);
    background: rgba(33, 150, 243, 0.95);
    color: white;
    padding: 0.75rem 1.5rem;
    border-radius: 0.5rem;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
    display: flex;
    align-items: center;
    gap: 0.75rem;
    z-index: 1000;
    font-size: 1rem;
    font-weight: 500;
    animation: slideDown 0.3s ease-out;
}

.heatmap-progress-banner i {
    animation: rotate 2s linear infinite;
    font-size: 1.25rem;
}

@keyframes slideDown {
    from {
        opacity: 0;
        transform: translateX(-50%) translateY(-10px);
    }
    to {
        opacity: 1;
        transform: translateX(-50%) translateY(0);
    }
}

@keyframes rotate {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
}

@media (max-width: 768px) {
    .heatmap-progress-banner {
        top: 3.5rem;
        padding: 0.6rem 1.2rem;
        font-size: 0.95rem;
    }
}

</style>
