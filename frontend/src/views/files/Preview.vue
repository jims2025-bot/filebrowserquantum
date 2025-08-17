<template>
  <div id="previewer" @mousemove="toggleNavigation" @touchstart="toggleNavigation">
    <div class="preview">
      <ExtendedImage v-if="previewType == 'image'" :src="raw"> </ExtendedImage>
      <audio
        v-else-if="previewType == 'audio'"
        ref="player"
        :src="raw"
        controls
        :autoplay="autoPlay"
        @play="autoPlay = true"
      ></audio>
      <video
        v-else-if="previewType == 'video'"
        ref="player"
        :src="raw"
        controls
        :autoplay="autoPlay"
        @play="autoPlay = true"
      >
        <track
          kind="captions"
          v-for="(sub, index) in subtitlesList"
          :key="index"
          :src="sub.src"
          :label="'Subtitle ' + sub.name"
          :default="index === 0"
        />
      </video>

      <object v-else-if="previewType == 'pdf'" class="pdf" :data="raw"></object>
      <div v-else class="info">
        <div class="title">
          <i class="material-icons">feedback</i>
          {{ $t("files.noPreview") }}
        </div>
        <div>
          <a target="_blank" :href="downloadUrl" class="button button--flat">
            <div>
              <i class="material-icons">file_download</i>{{ $t("buttons.download") }}
            </div>
          </a>
          <a
            target="_blank"
            :href="raw"
            class="button button--flat"
            v-if="req.type != 'directory'"
          >
            <div>
              <i class="material-icons">open_in_new</i>{{ $t("buttons.openFile") }}
            </div>
          </a>
        </div>
      </div>
    </div>

    <!-- START OF NEW TABBED INTERFACE -->
    <div class="metadata-container">
      <div class="tabs-header">
        <button
          v-for="tab in tabs"
          :key="tab.name"
          :class="{ active: activeTab === tab.name }"
          @click="selectTab(tab.name)"
        >
          {{ tab.label }}
        </button>
      </div>
      <div class="tabs-content">
        <!-- DETAILS Tab -->
        <div v-if="activeTab === 'details'" class="tab-pane">
          <h3>File Details</h3>
          <ul>
            <li><strong>Name:</strong> {{ req.name }}</li>
            <li><strong>Path:</strong> {{ req.path }}</li>
            <li><strong>Size:</strong> {{ req.size | bytesToSize }}</li>
            <li><strong>Type:</strong> {{ req.type }}</li>
            <li><strong>Modified:</strong> {{ formattedModifiedDate }}</li>
          </ul>
        </div>

        <!-- EXIF Tab -->
        <div v-if="activeTab === 'exif'" class="tab-pane">
          <h3>EXIF Metadata</h3>
          <!--
            TODO:
            Bind your fetched EXIF data here. Example:
            <ul v-if="metadata && metadata.exif">
              <li v-for="(value, key) in metadata.exif" :key="key">
                <strong>{{ key }}:</strong> {{ value }}
              </li>
            </ul>
            <p v-else>No EXIF data available.</p>
          -->
          <p>This tab will display EXIF metadata once implemented in the backend.</p>
        </div>

        <!-- IPTC Tab -->
        <div v-if="activeTab === 'iptc'" class="tab-pane">
          <h3>IPTC Metadata</h3>
          <!--
            TODO:
            Bind your fetched IPTC data here. Example:
            <ul v-if="metadata && metadata.iptc">
              <li v-for="(value, key) in metadata.iptc" :key="key">
                <strong>{{ key }}:</strong> {{ value }}
              </li>
            </ul>
            <p v-else>No IPTC data available.</p>
          -->
          <p>This tab will display IPTC metadata once implemented in the backend.</p>
        </div>

        <!-- XMP Tab -->
        <div v-if="activeTab === 'xmp'" class="tab-pane">
          <h3>XMP Metadata</h3>
          <!--
            TODO:
            Bind your fetched XMP data here. Example:
            <ul v-if="metadata && metadata.xmp">
              <li v-for="(value, key) in metadata.xmp" :key="key">
                <strong>{{ key }}:</strong> {{ value }}
              </li>
            </ul>
            <p v-else>No XMP data available.</p>
          -->
          <p>This tab will display XMP metadata once implemented in the backend.</p>
        </div>

        <!-- MAP Tab -->
        <div v-if="activeTab === 'map'" class="tab-pane">
          <h3>Map Location</h3>
          <!--
            TODO:
            Implement a map component here using latitude and longitude data from your API call.
            <div id="map-container"></div>
            <p v-else>No geographical data available.</p>
          -->
          <p>This tab will display a map of the image's location once implemented in the backend and frontend.</p>
        </div>
      </div>
    </div>
    <!-- END OF NEW TABBED INTERFACE -->

    <button
      @click="prev"
      @mouseover="hoverNav = true"
      @mouseleave="hoverNav = false"
      :class="{ hidden: !hasPrevious || !showNav }"
      :aria-label="$t('buttons.previous')"
      :title="$t('buttons.previous')"
    >
      <i class="material-icons">chevron_left</i>
    </button>
    <button
      @click="next"
      @mouseover="hoverNav = true"
      @mouseleave="hoverNav = false"
      :class="{ hidden: !hasNext || !showNav }"
      :aria-label="$t('buttons.next')"
      :title="$t('buttons.next')"
    >
      <i class="material-icons">chevron_right</i>
    </button>
    <link rel="prefetch" :href="previousRaw" />
    <link rel="prefetch" :href="nextRaw" />
  </div>
