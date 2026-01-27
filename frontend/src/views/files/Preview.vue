<template>
  <div 
    id="previewer" 
    @mousemove="toggleNavigation" 
    @touchstart.capture="handleTouchStart" 
    @touchend.capture="handleTouchEnd"
    @click="handlePreviewClick"
  >
    <!-- Preview Section -->
    <div class="preview" :class="{ 'full-height': !isMetadataVisible }" :style="{ maxHeight: previewMaxHeight }">
      
      <!-- Media Content Wrapper: Contains the actual viewer elements to preserve v-if chain -->
      <div class="media-wrapper" style="flex: 1; overflow: hidden; position: relative; width: 100%;">
          
          <div class="image-container" v-if="previewType == 'image'" :style="{ height: showInstructionsModal && !isMobile ? '100%' : '100%' }">
            <div ref="panzoomContent" class="panzoom-content" style="position: relative; display: inline-block; transform-origin: 0 0;">
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
                    <span class="face-label" :style="{ fontSize: faceFontSize + 'px', top: -faceFontSize * 1.5 + 'px' }">{{ region.Name || 'Unnamed' }}</span>
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
              <button
                @click="shareImage"
                class="button button--flat"
                :title="$t('buttons.share')"
                :style="{ opacity: canShare ? 1 : 0.5 }"
              >
                <div>
                  <i class="material-icons">share</i>{{ $t("buttons.share") }}
                </div>
              </button>
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
      <!-- End Media Wrapper -->

       <!-- Persistent Photoshop Instructions Editor (Split View) -->
       <!-- Now this is a sibling to media-wrapper, breaking the v-if interaction but thats ok because media-wrapper is the flex item -->
       <div v-if="showInstructionsModal" class="instructions-split-pane" :style="{ height: !isMobile ? '33%' : 'auto', flex: !isMobile ? '0 0 33%' : '0 0 auto' }">
          <div class="pane-header">
             <h4 title="Photoshop Instructions">Image Notes</h4>
             <button @click="closeInstructionsModal" class="close-icon"><i class="material-icons">close</i></button>
          </div>
          <textarea
            v-model="photoshopInstructions"
            :readonly="!canEditInstructions"
            placeholder="Add Notes to this Image"
            @keydown.stop
          ></textarea>
          <div class="button-row">
            <button @click="saveInstructions" class="button button--flat">Save</button>
            <button @click="closeInstructionsModal" class="button button--flat">Close</button>
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
            <li><strong>Size:</strong> {{ (req.size / 1048576).toFixed(2) }} MB</li>
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
			<div style="margin-top: 1rem; display: flex; flex-direction: column;">
                <div style="display: flex; align-items: flex-start;">
                    <i 
                        class="material-icons" 
                        style="font-size: 16px; margin-right: 8px; cursor: pointer; color: #aaa; margin-top: 3px;"
                        title="Photoshop Instructions"
                        @click="showTagPopup('Photoshop Instructions')"
                    >
                        info
                    </i>
				    <span v-if="photoshopInstructions" style="font-size: 0.85rem; flex: 1;">{{ photoshopInstructions }}</span>
                </div>
				<button 
					v-if="canEditInstructions" 
					@click="openInstructionsModal" 
					class="button button--flat"
                    style="align-self: flex-start; margin-left: 24px; margin-top: 0.5rem;"
				>
				Edit Notes
				</button>
			</div>

          <div v-if="metadata && Object.keys(metadata.iptc).length > 0" class="metadata-table">
            <table>
              <tbody>
                <tr v-for="(value, key) in metadata.iptc" :key="key">
                  <td style="display: flex; align-items: top;">
                      <i 
                        class="material-icons" 
                        style="font-size: 16px; margin-right: 8px; cursor: pointer; color: #aaa; margin-top: 2px;"
                        :title="key"
                        @click="showTagPopup(key)"
                      >
                        info
                      </i>
                      <span style="flex: 1;">{{ value }}</span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <p v-else-if="metadata && Object.keys(metadata.iptc).length === 0">No IPTC data found for this file.</p>
          <p v-else>Loading IPTC metadata...</p>

		<!-- REMOVED overlay-modal-bottom from here -->	
		</div> <!--  close IPTC tab-pane -->

        <div v-if="activeTab === 'xmp'" class="tab-pane">
          <h3>XMP Metadata</h3>
          
          <!-- Font Size Control for Face Boxes -->
          <div v-if="metadata.xmp && metadata.xmp.Regions && metadata.xmp.Regions.length > 0" style="margin-bottom: 1rem; padding: 10px; background: rgba(255,255,255,0.05); border-radius: 4px;">
             <label for="fontSizeRange" style="display: block; margin-bottom: 5px;">Face Label Size: {{ faceFontSize }}px</label>
             <input 
               type="range" 
               id="fontSizeRange" 
               min="10" 
               max="60" 
               step="1" 
               v-model.number="faceFontSize"
               style="width: 100%; cursor: pointer;"
             >
          </div>

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

          <div v-if="activeTab === 'map'" class="tab-pane" style="height: 100%; display: flex; flex-direction: column;">

            
            <div v-if="isEditingCoordinates" class="coordinate-editor" style="margin-bottom: 0px; padding: 5px; background: rgba(0,0,0,0.05); border-radius: 4px; display: flex; flex-direction: column; height: 100%; overflow: hidden;">
               
               <div style="flex: 0 0 auto;">
                   <!-- Inputs Line -->
                    <div style="display: flex; gap: 5px; align-items: center; margin-bottom: 5px;">
                         <div style="display: flex; gap: 5px; align-items: center; flex: 1; flex-wrap: wrap;">
                             <label style="font-size: 0.8em; margin: 0; white-space: nowrap;">Lat:</label>
                             <input type="number" step="any" v-model.number="editLat" @input="handleCoordInput($event, 'editLat')" @keydown.stop class="input input--block" style="padding: 2px 5px; height: 28px; font-size: 0.9em; min-width: 80px; flex: 1;">
                             <label style="font-size: 0.8em; margin: 0; white-space: nowrap;">Lon:</label>
                             <input type="number" step="any" v-model.number="editLon" @input="handleCoordInput($event, 'editLon')" @keydown.stop class="input input--block" style="padding: 2px 5px; height: 28px; font-size: 0.9em; min-width: 80px; flex: 1;">
                             
                             <div v-if="matchedLocationName" style="background: #2196f3; color: white; border-radius: 4px; padding: 2px 6px; font-size: 0.75em; white-space: nowrap; display: flex; align-items: center; margin-left: auto;">
                                <i class="material-icons" style="font-size: 14px; margin-right: 2px;">bookmark</i>
                                {{ matchedLocationName }}
                             </div>
                         </div>
                    </div>

                   <!-- Buttons Line -->
                   <div style="display: flex; gap: 5px; justify-content: flex-end; margin-bottom: 5px;">
                        <div style="display: flex; gap: 3px;">
                             <button 
                                @click="saveCoordinates" 
                                class="button button--flat"
                                :class="{'button--primary': hasUnsavedCoordinates}"
                                :style="hasUnsavedCoordinates ? 'background-color: #2196f3; color: white;' : ''"
                                style="padding: 2px 6px; min-height: 28px; line-height: 1; font-size: 0.9em;"
                             >Save to Image</button>
                            <button @click="cancelEditCoordinates" class="button button--flat" style="opacity: 0.7; padding: 2px 6px; min-height: 28px; line-height: 1; font-size: 0.9em;">Cancel</button>
                             <button @click="saveLocationToProfile" class="button button--flat" title="Save to My Locations" style="padding: 2px 6px; min-height: 28px;">
                                <i class="material-icons" style="font-size: 16px;">bookmark_add</i>
                            </button>
                            <button 
                                v-if="gpsCoordinates"
                                @click="handleClearLocation" 
                                class="button button--flat button--warn" 
                                title="Clear Location Data"
                                style="color: #f44336; padding: 2px 6px; min-height: 28px;"
                            >
                                <i class="material-icons" style="font-size: 16px;">location_off</i>
                            </button>
                       </div>
                   </div>

                    <!-- No Data Warning -->
                    <div v-if="!gpsCoordinates" style="padding: 5px; margin-bottom: 5px; background: rgba(255, 152, 0, 0.1); border-left: 3px solid #ff9800; font-size: 0.8em; color: #e65100;">
                        No location data embedded in this file.
                    </div>

                    <!-- Matched Location Info -->
                    <div v-if="gpsCoordinates && gpsMatchedLocationName" style="padding: 5px; margin-bottom: 5px; background: rgba(33, 150, 243, 0.1); border-left: 3px solid #2196f3; font-size: 0.8em; color: #1976d2;">
                        Location matches saved: <strong>{{ gpsMatchedLocationName }}</strong>
                    </div>

                    <div style="display: flex; gap: 5px; border-bottom: 1px solid rgba(0,0,0,0.1); margin-bottom: 5px; padding-bottom: 5px; align-items: center;">
                        <button @click="editTab = 'location'" class="button button--flat" :style="editTab === 'location' ? 'font-weight: bold; border-bottom: 2px solid #2196f3;' : ''" style="padding: 2px 8px; min-height: 24px; line-height: 1; font-size: 0.9em;">Map Search</button>
                        <button @click="editTab = 'locations'" class="button button--flat" :style="editTab === 'locations' ? 'font-weight: bold; border-bottom: 2px solid #2196f3;' : ''" style="padding: 2px 8px; min-height: 24px; line-height: 1; font-size: 0.9em;">My Locations</button>
                    </div>
                </div>

                <div v-show="editTab === 'location'" style="flex: 1; min-height: 0; display: flex; flex-direction: column;">
                     <!-- Search Box Moved Here -->
                     <div style="display: flex; gap: 5px; margin-bottom: 5px;">
                        <input type="text" v-model="searchQuery" @keyup.enter="searchLocation" @keydown.stop placeholder="Search..." class="input input--block" style="flex: 1; padding: 2px 5px; height: 28px; font-size: 0.9em;">
                        <button @click="searchLocation" class="button button--flat" title="Search" style="padding: 0 8px; min-height: 28px;"><i class="material-icons" style="font-size: 18px;">search</i></button>
                     </div>
                     <!-- Explicit height to fix 0px issue: flex-none with static height -->
                     <div class="map-container" style="flex: 0 0 300px; width: 100%; position: relative; background: #f0f0f0; border: 1px solid #ccc; height: 300px;">
                        <div id="leafletMap" ref="leafletMap" style="width: 100%; height: 100%; z-index: 1;"></div>
                     </div>
                    <div style="font-size: 0.7em; color: #888; margin-top: 2px; display: flex; justify-content: space-between;">
                        <span>{{ mapStatus }}</span>
                        <span style="opacity: 0.7;">Click map to set.</span>
                    </div>
               </div>
               
               <div v-if="editTab === 'locations'" style="flex: 1; overflow-y: auto;">
                   <div v-if="savedLocations.length === 0" style="padding: 10px; opacity: 0.6; font-style: italic; font-size: 0.9em;">No saved locations.</div>
                   <div v-for="(loc, idx) in savedLocations" :key="idx" style="display: flex; justify-content: space-between; align-items: center; padding: 5px 10px; border-bottom: 1px solid #eee; cursor: pointer;" @click="loadSavedLocationIdx(idx)">
                       <span style="font-weight: bold; color: #2196f3; font-size: 0.9em;">{{ loc.name }}</span>
                       <div @click.stop>
                            <button @click="deleteSavedLocationIdx(idx)" class="button button--flat" title="Delete" style="color: #f44336; padding: 2px; min-height: 24px;">
                               <i class="material-icons" style="font-size: 16px;">delete</i>
                            </button>
                       </div>
                   </div>
               </div>
            </div>

            <div v-else-if="gpsCoordinates" style="display: flex; flex-direction: column; height: 100%;">
              <!-- Compact Toolbar -->
              <div style="display: flex; align-items: center; gap: 10px; margin-bottom: 5px; flex-wrap: wrap; font-size: 0.9em; padding: 5px; background: rgba(0,0,0,0.03); border-radius: 4px;">
                  <div style="display: flex; align-items: center; gap: 5px;">
                      <strong style="white-space: nowrap;">Coords:</strong> 
                      <span style="font-family: monospace;">{{ gpsCoordinates.lat.toFixed(5) }}, {{ gpsCoordinates.lon.toFixed(5) }}</span>
                      
                      <div v-if="gpsMatchedLocationName" style="background: #2196f3; color: white; border-radius: 4px; padding: 2px 6px; font-size: 0.75em; white-space: nowrap; display: flex; align-items: center; margin-left: 5px;">
                        <i class="material-icons" style="font-size: 14px; margin-right: 2px;">bookmark</i>
                        {{ gpsMatchedLocationName }}
                     </div>
                  </div>
                  
                  <div style="flex: 1;"></div> <!-- Spacer -->

                  <div style="display: flex; gap: 5px;">
                      <button 
                        @click="copyCoordinates" 
                        class="button button--flat" 
                        title="Copy"
                        style="padding: 2px 6px; min-width: auto; min-height: 26px; line-height: 1;"
                      >
                        <i class="material-icons" style="font-size: 16px;">content_copy</i>
                      </button>
                      
                      <a 
                        :href="'https://www.google.com/maps/search/?api=1&query=' + gpsCoordinates.lat + ',' + gpsCoordinates.lon" 
                        target="_blank" 
                        class="button button--flat" 
                        title="Open Google Maps"
                        style="padding: 2px 6px; min-width: auto; min-height: 26px; line-height: 1; display: flex; align-items: center; text-decoration: none; color: inherit;"
                      >
                        <i class="material-icons" style="font-size: 16px;">map</i>
                      </a>

                      <button 
                        v-if="!gpsMatchedLocationName" 
                        @click="saveCurrentGpsLocation" 
                        class="button button--flat" 
                        title="Add to Saved Locations"
                        style="padding: 2px 6px; min-width: auto; min-height: 26px; line-height: 1;"
                      >
                        <i class="material-icons" style="font-size: 16px;">bookmark_add</i>
                      </button>

                      <button 
                        v-if="canEditCoordinates" 
                        @click="startEditCoordinates" 
                        class="button button--flat" 
                        title="Edit"
                        style="padding: 2px 6px; min-width: auto; min-height: 26px; line-height: 1;"
                      >
                        <i class="material-icons" style="font-size: 16px;">edit</i>
                      </button>
                  </div>
              </div>

              <!-- Map iframe: Flex 1 to fill height (Taller) -->
              <div style="flex: 1; width: 100%; position: relative; min-height: 400px;">
                 <iframe 
                    width="100%" 
                    height="100%" 
                    frameborder="0" 
                    scrolling="no" 
                    marginheight="0" 
                    marginwidth="0" 
                    :src="'https://maps.google.com/maps?q=' + gpsCoordinates.lat + ',' + gpsCoordinates.lon + '&z=15&output=embed'"
                    style="position: absolute; top: 0; left: 0;"
                 ></iframe>
              </div>
            </div>
            
            <!-- Empty State / Add Location -->
            <div v-else style="display: flex; flex-direction: column; align-items: center; justify-content: center; padding: 40px; text-align: center; height: 100%; opacity: 0.6;">
                 <i class="material-icons" style="font-size: 48px; margin-bottom: 10px;">location_off</i>
                 <div style="margin-bottom: 20px;">No location data.</div>
                 <button 
                    v-if="canEditCoordinates" 
                    @click="startEditCoordinates" 
                    class="button button--flat"
                    style="border: 1px solid currentColor; padding: 5px 15px;"
                 >
                    <i class="material-icons">add_location</i> Add Location
                 </button>
            </div>
      </div>
    </div>
  </div>

    <!-- Navigation Buttons -->
    <!-- Navigation Buttons - Hidden as per request for Click/Swipe Nav -->
    <div style="display: none;">
      <button
        @click="prev"
        class="nav-button nav-button-prev"
      >
        <i class="material-icons">chevron_left</i>
      </button>
      <button
        @click="next"
        class="nav-button nav-button-next"
      >
        <i class="material-icons">chevron_right</i>
      </button>
    </div>
    <link rel="prefetch" :href="previousRaw" />
    <link rel="prefetch" :href="nextRaw" />
  </div>
