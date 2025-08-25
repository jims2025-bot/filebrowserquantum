<template>
  <div id="previewer" @mousemove="toggleNavigation" @touchstart="toggleNavigation">
    <!-- Preview Section -->
    <div class="preview" :class="{ 'full-height': !isMetadataVisible }" :style="{ maxHeight: previewMaxHeight }">
      <div class="image-container" v-if="previewType == 'image'">
        <img 
          ref="image" 
          :src="raw" 
          @load="updateImageDimensions" 
          class="preview-image"
          :style="imageStyle"
          @wheel="handleWheel"
          @mousedown="startPan"
          @touchstart="handleTouchStart"
          @touchmove="handleTouchMove"
          @touchend="handleTouchEnd"
        >
        <div
          v-if="activeTab === 'xmp' && metadata && metadata.xmp && metadata.xmp.Regions && metadata.xmp.Regions.length > 0 && isMetadataVisible"
          class="face-overlay"
          :style="{ width: imageDimensions.width + 'px', height: imageDimensions.height + 'px', top: imageOffset.top + 'px', left: imageOffset.left + 'px' }"
        >
          <div
            v-for="(region, index) in metadata.xmp.Regions"
            :key="index"
            class="face-box"
            :style="getFaceBoxStyle(region)"
          >
            <span class="face-label">{{ region.Name || 'Unnamed' }}</span>
          </div>
        </div>
      </div>

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

    <!-- Metadata Section -->
    <div class="metadata-container" v-if="isMetadataVisible" :style="{ height: metadataHeight + 'px' }">
      <div class="resize-handle" @mousedown="startResize" @touchstart.stop="startResize"></div>

      <!-- Tabs Header -->
      <div class="tabs-header">
        <button
          v-for="tab in availableTabs"
          :key="tab.name"
          :class="[
			{ active: activeTab === tab.name },
			{ 'tab-empty': isTabEmpty(tab.name) }
		  ]"
          @click="selectTab(tab.name)"
        >
          {{ tab.label }}
        </button>
        <div>Debug: Available tabs - {{ availableTabs.map(tab => tab.label).join(', ') }}</div>
      </div>

      <!-- Tabs Content -->
      <div class="tabs-content">
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

        <div v-if="activeTab === 'exif'" class="tab-pane">
          <h3>EXIF Metadata</h3>
          <div v-if="metadata && Object.keys(metadata.exif).length > 0" class="metadata-table">
            <table>
              <thead>
                <tr>
                  <th>Tag</th>
                  <th>Value</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(value, key) in metadata.exif" :key="key">
                  <td>{{ key }}</td>
                  <td>{{ value }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <p v-else-if="metadata && Object.keys(metadata.exif).length === 0">No EXIF data found for this file.</p>
          <p v-else>Loading EXIF metadata...</p>
        </div>

        <div v-if="activeTab === 'iptc'" class="tab-pane">
          <h3>IPTC Metadata</h3>
          <div v-if="metadata && Object.keys(metadata.iptc).length > 0" class="metadata-table">
            <table>
              <thead>
                <tr>
                  <th>Tag</th>
                  <th>Value</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(value, key) in metadata.iptc" :key="key">
                  <td>{{ key }}</td>
                  <td>{{ value }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <p v-else-if="metadata && Object.keys(metadata.iptc).length === 0">No IPTC data found for this file.</p>
          <p v-else>Loading IPTC metadata...</p>

          <!-- Inline Photoshop Instructions Modal -->
          <div style="margin-top: 1rem;">
            <strong>Photoshop Instructions:</strong>
            <span v-if="photoshopInstructions">{{ photoshopInstructions }}</span>
            <span v-else>No instructions</span>
            <button @click="openInstructionsModal" class="button button--flat">Edit</button>

            <div v-if="showInstructionsModal" class="inline-modal">
              <textarea v-model="photoshopInstructions" rows="10"></textarea>
              <div style="margin-top:0.5rem; text-align:right">
                <button @click="saveInstructions" class="button button--flat">Save</button>
                <button @click="showInstructionsModal = false" class="button button--flat">Cancel</button>
              </div>
            </div>
          </div>
        </div>

        <div v-if="activeTab === 'xmp'" class="tab-pane">
          <h3>XMP Metadata</h3>
          <div v-if="metadata && metadata.xmp && Object.keys(metadata.xmp).length > 0" class="metadata-table">
            <table v-if="metadata.xmp.Regions && metadata.xmp.Regions.length > 0">
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Details</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(region, index) in metadata.xmp.Regions" :key="index">
                  <td>{{ region.Name || 'Unnamed' }}</td>
                  <td>{{ JSON.stringify(region) }}</td>
                </tr>
              </tbody>
            </table>
            <table v-else>
              <thead>
                <tr>
                  <th>Key</th>
                  <th>Value</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(value, key) in metadata.xmp" :key="key">
                  <td>{{ key }}</td>
                  <td>{{ JSON.stringify(value) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <p v-else-if="metadata && (!metadata.xmp || Object.keys(metadata.xmp).length === 0)">No XMP data found for this file.</p>
          <p v-else>Loading XMP metadata...</p>
        </div>

        <div v-if="activeTab === 'map'" class="tab-pane">
          <h3>Map Location</h3>
          <p>This tab will display a map of the image's location once implemented in the backend and frontend.</p>
        </div>
      </div>
    </div>

    <!-- Navigation Buttons -->
    <button
      @click="prev"
      @mouseover="hoverNav = true"
      @mouseleave="hoverNav = false"
      :class="{ hidden: !hasPrevious || !showNav }"
      :aria-label="$t('buttons.previous')"
      :title="$t('buttons.previous')"
      class="nav-button nav-button-prev"
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
      class="nav-button nav-button-next"
    >
      <i class="material-icons">chevron_right</i>
    </button>
    <link rel="prefetch" :href="previousRaw" />
    <link rel="prefetch" :href="nextRaw" />
  </div>
</template>


<script>
import * as filesApi from "@/api/files.js";
import url from "@/utils/url.js";
import throttle from "@/utils/throttle";
import { state, getters, mutations } from "@/store";
import { getFileExtension } from "@/utils/files";
import { convertToVTT } from "@/utils/subtitles";
import { getTypeInfo } from "@/utils/mimetype";
import moment from "moment";

export default {
  name: "preview",
  components: {},
  data() {
    return {
	  photoshopInstructions: '',      // Stores the value for display/edit
      showInstructionsModal: false,   // Controls modal popup visibility
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
      currentPrompt: null,
      subtitlesList: [],
      activeTab: "details",
      tabs: [
        { name: "details", label: "Details" },
        { name: "exif", label: "EXIF" },
        { name: "iptc", label: "IPTC" },
        { name: "xmp", label: "XMP" },
        { name: "map", label: "Map" },
      ],
      metadata: null,
      isResizing: false,
      metadataHeight: 300,
      imageDimensions: { width: 0, height: 0 },
      imageOffset: { top: 0, left: 0 },
      dimensionRetryCount: 0,
      // Zoom and pan state
      zoomLevel: 1,
      panPosition: { x: 0, y: 0 },
      isPanning: false,
      lastPanPosition: { x: 0, y: 0 },
      // Touch state for pinch-to-zoom
      touchStartDistance: 0,
      touchStartZoom: 1,
      isTouching: false,
    };
  },
  computed: {
    sidebarShowing() {
      return getters.isSidebarVisible();
    },
    previewType() {
      const type = getters.previewType();
      console.log("Preview type:", type);
      return type;
    },
	availableTabs() {
		const tabs = [
		{ name: "details", label: "Details" },
		{ name: "exif", label: "EXIF" },
		{ name: "iptc", label: "IPTC" },
		{ name: "xmp", label: "XMP" },
		{ name: "map", label: "Map" },
		];
		return tabs;
	},
    raw() {
      const url = filesApi.getDownloadURL(state.req.source, state.req.path, true);
      console.log("Image src URL:", url);
      return url;
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
        return "";
      }
      return moment(this.req.modified).format("LL");
    },
    isMetadataVisible() {
      const visible = getters.isMetadataVisible();
      console.log("isMetadataVisible:", visible);
      return visible;
    },
    previewMaxHeight() {
      return this.isMetadataVisible ? `calc(100vh - ${this.metadataHeight}px)` : '100vh';
    },
    // Computed style for the image based on zoom and pan
    imageStyle() {
      if (this.isMetadataVisible || this.previewType !== 'image') {
        return {};
      }
      
      return {
        transform: `translate(${this.panPosition.x}px, ${this.panPosition.y}px) scale(${this.zoomLevel})`,
        transformOrigin: 'center center',
        cursor: this.isPanning ? 'grabbing' : 'grab'
      };
    },
  },
  watch: {
    async req() {
      if (!getters.isLoggedIn()) {
        return;
      }
      this.dimensionRetryCount = 0;
      await this.updatePreview();
      this.toggleNavigation();
      await this.fetchMetadata();
      await this.updateImageDimensions();
      this.$nextTick(() => {
        const availableTabNames = this.availableTabs.map(tab => tab.name);
        console.log("Validating activeTab:", this.activeTab, "Available:", availableTabNames);
        if (!availableTabNames.includes(this.activeTab)) {
          this.activeTab = "details";
          console.log("Reset activeTab to 'details' as current tab is not available");
        }
      });
    },
    activeTab(newTab) {
      if (newTab === "xmp") {
        this.dimensionRetryCount = 0;
        this.$nextTick(() => this.updateImageDimensions());
      }
    },
    isMetadataVisible(newValue) {
      if (newValue) {
        // Reset zoom and pan when metadata becomes visible
        this.resetZoomAndPan();
      }
      this.$nextTick(() => this.updateImageDimensions());
    },
     photoshopInstructions(newVal) {
		if (this.metadata && this.metadata.xmp) {
		this.metadata.xmp['photoshop:Instructions'] = newVal;
		}
   	  }
  },
  async mounted() {
    window.addEventListener("keydown", this.key);
    this.subtitlesList = await this.subtitles();
    await this.updatePreview();
    mutations.resetSelected();
    mutations.addSelected({
      name: state.req.name,
      path: state.req.path,
      size: state.req.size,
      type: state.req.type,
      source: state.req.source,
      url: state.req.url,
    });
    await this.fetchMetadata();
    await this.updateImageDimensions();
    document.addEventListener("mousemove", this.resizeMetadata);
    document.addEventListener("mouseup", this.stopResize);
    document.addEventListener("touchmove", this.resizeMetadata);
    document.addEventListener("touchend", this.stopResize);
    window.addEventListener("resize", this.updateImageDimensions);
    // Add event listeners for panning
    document.addEventListener("mousemove", this.handlePan);
    document.addEventListener("mouseup", this.stopPan);
  },
  beforeUnmount() {
    window.removeEventListener("keydown", this.key);
    document.removeEventListener("mousemove", this.resizeMetadata);
    document.removeEventListener("mouseup", this.stopResize);
    document.removeEventListener("touchmove", this.resizeMetadata);
    document.removeEventListener("touchend", this.stopResize);
    window.removeEventListener("resize", this.updateImageDimensions);
    // Remove event listeners for panning
    document.removeEventListener("mousemove", this.handlePan);
    document.removeEventListener("mouseup", this.stopPan);
  },
  methods: {
    selectTab(tabName) {
      this.activeTab = tabName;
    },
    async fetchMetadata() {
      this.metadata = null;
      if (this.previewType !== "image") {
        console.log("Not an image, skipping metadata fetch");
        return;
      }
      try {
        const res = await filesApi.fetchMetadata(state.req.source, state.req.path);
        console.log("Full API response:", res);
        this.metadata = res.data || res;
		if (!this.metadata.xmp) {
			this.metadata.xmp = {};
		}
		this.parsePhotoshopInstructions(); // <-- parse instructions here
        console.log("Assigned metadata:", this.metadata);
      } catch (error) {
        console.error("Failed to fetch metadata:", error);
        this.metadata = { exif: {}, iptc: {}, xmp: {} };
      }
    },
    async updateImageDimensions() {
      if (this.previewType !== "image" || !this.$refs.image) {
        console.log("No image or ref, skipping dimension update");
        return;
      }
      await this.$nextTick();
      const imgElement = this.$refs.image;
      if (!imgElement) {
        console.error("No image element found");
        if (this.dimensionRetryCount < 5) {
          this.dimensionRetryCount++;
          console.log(`Retrying dimension update (${this.dimensionRetryCount}/5)`);
          setTimeout(() => this.updateImageDimensions(), 500);
        }
        return;
      }
      console.log("Found image element:", imgElement.tagName, imgElement);
      if (imgElement.complete || imgElement.readyState === 4) {
        const rect = imgElement.getBoundingClientRect();
        const width = rect.width;
        const height = rect.height;
        this.imageDimensions = { width, height };
        this.imageOffset = { top: rect.top, left: rect.left };
        console.log("Image dimensions:", this.imageDimensions);
        console.log("Image offset:", this.imageOffset);
        console.log("Image natural size:", { width: imgElement.naturalWidth, height: imgElement.naturalHeight });
      } else {
        console.log("Image not loaded, waiting for load event");
        imgElement.addEventListener(
          "load",
          () => {
            const rect = imgElement.getBoundingClientRect();
            const width = rect.width;
            const height = rect.height;
            this.imageDimensions = { width, height };
            this.imageOffset = { top: rect.top, left: rect.left };
            console.log("Image dimensions (loaded):", this.imageDimensions);
            console.log("Image offset (loaded):", this.imageOffset);
            console.log("Image natural size:", { width: imgElement.naturalWidth, height: imgElement.naturalHeight });
          },
          { once: true }
        );
        imgElement.addEventListener(
          "error",
          () => {
            console.error("Image failed to load:", this.raw);
          },
          { once: true }
        );
      }
    },
    
	isTabEmpty(tabName) {
		if (!this.metadata) return false;
		if (tabName === 'exif') return !this.metadata.exif || Object.keys(this.metadata.exif).length === 0;
		if (tabName === 'iptc') return !this.metadata.iptc || Object.keys(this.metadata.iptc).length === 0;
		if (tabName === 'xmp') return !this.metadata.xmp || Object.keys(this.metadata.xmp).length === 0;
		return false;
	},
  
    triggerHeaderVisibility() {
      // Emit an event that the header component can listen to
      this.$root.$emit('show-header-temporarily');
    },
    
    getFaceBoxStyle(region) {
      // Check if we have valid ALGArea data
      if (!region.ALGArea || 
          region.ALGArea.X === undefined || 
          region.ALGArea.Y === undefined || 
          region.ALGArea.W === undefined || 
          region.ALGArea.H === undefined ||
          !this.imageDimensions.width || 
          !this.imageDimensions.height) {
        console.log("Missing or invalid ALGArea or image dimensions, using fallback style");
        return {
          left: "10px",
          top: `${10 + 60 * this.metadata.xmp.Regions.indexOf(region)}px`,
          width: "100px",
          height: "100px",
          border: "2px solid var(--accent-blue)",
          background: "rgba(0, 0, 255, 0.2)",
        };
      }

      const { X, Y, W, H } = region.ALGArea;
      
      // Validate coordinates are numbers and within reasonable bounds
      if (typeof X !== 'number' || typeof Y !== 'number' || 
          typeof W !== 'number' || typeof H !== 'number' ||
          X < 0 || Y < 0 || W <= 0 || H <= 0 ||
          X > 1 || Y > 1 || W > 1 || H > 1) {
        console.log("Invalid ALGArea coordinates, using fallback style", { X, Y, W, H });
        return {
          left: "10px",
          top: `${10 + 60 * this.metadata.xmp.Regions.indexOf(region)}px`,
          width: "100px",
          height: "100px",
          border: "2px solid var(--accent-blue)",
          background: "rgba(0, 0, 255, 0.2)",
        };
      }

      const imgWidth = this.imageDimensions.width;
      const imgHeight = this.imageDimensions.height;
      const pixelWidth = W * imgWidth;
      const pixelHeight = H * imgHeight;
      const pixelX = X * imgWidth - pixelWidth / 2; // X is center
      const pixelY = Y * imgHeight - pixelHeight / 2; // Y is center
      
      let borderColor, backgroundColor;
      
      // Handle NameAssignType - even if it's empty or null
      const nameAssignType = region.NameAssignType || '';
      if (nameAssignType === 'auto') {
        borderColor = 'var(--accent-yellow)';
        backgroundColor = 'rgba(255, 255, 0, 0.2)';
      } else if (nameAssignType === 'manual') {
        borderColor = 'var(--accent-green)';
        backgroundColor = 'rgba(66, 185, 131, 0.2)';
      } else {
        borderColor = 'var(--accent-blue)';
        backgroundColor = 'rgba(0, 0, 255, 0.2)';
      }
      
      console.log("Face box for", region.Name || "Unnamed", {
        X, Y, W, H,
        pixelX, pixelY, pixelWidth, pixelHeight,
        imageWidth: imgWidth, imageHeight: imgHeight,
        NameAssignType: nameAssignType,
        borderColor, backgroundColor
      });
      
      return {
        left: `${pixelX}px`,
        top: `${pixelY}px`,
        width: `${pixelWidth}px`,
        height: `${pixelHeight}px`,
        border: `2px solid ${borderColor}`,
        background: backgroundColor,
      };
    },
	
	
    parsePhotoshopInstructions() {
      if (this.metadata && this.metadata.xmp) {
      // Photoshop instructions field may be nested, handle safely
      this.photoshopInstructions = this.metadata.xmp['photoshop:Instructions'] || '';
    }
    },
  
    openInstructionsModal() {
      this.showInstructionsModal = true;
    },
  
  async saveInstructions() {
    try {
      await filesApi.updateXMPInstructions(
        state.req.source,
        state.req.path,
        this.photoshopInstructions
      );
      this.showInstructionsModal = false;
    } catch (err) {
      console.error(err);
    }
  },
	
    
    // Zoom and pan methods
    handleWheel(event) {
      if (this.isMetadataVisible || this.previewType !== 'image') return;
      
      event.preventDefault();
      
      // Calculate zoom factor
      const zoomFactor = event.deltaY > 0 ? 0.9 : 1.1;
      const newZoom = Math.max(0.1, Math.min(5, this.zoomLevel * zoomFactor));
      
      // Calculate mouse position relative to image
      const rect = this.$refs.image.getBoundingClientRect();
      const mouseX = event.clientX - rect.left;
      const mouseY = event.clientY - rect.top;
      
      // Calculate the position relative to the image center
      const imageCenterX = rect.width / 2;
      const imageCenterY = rect.height / 2;
      
      // Adjust pan position to zoom toward mouse position
      const zoomChange = newZoom - this.zoomLevel;
      this.panPosition.x -= (mouseX - imageCenterX - this.panPosition.x) * (zoomChange / this.zoomLevel);
      this.panPosition.y -= (mouseY - imageCenterY - this.panPosition.y) * (zoomChange / this.zoomLevel);
      
      this.zoomLevel = newZoom;
    },
    
    startPan(event) {
      if (this.isMetadataVisible || this.previewType !== 'image') return;
      
      this.isPanning = true;
      this.lastPanPosition = { x: event.clientX, y: event.clientY };
    },
    
    handlePan(event) {
      if (!this.isPanning || this.isMetadataVisible || this.previewType !== 'image') return;
      
      const deltaX = event.clientX - this.lastPanPosition.x;
      const deltaY = event.clientY - this.lastPanPosition.y;
      
      this.panPosition.x += deltaX;
      this.panPosition.y += deltaY;
      
      this.lastPanPosition = { x: event.clientX, y: event.clientY };
    },
    
    stopPan() {
      this.isPanning = false;
    },
    
    handleTouchStart(event) {
      if (this.isMetadataVisible || this.previewType !== 'image') return;
      
      if (event.touches.length === 1) {
        // Single touch - start panning
        this.isTouching = true;
        this.lastPanPosition = { x: event.touches[0].clientX, y: event.touches[0].clientY };
      } else if (event.touches.length === 2) {
        // Two touches - start pinch to zoom
        this.isTouching = true;
        this.touchStartZoom = this.zoomLevel;
        this.touchStartDistance = this.getTouchDistance(event.touches);
      }
    },
    
    handleTouchMove(event) {
      if (!this.isTouching || this.isMetadataVisible || this.previewType !== 'image') return;
      
      if (event.touches.length === 1) {
        // Single touch - panning
        const deltaX = event.touches[0].clientX - this.lastPanPosition.x;
        const deltaY = event.touches[0].clientY - this.lastPanPosition.y;
        
        this.panPosition.x += deltaX;
        this.panPosition.y += deltaY;
        
        this.lastPanPosition = { x: event.touches[0].clientX, y: event.touches[0].clientY };
      } else if (event.touches.length === 2) {
        // Two touches - pinch to zoom
        event.preventDefault();
        
        const currentDistance = this.getTouchDistance(event.touches);
        const zoomFactor = currentDistance / this.touchStartDistance;
        this.zoomLevel = Math.max(0.1, Math.min(5, this.touchStartZoom * zoomFactor));
        
        // Calculate midpoint for centering zoom
        const midX = (event.touches[0].clientX + event.touches[1].clientX) / 2;
        const midY = (event.touches[0].clientY + event.touches[1].clientY) / 2;
        
        const rect = this.$refs.image.getBoundingClientRect();
        const imageCenterX = rect.width / 2;
        const imageCenterY = rect.height / 2;
        
        // Adjust pan position based on zoom
        const zoomChange = this.zoomLevel - this.touchStartZoom;
        this.panPosition.x -= (midX - rect.left - imageCenterX - this.panPosition.x) * (zoomChange / this.touchStartZoom);
        this.panPosition.y -= (midY - rect.top - imageCenterY - this.panPosition.y) * (zoomChange / this.touchStartZoom);
      }
    },
    
    handleTouchEnd(event) {
      this.isTouching = false;
    },
    
    getTouchDistance(touches) {
      const dx = touches[0].clientX - touches[1].clientX;
      const dy = touches[0].clientY - touches[1].clientY;
      return Math.sqrt(dx * dx + dy * dy);
    },
    
    resetZoomAndPan() {
      this.zoomLevel = 1;
      this.panPosition = { x: 0, y: 0 };
    },
    
    startResize(event) {
      this.isResizing = true;
      document.body.style.userSelect = "none";
      document.body.style.cursor = "ns-resize";
      if (event.type === "touchstart") {
        event.preventDefault();
      }
    },
    resizeMetadata(event) {
      if (!this.isResizing) return;
      const clientY = event.touches ? event.touches[0].clientY : event.clientY;
      const newHeight = window.innerHeight - clientY;
      const minHeight = 100;
      const maxHeight = window.innerHeight * 0.8;
      this.metadataHeight = Math.min(Math.max(newHeight, minHeight), maxHeight);
      this.dimensionRetryCount = 0;
      this.$nextTick(() => this.updateImageDimensions());
    },
    stopResize() {
      this.isResizing = false;
      document.body.style.userSelect = "";
      document.body.style.cursor = "";
    },
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
        const resp = await filesApi.fetchFiles(subtitleFile, true);
        let vttContent = resp.content;
        vttContent = convertToVTT(ext, resp.content);
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
        case "Escape":
        case "Backspace":
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
      this.dimensionRetryCount = 0;
      this.$nextTick(() => this.updateImageDimensions());
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
  
  // Trigger header visibility
  this.triggerHeaderVisibility();
}, 100),
    
    close() {
      mutations.replaceRequest({});
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
  --accent-green: #42b983;
  --accent-yellow: #ffff00;
  --accent-blue: #0000ff;
}

#previewer {
  display: flex;
  flex-direction: column;
  height: 100vh;
  width: 100%;
  background: var(--dark-theme-1);
  position: absolute;
  top: 0;
  left: 0;
  overflow-y: auto;
}

.preview {
  width: 100%;
  flex-grow: 1;
  flex-shrink: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: visible;

  &.full-height {
    max-height: 100vh;
  }

  .image-container {
    position: relative;
    display: flex;
    justify-content: center;
    align-items: center;
    width: 100%;
    height: 100%;
    overflow: visible;
  }

  .preview-image {
    max-width: 100%;
    max-height: 100%;
    object-fit: contain;
    display: block;
    transition: transform 0.1s ease-out;
  }

  .face-overlay {
    position: absolute;
    pointer-events: none;
    background: rgba(0, 0, 0, 0.1);
  }

  .face-box {
    position: absolute;
    box-sizing: border-box;
    min-width: 50px;
    min-height: 50px;
  }

  .face-label {
    position: absolute;
    top: -28px;
    left: 0;
    background: rgba(0, 0, 0, 0.8);
    color: #fff;
    padding: 4px 8px;
    font-size: 14px;
    white-space: nowrap;
    border-radius: 3px;
    z-index: 10;
  }
}

.inline-modal {
  position: relative;      /* instead of fixed */
  margin-top: 1rem;        /* spacing from above content */
  padding: 1rem;
  border: 1px solid #ccc;
  background: #fff;
  border-radius: 4px;
  box-shadow: 0 2px 8px rgba(0,0,0,0.15);
  width: 100%;
  max-width: 100%;
  z-index: 10;             /* relative stacking within container */
}

.inline-modal textarea {
  width: 100%;             // Fill modal width
  box-sizing: border-box;  // Include padding in width
}

.metadata-container {
  position: relative;
  width: 100%;
  min-height: 100px;
  max-height: 80vh;
  background: var(--dark-theme-1);
  color: #fff;
  border-top: 1px solid var(--dark-theme-2);
  padding: 1rem;
  overflow-y: auto;
  flex-shrink: 0;
}

.resize-handle {
  position: absolute;
  top: -10px;
  left: 0;
  width: 100%;
  height: 10px;
  cursor: ns-resize;
  background: transparent;
}

.tabs-header {
  display: flex;
  justify-content: center;
  margin-bottom: 1rem;
  border-bottom: 1px solid var(--dark-theme-2);
  padding-bottom: 0.5rem;

  button {
    background: none;
    border: none;
    color: #fff;
    padding: 0.5rem 1rem;
    margin: 0 0.25rem;
    cursor: pointer;
    border-radius: 4px;
    transition: background-color 0.2s;

    &:hover {
      background: rgba(255, 255, 255, 0.1);
    }

    &.active {
      background: var(--accent-green);
      color: #000;
    }

    &.tab-empty {
      background: #ffcccc; // light red for empty tabs
      color: #000;
    }

    // Ensure active overrides empty
    &.active.tab-empty {
      background: var(--accent-green);
      color: #000;
    }
  }
}

.tabs-content {
  .tab-pane {
    position:relative;
	
    h3 {
      margin-top: 0;
      color: var(--accent-green);
    }

    .metadata-table {
      max-height: 300px;
      overflow-y: auto;

      table {
        width: 100%;
        border-collapse: collapse;

        th, td {
          padding: 0.5rem;
          border: 1px solid var(--dark-theme-2);
          text-align: left;
        }

        th {
          background: rgba(255, 255, 255, 0.05);
        }
      }
    }
  }
}

.nav-button {
  position: fixed;
  top: 50%;
  transform: translateY(-50%);
  background: rgba(0, 0, 0, 0.5);
  color: white;
  border: none;
  border-radius: 50%;
  width: 50px;
  height: 50px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  z-index: 100;
  transition: opacity 0.3s;

  &.nav-button-prev {
    left: 20px;
  }

  &.nav-button-next {
    right: 20px;
  }

  &.hidden {
    opacity: 0;
    pointer-events: none;
  }

  &:hover {
    background: rgba(0, 0, 0, 0.8);
  }
}

.info {
  text-align: center;
  color: #fff;

  .title {
    font-size: 1.5rem;
    margin-bottom: 1rem;
  }

  .button {
    margin: 0 0.5rem;
  }
}
</style>