</template>
<script>
import { filesApi } from "@/api";
import url from "@/utils/url.js";
import throttle from "@/utils/throttle";
import ExtendedImage from "@/components/files/ExtendedImage.vue";
import { state, getters, mutations } from "@/store";
import { getFileExtension } from "@/utils/files";
import { convertToVTT } from "@/utils/subtitles";
import { getTypeInfo } from "@/utils/mimetype";
import moment from "moment";

export default {
  name: "preview",
  components: {
    ExtendedImage,
  },
  data() {
    return {
      previousLink: "",
      nextLink: "",
      listing: null,
      name: "",
      fullSize: true,
      showNav: true,
      navTimeout: null,
      hoverNav: false,
      autoPlay: false,
      previousRaw: "",
      nextRaw: "",
      currentPrompt: null, // Replaces Vuex getter `currentPrompt`
      subtitlesList: [],
      // START OF NEW DATA PROPERTIES
      activeTab: "details",
      tabs: [
        { name: "details", label: "Details" },
        { name: "exif", label: "EXIF" },
        { name: "iptc", label: "IPTC" },
        { name: "xmp", label: "XMP" },
        { name: "map", label: "Map" },
      ],
      metadata: null,
      // END OF NEW DATA PROPERTIES
    };
  },
  computed: {
    sidebarShowing() {
      return getters.isSidebarVisible();
    },
    previewType() {
      return getters.previewType();
    },
    raw() {
      return filesApi.getDownloadURL(state.req.source, state.req.path, true);
    },
    isDarkMode() {
      return getters.isDarkMode();
    },
    hasPrevious() {
      return this.previousLink !== "";
    },
    hasNext() {
      return this.nextLink !== "";
    },
    downloadUrl() {
      return filesApi.getDownloadURL(state.req.source, state.req.path);
    },
    showMore() {
      return getters.currentPromptName() === "more";
    },
    getSubtitles() {
      return this.subtitles();
    },
    req() {
      return state.req;
    },
    formattedModifiedDate() {
      if (!this.req || !this.req.modified) {
        return '';
      }
      return moment(this.req.modified).format('LL');
    }
  },
  watch: {
    req() {
      if (!getters.isLoggedIn()) {
        return;
      }
      this.updatePreview();
      this.toggleNavigation();
      // START OF NEW WATCHER LOGIC
      this.activeTab = "details"; // Reset to the first tab when the file changes
      this.fetchMetadata(); // Fetch metadata for the new file
      // END OF NEW WATCHER LOGIC
    },
  },
  async mounted() {
    window.addEventListener("keydown", this.key);
    this.subtitlesList = await this.subtitles();
    this.updatePreview();
    mutations.resetSelected();
    mutations.addSelected({
      name: state.req.name,
      path: state.req.path,
      size: state.req.size,
      type: state.req.type,
      source: state.req.source,
      url: state.req.url,
    });
    // START OF NEW MOUNTED LOGIC
    this.fetchMetadata(); // Fetch metadata on initial mount
    // END OF NEW MOUNTED LOGIC
  },
  beforeUnmount() {
    window.removeEventListener("keydown", this.key);
  },
  methods: {
    // START OF NEW METHODS
    selectTab(tabName) {
      this.activeTab = tabName;
    },
    async fetchMetadata() {
      // Check if the file is an image before trying to fetch metadata
      if (this.previewType !== 'image') {
        this.metadata = null;
        return;
      }
      
      // TODO: Replace this with your actual API call.
      // This is a placeholder to show the structure.
      // You should call your new /api/metadata endpoint here.
      // Example:
      // try {
      //   const res = await filesApi.fetchMetadata(state.req.source, state.req.path);
      //   this.metadata = res.data;
      // } catch (error) {
      //   console.error("Failed to fetch metadata:", error);
      //   this.metadata = null;
      // }
      console.log('Fetching metadata for:', state.req.path);
    },
    // END OF NEW METHODS
    async subtitles() {
      if (!state.req.subtitles || state.req.subtitles.length === 0) {
        return [];
      }
      let subs = [];
      for (let subtitleFile of state.req.subtitles) {
        if (state.serverHasMultipleSources) {
          subtitleFile = "/files/" + state.req.source + subtitleFile;
        } else {
          subtitleFile = "/files" + subtitleFile;
        }
        const ext = getFileExtension(subtitleFile);
        const resp = await filesApi.fetchFiles(subtitleFile, true); // Fetch .srt file
        let vttContent = resp.content;
        // Convert SRT to VTT (assuming srt2vtt() does this)
        vttContent = convertToVTT(ext, resp.content);
        // Create a virtual file (Blob) and get a URL for it
        const blob = new Blob([vttContent], { type: "text/vtt" });
        const vttURL = URL.createObjectURL(blob);
        subs.push({
          name: ext,
          src: vttURL,
        });
      }
      return subs;
    },
    deleteFile() {
      this.currentPrompt = {
        name: "delete",
        confirm: () => {
          this.listing = this.listing.filter((item) => item.name !== this.name);
          if (this.hasNext) {
            this.next();
          } else if (!this.hasPrevious && !this.hasNext) {
            this.close();
          } else {
            this.prev();
          }
        },
      };
    },
    prev() {
      this.hoverNav = false;
      this.$router.replace({ path: this.previousLink });
    },
    next() {
      this.hoverNav = false;
      this.$router.replace({ path: this.nextLink });
    },
    key(event) {
      if (getters.currentPromptName() != null) {
        return;
      }

      const { key } = event;

      switch (key) {
        case "ArrowRight":
          if (this.hasNext) {
            this.next();
          }
          break;
        case "ArrowLeft":
          if (this.hasPrevious) {
            this.prev();
          }
          break;
        case ("Escape", "Backspace"):
          this.close();
          break;
      }
    },
    async updatePreview() {
      if (this.$refs.player && this.$refs.player.paused && !this.$refs.player.ended) {
        this.autoPlay = false;
      }
      if (!this.listing) {
        const path = url.removeLastDir(getters.routePath());
        const res = await filesApi.fetchFiles(path);
        this.listing = res.items;
      }
      this.name = state.req.name;
      this.previousLink = "";
      this.nextLink = "";
      const path = state.req.path;

      let directoryPath = path.substring(0, path.lastIndexOf("/"));
      if (directoryPath == "") {
        directoryPath = "/";
      }
      for (let i = 0; i < this.listing.length; i++) {
        if (this.listing[i].name !== this.name) {
          continue;
        }
        for (let j = i - 1; j >= 0; j--) {
          let composedListing = this.listing[j];
          composedListing.path = directoryPath + "/" + composedListing.name;
          this.previousLink = composedListing.url;
          if (getTypeInfo(composedListing.type).simpleType == "image") {
            this.previousRaw = this.prefetchUrl(composedListing);
          }
          break;
        }
        for (let j = i + 1; j < this.listing.length; j++) {
          let composedListing = this.listing[j];
          composedListing.path = directoryPath + "/" + composedListing.name;
          this.nextLink = composedListing.url;
          if (getTypeInfo(composedListing.type).simpleType == "image") {
            this.nextRaw = this.prefetchUrl(composedListing);
          }
          break;
        }
        return;
      }
    },
    prefetchUrl(item) {
      return this.fullSize
        ? filesApi.getDownloadURL(state.req.source, item.path, true)
        : filesApi.getPreviewURL(state.req.source, item.path, item.modified);
    },
    openMore() {
      this.currentPrompt = "more";
    },
    resetPrompts() {
      this.currentPrompt = null;
    },
    toggleSize() {
      this.fullSize = !this.fullSize;
    },
    toggleNavigation: throttle(function () {
      this.showNav = true;

      if (this.navTimeout) {
        clearTimeout(this.navTimeout);
      }

      this.navTimeout = setTimeout(() => {
        this.showNav = false || this.hoverNav;
        this.navTimeout = null;
      }, 1500);
    }, 100),
    close() {
      mutations.replaceRequest({}); // Reset request data
      let uri = url.removeLastDir(state.route.path) + "/";
      this.$router.push({ path: uri });
    },
    download() {
      window.open(this.downloadUrl);
    },
  },
};
</script>
<style lang="scss">
:root {
  --dark-theme-1: #1a1a1a;
  --dark-theme-2: #242424;
  --accent: #42b983;
}

