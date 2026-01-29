<template>
  <header v-if="!isOnlyOffice" :class="['flexbar', { 'dark-mode-header': isDarkMode, 'header-hidden': !showHeader }]">
    <action
      v-if="!isShare"
      icon="close_back"
      :label="$t('buttons.close')"
      :disabled="isSearchActive"
      @action="multiAction"
    />
    <div class="header-middle">
        <search v-if="showSearch" />
        <title v-else-if="isSettings" class="topTitle">{{ $t("sidebar.settings") }}</title>
        <title v-else class="topTitle">{{ req.name }}</title>
    </div>
    
    <action
      v-if="isListingView"
      icon="map"
      :label="$t('sidebar.heatmap')"
      @action="openHeatmap"
    />
    <action
      v-if="isListingView"
      icon="sync"
      :label="regenerationLabel"
      :disabled="isRegenerating"
      @action="handleRegenerateHeatmap"
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
import Search from "@/components/Search.vue";
import * as filesApi from "@/api/files";
import { notify } from "@/notify";

export default {
  name: "UnifiedHeader",
  components: {
    Action,
    Search,
  },
  data() {
    return {
      viewModes: ["list", "compact", "normal", "gallery"],
      showHeader: true,
      headerTimeout: null,
      isRegenerating: false,
      regenerationLabel: "Regenerate Heatmap",
      regenInterval: null,
    };
  },
  computed: {
    isOnlyOffice() {
      return getters.currentView() === "onlyOfficeEditor";
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
    isMetadataToggleVisible() {
      return getters.currentView() === 'preview';
    },
  },
  watch: {
    req: {
      handler() {
        this.checkRegenerationStatus();
      },
      deep: true
    }
  },
  mounted() {
    this.checkRegenerationStatus();
  },
  beforeDestroy() {
    if (this.regenInterval) clearInterval(this.regenInterval);
  },
  methods: {
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
  mounted() {
    this.showHeader = true;
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
