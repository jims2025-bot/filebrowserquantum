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
            :ref="(el) => { if (el) { el.src = raw; el.load(); } }"
            controls
            :autoplay="autoPlay"
            @play="autoPlay = true"
          ></audio>

          <video
            v-else-if="previewType == 'video'"
            :ref="(el) => { if (el) { el.src = raw; el.load(); } }"
            :key="req.path"
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

          <div v-else-if="previewType == 'text'" class="text-preview">
            <pre><code>{{ textContent }}</code></pre>
          </div>

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
      </div>
      
      <div class="tabs-content">
        <div v-if="activeTab === 'details'" class="tab-pane">
          <h3>FILE</h3>
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
          <h3>NOTES</h3>

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
		</div> 

        <div v-if="activeTab === 'xmp'" class="tab-pane">
          <h3>FACE</h3>
          
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

          <div v-show="activeTab === 'map'" class="tab-pane" style="height: 100%; display: flex; flex-direction: column; overflow-y: auto;">
            
            <!-- 1. Embedded Location Info -->
            <div style="flex: 0 0 auto; padding: 10px; border-bottom: 2px solid #eee; background: #fafafa; color: #333;">
                <div style="display: flex; gap: 10px; align-items: start; flex-wrap: wrap;">
                    
                    <h3 style="margin: 0; font-size: 0.85em; text-transform: uppercase; color: #777; letter-spacing: 0.5px; white-space: nowrap; margin-top: 3px;">Current Embedded Location</h3>

                    <div v-if="gpsCoordinates" style="display: flex; flex-direction: column; gap: 4px; flex: 1;">
                         <div style="display: flex; align-items: center; gap: 8px;">
                             <div 
                                @click="panToEmbedded" 
                                title="Click to view on map"
                                style="font-family: monospace; font-size: 0.9em; font-weight: bold; color: #333; white-space: nowrap; cursor: pointer; text-decoration: underline; text-decoration-style: dotted;"
                             >
                                 {{ gpsCoordinates.lat.toFixed(6) }}, {{ gpsCoordinates.lon.toFixed(6) }}
                             </div>
                             
                             <div style="display: flex; gap: 8px;">
                                 <!-- Moved Clear Button here as Icon -->
                                 <button @click="handleClearLocation" class="button button--flat" title="Clear embedded location" style="padding: 6px; height: 32px; line-height: 1; min-width: 32px;">
                                     <i class="material-icons" style="font-size: 20px; color: #f44336;">delete</i>
                                 </button>
                                 
                                 <button @click="copyCoordinates" class="button button--flat" title="Copy" style="padding: 6px; height: 32px; line-height: 1; min-width: 32px;">
                                     <i class="material-icons" style="font-size: 14px;">content_copy</i>
                                 </button>
                                 <button @click="saveEmbeddedLocationToProfile" class="button button--flat" title="Save this location to My Locations" style="padding: 6px; height: 32px; line-height: 1; min-width: 32px;">
                                     <i class="material-icons" style="font-size: 16px; color: #2196f3;">bookmark_add</i>
                                 </button>
                                 <a :href="'https://www.google.com/maps/search/?api=1&query=' + gpsCoordinates.lat + ',' + gpsCoordinates.lon" 
                                    target="_blank" 
                                    class="button button--flat" 
                                    title="Open in Google Maps"
                                    style="padding: 2px; height: 20px; line-height: 1; display: flex; align-items: center; min-width: 24px; color: inherit; text-decoration: none;">
                                    <i class="material-icons" style="font-size: 16px;">map</i>
                                 </a>
                             </div>
                         </div>

                         <!-- Match Badge (Embedded) -->
                         <div v-if="gpsMatchedLocationName" style="padding: 2px 6px; background: #e3f2fd; color: #1565c0; border-radius: 4px; font-size: 0.75em; font-weight: 500; display: flex; align-items: center; white-space: nowrap; align-self: flex-start;">
                             <i class="material-icons" style="font-size: 12px; margin-right: 3px;">bookmark</i>
                             Matches: {{ gpsMatchedLocationName }}
                         </div>
                    </div>
                    
                    <div v-else style="color: #d32f2f; font-style: italic; font-weight: 500; display: flex; align-items: center; font-size: 0.9em;">
                         <i class="material-icons" style="font-size: 14px; margin-right: 5px;">location_off</i>
                         NONE
                    </div>
                </div>
            </div>

            <!-- 2. Interactive Area -->
            <div style="flex: 1; display: flex; flex-direction: column; min-height: 0; position: relative; overflow-y: auto;">
                 
                 <!-- Map Container (Fixed Height 40% - reduced from 50%) -->
                 <div class="map-wrapper" style="flex: 0 0 40%; position: relative; background: #e0e0e0; min-height: 150px; display: flex; flex-direction: column;">
                      <div id="leafletMap" ref="leafletMap" style="flex: 1; width: 100%; height: 100%; z-index: 1; display: block;"></div>
                      
                      <!-- Overlay Status -->
                      <div v-if="canEditCoordinates" style="position: absolute; bottom: 10px; left: 10px; right: 10px; z-index: 400; pointer-events: none;">
                          <div style="background: rgba(255,255,255,0.9); padding: 4px 10px; border-radius: 20px; box-shadow: 0 1px 3px rgba(0,0,0,0.2); display: inline-block; font-size: 0.8em; color: #333; pointer-events: auto;">
                              <i class="material-icons" style="font-size: 16px; vertical-align: text-top; color: #2e7d32;">touch_app</i>
                              Click map to set proposed coordinates
                          </div>
                      </div>
                 </div>

                 <!-- Dedicated Search Bar (Below Map) -->
                 <div v-if="canEditCoordinates" style="flex: 0 0 auto; padding: 8px 10px; background: #fff; border-bottom: 1px solid #eee; display: flex; gap: 5px;">
                      <input ref="searchInput" type="text" v-model="searchQuery" @keyup.enter="searchLocation" placeholder="Search places..." class="input input--block" style="flex: 1; height: 30px; font-size: 0.9em; background: #f9f9f9; padding: 2px 8px; color: #333;">
                      <button @click="searchLocation" class="button button--flat" :disabled="!searchQuery" style="padding: 0 8px; min-width: 30px; height: 30px;" title="Search"><i class="material-icons" style="font-size: 22px;">search</i></button>
                 </div>

                 <!-- Editable Header & Inputs -->
                 <div class="edit-toolbar" style="flex: 0 0 auto; padding: 10px; background: white; border-bottom: 1px solid #ddd; border-top: 1px solid #ddd; box-shadow: 0 2px 4px rgba(0,0,0,0.05); z-index: 10; color: #333;">
                      
                      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px;">
                          <h3 style="margin: 0; font-size: 0.85em; color: #2196f3; display: flex; align-items: center; font-weight: 600;">
                               <span v-if="canEditCoordinates">EDITABLE LOCATION (PROPOSED)</span>
                               <span v-else>MAP PREVIEW</span>
                               
                               <span v-if="canEditCoordinates" style="margin-left: 10px; font-size: 0.7em; background: #e8f5e9; color: #2e7d32; padding: 1px 6px; border-radius: 4px; border: 1px solid #c8e6c9;">
                                   EDIT MODE
                               </span>
                          </h3>
                          
                          <!-- Save Actions -->
                          <div v-if="canEditCoordinates" style="display: flex; gap: 5px;">
                               <!-- Removed 'Clear' Button from here -->
                               
                               <button 
                                  @click="saveCoordinates" 
                                  :disabled="!hasUnsavedCoordinates" 
                                  class="button" 
                                  :class="hasUnsavedCoordinates ? 'button--primary' : 'button--flat'" 
                                  style="padding: 2px 8px; font-size: 0.8em; transition: all 0.2s; height: 24px;"
                               >
                                   {{ hasUnsavedCoordinates ? 'Save' : 'Saved' }}
                               </button>
                          </div>
                      </div>

                      <!-- Inputs Row -->
                      <div style="display: flex; gap: 10px; align-items: center;">
                           <div style="display: flex; gap: 5px; align-items: center;">
                               <label style="font-size: 0.85em; color: #666; font-weight: bold;">Lat:</label>
                               <input type="number" step="any" v-model.lazy.number="editLat" :disabled="!canEditCoordinates" class="input" style="width: 110px; height: 26px; font-size: 0.9em; padding: 2px 5px;">
                           </div>
                           <div style="display: flex; gap: 5px; align-items: center;">
                               <label style="font-size: 0.85em; color: #666; font-weight: bold;">Lon:</label>
                               <input type="number" step="any" v-model.lazy.number="editLon" :disabled="!canEditCoordinates" class="input" style="width: 110px; height: 26px; font-size: 0.9em; padding: 2px 5px;">
                           </div>
                           
                           <!-- Search Box MOVED from here -->
                      </div>
                      
                      <!-- Proposed Matches Status -->
                      <div v-if="canEditCoordinates" style="margin-top: 6px; font-size: 0.8em; color: #555; display: flex; align-items: center;">
                          <span style="font-weight: bold; margin-right: 4px;">Proposed Matches:</span>
                          <span v-if="matchedLocationName || gpsMatchedLocationName" 
                                @click="applyMatchedLocation(matchedLocationName || gpsMatchedLocationName)"
                                style="color: #1565c0; background: #e3f2fd; padding: 0 4px; border-radius: 3px; cursor: pointer; text-decoration: underline;"
                                title="Click to apply this location">
                               {{ matchedLocationName || gpsMatchedLocationName }}
                          </span>
                          <span v-else style="font-style: italic; color: #777;">None</span>
                      </div>
                 </div>

                 <!-- Saved Locations Footer (Stretches to bottom) -->
                 <div style="flex: 1; display: flex; flex-direction: column; background: white; color: #333; overflow: hidden; border-top: 1px solid #ddd; min-height: 0;">
                      <div @click="editTab = editTab === 'locations' ? 'location' : 'locations'" style="flex: 0 0 auto; padding: 6px 10px; cursor: pointer; display: flex; justify-content: space-between; align-items: center; background: #f5f5f5; border-bottom: 1px solid #eee;">
                           <div style="display: flex; align-items: center; gap: 5px;">
                                <i class="material-icons" style="font-size: 22px; color: #2196f3;">bookmark</i>
                                <span style="font-weight: 600; font-size: 0.9em; color: #333;">Saved Locations</span>
                           </div>
                           <i class="material-icons" style="transform: rotate(0deg); transition: transform 0.2s;" :style="editTab === 'locations' ? 'transform: rotate(180deg)' : ''">expand_less</i>
                      </div>
                      
                      <!-- Scrollable Area fills ALL remaining height -->
                      <div v-if="editTab === 'locations'" style="flex: 1; overflow-y: auto; padding: 0; min-height: 0;">
                            <div v-if="!savedLocations || savedLocations.length === 0" style="padding: 15px; opacity: 0.6; font-style: italic; font-size: 0.9em; text-align: center; color: #666;">
                                No saved locations. <br>Use 'Add to Saved Locations' button above or on embedded data.
                            </div>
                            <div v-for="(loc, idx) in savedLocations" :key="idx" 
                                style="padding: 8px 10px; border-bottom: 1px solid #eee; cursor: pointer; font-size: 0.9em; display: flex; justify-content: space-between; align-items: center; color: #333;"
                                @click="editLat = loc.lat; editLon = loc.lon; updateMapMarker(loc.lat, loc.lon);"
                            >
                                <span style="font-weight: 500;">{{ loc.name }}</span>
                                <div style="display: flex; align-items: center; gap: 10px;">
                                    <span style="font-size: 0.8em; color: #777;">{{ loc.lat.toFixed(4) }}, {{ loc.lon.toFixed(4) }}</span>
                                    <button @click.stop="deleteSavedLocationIdx(idx)" class="button button--flat" title="Delete" style="color: #f44336; padding: 2px; min-height: 20px; line-height: 1;">
                                        <i class="material-icons" style="font-size: 16px;">delete</i>
                                    </button>
                                </div>
                            </div>
                      </div>
                 </div>
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
      activeTab: "iptc",
      
      // Map Editing
      // isEditingCoordinates removed
      searchQuery: "",
      editLat: 0,
      editLon: 0,
      savedLocations: [], // fetched from user profile
      selectedSavedLocationIdx: null,
      mapStatus: "", // Text status is fine to be reactive
      
      // Edit Settings Tabs
      editTab: 'location', // 'location' | 'locations'

      tabs: [
        { name: "iptc", label: "NOTES" },
        { name: "map", label: "MAP" },
        { name: "xmp", label: "XMP" },
        { name: "details", label: "FILE" },
        { name: "exif", label: "EXIF" },
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
      textContent: '', // For text/JSON file preview
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
		{ name: "iptc", label: "NOTES" },
		{ name: "map", label: "MAP" },
		{ name: "xmp", label: "FACE" },
		{ name: "exif", label: "EXIF" },
		{ name: "details", label: "FILE" },
		];
		return tabs;
	},
    raw() {
      return filesApi.getDownloadURL(state.req.source, state.req.path, true);
    },
    req() {
      return state.req;
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
  methods: {
    async fetchTextContent() {
      console.log('fetchTextContent called, previewType:', this.previewType, 'req.type:', state.req.type);
      
      // Only fetch for text files
      if (this.previewType !== 'text') {
        this.textContent = '';
        return;
      }

      try {
        console.log('Fetching text content from:', this.raw);
        const response = await fetch(this.raw);
        if (!response.ok) {
          throw new Error('Failed to fetch text content');
        }
        let text = await response.text();
        console.log('Fetched text, length:', text.length);

        // Pretty-print JSON
        if (state.req.type === 'application/json') {
          console.log('Pretty-printing JSON');
          try {
            const json = JSON.parse(text);
            text = JSON.stringify(json, null, 2);
          } catch (e) {
            // If JSON parsing fails, just show raw text
            console.warn('Failed to parse/stringify JSON:', e);
          }
        }

        this.textContent = text;
        console.log('Text content set, preview should render');
      } catch (error) {
        console.error('Error fetching text content:', error);
        this.textContent = 'Error loading file content';
      }
    },
  },
  mounted() {
    this.fetchTextContent();
  },
  watch: {
    async raw() {
      console.log('🔥 RAW WATCHER TRIGGERED!', this.raw);
      
      if (!getters.isLoggedIn()) {
        return;
      }
      
      // Reset panzoom on new file
      this.disposePanzoom();
      
      this.dimensionRetryCount = 0;
      
      try {
        if (this.updatePreview) await this.updatePreview();
      } catch (e) {
        console.warn('updatePreview error:', e);
      }
      
      try {
        if (this.toggleNavigation) this.toggleNavigation();
      } catch (e) {
        console.warn('toggleNavigation error:', e);
      }
      
      try {
        if (this.retrieveSavedLocations) this.retrieveSavedLocations();
      } catch (e) {
        console.warn('retrieveSavedLocations error:', e);
      }
      
      try {
        await this.fetchTextContent();
      } catch (e) {
        console.error('fetchTextContent error:', e);
      }
      
      try {
        if (this.updateImageDimensions) await this.updateImageDimensions();
      } catch (e) {
        console.warn('updateImageDimensions error:', e);
      }
      
      this.$nextTick(() => {
        const availableTabNames = this.availableTabs.map(tab => tab.name);
        //console.log("Validating activeTab:", this.activeTab, "Available:", availableTabNames);
        if (!availableTabNames.includes(this.activeTab)) {
          this.activeTab = "details";
        //  console.log("Reset activeTab to 'details' as current tab is not available");
        }
      });
    },

    gpsCoordinates(newVal) {
       if (!this.mapInstance) return;

       // 1. Update Embedded Marker (Gold)
       if (newVal) {
           const lat = newVal.lat;
           const lon = newVal.lon;
           
           if (this.embeddedMarker) {
               this.embeddedMarker.setLatLng([lat, lon]);
           } else {
               const goldIcon = new L.Icon({
                    iconUrl: 'https://raw.githubusercontent.com/pointhi/leaflet-color-markers/master/img/marker-icon-gold.png',
                    shadowUrl: 'https://cdnjs.cloudflare.com/ajax/libs/leaflet/1.7.1/images/marker-shadow.png',
                    iconSize: [25, 41],
                    iconAnchor: [12, 41],
                    popupAnchor: [1, -34],
                    shadowSize: [41, 41]
                });
               this.embeddedMarker = L.marker([lat, lon], { icon: goldIcon })
                   .addTo(this.mapInstance)
                   .bindPopup("Embedded Location");
           }
           
           // 2. Pan Map to new location
           this.mapInstance.setView([lat, lon], 16);

           // REMOVED: Sync Edit Marker & Inputs (User wants to keep previous inputs)
           // this.editLat = parseFloat(lat.toFixed(6));
           // this.editLon = parseFloat(lon.toFixed(6));
           // this.updateMapMarker(this.editLat, this.editLon);

       } else {
           if (this.embeddedMarker) {
               this.mapInstance.removeLayer(this.embeddedMarker);
               this.embeddedMarker = null;
           }
           // Optionally reset view/inputs if no GPS, but maybe better to leave map where it is?
           // For inputs, we should probably reset to 0 or clear them to avoid confusion
           // this.editLat = 0;
           // this.editLon = 0;
           // this.updateMapMarker(0, 0); // This removes the blue marker
       }
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
         // Initialize map immediately when switching to tab
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
        
        // Init map if starting on map tab
        if (this.activeTab === 'map') {
            this.$nextTick(() => this.initMap());
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
    // Clean up player
    if (this.$refs.player && typeof this.$refs.player.pause === "function") {
        this.$refs.player.pause();
        this.$refs.player.src = "";
        this.$refs.player.load();
    }
  
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
        // Deprecated - logic merged into initMap / created
    },
    cancelEditCoordinates() {
        // Deprecated - logic merged into initMap
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

    saveEmbeddedLocationToProfile() {
        if (!this.gpsCoordinates) return;
        this.saveLocationToProfile(this.gpsCoordinates.lat, this.gpsCoordinates.lon);
    },
    
    updateEditCoord(event, field) {
        const val = parseFloat(event.target.value);
        if (!isNaN(val)) {
            // Store raw value but formatted
            this[field] = parseFloat(val.toFixed(6));
        }
    },
    
    // Updated helper to accept optional lat/lon
    async saveLocationToProfile(overrideLat, overrideLon) {
         let lat, lon;
         // Check if called from button click (event object) or with coords
         if (typeof overrideLat === 'number' && typeof overrideLon === 'number') {
             lat = overrideLat;
             lon = overrideLon;
         } else {
             lat = this.editLat;
             lon = this.editLon;
         }

         if (lat === 0 && lon === 0) {
             notify.showError("Invalid coordinates");
             return;
         }
         
         const name = prompt("Enter a name for this location:", this.matchedLocationName || "New Location");
         if (!name) return;
         
         try {
             const newLoc = { name, lat, lon };
             // Use API to save to profile
             // Assuming explicit API method exists or updating user object
             // Let's assume we need to update the whole savedLocations array
             
             let currentLocs = this.currentUserSavedLocations ? JSON.parse(JSON.stringify(this.currentUserSavedLocations)) : [];
             currentLocs.push(newLoc);
             
             // Optimistically update local
             this.savedLocations = currentLocs;
             
             // Push to backend
             await usersApi.update({ savedLocations: currentLocs }); 
             notify.showSuccess("Location saved to profile");
         } catch(e) {
             console.error(e);
             notify.showError("Failed to save location");
         }
    },

    async deleteSavedLocationIdx(idx) {
        if (!confirm("Delete this saved location?")) return;
        try {
             let currentLocs = this.currentUserSavedLocations ? JSON.parse(JSON.stringify(this.currentUserSavedLocations)) : [];
             currentLocs.splice(idx, 1);
             this.savedLocations = currentLocs;
             await usersApi.update({ savedLocations: currentLocs });
             notify.showSuccess("Location deleted");
        } catch(e) {
             console.error(e);
             notify.showError("Failed to delete location");
        }
    },

    applyMatchedLocation(nameToApply) {
        if (!this.savedLocations || this.savedLocations.length === 0) return;
        
        // Use passed name if available, otherwise fallback to GPS match logic (for safety)
        const targetName = nameToApply || this.gpsMatchedLocationName;

        if (targetName) {
             const loc = this.savedLocations.find(l => l.name === targetName);
             if (loc) {
                 this.editLat = loc.lat;
                 this.editLon = loc.lon;
                 this.updateMapMarker(loc.lat, loc.lon);
                 notify.showSuccess("Applied location: " + loc.name);
             }
        }
    },
    initMap() {
        // Use ref instead of ID
        const mapContainer = this.$refs.leafletMap;
        if (!mapContainer) {
             // If tab is not active, container won't exist yet.
             return; 
        }

        if (this.mapInstance) {
             try {
                 this.mapInstance.remove(); 
             } catch(e) { /* ignore */ }
             this.mapInstance = null;
             this.editMarker = null; 
             this.embeddedMarker = null;
        }
        
        // Define Gold Icon for Embedded Location
        const goldIcon = new L.Icon({
            iconUrl: 'https://raw.githubusercontent.com/pointhi/leaflet-color-markers/master/img/marker-icon-gold.png',
            shadowUrl: 'https://cdnjs.cloudflare.com/ajax/libs/leaflet/1.7.1/images/marker-shadow.png',
            iconSize: [25, 41],
            iconAnchor: [12, 41],
            popupAnchor: [1, -34],
            shadowSize: [41, 41]
        });

        // Always sync inputs with current GPS on map init for this file
        if (this.gpsCoordinates) {
            this.editLat = parseFloat(this.gpsCoordinates.lat.toFixed(6));
            this.editLon = parseFloat(this.gpsCoordinates.lon.toFixed(6));
        } else {
             this.editLat = 0;
             this.editLon = 0;
        }

        // Default view
        let lat = this.editLat;
        let lon = this.editLon;
        let zoom = (lat === 0 && lon === 0) ? 2 : 13;

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
        this.mapInstance = L.map(mapContainer, { 
            preferCanvas: true,
            layers: [hybrid], // Default layer
            attributionControl: false
        }).setView([lat, lon], zoom);

        // Add Layer Control
        const baseMaps = {
            "Satellite (Hybrid)": hybrid,
            "Standard (Streets)": streets,
            "OpenStreetMap": osm,
            "Dark Mode": dark
        };
        L.control.layers(baseMaps).addTo(this.mapInstance);

        // 1. Add Embedded Marker (Gold) if exists
        if (this.gpsCoordinates) {
            this.embeddedMarker = L.marker([this.gpsCoordinates.lat, this.gpsCoordinates.lon], { icon: goldIcon })
                .addTo(this.mapInstance)
                .bindPopup("Embedded Location");
        }

        // 2. Add Edit Marker (Blue) - initially at same spot if syncing
        this.updateMapMarker(lat, lon);
        
        this.mapStatus = `Map initialized.`;

        // Only allow clicking to set coordinates if user has permission
        if (this.canEditCoordinates) {
            this.mapInstance.on('click', (e) => {
                 this.editLat = parseFloat(e.latlng.lat.toFixed(6));
                 this.editLon = parseFloat(e.latlng.lng.toFixed(6));
                 this.updateMapMarker(this.editLat, this.editLon);
            });
        }
        
        // Force resize
        setTimeout(() => {
            if (this.mapInstance) {
                this.mapInstance.invalidateSize();
            }
        }, 500);
    },
    
    forceReloadMap() {
        this.initMap();
    },

    panToEmbedded() {
        if (this.gpsCoordinates && this.mapInstance) {
            this.mapInstance.setView([this.gpsCoordinates.lat, this.gpsCoordinates.lon], 16);
            if (this.embeddedMarker) {
                this.embeddedMarker.openPopup();
            }
        }
    },

    updateMapMarker(lat, lon) {
        if (!this.mapInstance) return;

        // If invalid coords, remove marker
        if (lat === 0 && lon === 0) {
            if (this.editMarker) {
                this.mapInstance.removeLayer(this.editMarker);
                this.editMarker = null;
            }
            return;
        }

        const blueIcon = new L.Icon({
            iconUrl: 'https://raw.githubusercontent.com/pointhi/leaflet-color-markers/master/img/marker-icon-blue.png',
            shadowUrl: 'https://cdnjs.cloudflare.com/ajax/libs/leaflet/1.7.1/images/marker-shadow.png',
            iconSize: [18, 30], // Smaller size (orig: 25, 41)
            iconAnchor: [9, 30], // Adjusted anchor (orig: 12, 41)
            popupAnchor: [1, -25],
            shadowSize: [30, 30] // Smaller shadow
        });

        if (this.editMarker) {
            this.editMarker.setLatLng([lat, lon]);
            this.editMarker.setIcon(blueIcon);
        } else {
            this.editMarker = L.marker([lat, lon], { icon: blueIcon, zIndexOffset: 1000 }).addTo(this.mapInstance); // Keep blue on top but smaller
        }
        
        // Pan map to new proposed location
        this.mapInstance.setView([lat, lon]);
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

      // Explicitly pause legacy player to prevent background audio
      if (this.$refs.player && typeof this.$refs.player.pause === "function") {
          this.$refs.player.pause();
          this.$refs.player.currentTime = 0;
          this.$refs.player.src = ""; // Detach source
          this.$refs.player.load();   // Force cleanup
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

  .text-preview {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    overflow: auto;
    background: #1e1e1e;
    color: #d4d4d4;
    padding: 1rem;
    box-sizing: border-box;
  }

  .text-preview pre {
    margin: 0;
    white-space: pre-wrap;
    word-wrap: break-word;
    font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
    font-size: 14px;
    line-height: 1.5;
  }

  .text-preview code {
    font-family: inherit;
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
  
  /* Flexbox for layout */
  display: flex;
  flex-direction: column;
  overflow: hidden; /* Hide global scroll, let children scroll */
}

@media (min-width: 1024px) {
  .metadata-container {
    width: 350px; /* Default width on desktop */
    height: 100% !important; /* Force full height on desktop, overriding inline styles if any */
    max-height: none !important;
    border-top: none;
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
  margin-bottom: 0.5rem; /* Reduced margin */
  border-bottom: 1px solid var(--dark-theme-2);
  padding-bottom: 0.5rem;
  flex: 0 0 auto; /* Fixed height for header */

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
  flex: 1; /* Take remaining height */
  min-height: 0; /* Important for scroll */
  overflow: hidden; /* Prevent container scroll */
  display: flex; /* To stretch children */
  flex-direction: column;

  .tab-pane {
    position:relative;
    flex: 1; /* Fill parent */
    overflow-y: auto; /* Scroll internally by default */
    overflow-x: hidden;
    padding-bottom: 1rem;
	
    h3 {
      margin-top: 0;
      color: var(--accent-green);
      font-size: 1rem; /* Smaller header */
    }

    .metadata-table {
      max-height: none; /* Let the container handle scroll */
      overflow-y: visible; /* Let the pane handle scroll */

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