#previewer {
  display: flex;
  height: 100%;
  width: 100%;
  align-items: center;
  justify-content: center;
  background: var(--dark-theme-1);
  position: absolute;
  top: 0;
  left: 0;
  
  .preview {
    width: 100%;
    height: calc(100% - 150px);
    display: flex;
    align-items: center;
    justify-content: center;
  }
}

.metadata-container {
  position: absolute;
  bottom: 0;
  left: 0;
  width: 100%;
  max-height: 150px;
  background: var(--dark-theme-1);
  color: #fff;
  border-top: 1px solid var(--dark-theme-2);
  padding: 1rem;
  overflow-y: auto;
  
  .tabs-header {
    display: flex;
    justify-content: center;
    margin-bottom: 1rem;
    
    button {
      background: none;
      border: none;
      color: #fff;
      font-weight: bold;
      padding: 0.5rem 1rem;
      cursor: pointer;
      opacity: 0.6;
      transition: opacity 0.2s ease-in-out;
      
      &:hover {
        opacity: 1;
      }
      
      &.active {
        opacity: 1;
        border-bottom: 2px solid var(--accent);
      }
    }
  }
  
  .tab-pane {
    h3 {
      margin-top: 0;
      color: var(--accent);
      text-align: center;
    }
    
    ul {
      list-style-type: none;
      padding: 0;
      li {
        padding: 0.25rem 0;
      }
    }
  }
}
</style>