</template>


<script>
import * as filesApi from "@/api/files.js";
import * as usersApi from "@/api/users.js"; // Import usersApi
import { notify } from "@/notify"; // Import notify
import url from "@/utils/url.js";
import throttle from "@/utils/throttle";
import { state, getters, mutations } from "@/store";
import { getFileExtension } from "@/utils/files";
import { convertToVTT } from "@/utils/subtitles";
import { getTypeInfo } from "@/utils/mimetype";
import moment from "moment";
import panzoom from "panzoom"; // Import panzoom
import L from "leaflet";
import "leaflet/dist/leaflet.css";

// Fix Leaflet icon issue
delete L.Icon.Default.prototype._getIconUrl;

L.Icon.Default.mergeOptions({
  iconRetinaUrl: 'https://cdnjs.cloudflare.com/ajax/libs/leaflet/1.7.1/images/marker-icon-2x.png',
  iconUrl: 'https://cdnjs.cloudflare.com/ajax/libs/leaflet/1.7.1/images/marker-icon.png',
  shadowUrl: 'https://cdnjs.cloudflare.com/ajax/libs/leaflet/1.7.1/images/marker-shadow.png',
});


export default {
  name: "preview",
  components: {},
  data() {
    return {
      photoshopInstructions: '',      // Stores the value for display/edit
      originalInstructions: '',       // For dirty check (auto-save)
      // showInstructionsModal: false,   // Moved to global store for persistence
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
      
      // Map Editing
      isEditingCoordinates: false,
      searchQuery: "",
      editLat: 0,
      editLon: 0,
      savedLocations: [], // fetched from user profile
      selectedSavedLocationIdx: null,
      mapStatus: "", // Text status is fine to be reactive
      
      // Edit Settings Tabs
      editTab: 'location', // 'location' | 'locations'

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
      faceFontSize: 24, // Default font size
      metadataCache: {}, // Cache for pre-fetched metadata
    };
  },
  computed: {
    isMobile() {
        return state.isMobile;
    },
    canShare() {
      // Check if basic sharing is supported. Strict file sharing check happens at runtime or we assume support if navigator.share exists.
      // Note: navigator.canShare({ files: ... }) requires the files to check, so we essentially just check for API existence here.
      return typeof navigator.share === 'function' && 
             this.req.type !== 'directory' && 
             this.previewType === 'image';
    },
	canEditInstructions() {
		return state.user?.permissions?.modify === true;
	},
	
	overlayHeight() {
		// metadataHeight comes from component data; keep a sensible min so buttons fit
		return Math.max(this.metadataHeight || 200, 200);		
	},  
  
    showInstructionsModal() {
      return getters.isInstructionsEditMode();
    },
    canEditCoordinates() {
        return state.user?.permissions?.modify === true;
    },
    hasUnsavedCoordinates() {
        if (!this.gpsCoordinates) {
            return this.editLat !== 0 || this.editLon !== 0; // If no original, any change from 0 is "unsaved"
        }
        // Use a small epsilon for float comparison or just direct
        return Math.abs(this.editLat - this.gpsCoordinates.lat) > 0.000001 || 
               Math.abs(this.editLon - this.gpsCoordinates.lon) > 0.000001;
    },

    sidebarShowing() {
      return getters.isSidebarVisible();
    },
    previewType() {
      const type = getters.previewType();
      //console.log("Preview type:", type);
      return type;
    },
    currentUserSavedLocations() {
      return state.user ? state.user.savedLocations : null;
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
    matchedLocationName() {
        if (!this.savedLocations || this.savedLocations.length === 0) return null;
        // Use a small epsilon for float comparison
        const epsilon = 0.00001;
        const match = this.savedLocations.find(loc => 
            Math.abs(loc.lat - this.editLat) < epsilon && 
            Math.abs(loc.lon - this.editLon) < epsilon
        );
        return match ? match.name : null;
    },
    gpsMatchedLocationName() {
        if (!this.gpsCoordinates || !this.savedLocations || this.savedLocations.length === 0) return null;
        const epsilon = 0.00001;
        const match = this.savedLocations.find(loc => 
            Math.abs(loc.lat - this.gpsCoordinates.lat) < epsilon && 
            Math.abs(loc.lon - this.gpsCoordinates.lon) < epsilon
        );
        return match ? match.name : null;
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
      // On desktop (side-by-side), height is not constrained by metadata panel height
      if (window.innerWidth >= 1024) { 
          return '100%';
      }
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
      this.retrieveSavedLocations();
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
         // Only update dimensions if we don't have a panzoom instance, 
         // otherwise toggling the tab might reset the user's zoom/pan.
         if (this.previewType === 'image' && this.panzoomInstance) {
             return;
         }
         this.dimensionRetryCount = 0;
         this.$nextTick(() => this.updateImageDimensions());
      }
      if (newTab === "map") {
         this.$nextTick(() => {
             this.initMap();
         });
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
   	  },
      currentUserSavedLocations: {
        handler() {
            this.retrieveSavedLocations();
        },
        deep: true
      }
  },
  created() {
      // Non-reactive properties for Leaflet
      this.mapInstance = null;
      this.mapMarker = null;
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

	this.retrieveSavedLocations();
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
    if (this.mapInstance) {
        this.mapInstance.remove();
        this.mapInstance = null;
    }
  },
  methods: {
    handlePreviewClick(event) {
        // Desktop & Mobile Click Navigation (Edge Tapping)
        
        // Ignore clicks on interactive elements or metadata pane
        if (event.target.closest('button, a, input, textarea, .metadata-container, .tabs-header, .instructions-split-pane')) return;

        // Use the .preview container for relative coordinates to handle Split Views/Sidebars
        const previewEl = this.$el.querySelector('.preview');
        if (!previewEl) return;

        const rect = previewEl.getBoundingClientRect();
        const x = event.clientX - rect.left; // x relative to the preview container
        const width = rect.width;
        
        // Navigation Logic (30% zones)
        if (x < width * 0.3) {
            this.prev();
        } else if (x > width * 0.7) {
            this.next();
        }
    },
    
    handleTouchStart(event) {
        // Ignore touches on interactive elements or metadata pane
        if (event.target.closest('button, a, input, textarea, .metadata-container, .tabs-header, .instructions-split-pane')) return;

        this.toggleNavigation();
        if (event.touches.length === 1) {
            this.touchStartX = event.touches[0].clientX;
            this.touchStartY = event.touches[0].clientY;
            this.touchStartTime = new Date().getTime();
        }
    },
    
    handleTouchEnd(event) {
        if (!this.touchStartX || !this.touchStartY) return;
        
        const touchEndX = event.changedTouches[0].clientX;
        const touchEndY = event.changedTouches[0].clientY;
        const timeDiff = new Date().getTime() - this.touchStartTime;
        
        const diffX = Math.abs(this.touchStartX - touchEndX);
        const diffY = Math.abs(this.touchStartY - touchEndY);
        
        // Reset
        this.touchStartX = null;
        this.touchStartY = null;

        // TAP DETECTION: Moderate time (<500ms) and moderate movement (<30px)
        if (timeDiff < 500 && diffX < 30 && diffY < 30) {
            // It's a tap!
            
            // Prevent ghost clicks
            if (event.cancelable) event.preventDefault();
            
            // Calculate relative to Preview container
            const previewEl = this.$el.querySelector('.preview');
            if (!previewEl) return;

            const rect = previewEl.getBoundingClientRect();
            const relativeX = touchEndX - rect.left;
            const width = rect.width;
            
            if (relativeX < width * 0.3) {
                this.prev();
            } else if (relativeX > width * 0.7) {
                this.next();
            } else {
                 // Center tap - toggle header visibility
                 this.triggerHeaderVisibility();
            }
        }
    },

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
	

    
    async retrieveSavedLocations() {
         // Should be in user profile.
         // We might need an API to update just the user settings/profile?
         // Assuming state.user has it, but local modification needs to push change back.
         // For now, let's look at state.user.savedLocations
         if (state.user && state.user.savedLocations) {
             this.savedLocations = JSON.parse(JSON.stringify(state.user.savedLocations));
         }
    },
    
    startEditCoordinates() {
        this.retrieveSavedLocations();
        this.isEditingCoordinates = true;
        this.activeTab = 'map'; // Ensure map tab is active
        this.editTab = 'location'; // Ensure location edit tab is active

        if (this.gpsCoordinates) {
            this.editLat = parseFloat(this.gpsCoordinates.lat.toFixed(5));
            this.editLon = parseFloat(this.gpsCoordinates.lon.toFixed(5));
        } else {
            // Default to 0,0 or map center
            this.editLat = 0;
            this.editLon = 0;
        }
        
        // Wait for DOM to render the map container
        this.$nextTick(() => {
             this.initMap(this.editLat, this.editLon); // Initialize map with current/default coords
             this.updateMapMarker(this.editLat, this.editLon);
        });
    },
    cancelEditCoordinates() {
        this.isEditingCoordinates = false;
        // Reset marker to actual GPS if exists
        if (this.gpsCoordinates) {
             this.updateMapMarker(this.gpsCoordinates.lat, this.gpsCoordinates.lon);
        } else if (this.mapMarker) {
            this.mapInstance.removeLayer(this.mapMarker);
            this.mapMarker = null;
        }
    },
    async saveCoordinates() {
        try {
            const req = this.req; // Use computed property directly
            
            // Use the dedicated updateExif function
            // Use editLat/editLon for the NEW values
            await filesApi.updateExif(req.source, req.path, {
                latitude: parseFloat(this.editLat.toFixed(5)),
                longitude: parseFloat(this.editLon.toFixed(5))
            });

            // Use imported 'notify' or if available globally, but import is safer.
            // Looking at other methods, this.$showSuccess might be undefined.
            // Reverting to notify.showSuccess if imported, or confirm if $showSuccess is available.
            // Earlier code used this.$showSuccess. If user said it's not a function, then it's not.
            // Using logic from surrounding code (e.g. imports).
            // Assuming `notify` is imported from '@/notify' based on other files.
            // If not imported in this file, I'll assume global or mixin.
            // Wait, previous error said `this.$showError is not a function`.
            // So we must rely on `notify` module if imported.
            
            // Checking imports... notify IS usually imported.
            // If not, I'll add the import.
            // For now, assuming notify is imported or available.
            notify.showSuccess('Coordinates Updated');
            
            // Reload metadata to ensure UI is in sync
            // Clear cache if needed, though fetchMetadata might handle it?
            // Previous code did: this.metadataCache = {}; await this.fetchMetadata();
            // Let's stick to simple fetchMetadata first, or check if cache clearing is needed.
            this.metadataCache = {}; 
            await this.fetchMetadata();
        } catch (e) {
            console.error(e);
            notify.showError(e.message || 'Failed to update coordinates');
        }
    },
    async handleClearLocation() {
        if (!confirm('Are you sure you want to remove the location data from this file? This cannot be undone.')) {
            return;
        }
        try {
            const req = this.req;
            await filesApi.clearCoordinates(req.source, req.path);
            
            notify.showSuccess('Location data removed');
            
            
            // Reload metadata
            this.metadataCache = {};  
            await this.fetchMetadata();
        } catch (e) {
            console.error(e);
            notify.showError(e.message || 'Failed to clear location');
        }
    },
    async searchLocation() {
        if (!this.searchQuery) return;
        try {
            const res = await fetch(`https://nominatim.openstreetmap.org/search?format=json&q=${encodeURIComponent(this.searchQuery)}`);
            const data = await res.json();
            if (data && data.length > 0) {
                const lat = parseFloat(data[0].lat);
                const lon = parseFloat(data[0].lon);
                
                // Truncate to 8 decimals as requested
                this.editLat = parseFloat(lat.toFixed(8));
                this.editLon = parseFloat(lon.toFixed(8));
                
                this.mapInstance.setView([lat, lon], 13);
                this.updateMapMarker(this.editLat, this.editLon);
            } else {
                  console.log("No results found for", this.searchQuery);
                  notify.showError('Location not found');
            }
        } catch (e) {
            console.error(e);
             notify.showError('Search failed');
        }
    },

    handleCoordInput(event, field) {
        let val = event.target.value;
        if (val.indexOf('.') > -1) {
            const parts = val.split('.');
            if (parts[1].length > 8) {
                // Truncate to 8 decimals
                val = parts[0] + '.' + parts[1].substring(0, 8);
                // Update component data
                this[field] = parseFloat(val);
                // Force update input value visually if needed (Vue's v-model might lag on raw string edit)
                this.$forceUpdate();
            }
        }
    },
    initMap() {
        // Use ref instead of ID
        const mapContainer = this.$refs.leafletMap;
        if (!mapContainer) {
             console.warn("Leaflet container ref not found, retrying...");
             return;
        }

        if (this.mapInstance) {
             try {
                 this.mapInstance.remove(); 
             } catch(e) { /* ignore */ }
             this.mapInstance = null;
             this.mapMarker = null; // Important: Reset marker so it gets recreated on new map
        }

        // Default view
        let lat = 0;
        let lon = 0;
        let zoom = 2;

        if (this.gpsCoordinates) {
            lat = this.gpsCoordinates.lat;
            lon = this.gpsCoordinates.lon;
            zoom = 13;
        } else if (this.editLat !== 0 || this.editLon !== 0) {
            lat = this.editLat;
            lon = this.editLon;
            zoom = 13;
        }

        // Create Layers
        const hybrid = L.tileLayer('https://mt1.google.com/vt/lyrs=y&x={x}&y={y}&z={z}',{
            maxZoom: 20,
            attribution: 'Google'
        });

        const streets = L.tileLayer('https://mt1.google.com/vt/lyrs=m&x={x}&y={y}&z={z}',{
            maxZoom: 20,
            attribution: 'Google'
        });

        const osm = L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
            maxZoom: 19,
            attribution: '© OpenStreetMap'
        });

        // Dark Matter (CartoDB)
        const dark = L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', {
            attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>',
            subdomains: 'abcd',
            maxZoom: 20
        });

        // Initialize map
        // Default to Hybrid (Aerial) but respect isDarkMode maybe? Or just default.
        // Let's default to standard OSM for clarity or Hybrid as before.
        // User asked for "Standard, Satellite, Hybrid, Dark Mode".
        // Let's stick to Hybrid as default if set, or just add them all.
        
        this.mapInstance = L.map(mapContainer, { 
            preferCanvas: true,
            layers: [hybrid] // Default layer
        }).setView([lat, lon], zoom);

        // Add Layer Control
        const baseMaps = {
            "Satellite (Hybrid)": hybrid,
            "Standard (Streets)": streets,
            "OpenStreetMap": osm,
            "Dark Mode": dark
        };
        L.control.layers(baseMaps).addTo(this.mapInstance);

        this.updateMapMarker(lat, lon);
        
        this.mapStatus = `Map initialized. Size: ${mapContainer.offsetWidth}x${mapContainer.offsetHeight}`;

        this.mapInstance.on('click', (e) => {
             this.editLat = parseFloat(e.latlng.lat.toFixed(8));
             this.editLon = parseFloat(e.latlng.lng.toFixed(8));
             this.updateMapMarker(this.editLat, this.editLon);
        });
        
        // Force resize
        setTimeout(() => {
            if (this.mapInstance) {
                this.mapInstance.invalidateSize();
                this.mapStatus += " -> Resized";
            }
        }, 500);
    },
    
    forceReloadMap() {
        this.initMap();
    },



    updateMapMarker(lat, lon) {
        if (!this.mapInstance) return;

        if (this.mapMarker) {
            this.mapMarker.setLatLng([lat, lon]);
        } else {
            this.mapMarker = L.marker([lat, lon]).addTo(this.mapInstance);
        }
        // Pan map to marker if it's far off?
        // Maybe optional.
    },
    
    // Resume existing getMetadata
    async getMetadata(source, path) {
       if (this.metadataCache[path]) {
           return this.metadataCache[path];
       }
       
       try {
           const res = await filesApi.fetchMetadata(source, path);
           const data = res.data || res;
           // Cache it
           this.metadataCache[path] = data;
           return data;
       } catch (error) {
           console.error("Failed to fetch metadata for", path, error);
           return { exif: {}, iptc: {}, xmp: {} }; // Return empty structure on fail
       }
    },

    async fetchMetadata() {
      this.metadata = null;
      if (this.previewType !== "image") {
        console.log("Not an image, skipping metadata fetch");
        return;
      }
      
      // Use getMetadata to fetch for current file (checks cache)
      const data = await this.getMetadata(state.req.source, state.req.path);
      
      // Clone to ensure we don't mutate cache directly if we don't want to, 
      // OR mostly we do want to cache the parsed result. 
      // For now, let's just assign.
      this.metadata = data;

      if (!this.metadata.xmp) {
        this.metadata.xmp = {};
      }
      this.parsePhotoshopInstructions();
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
		if (tabName === 'map') return !this.gpsCoordinates;
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

      // Handle NameAssignType - even if it's empty or null
      let borderColor, backgroundColor;
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

      return {
          left: `${left}%`,
          top: `${top}%`,
          width: `${width}%`,
          height: `${height}%`,
          border: `2px solid ${borderColor}`,
          background: backgroundColor,
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
        autocenter: true,  // Enable autocenter for initial positioning
        zoomDoubleClickSpeed: 1, 
        onTouch: function() {
           return true; 
        }
      });
      
      // Force initial center after a small tick to ensure dimensions are ready
      setTimeout(() => {
          if (this.panzoomInstance && this.$refs.panzoomContent) {
              const transform = this.panzoomInstance.getTransform();
              // Explicitly center if scale is 1 (initial load)
              if (transform.scale === 1 || transform.scale === 0.1) { // 0.1 is minZoom default sometimes
                   const container = this.$refs.panzoomContent.parentNode;
                   const content = this.$refs.panzoomContent;
                   if (container && content) {
                       // Center: (ContainerWidth - ContentWidth) / 2
                       // But since we are using panzoom, we need to consider if the content is smaller than container
                       const cx = (container.clientWidth - content.offsetWidth) / 2;
                       const cy = (container.clientHeight - content.offsetHeight) / 2;
                       
                       this.panzoomInstance.moveTo(cx, cy);
                   }
              }
          }
      }, 100);
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
  this.originalInstructions = this.photoshopInstructions;

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
  
    openInstructionsModal() {
        mutations.toggleInstructionsEditMode(true);
    },
    
    closeInstructionsModal() {
        mutations.toggleInstructionsEditMode(false);
    },

    showTagPopup(key) {
        if (this.$showSuccess) {
            this.$showSuccess(key);
        } else {
            alert(key);
        }
    },

    saveInstructions: async function() {
      if (!this.canEditInstructions) return; 
      try {
        const metadataPath = this.metadata?.path || state.req.path;
        // Use existing API call if it works, or fallback to generic updateMetadata
        if (filesApi.updateXMPInstructions) {
             await filesApi.updateXMPInstructions(state.req.source, metadataPath, this.photoshopInstructions);
        } else {
             // Fallback to generic metadata update
             await filesApi.updateMetadata(state.req.source, state.req.path, {
                xmp: { 'photoshop:Instructions': this.photoshopInstructions }
             });
        }
        
        // Update original to prevent re-saving
        this.originalInstructions = this.photoshopInstructions;
      } catch (err) {
        console.error("Failed to save Photoshop instructions:", err);
        alert("Failed to save instructions.");
      }
    },

    loadSavedLocationIdx(idx) {
        if (idx === null || !this.savedLocations[idx]) return;
        
        const loc = this.savedLocations[idx];
        this.editLat = parseFloat(Number(loc.lat).toFixed(5));
        this.editLon = parseFloat(Number(loc.lon).toFixed(5));
        this.updateMapMarker(loc.lat, loc.lon);
        if (this.mapInstance) {
            this.mapInstance.setView([loc.lat, loc.lon], 13);
        }
    },
    
    async deleteSavedLocationIdx(idx) {
        if (!confirm("Are you sure you want to delete this saved location?")) return;
        
        // Remove from array
        this.savedLocations.splice(idx, 1);
        
         try {
            const userUpdate = { ...state.user, savedLocations: this.savedLocations };
            delete userUpdate.viewMode; 
            
            await usersApi.update(userUpdate, ['savedLocations']);
            await usersApi.update(userUpdate, ['savedLocations']);
            mutations.updateCurrentUser({ ...state.user, savedLocations: this.savedLocations });
             notify.showSuccess('Location deleted');
        } catch (e) {
             console.error(e);
             notify.showError('Failed to delete location');
        }
    },



    
    async saveLocationToProfile() {
        if (!this.editLat && !this.editLon) return;
        
        let name = prompt("Enter a name for this location:", this.searchQuery || "My Location");
        if (!name) return;
        
        await this.handleSaveLocation(name, this.editLat, this.editLon);
    },
    
    async saveCurrentGpsLocation() {
        if (!this.gpsCoordinates) return;
        
        let name = prompt("Enter a name for this location:", "New Location");
        if (!name) return;
        
        await this.handleSaveLocation(name, this.gpsCoordinates.lat, this.gpsCoordinates.lon);
    },

    async handleSaveLocation(name, lat, lon) {
        // Ensure we have the latest list from state before modifying
        if (state.user && state.user.savedLocations) {
             // Merge with any local component state if needed, but really we should trust state.user
             // If we haven't loaded them yet, this prevents overwriting with empty array.
             this.savedLocations = JSON.parse(JSON.stringify(state.user.savedLocations));
        }


        const newLoc = {
            name: name,
            lat: parseFloat(Number(lat).toFixed(5)),
            lon: parseFloat(Number(lon).toFixed(5))
        };
        
        if (!this.savedLocations) this.savedLocations = [];

        // Limit to 50 items, but do NOT auto-delete old ones
        if (this.savedLocations.length >= 50) {
            notify.showError("You have reached the limit of 50 saved locations. Please delete some before adding more.");
            return;
        }

        this.savedLocations.unshift(newLoc);
        
        try {
            const userUpdate = { ...state.user, savedLocations: this.savedLocations };
            delete userUpdate.viewMode; 
            
            await usersApi.update(userUpdate, ['savedLocations']);
            mutations.updateCurrentUser({ ...state.user, savedLocations: this.savedLocations });
             notify.showSuccess('Location saved to profile');
        } catch (e) {
             console.error(e);
             notify.showError('Failed to save location');
        }
    },

    loadSavedLocation(event) {
        // v-model update handles the value, but we might need event for simple change
        // OR just watch selectedSavedLocationIdx.
        // If event provided (from @change), use it. But v-model "selectedSavedLocationIdx" is cleaner.
        
        const idx = this.selectedSavedLocationIdx;
        if (idx === null || idx === "" || !this.savedLocations[idx]) return;
        
        const loc = this.savedLocations[idx];
        this.editLat = loc.lat;
        this.editLon = loc.lon;
        this.updateMapMarker(loc.lat, loc.lon);
        if (this.mapInstance) {
            this.mapInstance.setView([loc.lat, loc.lon], 13);
        }
    },
    
    async deleteSavedLocation() {
        if (this.selectedSavedLocationIdx === null) return;
        
        if (!confirm("Are you sure you want to delete this saved location?")) return;
        
        const idx = this.selectedSavedLocationIdx;
        // Remove from array
        this.savedLocations.splice(idx, 1);
        this.selectedSavedLocationIdx = null; // Reset selection
        
         try {
            const userUpdate = { ...state.user, savedLocations: this.savedLocations };
            delete userUpdate.viewMode; 
            
            await usersApi.update(userUpdate, ['savedLocations']);
            await usersApi.update(userUpdate, ['savedLocations']);
            mutations.updateUser({ ...state.user, savedLocations: this.savedLocations });
             notify.showSuccess('Location deleted');
        } catch (e) {
             console.error(e);
             notify.showError('Failed to delete location');
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
    async tryAutoSave() {
        if (this.canEditInstructions && this.photoshopInstructions !== this.originalInstructions) {
             //console.log("Auto-saving Modified Instructions...");
             await this.saveInstructions();
        }
    },
    async prev() {
      await this.tryAutoSave();  
      this.hoverNav = false;
      this.$router.replace({ path: this.previousLink });
    },
    async next() {
      await this.tryAutoSave();
      this.hoverNav = false;
      this.$router.replace({ path: this.nextLink });
    },
    key(event) {
      if (getters.currentPromptName() != null) {
        return;
      }
      
      // Ignore navigation keys if typing in an input or textarea
      if (['INPUT', 'TEXTAREA'].includes(document.activeElement.tagName)) {
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
            // Prefetch metadata for previous image
            this.getMetadata(state.req.source, composedListing.path);
          }
          break;
        }
        for (let j = i + 1; j < this.listing.length; j++) {
          let composedListing = this.listing[j];
          composedListing.path = directoryPath + "/" + composedListing.name;
          this.nextLink = composedListing.url;
          if (getTypeInfo(composedListing.type).simpleType == "image") {
            this.nextRaw = this.prefetchUrl(composedListing);
            // Prefetch metadata for next image
            this.getMetadata(state.req.source, composedListing.path);
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
  overflow: hidden;
  box-sizing: border-box; /* Ensure padding doesn't add to height */
  padding-top: 4em;       /* Push content below the header */
}

/* Desktop Layout: Side-by-side */
@media (min-width: 1024px) {
  #previewer {
    flex-direction: row;
  }
}

/* Instructions Split Pane (Persistent) */
.instructions-split-pane {
  display: flex;
  flex-direction: column;
  width: 100%;
  background: #fff;
  border-top: 1px solid #ccc;
  padding: 1rem;
  box-sizing: border-box;
  color: #333; /* ensure text is readable */
  position: relative; /* layout context */
  z-index: 50; /* ensure it's above image */
}

.pane-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.5rem;
  
  h4 {
    margin: 0;
    font-size: 1.1rem;
    font-weight: bold;
    color: #333;
  }
  
  .close-icon {
    background: none;
    border: none;
    cursor: pointer;
    padding: 0;
    display: flex;
    align-items: center;
    
    i {
      font-size: 1.5rem;
      color: #666;
    }
    
    &:hover i {
      color: #000;
    }
  }
}

.instructions-split-pane textarea {
  flex: 1;
  width: 100%;
  resize: none;
  font-family: inherit;
  padding: 0.5rem;
  box-sizing: border-box;
  margin-bottom: 0.5rem;
  min-height: 0; /* allows flex shrink */
  border: 1px solid #ccc;
  border-radius: 4px;
}

/* button row reused */
.button-row {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
}

/* When instructions are showing, the preview column handles split (image top, editor bottom) */
.preview {
  display: flex;
  flex-direction: column; /* Stack image and editor vertically */
  /* width, flex-grow etc inherited */
}

/* ... overlay styles ... */

@media (min-width: 1024px) {
  .metadata-container {
    width: 450px;
    height: 100%;      /* Fill parent height */
    max-height: none;  /* Remove constraint */
    border-top: none;
    border-left: 1px solid var(--dark-theme-2);
  }
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
  overflow: hidden; /* Prevent spillover */
  position: relative;

  &.full-height {
    max-height: 100vh;
  }

  .image-container {
    position: relative;
    display: block;
    width: 100%;
    height: 100%;
    overflow: hidden; /* Ensure panzoom doesn't cause scrollbars on parent */
    background-color: black; /* Visual background */
    touch-action: none; /* Disable browser handling of gestures */
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
    top: -40px; /* Adjusted for larger font */
    left: 0;
    background: rgba(0, 0, 0, 0.8);
    color: #fff;
    padding: 8px 12px;
    font-size: 24px; /* Significantly larger */
    font-weight: bold;
    white-space: nowrap;
    border-radius: 4px;
    z-index: 10;
    pointer-events: none; /* Ensure it doesn't block interactions */
  }

  /* Increase size for desktop/larger screens */
  @media (min-width: 1024px) {
    .face-label {
        font-size: 14px;
        top: -24px;
        padding: 4px 8px;
    }
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
  background: var(--dark-theme-1);
  color: #fff;
  padding: 1rem;
  overflow-y: auto;
  flex-shrink: 0;
  z-index: 20;

  /* Mobile Pattern (Default) */
  width: 100%;
  border-top: 1px solid var(--dark-theme-2);
  /* Remove max-height constraint to start, control via style binding or default */
}

@media (min-width: 1024px) {
  .metadata-container {
    width: 350px; /* Default width on desktop */
    height: 100% !important; /* Force full height on desktop, overriding inline styles if any */
    max-height: none !important;
    border-top: none;
    border-left: 1px solid var(--dark-theme-2);
  }
}

.resize-handle {
  position: absolute;
  top: -10px;
  left: 0;
  width: 100%;
  height: 10px;
  cursor: ns-resize;
  background: transparent;
  z-index: 21;
}

@media (min-width: 1024px) {
  .resize-handle {
    top: 0;
    left: -5px;
    width: 10px;
    height: 100%;
    cursor: ew-resize; /* Horizontal resize cursor */
  }
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
    padding: 0.25rem 0.5rem;    /* Tighter padding */
    margin: 0 0.1rem;           /* Tighter margin */
    cursor: pointer;
    border-radius: 4px;
    transition: background-color 0.2s;
    font-size: 0.85rem;         /* Smaller text */

    &:hover {
      background: rgba(255, 255, 255, 0.1);
    }

    &.active {
      background: var(--accent-green);
      color: #000;
    }

    &.tab-empty {
      background: #ffcccc; 
      color: #000;
    }

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
      font-size: 1rem; /* Smaller header */
    }

    .metadata-table {
      max-height: 400px; /* Increased height */
      overflow-y: auto;

      table {
        width: 100%;
        border-collapse: collapse;

        th, td {
          padding: 0.25rem 0.4rem;
          border: 1px solid var(--dark-theme-2);
          text-align: left;
          vertical-align: top;
        }

        th {
          background: rgba(255, 255, 255, 0.05);
          color: #aaa;
          font-weight: bold;
          font-size: 0.75rem;       /* Shrunk text for Tab Column */
          text-transform: uppercase;
          width: 25%;               /* Reduced width for Tab Column */
        }

        td {
            color: #fff;
            font-size: 0.9rem;      /* Increased size for Value Column */
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