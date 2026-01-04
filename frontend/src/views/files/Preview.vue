<template>
  <div id="previewer" @mousemove="toggleNavigation" @touchstart="toggleNavigation">
    <!-- Preview Section -->
    <div class="preview" :class="{ 'full-height': !isMetadataVisible }" :style="{ maxHeight: previewMaxHeight }">
      <div class="image-container" v-if="previewType == 'image'">
        <div ref="panzoomContent" class="panzoom-content" style="position: relative; display: inline-block;">
            <img 
              ref="image" 
              :src="raw" 
              @load="updateImageDimensions" 
              class="preview-image"
              style="display: block; max-width: 100%; max-height: 100%;"
            >
            <div
              v-if="activeTab === 'xmp' && metadata && metadata.xmp && metadata.xmp.Regions && metadata.xmp.Regions.length > 0 && isMetadataVisible"
              class="face-overlay"
              style="position: absolute; top: 0; left: 0; width: 100%; height: 100%; pointer-events: none;"
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
      <div class="tabs-header" style="display: flex; align-items: center; justify-content: space-between; padding-right: 10px;">
        <div class="tabs-buttons">
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
        </div>
        
        <!-- Toggle Height Button -->
        <button 
          @click="toggleMetadataHeight" 
          class="button button--flat" 
          :title="isMetadataExpanded ? 'Collapse' : 'Expand'"
          style="padding: 0 10px;"
        >
          <i class="material-icons">{{ isMetadataExpanded ? 'expand_more' : 'expand_less' }}</i>
        </button>
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

			<!-- photoshop:Instructions (XMP) Section -->
			<div style="margin-top: 1rem;">
				<strong>Photoshop Instructions:&nbsp;</strong>
				<span v-if="photoshopInstructions">{{ photoshopInstructions }}</span>
				<button 
					v-if="canEditInstructions" 
					@click="openInstructionsModal" 
					class="button button--flat"
				>
				Edit
				</button>
			</div>

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

		<!-- Full-width bottom overlay that covers the metadata-container -->
		<div
			v-if="showInstructionsModal && activeTab === 'iptc'" 
			class="overlay-modal-bottom"
			:style="{ height: overlayHeight + 'px' }"
			@keydown.esc="closeInstructionsModal"
			tabindex="-1"
		>
		<div class="overlay-content" role="dialog" aria-modal="true">
		<h4>Edit Photoshop Instructions</h4>

		<!-- textarea grows to fill space, scrolls internally if content is long -->
		<textarea
			v-model="photoshopInstructions"
			:readonly="!canEditInstructions"
			autofocus
			aria-label="Photoshop instructions editor"
		></textarea>

		<!-- button row always visible at bottom -->
		<div class="button-row">
			<button @click="saveInstructions" class="button button--flat">Save</button>
			<button @click="closeInstructionsModal" class="button button--flat">Cancel</button>
		</div>
		</div>
		</div>
			
			</div> <!--  close IPTC tab-pane -->

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
          <div v-if="gpsCoordinates">
            <div style="display: flex; align-items: center; margin-bottom: 1rem;">
              <p style="margin: 0; margin-right: 10px;">
                <strong>Coordinates:</strong> {{ gpsCoordinates.lat.toFixed(6) }}, {{ gpsCoordinates.lon.toFixed(6) }}
              </p>
              <button 
                @click="copyCoordinates" 
                class="button button--flat" 
                title="Copy coordinates"
                aria-label="Copy coordinates"
                style="padding: 0; min-width: 36px; width: 36px; height: 36px; display: flex; align-items: center; justify-content: center;"
              >
                <i class="material-icons" style="font-size: 18px;">content_copy</i>
              </button>
            </div>

            <div class="map-container">
              <iframe 
                width="100%" 
                height="400" 
                frameborder="0" 
                scrolling="no" 
                marginheight="0" 
                marginwidth="0" 
                :src="`https://www.openstreetmap.org/export/embed.html?bbox=${gpsCoordinates.lon-0.01}%2C${gpsCoordinates.lat-0.01}%2C${gpsCoordinates.lon+0.01}%2C${gpsCoordinates.lat+0.01}&amp;layer=mapnik&amp;marker=${gpsCoordinates.lat}%2C${gpsCoordinates.lon}`"
                style="border: 1px solid black"
              ></iframe>
              <br/>
              <small>
                <a :href="`https://www.openstreetmap.org/?mlat=${gpsCoordinates.lat}&amp;mlon=${gpsCoordinates.lon}#map=16/${gpsCoordinates.lat}/${gpsCoordinates.lon}`" target="_blank">
                  View Larger Map
                </a>
              </small>
            </div>
          </div>
          <p v-else>No GPS data found for this image.</p>
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
import panzoom from "panzoom"; // Import panzoom

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
      panzoomInstance: null, // Store panzoom instance
      isMetadataExpanded: false,
    };
  },
  computed: {
	canEditInstructions() {
		return state.user?.permissions?.modify === true;
	},
	
	overlayHeight() {
		// metadataHeight comes from component data; keep a sensible min so buttons fit
		return Math.max(this.metadataHeight || 200, 200);		
	},  
  
    sidebarShowing() {
      return getters.isSidebarVisible();
    },
    previewType() {
      const type = getters.previewType();
      //console.log("Preview type:", type);
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
      //console.log("Image src URL:", url);
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
      //console.log("isMetadataVisible:", visible);
      return visible;
    },
    previewMaxHeight() {
      return this.isMetadataVisible ? `calc(100vh - ${this.metadataHeight}px)` : '100vh';
    },
    gpsCoordinates() {
      if (!this.metadata || !this.metadata.exif) return null;

      const exif = this.metadata.exif;
      const lat = exif.GPSLatitude;
      const latRef = exif.GPSLatitudeRef;
      const lon = exif.GPSLongitude;
      const lonRef = exif.GPSLongitudeRef;

      // Handle simple decimal case (if backend converts it already)
      if (typeof lat === 'number' && typeof lon === 'number') {
         return { lat, lon };
      }

      // Handle Array DMS format
      if (Array.isArray(lat) && Array.isArray(lon) && latRef && lonRef) {
        const convertDMS = (dms, ref) => {
          let degrees = dms[0];
          let minutes = dms[1];
          let seconds = dms[2];
          
          let dd = degrees + minutes / 60 + seconds / 3600;

          if (ref === "S" || ref === "W") {
            dd = dd * -1;
          }
          return dd;
        };

        return {
          lat: convertDMS(lat, latRef),
          lon: convertDMS(lon, lonRef),
        };
      }
      
      // Handle string format: "40 deg 42' 46.00" N" or "83 deg 3' 57.60" West"
      // Relaxed Regex: allows optional seconds, handles "West" full word
      // Handle string format: "40 deg 42' 46.00" N" or "83 deg 3' 57.60" West"
      // Relaxed Regex: allows optional seconds, handles "West" full word, optional direction char at end
      const dmsRegex = /([\d\.]+)\s*deg\s*([\d\.]+)'?\s*([\d\.]*)"?\s*([NESW])?/i;
      
      const parseDmsString = (val, externalRef) => {
        if (typeof val !== 'string') return null;
        const match = val.match(dmsRegex);
        
        let dd = 0;
        let refFromRegex = '';

        if (match) {
            let d = parseFloat(match[1]);
            let m = parseFloat(match[2]);
            let s = match[3] ? parseFloat(match[3]) : 0;
            refFromRegex = match[4] ? match[4].toUpperCase() : ''; 
            
            dd = d + m / 60 + s / 3600;
        } else {
             // Fallback: try to just extract numbers if regex failed
             // e.g. "33.123" string
             const simpleFloat = parseFloat(val);
             if (!isNaN(simpleFloat)) {
                 dd = simpleFloat;
             } else {
                 return null;
             }
        }

        // Determine direction:
        // Priority: 1. Regex capture, 2. External Ref variable, 3. String content search
        const isWest = (refFromRegex === 'W') 
                    || (externalRef === 'W' || externalRef === 'West') 
                    || val.toUpperCase().includes("WEST");
                    
        const isSouth = (refFromRegex === 'S') 
                     || (externalRef === 'S' || externalRef === 'South') 
                     || val.toUpperCase().includes("SOUTH");

        if (isWest || isSouth) {
            dd = -Math.abs(dd);
        }

        return dd;
      };

      // Try parsing both as strings, passing the external refs
      const latParsed = parseDmsString(lat, latRef);
      const lonParsed = parseDmsString(lon, lonRef);

      if (latParsed !== null && lonParsed !== null) {
          return { lat: latParsed, lon: lonParsed };
      }

      // Handle simple stringified number "40.123" without DMS formatting
      const latNum = parseFloat(lat);
      const lonNum = parseFloat(lon);
      if (!isNaN(latNum) && !isNaN(lonNum) && (typeof lat === 'string' || typeof lat === 'number')) {
         // Check refs
         let finalLat = latNum;
         let finalLon = lonNum;
         
         const latStr = String(lat).toUpperCase();
         const lonStr = String(lon).toUpperCase();

         if (latRef === 'S' || latRef === 'South' || latStr.includes("S")) finalLat = -Math.abs(latNum);
         if (lonRef === 'W' || lonRef === 'West' || lonStr.includes("W") || lonStr.includes("WEST")) finalLon = -Math.abs(lonNum);
         
         return { lat: finalLat, lon: finalLon };
      }
      
      return null;
    },
  },
  watch: {
    async req() {
      if (!getters.isLoggedIn()) {
        return;
      }
      // Reset panzoom on new file
      this.disposePanzoom();
      
      this.dimensionRetryCount = 0;
      await this.updatePreview();
      this.toggleNavigation();
      await this.fetchMetadata();
      await this.updateImageDimensions();
      this.$nextTick(() => {
        const availableTabNames = this.availableTabs.map(tab => tab.name);
        //console.log("Validating activeTab:", this.activeTab, "Available:", availableTabNames);
        if (!availableTabNames.includes(this.activeTab)) {
          this.activeTab = "details";
        //  console.log("Reset activeTab to 'details' as current tab is not available");
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
        // Reset panzoom when metadata becomes visible
        if (this.panzoomInstance) {
          // You might choose to pause or dispose here
          // For now, let's reset transform
          this.panzoomInstance.moveTo(0, 0);
          this.panzoomInstance.zoomAbs(0, 0, 1);
        }
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
  },
  beforeUnmount() {
    window.removeEventListener("keydown", this.key);
    document.removeEventListener("mousemove", this.resizeMetadata);
    document.removeEventListener("mouseup", this.stopResize);
    document.removeEventListener("touchmove", this.resizeMetadata);
    document.removeEventListener("touchend", this.stopResize);
    window.removeEventListener("resize", this.updateImageDimensions);
    
    this.disposePanzoom();
  },
  methods: {
    toggleMetadataHeight() {
        this.isMetadataExpanded = !this.isMetadataExpanded;
        this.metadataHeight = this.isMetadataExpanded ? 600 : 300;
    },

    disposePanzoom() {
      if (this.panzoomInstance) {
        this.panzoomInstance.dispose();
        this.panzoomInstance = null;
      }
    },
    selectTab(tabName) {
      this.activeTab = tabName;
    },

	closeInstructionsModal() {
		this.showInstructionsModal = false;
	},	
	
    async fetchMetadata() {
      this.metadata = null;
      if (this.previewType !== "image") {
        console.log("Not an image, skipping metadata fetch");
        return;
      }
      try {
        const res = await filesApi.fetchMetadata(state.req.source, state.req.path);
        //console.log("Full API response:", res);
        this.metadata = res.data || res;
		if (!this.metadata.xmp) {
			this.metadata.xmp = {};
		}
		this.parsePhotoshopInstructions(); // <-- parse instructions here
        //console.log("Assigned metadata:", this.metadata);
      } catch (error) {
        console.error("Failed to fetch metadata:", error);
        this.metadata = { exif: {}, iptc: {}, xmp: {} };
      }
    },
    async updateImageDimensions() {
      if (this.previewType !== "image" || !this.$refs.image) {
        //console.log("No image or ref, skipping dimension update");
        return;
      }
      await this.$nextTick();
      const imgElement = this.$refs.image;
      if (!imgElement) {
        console.error("No image element found");
        if (this.dimensionRetryCount < 5) {
          this.dimensionRetryCount++;
          //console.log(`Retrying dimension update (${this.dimensionRetryCount}/5)`);
          setTimeout(() => this.updateImageDimensions(), 500);
        }
        return;
      }
      //console.log("Found image element:", imgElement.tagName, imgElement);
      if (imgElement.complete || imgElement.readyState === 4) {
        this.setupImage(imgElement);
      } else {
        console.log("Image not loaded, waiting for load event");
        imgElement.addEventListener(
          "load",
          () => {
           this.setupImage(imgElement);
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
    setupImage(imgElement) {
        const rect = imgElement.getBoundingClientRect();
        const width = rect.width;
        const height = rect.height;
        this.imageDimensions = { width, height };
        this.imageOffset = { top: rect.top, left: rect.left };
        
        // Initialize panzoom here once image is laid out
        this.initPanzoom();
    },
	
    copyCoordinates() {
        if (!this.gpsCoordinates) return;
        const text = `${this.gpsCoordinates.lat.toFixed(6)}, ${this.gpsCoordinates.lon.toFixed(6)}`;
        navigator.clipboard.writeText(text).then(() => {
            this.$showSuccess(this.$t('Copied to clipboard'));
        }, (err) => {
            console.error('Async: Could not copy text: ', err);
        });
    },

		openInstructionsModal() {
		if (!this.canEditInstructions) {
			console.warn("User not allowed to edit instructions");
				return;
			}
		this.showInstructionsModal = true;
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
          region.ALGArea.H === undefined) {
        return {
          left: "10px",
          top: "10px",
          width: "100px",
          height: "100px",
          border: "2px solid var(--accent-blue)",
          background: "rgba(0, 0, 255, 0.2)",
          position: "absolute"
        };
      }

      const { X, Y, W, H } = region.ALGArea;
      
      // Calculate percentages based on center X/Y
      // Left = (CenterX - Width/2) * 100%
      // Top = (CenterY - Height/2) * 100%
      // Width = W * 100%
      // Height = H * 100%
      
      const left = (X - W / 2) * 100;
      const top = (Y - H / 2) * 100;
      const width = W * 100;
      const height = H * 100;

      return {
          left: `${left}%`,
          top: `${top}%`,
          width: `${width}%`,
          height: `${height}%`,
          border: "2px solid yellow",
          boxShadow: "0 0 4px rgba(0,0,0,0.5)",
          position: "absolute",
          pointerEvents: "auto" // Allow clicking the box itself if needed later
      };
    },
    
    // ... helper ...
    initPanzoom() {
      // Target the wrapper content instead of the image directy
      if (this.previewType !== 'image' || !this.$refs.panzoomContent) return;
      
      this.disposePanzoom(); 
      
      this.panzoomInstance = panzoom(this.$refs.panzoomContent, {
        maxZoom: 5,
        minZoom: 0.1,
        bounds: true,
        boundsPadding: 0.1,
        autocenter: true, 
        onTouch: function() {
           return true; 
        }
      });
    },
      

	
	
    parsePhotoshopInstructions() {
  const m = this.metadata || {};
  const x = (m.xmp  || {});
  const i = (m.iptc || {});

  // Common XMP variants
  const fromXmp =
    x['photoshop:Instructions'] ??
    x['Photoshop:Instructions'] ??
    x['Instructions'] ??
    this.findInstructionDeep(x);  // catch any nested/key-casing variants

  // IPTC/IIM (2:40) + common shapes from extractors
  const iptcApp = i.ApplicationRecord || i['Application Record'] || {};
  const fromIptc =
    i['SpecialInstructions'] ??
    i['Special Instructions'] ??
    i['Instructions'] ??
    i['2#040'] ??                 // some libs expose IIM tags like this
    iptcApp['SpecialInstructions'] ??
    iptcApp['Special Instructions'] ??
    this.findInstructionDeep(i);  // last-resort deep search

  const val = [fromXmp, fromIptc].find(v => typeof v === 'string' && v.trim());
  this.photoshopInstructions = val || '';

  // Optional: quick debug to see what keys you actually have
  if (!this.photoshopInstructions) {
    //console.log('No Instructions found. XMP keys:', Object.keys(x));
    //console.log('No Instructions found. IPTC keys:', Object.keys(i));
  }
},

// Recursively search objects/arrays for any key that looks like "instructions"
findInstructionDeep(obj) {
  if (!obj || typeof obj !== 'object') return null;
  for (const [k, v] of Object.entries(obj)) {
    const keyMatch = /(^|[^a-z])instructions?$/i.test(k) || /special.*instructions/i.test(k);
    if (keyMatch) {
      if (typeof v === 'string' && v.trim()) return v;
      if (v && typeof v === 'object' && typeof v.value === 'string' && v.value.trim()) return v.value;
      if (Array.isArray(v)) {
        const s = v.find(x => typeof x === 'string' && x.trim());
        if (s) return s;
      }
    }
    const nested = this.findInstructionDeep(v);
    if (nested) return nested;
  }
  return null;
},
  
async saveInstructions() {
  if (!this.canEditInstructions) return; // prevent saving if not an editor
  try {
    const metadataPath = this.metadata?.path || state.req.path;
    await filesApi.updateXMPInstructions(
      state.req.source,
      metadataPath,
      this.photoshopInstructions
    );
    this.showInstructionsModal = false;
    //console.log("Photoshop instructions saved to:", metadataPath);
  } catch (err) {
    console.error("Failed to save Photoshop instructions:", err);
  }
},
	
    
    // Zoom and pan methods - REMOVED (Replaced by panzoom library)

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


.overlay-modal {
  position: fixed;           /* stays fixed on screen */
  top: 0;
  left: 0;
  width: 100vw;              /* full screen width */
  height: 100vh;             /* full screen height */
  background: rgba(0, 0, 0, 0.6);  /* dimmed background */
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 2000;             /* above everything else */
}

.overlay-content h4 {
  margin: 0 0 0.5rem;
}


/* overlay sits fixed at the bottom and spans full viewport width */
.overlay-modal-bottom {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  width: 100vw;              /* full width of the viewport */
  display: flex;
  justify-content: center;
  align-items: stretch;
  background: rgba(0, 0, 0, 0.45); /* dim background */
  z-index: 1500;             /* above metadata-container */
  box-sizing: border-box;
}

/* content fills the overlay and uses the full width of the screen */
.overlay-content {
  width: 100vw;              /* user requested full-screen width */
  max-width: none;
  height: 100%;              /* match overlay height (metadataHeight) */
  background: #fff;
  padding: 1rem;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
}

/* textarea expands to available vertical space and scrolls internally */
.overlay-content textarea {
  flex: 1;
  min-height: 0;             /* important for flex + overflow in browsers */
  width: 100%;
  resize: none;
  box-sizing: border-box;
  padding: 0.5rem;
  font-family: inherit;
  font-size: 14px;
  overflow: auto;
}

/* button row pinned to the bottom of overlay-content (always visible) */
.button-row {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  margin-top: 0.5rem;
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
    display: block; /* Removed flex centering */
    width: 100%;
    height: 100%;
    overflow: hidden; /* Ensure panzoom doesn't cause scrollbars on parent */
    background-color: black; /* Visual background */
  }

  .preview-image {
    max-width: 100%;
    max-height: 100%;
    object-fit: contain;
    display: block;
    transform-origin: 0 0; /* Important for panzoom */
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