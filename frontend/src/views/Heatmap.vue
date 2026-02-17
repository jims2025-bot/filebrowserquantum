<template>
  <div id="heatmap-container" @mouseleave="cursorCoords = {lat: 999, lng: 999}"></div>
  
  <!-- Selection Mode Overlay -->
  <div v-if="isBoxSelectMode" 
       ref="selectionOverlay"
       style="position:absolute; top:0; left:0; width:100%; height:100%; z-index:10000; cursor:crosshair; touch-action:none;"
       @mousedown="startSelection" @mousemove="updateSelection" @mouseup="endSelection"
       @touchstart="startSelection" @touchmove="updateSelection" @touchend="endSelection">
      <div v-if="selectionBox.visible" :style="selectionBox.style" style="position:absolute; border:2px dashed yellow; background:rgba(255,255,0,0.2); pointer-events:none;"></div>
  </div>

  <!-- Selection Mode Active Indicator / Cancel -->
  <div v-if="isBoxSelectMode" style="position:absolute; top:10px; left:50%; transform:translateX(-50%); z-index:10001; background:rgba(0,0,0,0.8); color:white; padding:8px 16px; border-radius:20px; font-size:14px; font-weight:bold; display:flex; gap:10px; align-items:center;">
      <span>Selection Mode Active</span>
      <button @click="toggleSelectionMode" style="background:none; border:none; color:white; cursor:pointer; font-weight:bold;">CANCEL</button>
  </div>
  
  <!-- LEADER LINE OVERLAY -->
  <svg v-if="leaderLine.visible" 
       ref="leaderLineSvg"
       :width="leaderLine.svgWidth"
       :height="leaderLine.svgHeight"
       style="position:absolute; top:0; left:0; z-index:9000; pointer-events:none;">
      <defs>
        <marker id="arrowhead" markerWidth="10" markerHeight="7" refX="10" refY="3.5" orient="auto">
          <polygon points="0 0, 10 3.5, 0 7" fill="red" />
        </marker>
        <filter id="glow" x="-20%" y="-20%" width="140%" height="140%">
          <feGaussianBlur stdDeviation="2" result="blur" />
          <feComposite in="SourceGraphic" in2="blur" operator="over" />
        </filter>
      </defs>
      <line :x1="leaderLine.x1" :y1="leaderLine.y1" 
            :x2="leaderLine.x2" :y2="leaderLine.y2" 
            stroke="red" stroke-width="2" 
            stroke-dasharray="5,5"
            marker-end="url(#arrowhead)"
            filter="url(#glow)" />
      <circle :cx="leaderLine.x2" :cy="leaderLine.y2" r="4" fill="red" />
  </svg>
  
  <!-- Map Header Toolbar -->
  <div style="position:absolute; top:10px; right:10px; z-index:2000; display:flex; gap:10px;">
      <!-- REMOVED: Map Overlays Button (Moved to Side Tab) -->

      <button @click="toggleSelectionMode" :title="isBoxSelectMode ? 'Cancel Selection' : 'Select Area'" class="button button--flat" :style="isBoxSelectMode ? 'background:rgba(255,100,0,0.8); color:white;' : 'background:rgba(255,255,255,0.9); color:#333; box-shadow:0 2px 4px rgba(0,0,0,0.2);'">
          <i class="material-icons">{{ isBoxSelectMode ? 'close' : 'select_all' }}</i>
          <span style="margin-left:5px; font-weight:bold; font-size:12px; white-space:nowrap;">{{ isBoxSelectMode ? 'Cancel' : 'Select Area' }}</span>
      </button>
  </div>

  <!-- Back Button Overlay -->
  <div v-if="isFolderMode || route.query.overlay || overlaySiblings.length > 0" style="position: absolute; top: 10px; left: 60px; z-index: 2000;">
      <button @click="goBack" class="button button--flat" style="background: rgba(0,0,0,0.6); color: white; border: 1px solid rgba(255,255,255,0.3); backdrop-filter: blur(4px);">
          <i class="material-icons">arrow_back</i> Back to Folder
      </button>
  </div>
  <div v-if="showDebugInfo" style="position: absolute; bottom: 10px; left: 10px; background: rgba(0,0,0,0.7); color: white; padding: 10px; z-index: 9999; font-size: 12px; max-width: 300px; border-radius: 4px;">
      <b>Debug Info:</b><br>
      Zoom: {{ currentZoom }}<br>
      Coords: <span v-if="cursorCoords.lat <= 90">{{ cursorCoords.lat.toFixed(5) }}, {{ cursorCoords.lng.toFixed(5) }}</span><span v-else>--</span> 

      <div style="font-size: 10px; opacity: 0.7; margin-top: 5px;">
        Path: {{ route.query.path || "(Global)" }}<br>
        Source: {{ route.query.source || "None" }}
      </div>
      Total Clusters: {{ debugClusterCount }}<br>
      Status: {{ debugStatus }}
  </div>
  
  <!-- Collapsed Panel Tab REMOVED -->

  <!-- Map Overlays Side Tab (Always visible if panel is closed/collapsed or showing inspection) -->
  <div v-if="!showSidePanel || isPanelCollapsed || activeTab !== 'overlays'"
       @click="openOverlaysTab"
       title="Show Inspection Panel"
       class="overlays-tab"
       style="position:absolute; top:120px; right:0; z-index:2500; background-color:#2c3e50; color:white; padding:10px 4px 10px 8px; border-radius:6px 0 0 6px; cursor:pointer; box-shadow:-2px 2px 5px rgba(0,0,0,0.3); display:flex; align-items:center;">
       <i class="material-icons">layers</i>
  </div>

  <!-- Side Panel for Inspection -->
  <div v-if="showSidePanel" class="heatmap-side-panel" :class="{ 'collapsed': isPanelCollapsed }">
      <!-- Unified Header (Inline Styled for reliability) -->
      <div class="panel-header" :class="{ 'with-nav': currentInspectionFolder }" 
           style="display:flex; flex-direction:column; padding:8px 16px; background-color:#2c3e50; border-bottom:1px solid rgba(255,255,255,0.1); width:100%; box-sizing:border-box;">
          
          <!-- Row 1: Controls -->
          <div class="panel-controls-row" style="display:flex; width:100%; align-items:center; justify-content:space-between; margin-bottom:4px;">
              <button v-if="currentInspectionFolder" @click="backToFolders" class="back-btn" title="Back to Folder List"
                      style="background:transparent; border:none; color:#b0bec5; cursor:pointer; padding:6px; display:flex; align-items:center;">
                  <i class="material-icons" style="font-size:18px;">arrow_back</i>
                  <i class="material-icons" style="font-size:16px; margin-left:4px;">folder_copy</i>
              </button>
              
              <div class="spacer" style="flex:1;"></div>

              <button v-if="currentInspectionFolder && state.user.permissions.updateMap" 
                      @click="regenerateHeatmap(currentInspectionFolder.path, currentInspectionFolder.source)" 
                      class="panel-action-btn refresh" 
                      title="Force Re-scan"
                      style="background:transparent; border:none; color:#b0bec5; cursor:pointer; padding:6px; display:flex; align-items:center;">
                  <i class="material-icons" style="font-size:18px;">refresh</i>
              </button>
              
              <!-- Collapse Button REMOVED -->
              
              <button @click="closeSidePanel" class="panel-action-btn close" title="Close Panel"
                      style="background:transparent; border:none; color:#b0bec5; cursor:pointer; padding:6px; display:flex; align-items:center;">
                  <i class="material-icons" style="font-size:18px;">close</i>
              </button>
          </div>
          
          <!-- Row 2: Folder Name -->
          <div class="panel-folder-row" style="width:100%; text-align:center; padding-bottom:4px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;">
              <span v-if="!currentInspectionFolder" class="panel-title-text" style="font-weight:bold; font-size:1rem; color:white;">{{ sidePanelTitle }}</span>
              <span v-else class="panel-folder-name small" @click="navToFolder(currentInspectionFolder)" :title="currentInspectionFolder.path"
                    style="font-size:0.9rem; font-weight:500; cursor:pointer; color:#42a5f5; text-decoration:underline;">
                  {{ currentInspectionFolder.path.split('/').pop() }}
              </span>
              <div v-if="currentInspectionFolder && currentInspectionFolder.totalImageCount" style="font-size: 11px; opacity: 0.7; color: #ccc; margin-top: 2px;">
                  Total Folder Image Count: {{ currentInspectionFolder.totalImageCount }}
              </div>
          </div>
      </div>

      <div class="panel-content">
          <div v-if="sidePanelLoading" class="panel-loading">Loading...</div>
          
          <!-- Map Overlays Section -->
          <!-- Tab Navigation -->
          <div class="panel-tabs" style="display:flex; justify-content:space-around; background:#2c3e50; padding:5px 0 0 0; margin-bottom:0;">
              <div @click="activeTab = 'inspection'" title="Inspection"
                   :style="{ borderBottom: activeTab === 'inspection' ? '3px solid #fbc02d' : '3px solid transparent', opacity: activeTab === 'inspection' ? 1 : 0.6, flex: 1, textAlign:'center', padding:'8px', cursor:'pointer', color:'white' }">
                   <i class="material-icons" style="font-size: 24px; color: #fbc02d;">folder</i>
              </div>
              <div @click="activeTab = 'overlays'" title="Overlays"
                   :style="{ borderBottom: activeTab === 'overlays' ? '3px solid #4f83cc' : '3px solid transparent', opacity: activeTab === 'overlays' ? 1 : 0.6, flex: 1, textAlign:'center', padding:'8px', cursor:'pointer', color:'white', display:'flex', alignItems:'center', justifyContent:'center' }">
                   <i class="material-icons" style="font-size: 20px; color: #42a5f5; margin-right: 6px;">map</i>
                   <span style="font-weight: bold;">Overlays</span>
              </div>
          </div>
          
          <!-- Overlays View -->
          <div v-if="activeTab === 'overlays'" class="overlay-section" style="padding:10px;">
              <div style="margin-bottom:10px; display:flex; gap:10px; justify-content:center;">
                  <button @click="overlayMode = 'context'" :class="{active: overlayMode==='context'}" class="btn-toggle" style="padding:6px 12px; border-radius:4px; border:1px solid #555; background:transparent; color:white; cursor:pointer;" :style="overlayMode === 'context' ? 'background:#4f83cc; border-color:#4f83cc' : ''">Current Folder</button>
                  <button @click="overlayMode = 'all'" :class="{active: overlayMode==='all'}" class="btn-toggle" style="padding:6px 12px; border-radius:4px; border:1px solid #555; background:transparent; color:white; cursor:pointer;" :style="overlayMode === 'all' ? 'background:#4f83cc; border-color:#4f83cc' : ''">Show ALL</button>
              </div>

              <!-- Grouping by Folder -->
               <div v-for="(group, folderPath) in Object.groupBy(availableOverlays, o => o.path.substring(0, o.path.lastIndexOf('/')) || '/')" :key="folderPath" style="margin-bottom:15px;">
                   <!-- Path Header REMOVED to simplify list -->
                   <!-- <div style="font-weight:bold; color:#aaa; font-size:12px; margin-bottom:5px; border-bottom:1px solid #444;">{{ folderPath }}</div> -->
                  
                  <div v-for="item in group" :key="item.path" 
                       class="overlay-item" 
                       style="display:flex; align-items:center; padding:8px; cursor:pointer; border-radius:4px; margin-bottom:2px;"
                       :style="isOverlayActive(item.path) ? 'background:rgba(79, 131, 204, 0.3); border:1px solid #4f83cc;' : 'background:rgba(255,255,255,0.05);'"
                       @click="toggleOverlay(item.path)">
                       
                      <i class="material-icons" style="margin-right:10px; color:#42a5f5;">{{ isOverlayActive(item.path) ? 'check_box' : 'check_box_outline_blank' }}</i>
                      <div style="flex:1; overflow:hidden;">
                          <!-- Clean Name Display: Use Name if available, else filename -->
                          <div class="overlay-name" style="font-weight:500; font-size: 13px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;">
                             {{ item.name || item.path.split('/').pop() }}
                          </div>
                          <!-- Description Restored -->
                          <div v-if="item.description" style="font-size:11px; opacity:0.7; word-break: break-word;">
                              {{ item.description }}
                          </div>
                      </div>
                      
                      <!-- Info Icon with Tooltip via Title -->
                      <div :title="(item.description ? item.description + '\n' : '') + 'Path: ' + item.path"
                           style="padding: 4px; opacity: 0.5; cursor: help;"
                           @click.stop="showOverlayInfo(item)">
                          <i class="material-icons" style="font-size: 16px;">info_outline</i>
                      </div>
                  </div>
              </div>
              
              <div v-if="availableOverlays.length === 0" style="text-align:center; opacity:0.6; margin-top:20px;">
                  No overlays found.
              </div>
          </div>

          <!-- Inspection View (Existing Content wrapped) -->
          <div v-else-if="activeTab === 'inspection'">

          <div v-if="sidePanelData.length === 0 && !sidePanelLoading" class="panel-empty">
              <div v-if="inspectionHistory.length > 0" style="width: 100%; padding-bottom: 10px; border-bottom: 1px solid rgba(255,255,255,0.1); margin-bottom: 10px;">
                  <button @click="backToFolders" class="back-btn" title="Back to Folder List" style="display: flex; align-items: center; background: none; border: none; color: white; cursor: pointer;">
                      <i class="material-icons">arrow_back</i>
                      <span style="margin-left: 5px;">Back</span>
                  </button>
              </div>
              <span v-if="overlaySiblings.length === 0">No files found.</span>
              <span v-else style="font-size:12px; opacity:0.7;">Select an overlay above.</span>
          </div>
          
          <!-- Folder List View -->
          <div v-else-if="!currentInspectionFolder && sidePanelData.length > 0" class="panel-folder-list">
             <div v-for="grp in displayedFolders" :key="grp.path" class="folder-item" @click="openFolderView(grp)">
                <i class="material-icons">folder</i>
                <div class="folder-info">
                    <span class="folder-name">{{ grp.path.split('/').pop() }}</span>
                    <span class="folder-count">{{ grp.count }} items</span>
                </div>
                <i class="material-icons chevron">chevron_right</i>
             </div>
          </div>

          <!-- Image Grid (Inside Folder) -->
          <div v-else-if="currentInspectionFolder" class="panel-grid-container">
              <!-- Sticky header removed as it is now merged into main header -->
              
              <div class="panel-grid-grouped">
                  <div v-for="group in groupedDisplayedItems" :key="group.clusterID" class="cluster-group">
                      <div class="cluster-sidebar" 
                           :style="{ backgroundColor: group.color }" 
                           @click="zoomToLocation(group.lat, group.lon)" 
                           @mouseenter="drawLeaderLine($event, group.lat, group.lon)"
                           @mouseleave="clearLeaderLine"
                           title="Zoom to this cluster">
                          <i class="material-icons cluster-target-icon">my_location</i>
                      </div>
                      <div class="cluster-items-grid">
                          <div v-for="file in group.items" :key="file.path" class="panel-item" 
                               :class="{ 'active-preview': quickViewFile && (quickViewFile.path === file.path || quickViewFile.path.endsWith(file.path) || file.path.endsWith(quickViewFile.path)) }"
                               @click="openQuickView(file)">
                              <img :src="file.thumbUrl" class="panel-thumb" loading="lazy">
                              <span class="panel-item-name">{{ file.name }}</span>
                          </div>
                      </div>
                  </div>
              </div>
              <div v-if="displayedCount < currentInspectionFolder.items.length" style="padding: 10px; text-align: center;">
                  <button @click="loadMore" class="button button--flat" style="width: 100%; justify-content: center;">
                      Load More ({{ currentInspectionFolder.items.length - displayedCount }} remaining)
                  </button>
              </div>
          </div>
          </div> <!-- Close inspection tab -->
      </div>
  </div>

  <!-- Quick View Modal -->
  <!-- Enhanced Quick View Modal (Restored Layout) -->
  <div v-if="quickViewFile" class="quick-view-modal">
      <div class="quick-view-content" @click.stop 
           style="display:inline-flex; flex-direction:column; width:auto; max-width:90vw; max-height:90vh; background-color:rgba(59, 82, 206, 0.9); padding:6px; box-shadow:0 14px 40px rgba(0,0,0,0.8); border-radius:4px; box-sizing:border-box; position:relative;">
          
          <!-- Navigation Zones (Absolute Overlay) -->
          <div class="nav-zone nav-zone-left" @click="prevItem" title="Previous Image" style="position:absolute; top:0; left:0; width:33%; height:100%; z-index:10; cursor:pointer;"></div>
          <div class="nav-zone nav-zone-right" @click="nextItem" title="Next Image" style="position:absolute; top:0; right:0; width:33%; height:100%; z-index:10; cursor:pointer;"></div>

          <!-- Controls (Top, Restored Original Style) -->
          <div class="quick-view-header controls-only" style="width:100%; display:flex; justify-content:flex-end; padding-bottom:6px; z-index:20; position:relative;">
              <div class="quick-view-controls" style="padding:0; display:flex; gap:12px;">
                  <button @click="navToFile(quickViewFile)" class="qv-btn" title="Go to Image" style="background:transparent; border:none; color:white; cursor:pointer;"><i class="material-icons">image</i></button>
                  <button @click="navToFolder(quickViewFile)" class="qv-btn" title="Open Folder" style="background:transparent; border:none; color:white; cursor:pointer;"><i class="material-icons">folder</i></button>
                  <button @click="closeQuickView" class="qv-btn close" style="background:transparent; border:none; color:white; cursor:pointer;"><i class="material-icons">close</i></button>
              </div>
          </div>
          
          <!-- Main Image -->
          <img :src="quickViewFile.previewUrl" class="quick-view-img" 
               style="flex:1; display:block; width:auto; object-fit:contain; background:black; max-height:calc(90vh - 100px); z-index:5; position:relative;" />
          
          <!-- Footer (Text at bottom + Metadata) -->
          <div class="quick-view-footer"
               style="width:0; min-width:100%; box-sizing:border-box; padding:8px 4px 4px 4px; z-index:20; position:relative; pointer-events:none;">
               
               <!-- Metadata Section -->
               <div v-if="quickViewFile.metadata" style="margin-bottom:8px;">
                   <div v-if="quickViewFile.metadata.instructions" style="color:#ffca28; font-weight:bold; font-size:13px; text-shadow: 0 1px 2px black;">
                       {{ quickViewFile.metadata.instructions }}
                   </div>
                   <div style="color:#ddd; font-size:11px; text-shadow: 0 1px 2px black;">
                       <span v-if="quickViewFile.metadata.caption">{{ quickViewFile.metadata.caption }}</span>
                       <span v-if="quickViewFile.metadata.caption && quickViewFile.metadata.byline"> | </span>
                       <span v-if="quickViewFile.metadata.byline">Photo: {{ quickViewFile.metadata.byline }}</span>
                   </div>
               </div>

               <span class="quick-view-path" style="white-space:pre-wrap; word-break:break-word; color:white; font-size:0.9em; font-weight:500; line-height:1.3;">{{ quickViewFile.path }}</span>
          </div>
      </div>
  </div>
</template>

<script setup>
import { onMounted, onBeforeUnmount, watch, nextTick } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import L from 'leaflet';
import 'leaflet/dist/leaflet.css';
import 'leaflet.markercluster/dist/MarkerCluster.css';
import 'leaflet.markercluster/dist/MarkerCluster.Default.css';
import 'leaflet.markercluster';
import 'leaflet.heat';

import { fetchJSON } from "@/api/utils";
import { state } from "@/store";
import { ref, computed } from 'vue'; // Import ref and computed

import { notify } from "@/notify";
import { processDirectItems } from "@/utils/heatmapInspector";

const route = useRoute();
const router = useRouter();
let map = null;
let markers = null;
let heatLayer = null;
let tileLayer = null; // Store reference to tile layer
let overlayLayer = null; // GeoJSON Overlay Layer
const isFolderMode = ref(false);

// Abort controller for canceling tile requests
let abortController = null;

const debugClusterCount = ref(0);
const debugStatus = ref("Initializing...");
const currentZoom = ref(0);
const cursorCoords = ref({ lat: 0, lng: 0 });

// Side Panel State
const showSidePanel = ref(false);
const sidePanelTitle = ref("Location Inspector");
const sidePanelData = ref([]);
const formattedSidePanelData = ref([]); // Array of {path, count, items}
const currentInspectionFolder = ref(null); // Reference to currently viewing folder object
const overlaySiblings = ref([]); // Sibling GeoJSON files for overlay navigation
const sidePanelLoading = ref(false);
const isPanelCollapsed = ref(false);



const isBoxSelectMode = ref(false); // Box Selection Mode State
const selectionBox = ref({ visible: false, startX: 0, startY: 0, currentX: 0, currentY: 0, style: {} });
const itemsPerPage = 50;
const displayedCount = ref(itemsPerPage);
const leaderLine = ref({ visible: false, x1: 0, y1: 0, x2: 0, y2: 0, targetLat: null, targetLon: null, startEl: null, svgWidth: 0, svgHeight: 0 });
const leaderLineSvg = ref(null);

const displayedItems = computed(() => {
    // If viewing a folder, show its items
    if (currentInspectionFolder.value && currentInspectionFolder.value.items) {
        return currentInspectionFolder.value.items.slice(0, displayedCount.value);
    }
    // Otherwise show folders list? No, "groups" are needed in the template.
    return [];
});

const getClusterColor = (str) => {
    if (!str) return '#ccc';
    let hash = 0;
    for (let i = 0; i < str.length; i++) {
        hash = str.charCodeAt(i) + ((hash << 5) - hash);
    }
    const c = (hash & 0x00FFFFFF).toString(16).toUpperCase();
    return '#' + '00000'.substring(0, 6 - c.length) + c;
};


const recursiveDecode = (str) => {
    if (!str) return str;
    let decoded = str;
    let limit = 0;
    while (decoded.includes('%') && limit < 5) {
        try {
            const next = decodeURIComponent(decoded);
            if (next === decoded) break;
            decoded = next;
        } catch (e) { break; }
        limit++;
    }
    return decoded;
};

const groupedDisplayedItems = computed(() => {
    const items = displayedItems.value;
    if (!items || items.length === 0) return [];

    const groups = {};
    items.forEach(item => {
        const cid = item.clusterID || 'unknown';
        if (!groups[cid]) {
            groups[cid] = {
                clusterID: cid,
                color: getClusterColor(String(cid)),
                lat: 0,
                lon: 0,
                count: 0,
                items: []
            };
        }
        groups[cid].items.push(item);
        
        // DEBUG: Check where coordinates are
        // console.log("Debug Item Coords:", item.path, item.lat, item.lon, item.exif);

        // Handle various coordinate locations
        let lat = item.lat;
        let lon = item.lon;
        
        if (lat === undefined && item.exif) {
             lat = item.exif.latitude;
             lon = item.exif.longitude;
        }
        
        // If still undefined, we cannot use this item for centroid
        if (lat !== undefined && lon !== undefined) {
            groups[cid].lat += Number(lat);
            groups[cid].lon += Number(lon);
            groups[cid].count++;
        }
    });
    
    // Average coordinates
    return Object.values(groups).map(g => {
        if (g.count > 0) {
            g.lat = g.lat / g.count;
            g.lon = g.lon / g.count;
        }
        return g;
    });
});

// Dynamic Leader Line Logic
const updateVisibleMarkers = () => {
    // Placeholder to prevent ReferenceError. 
    // Logic for filtering sidebar items by map bounds can be added here if needed.
};

const drawLeaderLine = (event, lat, lon) => {
    if (!map || !lat || !lon) return;

    // Store target and element for dynamic updates
    leaderLine.value.targetLat = lat;
    leaderLine.value.targetLon = lon;
    leaderLine.value.startEl = event.currentTarget;

    // Show first, then update position after render
    leaderLine.value.visible = true;
    nextTick(() => {
        updateLeaderLine();
    });
};

const updateLeaderLine = () => {
   if (!leaderLine.value.visible || !leaderLine.value.startEl || !map) return;
   
   // 1. Ensure SVG matches map container size EXACTLY to avoid scaling
   const mapSize = map.getSize();
   leaderLine.value.svgWidth = mapSize.x;
   leaderLine.value.svgHeight = mapSize.y;

   // 2. Get Button and SVG Rects
   const buttonRect = leaderLine.value.startEl.getBoundingClientRect();
   const svgEl = leaderLineSvg.value;
   if (!svgEl) return;
   
   // FORCE SVG to be same size as map (if not updated by reactivity yet)
   // But we rely on Vue.
   const svgRect = svgEl.getBoundingClientRect();

   // 3. Start Point (Side Panel Button) - Relative to SVG
   const x1 = buttonRect.right - svgRect.left; 
   const y1 = (buttonRect.top + (buttonRect.height / 2)) - svgRect.top;

   // 4. Get Map Point logic
   // map.latLngToContainerPoint provides x/y relative to the map container.
   // We must convert this to "SVG Space" to account for any offset between the Map Div and the SVG.
   const point = map.latLngToContainerPoint([leaderLine.value.targetLat, leaderLine.value.targetLon]);
   const mapRect = map.getContainer().getBoundingClientRect();
   
   // Map Container Point -> Screen Point -> SVG Point
   const screenX = point.x + mapRect.left;
   const screenY = point.y + mapRect.top;
   
   leaderLine.value.x1 = x1;
   leaderLine.value.y1 = y1;
   leaderLine.value.x2 = screenX - svgRect.left;
   leaderLine.value.y2 = screenY - svgRect.top;
};

const clearLeaderLine = () => {
    leaderLine.value.visible = false;
    leaderLine.value.startEl = null;
    leaderLine.value.targetLat = null;
    leaderLine.value.targetLon = null;
};

const zoomToLocation = (lat, lon) => {
    if (map && lat && lon) {
        // Fly to location with moderate zoom
        map.flyTo([lat, lon], 16, {
            duration: 1.5
        });

        // Flash Effect
        const flashIcon = L.divIcon({
            className: 'zoom-flash-marker',
            iconSize: [40, 40],
            html: '<div class="zoom-flash-circle"></div>'
        });

        // Current duration is 1.5s. User wants flash 0.5s AFTER move ends.
        // Total delay = 1500ms + 500ms = 2000ms.
        setTimeout(() => {
            const flashMarker = L.marker([lat, lon], { icon: flashIcon }).addTo(map);

            // Remove after animation (1s)
            setTimeout(() => {
                map.removeLayer(flashMarker);
            }, 1200);
        }, 2000);
    }
};

const displayedFolders = computed(() => {
    // Top level list of folders
    return formattedSidePanelData.value; 
});

const inspectionHistory = ref([]); // Stack to store previous panel states

const backToFolders = () => {
    if (inspectionHistory.value.length > 0) {
        console.log("[Heatmap] Restoring previous panel state...");
        const prev = inspectionHistory.value.pop();
        sidePanelData.value = prev.data;
        formattedSidePanelData.value = prev.formatted;
        sidePanelTitle.value = prev.title;
        currentInspectionFolder.value = null; 
    } else {
        currentInspectionFolder.value = null;
    }
    // Ensure we switch to inspection tab
    activeTab.value = 'inspection';
};



const loadMore = () => {
    displayedCount.value += itemsPerPage;
};

const closeSidePanel = () => {
    showSidePanel.value = false;
    displayedCount.value = itemsPerPage; // Reset logic check
};

const openOverlaysPanel = () => {
    showSidePanel.value = true;
    activeTab.value = 'overlays';
    // Load overlays if not loaded
    fetchOverlays();
};

// Quick View State
const quickViewFile = ref(null);

// Overlay State
const activeTab = ref('inspection'); // 'inspection' or 'overlays'
const overlayMode = ref('context'); // 'context' (current folder) or 'all' (global)
const availableOverlays = ref([]); // List of overlay objects
const activeOverlayLayers = ref({}); // Map path -> Leaflet Layer
const showDebugInfo = ref(false); // Debug overlay visibility

// Watch for folder changes to fetch overlays (if in context mode)
watch([() => route.query.path, () => route.query.source, overlayMode], () => {
    if (activeTab.value === 'overlays') {
        fetchOverlays();
    }
}, { immediate: true });

// Watch tab changes
watch(activeTab, (val) => {
    if (val === 'overlays') {
        fetchOverlays();
    }
});

const fetchOverlays = async () => {
    let s = route.query.source || state.source;
    let p = route.query.path || "";
    
    // If we are inspecting a specific folder, use that as context
    if (currentInspectionFolder.value) {
        if (!p && currentInspectionFolder.value.path) {
            p = currentInspectionFolder.value.path;
        }
        // Use source from folder if available and valid
        if (currentInspectionFolder.value.source) {
             s = currentInspectionFolder.value.source;
        }
    }

    if (!s) {
        console.warn("[Heatmap] fetchOverlays: Source missing, skipping fetch.");
        return;
    }

    // Aggressively decode source and path to prevent double/triple encoding
    s = recursiveDecode(s);
    p = recursiveDecode(p);

    console.log(`[Heatmap] Fetching overlays for source='${s}', path='${p}', mode='${overlayMode.value}'`);

    sidePanelLoading.value = true;
    try {
        const mode = overlayMode.value;
        const url = `/api/heatmap/overlays?source=${encodeURIComponent(s)}&path=${encodeURIComponent(p)}&mode=${mode}`;
        // console.log("Fetching overlays: " + url);
        const res = await fetch(url);
        if (res.ok) {
            const rawBody = await res.text();
            console.log("[Heatmap] Fetch Overlays Raw Response:", rawBody);
            const data = JSON.parse(rawBody);
            if ((!data.overlays || data.overlays.length === 0) && !url.includes('scan=true')) {
                 // Optimization: If no overlays found, maybe they are not generated yet?
                 // Trigger a scan and retry once.
                 console.log("No overlays found, triggering on-demand scan...");
                 const scanUrl = url + "&scan=true";
                 const res2 = await fetch(scanUrl);
                 if (res2.ok) {
                     const data2 = await res2.json();
                     availableOverlays.value = data2.overlays || [];
                     return;
                 }
            }
            availableOverlays.value = data.overlays || [];
        } else {
             if (res.status === 404) {
                 // 404 is expected if no overlays exist for this path
                 availableOverlays.value = [];
             } else {
                 console.error("Fetch overlays failed: " + res.status);
                 availableOverlays.value = [];
             }
        }
    } catch (e) {
        console.error("Failed to fetch overlays", e);
    } finally {
        sidePanelLoading.value = false;
    }
};

const toggleOverlay = async (path, providedSource = null) => {
    // Check if active
    if (activeOverlayLayers.value[path]) {
        // Remove
        if (map) {
            map.removeLayer(activeOverlayLayers.value[path]);
        }
        delete activeOverlayLayers.value[path];
    } else {
        // Add
        if (Object.keys(activeOverlayLayers.value).length >= 5) {
            notify.show("Maximum 5 overlays allowed.", 'error');
            return;
        }

        let overlay = availableOverlays.value.find(o => o.path === path);
        
        // If not in available list (e.g. initial load from share link), construct a virtual overlay object
        if (!overlay && providedSource) {
            overlay = {
                path: path,
                source: providedSource
            };
        }

        if (!overlay) return;

        // Fetch GeoJSON content
        // Uses /api/raw which expects: source=<source>&path=<path_relative_to_source>
        // overlay.path comes from backend as relative to source (e.g. "2019/Trip.geojson")
        // We ensure it starts with / for api compatibility if needed.
        let rawPath = overlay.path;
        if (!rawPath.startsWith('/')) {
            rawPath = '/' + rawPath;
        }

        try {
            // AGGRESSIVE DECODE: Prevent double/triple encoding issues from shared links
            const cleanSource = recursiveDecode(overlay.source);
            const cleanPath = recursiveDecode(rawPath);

            // Construct request for /api/raw
            // Format: files=SOURCE::PATH
            const fileSpec = `${cleanSource}::${cleanPath}`;
            const url = `/api/raw?files=${encodeURIComponent(fileSpec)}`;
            
            // 1. Try to fetch Sidecar Style File first
            let styleConfig = null;
            try {
                const stylePath = cleanPath + ".style.json";
                const styleSpec = `${cleanSource}::${stylePath}`;
                const styleUrl = `/api/raw?files=${encodeURIComponent(styleSpec)}`;
                const styleRes = await fetch(styleUrl);
                if (styleRes.ok) {
                    styleConfig = await styleRes.json();
                    console.log("[Heatmap] Loaded sidecar style for:", path);
                }
            } catch (e) { /* Ignore style load errors */ }

            console.log("Loading overlay from: " + url);
            
            const res = await fetch(url);
            if (!res.ok) throw new Error("Failed to fetch GeoJSON: " + res.status);
            
            const geojson = await res.json();

            // Helper for Feature Styling (Copied from loadOverlay)
            const getStyleForFeature = (feature) => {
                let finalStyle = { color: "#ff7800", weight: 5, opacity: 0.65 };

                if (!styleConfig) {
                    if (feature.properties && feature.properties.color) {
                            finalStyle.color = feature.properties.color;
                    }
                    // Simple fallback if user defined stroke in properties (like standard geojson.io)
                    if (feature.properties && feature.properties.stroke) {
                        finalStyle.color = feature.properties.stroke;
                    }
                    if (feature.properties && feature.properties['stroke-width']) {
                        finalStyle.weight = feature.properties['stroke-width'];
                    }
                    return finalStyle;
                }
                
                // 1. Apply Global Default
                if (styleConfig.default) {
                    finalStyle = { ...finalStyle, ...styleConfig.default };
                }
                
                // 2. Apply Rules
                if (styleConfig.rules && Array.isArray(styleConfig.rules)) {
                    for (const rule of styleConfig.rules) {
                        if (!rule.if || !rule.style) continue;
                        
                        let match = true;
                        // Simple property match
                        if (rule.if.property) {
                             if (feature.properties[rule.if.property] !== rule.if.value) {
                                 match = false;
                             }
                        }
                        
                        // ID match? - Add if needed, simplified for now
                        
                        if (match) {
                            finalStyle = { ...finalStyle, ...rule.style };
                        }
                    }
                }
                return finalStyle;
            };

            // Add to map with custom style and popup
            const layer = L.geoJSON(geojson, {
                style: getStyleForFeature,
                pointToLayer: function(feature, latlng) {
                    const props = feature.properties || {};
                    
                    // 1. Circle Marker
                    if (props.markerType === 'circle') {
                        const style = {
                            radius: props.radius || 6,
                            fillColor: props.fill || "#ff7800",
                            color: props.stroke || "#000",
                            weight: props.strokeWidth || 1,
                            opacity: props.strokeOpacity || 1,
                            fillOpacity: props.fillOpacity || 0.8
                        };
                        return L.circleMarker(latlng, style);
                    }
                    
                    // 2. Custom Icon
                    const iconUrl = props.icon || props.iconUrl;
                    if (iconUrl) {
                        // Determine Base Size
                        let w = 32; 
                        let h = 32;
                        
                        if (props.iconSize) {
                            w = props.iconSize[0];
                            h = props.iconSize[1];
                        }
                        
                        // Handle KML-style icon-scale
                        if (props['icon-scale']) {
                            const scale = parseFloat(props['icon-scale']);
                            w = w * scale;
                            h = h * scale;
                        }

                        // Anchor
                        let anchor = [w/2, h]; // Default bottom-center
                        if (props.iconAnchor) {
                            anchor = props.iconAnchor;
                        }
                        
                        const myIcon = L.icon({
                            iconUrl: iconUrl,
                            iconSize: [w, h],
                            iconAnchor: anchor,
                            popupAnchor: [0, -h]
                        });
                        return L.marker(latlng, {icon: myIcon});
                    }

                    // 3. Default Marker
                    return L.marker(latlng);
                },
                onEachFeature: function (feature, layer) {
                     if (feature.properties) {
                         let content = "";
                         
                         // Robust Title / Name Check
                         const name = feature.properties.name || feature.properties.Name || feature.properties.title || feature.properties.Title;
                         if (name) {
                             content += `<div style='font-weight:bold'>${name}</div>`;
                         }
                         
                         // Robust Description Check
                         const desc = feature.properties.description || feature.properties.Description || feature.properties.desc || feature.properties.Desc;
                         if (desc) {
                             content += `<div style='margin-top:4px; max-height:200px; overflow-y:auto'>${desc}</div>`;
                         }
                         
                         if (content) {
                             layer.bindPopup(content);
                         }
                     }
                }
            }).addTo(map);

            activeOverlayLayers.value[path] = layer;
            
            // Zoom to it if it's the first one?
            if (Object.keys(activeOverlayLayers.value).length === 1) {
                map.fitBounds(layer.getBounds());
            }
        } catch (e) {
            notify.showError("Failed to load overlay: " + e.message);
        }
    }
};

const isOverlayActive = (path) => !!activeOverlayLayers.value[path];

const metadataCache = new Map(); // Cache for IPTC/Instructions

const fetchMetadata = async (file) => {
    if (!file || !file.path) return null;
    if (metadataCache.has(file.path)) return metadataCache.get(file.path);

    // Using existing api/files.js functionality logic manually to avoid importing full heavy object if possible.
    // Or just use the global `files` API if available?
    // We'll use a direct fetch to /api/files/metadata or similar?
    // Actually, `files.js` has `get(path)`.
    
    // Construct Path
    let reqPath = file.path;
    // ensure leading slash
    if (!reqPath.startsWith('/')) reqPath = '/' + reqPath;
    
    // API expects /api/metadata?path={path}&source={source}
    const source = file.source || state.source;
    // Construct URL for /api/metadata
    // Note: getMetadataHandler expects 'path' and 'source' query params
    const url = `/api/metadata?path=${encodeURIComponent(reqPath)}&source=${encodeURIComponent(source)}`;
    
    try {
        const res = await fetch(url);
        if (res.ok) {
            const data = await res.json();
            console.log("[QuickView] Metadata Raw:", data); // DEBUG
            // Extract IPTC and Instructions
            // Structure: { exif: { ... }, iptc: { ... }, ... }
            const meta = {
                instructions: data.instructions || 
                              (data.iptc ? (data.iptc.SpecialInstructions || data.iptc.Instructions) : "") || 
                              (data.xmp ? (data.xmp["photoshop:Instructions"] || data.xmp.Instructions) : "") || "", // Check XMP too
                byline: (data.iptc ? (data.iptc.Byline || data.iptc["By-line"] || data.iptc.Artist) : "") || 
                        (data.exif ? data.exif.Artist : "") || "",
                caption: (data.iptc ? (data.iptc.Caption || data.iptc["Caption-Abstract"] || data.iptc.Description) : "") || "",
            };
            console.log("[QuickView] Metadata Parsed:", meta); // DEBUG
            metadataCache.set(file.path, meta);
            return meta;
        }
    } catch (e) {
        console.warn("Metadata fetch failed", e);
    }
    return null;
};

// Prefetch Window
const prefetchMetadata = async (currentIndex) => {
    const list = sidePanelData.value;
    if (!list || list.length === 0) return;

    // Prefetch Next 5
    for (let i = 1; i <= 5; i++) {
        const idx = currentIndex + i;
        if (idx < list.length) {
            const item = list[idx];
             if (!metadataCache.has(item.path)) {
                 fetchMetadata(item); // Fire and forget
             }
        }
    }
    // Prefetch Prev 5
    for (let i = 1; i <= 5; i++) {
        const idx = currentIndex - i;
        if (idx >= 0) {
            const item = list[idx];
             if (!metadataCache.has(item.path)) {
                 fetchMetadata(item); // Fire and forget
             }
        }
    }
};

const updateQuickViewMetadata = async () => {
    if (!quickViewFile.value) return;
    
    // Set loading state or clear previous?
    quickViewFile.value.metadata = null;
    
    const meta = await fetchMetadata(quickViewFile.value);
    if (quickViewFile.value && meta) {
        // Ensure we are still viewing the same file
        quickViewFile.value.metadata = meta;
    }
};

const openQuickView = async (file) => {
    console.log("[QuickView] Opening:", file.path); // DEBUG
    quickViewFile.value = {
        ...file,
        previewUrl: getPreviewUrl(file.path, file.source, 'large'),
        metadata: metadataCache.get(file.path) || null
    };
    console.log("[QuickView] Initial Metadata:", quickViewFile.value.metadata); // DEBUG
    
    // Find index
    const index = sidePanelData.value.findIndex(f => f.path === file.path);
    if (index >= 0) {
        prefetchMetadata(index);
    }
    
    if (!quickViewFile.value.metadata) {
        updateQuickViewMetadata();
    }
    
    // Scroll active thumbnail into view
    nextTick(() => {
        const activeEl = document.querySelector('.panel-item.active-preview');
        if (activeEl) {
            activeEl.scrollIntoView({ behavior: 'smooth', block: 'center' });
        }
    });
};

const closeQuickView = () => {
    quickViewFile.value = null;
};

const nextItem = (e) => {
    if (e) e.stopPropagation();
    if (!quickViewFile.value) return;
    
    const list = sidePanelData.value;
    const idx = list.findIndex(f => f.path === quickViewFile.value.path);
    if (idx < list.length - 1) {
        openQuickView(list[idx + 1]);
    }
};

const prevItem = (e) => {
    if (e) e.stopPropagation();
    if (!quickViewFile.value) return;
    
    const list = sidePanelData.value;
    const idx = list.findIndex(f => f.path === quickViewFile.value.path);
    if (idx > 0) {
        openQuickView(list[idx - 1]);
    }
};

const navToFile = (file, editMode = false) => {
    // Navigate to Preview
    let p = file.path;
    if (p.startsWith('/')) p = p.substring(1);
    
    // Logic for path construction similar to heatmapNavigate
    let navPath = '/files/';
    if (state.serverHasMultipleSources && file.source) {
         navPath += file.source + '/';
         if (p.startsWith(file.source + '/')) {
             p = p.substring(file.source.length + 1);
         }
    }
    navPath += p;
    
    // If edit mode requested, we might pass query param or just rely on user clicking edit
    // But Preview.vue opens in view mode by default.
    // For now, just go to file.
    router.push(navPath).catch(console.error);
};

const navToFolder = (file) => {
    // If called with a folder object (has 'items' property), navigate to that folder
    // Otherwise, navigate to the parent folder of the file
    const targetPath = file.items ? file.path : (file.parentPath || "/");
    window.heatmapNavigate(targetPath, file.source, false);
};

const openFolderView = async (folderGroup) => {
    console.log("[Heatmap] openFolderView called for:", folderGroup);
    
    // Logic: 
    // Always drill down to fetch the full folder contents.
    // The current 'items' list only contains the representative items from the parent cluster.
    // To see ALL items (e.g. 28 items), we MUST fetch from backend.
    
    const targetPath = folderGroup.path;
    // CRITICAL FIX: Ensure source is never undefined
    const targetSource = folderGroup.source || state.source || route.query.source || "";
    
    // Explicitly Drill Down
    console.log(`[Heatmap] Force drilling down into: ${targetPath} Source: ${targetSource} (v12-Debug)`);
    console.log(`[Heatmap] Group Items:`, folderGroup.items);
    
    if (targetSource === "undefined" || !targetSource) {
         console.error("[Heatmap] Source is MISSING for drill-down. This will likely fail.");
         // Try to find source from items?
         if (folderGroup.items && folderGroup.items.length > 0) {
             const s = folderGroup.items[0].source;
             if (s) {
                 console.log("[Heatmap] Recovered source from items:", s);
                 // use s... but const is immutable. Reworking logic below.
             }
         }
    }
    
    // If we have a cluster ID, pass it for precision
    const extraContext = {};
    
    // SEARCH for a valid Cluster ID in the group items
    // The first item might be a warning or have missing ID, so check them all.
    let cID = folderGroup.clusterID;
    console.log(`[Heatmap] Initial Group ID:`, cID);
    
    // DEBUG: Print the first few items to see their structure
    if (folderGroup.items && folderGroup.items.length > 0) {
        console.log(`[Heatmap] Item 0 Structure:`, JSON.stringify(folderGroup.items[0]));
        if (folderGroup.items.length > 1) {
            console.log(`[Heatmap] Item 1 Structure:`, JSON.stringify(folderGroup.items[1]));
        }
    }


    // Check for Cluster IDs (Single or Multiple)
    let finalCID = null;

    if (folderGroup.clusterIDs && folderGroup.clusterIDs.size > 0) {
        // Support for Multiple IDs (Backend now supports comma-separated)
        const ids = Array.from(folderGroup.clusterIDs);
        finalCID = ids.join(',');
    } else if (folderGroup.clusterID) {
        // Fallback for older logic / direct items
        finalCID = folderGroup.clusterID;
    }

    if (finalCID && finalCID !== "warning-all-files") {
         console.log(`[Heatmap] Drilling down enabled for Cluster(s): ${finalCID}`);
         
         // SAVE STATE to History before navigating
         inspectionHistory.value.push({
             data: [...sidePanelData.value],
             formatted: [...formattedSidePanelData.value],
             title: sidePanelTitle.value
         });

         // Use Inspect API to show items from these clusters. KEEP HISTORY.
         // Pass totalImageCount from the folder group so it can be preserved
         inspectLocation(targetPath, targetSource, null, finalCID, true, folderGroup.totalImageCount);
    } else {
         // No ID -> Fallback to Resource API (Show All)
         console.log(`[Heatmap] Drilling down via Resource API (Show All) for: ${targetPath}`);
         
         inspectionHistory.value.push({
             data: [...sidePanelData.value],
             formatted: [...formattedSidePanelData.value],
             title: sidePanelTitle.value
         });

         inspectLocation(targetPath, targetSource, null, null, true, folderGroup.totalImageCount);
    }
};

// Helper to get preview URL
const getPreviewUrl = (path, source, size) => {
     if (!path || path.includes("Warning:")) return ""; // Prevent 500s for warning items
     const params = new URLSearchParams();
     params.append('path', path);
     if (source) params.append('source', source);
     params.append('size', size);
     const url = '/api/preview?' + params.toString();
     // console.log(`[Heatmap] getPreviewUrl: path='${path}', source='${source}', size='${size}' => ${url}`);
     return url;
};

const isImageFile = (filename) => {
    if (!filename) return false;
    const ext = filename.split('.').pop().toLowerCase();
    return ['jpg', 'jpeg', 'png', 'gif', 'webp', 'bmp', 'heic'].includes(ext);
};


const inspectLocation = async (path, sourceArg, directItems = null, coords = null, keepHistory = false, preservedTotalImageCount = null) => {
    console.log(`[Heatmap] inspectLocation called: path='${path}', source='${sourceArg}', keepHistory=${keepHistory}, preservedTotalImageCount=${preservedTotalImageCount}`);
    
    showSidePanel.value = true;
    isPanelCollapsed.value = false; // Ensure expanded
    activeTab.value = 'inspection'; // Force Inspection Tab
    sidePanelLoading.value = true;
    
    // Manage History
    if (!keepHistory) {
        inspectionHistory.value = [];
    }

    sidePanelData.value = [];
    currentInspectionFolder.value = null; // Reset folder view
    formattedSidePanelData.value = []; // Reset grouped data
    displayedCount.value = itemsPerPage; // Reset pagination
    
    // Direct Items (e.g. from Box Select)
    if (directItems && directItems.length > 0) {
         console.log(`[Heatmap] Loading ${directItems.length} direct items`);
         
         const { sidePanelData: spd, formattedSidePanelData: fspd, singleGroup } = processDirectItems(directItems, getPreviewUrl);
         
         sidePanelData.value = spd;
         formattedSidePanelData.value = fspd;
         
         if (singleGroup) {
             console.log("[Heatmap] Single folder via direct items (Auto-Drill).");
             openFolderView(singleGroup);
             return;
         }

         sidePanelTitle.value = `Selected Items (${sidePanelData.value.length})`;
         sidePanelLoading.value = false;
         return;
    }
    
    // Exact location inspection via Backend API (Tile Clusters)
    if (coords) {
         try {
            // Include Zoom Level for radius search
            const currentZoom = map.getZoom();
            // Aggressive Decode incoming parameters to prevent double-encoding
            const cleanSource = recursiveDecode(sourceArg || '');
            const cleanPath = recursiveDecode(path || '');
            
            let url = `/api/heatmap/inspect?zoom=${currentZoom}&source=${encodeURIComponent(cleanSource)}&path=${encodeURIComponent(cleanPath)}`;

            // Handle Coordinates or ID
            if (typeof coords === 'string') {
                 // It's a Cluster ID (or comma list)
                 url += `&cluster_id=${encodeURIComponent(coords)}`;
                 // Add dummy lat/lon to satisfy strict backend validation (until backend is relaxed)
                 url += `&lat=0&lon=0`;
            } else if (coords) {
                 // It's an object {lat, lon, minLat...}
                 url += `&lat=${coords.lat}&lon=${coords.lon}`;
                 if (coords.minLat !== undefined) {
                     url += `&minLat=${coords.minLat}&maxLat=${coords.maxLat}&minLon=${coords.minLon}&maxLon=${coords.maxLon}`;
                 }
                 if (coords.clusterID) {
                     url += `&cluster_id=${encodeURIComponent(coords.clusterID)}`;
                 }
            }

            console.log(`[Heatmap] inspectLocation called: path='${cleanPath}', source='${cleanSource}', keepHistory=${keepHistory}, preservedTotalImageCount=${preservedTotalImageCount}`);
            console.log(`[Heatmap] Inspect API URL: ${url}`);

            const res = await fetch(url);
            if (res.ok) {
                const rawText = await res.text();
                console.log('[Heatmap] Inspect Raw Response:', rawText);
                const items = JSON.parse(rawText);
                if (items && items.length > 0) {
                     console.log('[Heatmap] Parsed Item 0 ID:', items[0].id);
                     sidePanelData.value = items.map(item => ({
                        name: item.previewID || item.path.split('/').pop(),
                        path: item.path,
                        source: item.source || cleanSource,
                        parentPath: item.path.substring(0, item.path.lastIndexOf('/')), // Grouping Key
                        thumbUrl: getPreviewUrl(item.path, item.source || cleanSource, 'small'),
                        // Drill-down fields
                        type: item.type,
                        count: item.count,
                        clusterID: item.id,
                        lat: item.lat,
                        lon: item.lon,
                        totalImageCount: item.totalImageCount
                    }));
                    if (sidePanelData.value.length > 0) {
                        console.log('[Heatmap] Mapped SidePanel Item 0 ClusterID:', sidePanelData.value[0].clusterID);
                    }
                    
                    // GROUPING LOGIC:
                    // Create formatted data: List of Folders
                    const groups = {};
                    sidePanelData.value.forEach(item => {
                        // Virtual Folder Logic:
                        // If the item returned is ITSELF a folder (drill-down node),
                        // treat it as a top-level group that can be clicked.
                        if (item.type === 'folder') {
                            groups[item.path] = { 
                                path: item.path, 
                                count: item.count, 
                                items: [], 
                                isVirtual: true,
                                clusterID: item.clusterID,
                                source: item.source,
                                lat: item.lat,
                                lon: item.lon
                            };
                        } else {
                            const p = item.parentPath || "Root";
                            // CAPTURE CLUSTER ID from the item so we can drill down accurately
                            if (!groups[p]) groups[p] = { 
                                path: p, 
                                count: 0, 
                                items: [], 
                                clusterID: item.clusterID,
                                source: item.source,
                                lat: item.lat,
                                lon: item.lon
                            };
                            // Robust count: If item has count > 1, use that. Otherwise count as 1 item.
                            groups[p].count += (item.count && item.count > 1 ? item.count : 1);
                            groups[p].items.push(item);

                            if (item.totalImageCount) {
                                groups[p].totalImageCount = item.totalImageCount;
                            }
                        }
                    });
                    
                    // Apply preserved totalImageCount if not already set from items
                    if (preservedTotalImageCount) {
                        Object.values(groups).forEach(group => {
                            if (!group.totalImageCount) {
                                group.totalImageCount = preservedTotalImageCount;
                            }
                        });
                    }
                    
                    formattedSidePanelData.value = Object.values(groups);
                    
                    // If only one folder, auto-open it?
                    // ONLY if it's NOT a virtual folder (virtual folders require click to drill)
                    if (formattedSidePanelData.value.length === 1 && !formattedSidePanelData.value[0].isVirtual) {
                        console.log('[Heatmap] Auto-opening folder. TotalImageCount:', formattedSidePanelData.value[0].totalImageCount);
                        currentInspectionFolder.value = formattedSidePanelData.value[0];
                    }
                    
                    sidePanelTitle.value = `Inspection (${sidePanelData.value.length})`;
                    sidePanelLoading.value = false;
                    return;
                } else {
                     // API returned empty list - DO NOT FALLBACK to folder view
                     console.log("Inspect returned 0 items");
                     sidePanelData.value = [];
                     sidePanelTitle.value = "No files in cluster";
                     sidePanelLoading.value = false;
                     return;
                }
            } else {
                 console.warn("Inspect API failed:", res.status);
                 sidePanelTitle.value = "Inspection Failed";
                 sidePanelLoading.value = false;
                 return;
            }
         } catch (e) {
             console.error("Exact inspect failed:", e);
             sidePanelTitle.value = "Error";
             sidePanelLoading.value = false;
             return;
         }
    }
    
    // If we have direct items 
    if (directItems && directItems.length > 0) {
        sidePanelData.value = directItems.map(item => ({
            name: item.name || item.path.split('/').pop(),
            path: item.path,
            source: item.source,
            count: item.count,
            parentPath: item.path.substring(0, item.path.lastIndexOf('/')),
            thumbUrl: getPreviewUrl(item.path, item.source, 'small'),
            type: item.type,
            clusterID: item.clusterID || item.id, // Ensure we capture it
            lat: item.lat,
            lon: item.lon
        }));
        
        // Grouping
        const groups = {};
        const visitedClusters = new Set(); // Deduplication for Fan Markers

        sidePanelData.value.forEach(item => {
             const p = item.parentPath || "Root";
             if (!groups[p]) groups[p] = { 
                 path: p, 
                 count: 0, 
                 items: [],
                 clusterIDs: new Set(),
                 source: item.source,
                 lat: item.lat,
                 lon: item.lon       
             };
             
             if (item.clusterID) {
                 groups[p].clusterIDs.add(item.clusterID);
             }

             // Count Logic:
             // If item has a ClusterID, ensure we only add its 'count' ONCE per group.
             // This applies to both Fan Markers (leaves) AND Cluster Markers.
             if (item.clusterID) {
                 if (!visitedClusters.has(item.clusterID)) {
                     visitedClusters.add(item.clusterID);
                     groups[p].count += item.count;
                 }
             } else {
                 // No ID? Treat as individual file.
                 groups[p].count += (item.count || 1);
             }
             
             groups[p].items.push(item);
        });
        
        console.log("[Heatmap] SidePanel Groups:", groups);
        formattedSidePanelData.value = Object.values(groups);
        console.log("[Heatmap] Formatted SidePanel Data:", formattedSidePanelData.value);
        
        if (formattedSidePanelData.value.length === 1) {
            // Auto-Drill Down:
            // Since we selected a single folder via Box Select, we want to see the FILES,
            // not just the Cluster Markers we grabbed from the map.
            // Trigger the backend inspection for this group.
            console.log("[Heatmap] Single folder selected via box. Auto-drilling...");
            openFolderView(formattedSidePanelData.value[0]);
            return; // openFolderView handles the rest
        }

        sidePanelTitle.value = `Selected Items (${sidePanelData.value.length})`;
        sidePanelLoading.value = false;
        return;
    }

    sidePanelTitle.value = "Loading...";
    
    // Fallback logic for general folder inspection (no coordinates interaction)
    try {
        let targetPath = path;
        const cleanS = recursiveDecode(sourceArg || "");
        const apiUrl = `/api/resources?path=${encodeURIComponent(targetPath)}&source=${encodeURIComponent(cleanS)}`;
        const res = await fetchJSON(apiUrl);
        let items = res.items || [];
        if (!items.length && (res.files || res.folders)) {
             items = [...(res.folders || []), ...(res.files || [])];
        }

        if (items && items.length > 0) {
             sidePanelData.value = items
                .filter(item => !item.isDir && (item.type === 'image' || item.type === 'video' || isImageFile(item.name)))
                .map(item => ({
                    name: item.name,
                    path: item.path || (targetPath + "/" + item.name), 
                    source: item.source || cleanS,
                    parentPath: targetPath,
                    thumbUrl: getPreviewUrl(item.path || (targetPath + "/" + item.name), item.source || cleanS, 'small'),
                    type: item.type,
                    clusterID: item.id // Use id if available
                }));
             
             // Grouping
             const groups = {};
             sidePanelData.value.forEach(item => {
                 const p = item.parentPath || "Root";
                 if (!groups[p]) groups[p] = { path: p, count: 0, items: [] };
                 groups[p].count++;
                 groups[p].items.push(item);
            });
            formattedSidePanelData.value = Object.values(groups);
            
            if (formattedSidePanelData.value.length === 1) {
                currentInspectionFolder.value = formattedSidePanelData.value[0];
            }

             sidePanelTitle.value = `Location: ${targetPath} (${sidePanelData.value.length})`;
        } else {
             sidePanelTitle.value = "No files found";
        }
    } catch (e) {
        console.error("Failed to inspect location", e);
        sidePanelTitle.value = "Error loading files";
        notify.showError("Failed to load location files");
    } finally {
        sidePanelLoading.value = false;
    }
};

const regenerateHeatmap = async (targetPath, sourceVal) => {
    if (!confirm("Regenerate heatmap for " + targetPath + "? This may take a moment.")) return;
    
    try {
        const url = `/api/heatmap/regenerate?path=${encodeURIComponent(targetPath)}&source=${encodeURIComponent(sourceVal)}`;
        const res = await fetch(url, { method: 'POST' });
        if (res.ok) {
            notify.showSuccess("Heatmap regeneration started. Please wait.");
        } else {
             const txt = await res.text();
             notify.showError("Failed to start regeneration: " + txt);
        }
    } catch (e) {
        console.error(e);
        notify.showError("Error calling regenerate API");
    }
};





// Error tracking
const failedImageCount = ref(0);
const failedImages = ref(new Set());
let errorNotificationTimeout = null;

const showErrorNotification = () => {
    if (errorNotificationTimeout) clearTimeout(errorNotificationTimeout);
    errorNotificationTimeout = setTimeout(() => {
        if (failedImageCount.value > 0) {
            notify.showError(`Warning: ${failedImageCount.value} images failed to load (corrupted or unsupported).`);
            console.warn("--- HEATMAP FAILED IMAGES ---");
            failedImages.value.forEach(path => console.warn("Failed:", path));
            console.warn("-----------------------------");
        }
    }, 2000); // 2 second debounce
};

// Stores for dynamic reshuffling
const clusterDataStore = new Map(); // Key: `${lat},${lon}`, Value: Array of all points for that location
const renderedMarkerStore = new Map(); // Key: `${lat},${lon}`, Value: Array of L.Marker objects currently rendered for that location
const goBack = () => {
    // Handle Overlay Back Navigation
    // Priority: Explicit Overlay Param -> Implicit Overlay Context (Siblings) -> Path Param
    const effectiveOverlay = route.query.overlay || (overlaySiblings.value.length > 0 ? overlaySiblings.value[0].path : null);
    
    if (effectiveOverlay) {
        const ov = effectiveOverlay;
        const srcRaw = route.query.source || "";
        const src = recursiveDecode(srcRaw);

        // Get parent path
        let parent = ov.substring(0, ov.lastIndexOf('/'));
        if (parent === "") parent = "/";
        
        let cleanP = parent.startsWith('/') ? parent.substring(1) : parent;
        // Logic to remove source prefix if present
        if (src && cleanP.startsWith(src + '/')) {
             cleanP = cleanP.substring(src.length + 1);
        }
        
        let target = `/files/`;
        if (state.serverHasMultipleSources && src) {
            target += `${src}/`;
        }
        target += cleanP;
        
        console.log(`[Heatmap] goBack (Overlay) target='${target}' src='${src}'`);
        router.push({ path: target }).catch(err => console.error(err));
        return;
    }

    if (route.query.source && route.query.path) {
         const srcRaw = route.query.source;
         const src = recursiveDecode(srcRaw);
         
         const pRaw = route.query.path;
         const p = recursiveDecode(pRaw);

         // Ensure no double slashes if path starts with /
         let cleanP = p.startsWith('/') ? p.substring(1) : p;
         // Prevent double source in path (e.g. PHOTOS/PHOTOS/...)
         if (cleanP.startsWith(src + '/')) {
             cleanP = cleanP.substring(src.length + 1);
         }
         
         // Route push needs "path" to be browser URL path, not query path
         let navPath = '/files/';
         if (state.serverHasMultipleSources && src) {
             navPath += src + '/';
         }
         navPath += cleanP;
         
         console.log(`[Heatmap] goBack target='${navPath}' src='${src}'`);
         router.push({ path: navPath }).catch(err => console.error(err));
    } else {
         router.push({ path: '/files/' }).catch(err => console.error(err));
    }
};

// Helper: Generate consistent color from string (Folder Path)
const stringToColor = (str) => {
    if (!str) return '#ffffff';
    let hash = 0;
    for (let i = 0; i < str.length; i++) {
        hash = str.charCodeAt(i) + ((hash << 5) - hash);
    }
    const hue = Math.abs(hash % 360);
    // Fixed Saturation/Lightness for consistency and visibility
    return `hsl(${hue}, 85%, 55%)`;
};

// Helper to generate marker HTML
const generateMarkerIcon = (markerPath, sourceArg, count) => {
    if (markerPath) {
        const thumbUrl = '/api/preview?path=' + encodeURIComponent(markerPath) + '&source=' + encodeURIComponent(sourceArg) + '&size=small';
        const countOverlay = count > 1 ? `<span style="position:absolute; top:50%; left:50%; transform:translate(-50%, -50%); background:rgba(0,0,0,0.7); border-radius:10px; padding:1px 5px; color:white; font-size:11px; font-weight:bold; white-space:nowrap;">${count}</span>` : '';
                    
        // Calculate Border Color based on Folder
        let folderPath = "/";
        const lastSlash = markerPath.lastIndexOf('/');
        if (lastSlash > 0) {
            folderPath = markerPath.substring(0, lastSlash);
        } else if (lastSlash === 0) {
            folderPath = "/";
        }
        
        // Safety Clean: If markerPath somehow already has source prefixed (which it shouldn't for color but might for url),
        // we just use it for color hashing.
        const borderColor = stringToColor(folderPath);



        const html = `<div class="fan-thumb-container" style="border:3px solid ${borderColor} !important; box-shadow:0 2px 5px rgba(0,0,0,0.5); background-color: #555; width:48px !important; height:48px !important; border-radius:50%; overflow:hidden; position:relative; box-sizing:border-box;">
            <img src="${thumbUrl}" class="fan-thumb-img" style="width:100% !important; height:100% !important; max-width:100% !important; max-height:100% !important; object-fit:cover !important; border-radius:50%; display:block;" onerror="window.fileBrowserHeatmapImageError(this, '${markerPath.replace(/'/g, "\\'")}')" />
            ${countOverlay}
        </div>`;
        return L.divIcon({
            html: html,
            className: 'custom-fan-marker', 
            iconSize: new L.Point(48, 48),
            iconAnchor: [24, 24]
        });
    } else {
         return L.divIcon({
            html: `<div style="background-color:rgba(100,100,100,0.5);border-radius:50%;width:24px;height:24px;text-align:center;"><span style="color:white;text-shadow:0 0 2px black;line-height:24px;">${count}</span></div>`,
            className: 'marker-cluster marker-cluster-small',
            iconSize: new L.Point(24, 24)
        });
    }
};

// Helper to generate popup HTML with context
const generatePopupHtml = (markerPath, sourceArg, count, extraContext) => {
    // Context-aware logic
    const currentReqPath = route.query.path || "/"; // Access route directly here
    const clusterPath = markerPath || "";
    
    // Normalize slashes
    const normReq = currentReqPath.endsWith('/') ? currentReqPath : currentReqPath + '/';
    const isDescendant = clusterPath.startsWith(normReq) || (currentReqPath === "/" && clusterPath.startsWith("/"));
    
    let label = "Unknown";
    let subfolderName = "";
    let showThumbnail = false;
    
    if (isDescendant) {
        const relative = clusterPath.substring(normReq.length === 1 ? 0 : normReq.length); 
        const firstSlash = relative.indexOf('/');
        if (firstSlash === -1) {
            label = relative;
            showThumbnail = true;
        } else {
            subfolderName = relative.substring(0, firstSlash);
            label = subfolderName;
        }
    } else {
            label = clusterPath;
    }

    let popupHtml = `<b>${showThumbnail ? 'File' : 'Folder'}: ${label}</b><br>`;

    if (markerPath) {
            const s = sourceArg || "";
            const safeSource = s.replace(/'/g, "\\'");
            const safePath = markerPath.replace(/'/g, "\\'");
            
            // Calculate parent folder path for "Open Folder" option
            let parentPath = "";
            const lastSlash = markerPath.lastIndexOf('/');
            if (lastSlash > 0) {
                parentPath = markerPath.substring(0, lastSlash);
            } else {
                parentPath = "/";
            }
            const safeParentPath = parentPath.replace(/'/g, "\\'");

            const viewAction = `window.heatmapNavigate('${safePath}', '${safeSource}', false)`;
            const folderAction = `window.heatmapNavigate('${safeParentPath}', '${safeSource}', false)`;
            
            // Default Inspect Action (Global Folder Fallback)
            let inspectAction = `window.inspectLocationGlobal('${safeParentPath}', '${safeSource}')`;
            
            // PRECISE Inspect Action (Drill-Down)
            if (extraContext) {
                 const lat = extraContext.lat;
                 const lon = extraContext.lon;
                 const cid = extraContext.clusterID || ""; // Ensure string
                 // Pass bounds? If provided.
                 const boundsObj = {
                     clusterID: cid,
                     minLat: extraContext.minLat,
                     minLon: extraContext.minLon,
                     maxLat: extraContext.maxLat,
                     maxLon: extraContext.maxLon
                 };
                 const boundsJson = JSON.stringify(boundsObj).replace(/"/g, "&quot;");
                 // Use SINGLE QUOTES for string arguments to avoid breaking the double-quoted HTML attribute
                 inspectAction = `window.inspectLocationByCoords('${lat}', '${lon}', '${safePath}', '${safeSource}', ${boundsJson})`;
            }
            
            popupHtml += `<div style="display:flex; flex-direction:column; gap:5px; margin-top:5px;">`;
            popupHtml += `<button onclick="${viewAction}" class="button button--flat" style="width:100%;cursor:pointer;">View Image</button>`;
            popupHtml += `<button onclick="${folderAction}" class="button button--flat" style="width:100%;cursor:pointer;">Open Folder</button>`;
            
            popupHtml += `<button onclick="${inspectAction}" class="button button--flat" style="width:100%;cursor:pointer;background:rgba(0,100,200,0.3);">Inspect Files</button>`;
            
            if (count > 5) {
                 popupHtml += `<div style="font-size:10px; color:#aaa; margin-top:5px; text-align:center;">(Right-click / Long-press for options)</div>`;
            }
            
            popupHtml += `</div>`;
    }
    return popupHtml;
};



const fetchSiblings = async (parentPath, sourceVal) => {
    try {
        const apiUrl = `/api/resources?path=${encodeURIComponent(parentPath)}&source=${encodeURIComponent(sourceVal || "")}`;
        const res = await fetchJSON(apiUrl);
        let items = res.items || [];
            if (!items.length && (res.files || res.folders)) {
                items = [...(res.folders || []), ...(res.files || [])];
        }
        
        const geoFiles = items.filter(f => f.name.toLowerCase().endsWith('.geojson'));
        
        // Identify companion images
        const itemMap = new Map();
        items.forEach(f => itemMap.set(f.name, f));

        overlaySiblings.value = geoFiles.map(f => {
            const base = f.name.substring(0, f.name.lastIndexOf('.'));
            // Check if any image exists with this base name
            const exts = ['.jpg', '.jpeg', '.png', '.webp'];
            let companionName = null;
            for (const ext of exts) {
                if (itemMap.has(base + ext)) {
                    companionName = base + ext;
                    break;
                }
            }
            
            let thumbUrl = "";
            if (companionName) {
                    const path = parentPath + "/" + companionName;
                    thumbUrl = `/api/preview?path=${encodeURIComponent(path)}&source=${encodeURIComponent(sourceVal)}&size=small`;
            }

            return {
                name: f.name,
                path: parentPath + "/" + f.name,
                source: sourceVal,
                thumbUrl: thumbUrl
            };
        });
        
    } catch (e) {
        console.error("fetchSiblings failed", e);
    }
};

const loadOverlay = async () => {
    let overlayInput = route.query.overlay;
    let source = route.query.source;
    if (!overlayInput || !source) return;

    overlayInput = recursiveDecode(overlayInput);
    source = recursiveDecode(source);

    console.log("[Heatmap] Auto-loading overlay requested:", overlayInput);

    // Wait for availableOverlays to be populated (it might still be fetching)
    let attempts = 0;
    while (availableOverlays.value.length === 0 && attempts < 10 && sidePanelLoading.value) {
        await new Promise(r => setTimeout(r, 500));
        attempts++;
    }

    // Reconciliation: Shared Link might provide Name or Path
    let targetOverlay = availableOverlays.value.find(o => o.path === overlayInput || o.name === overlayInput);

    if (targetOverlay) {
        console.log("[Heatmap] Resolved overlay to path:", targetOverlay.path);
        toggleOverlay(targetOverlay.path, source);
    } else {
        console.warn("[Heatmap] Could not resolve overlay:", overlayInput);
        // Fallback to direct path if it looks like one
        if (overlayInput.includes('/') || overlayInput.endsWith('.geojson')) {
             toggleOverlay(overlayInput, source);
        }
    }
};


// Watch for overlay changes
watch(() => route.query.overlay, () => {
    loadOverlay();
});

// Clear overlay siblings if context changes (Path or Source)
watch(() => [route.query.path, route.query.source], (newVals, oldVals) => {
    // Explicitly check for changes to avoid triggering on referential changes
    const [newPath, newSource] = newVals;
    const [oldPath, oldSource] = oldVals || [];
    
    if (newPath !== oldPath || newSource !== oldSource) {
        // FIX: Don't clear if we have an overlay active, as that might be driving the view
        if (!route.query.overlay) {
                console.log("[Heatmap] Context changed and no overlay, clearing overlay siblings.");
                overlaySiblings.value = [];
        }
    }
});

// Watch for major context changes in URL (Source or Path)
// This ensures we react when redirected from Login or when navigating via browser history
watch(() => [route.query.source, route.query.path], () => {
    console.log("[Heatmap] Route query changed - source/path update detected. Re-loading data.");
    if (map) {
        loadData();
        fetchOverlays();
    }
});

const initMap = async () => {
    if (map) return;
    await nextTick();
    
    // Ensure container exists
    const container = document.getElementById('heatmap-container');
    if (!container) return;

    map = L.map('heatmap-container', {
        worldCopyJump: true // Enable infinite scrolling
    });
    


    // GeoJSON Overlay Layer definitions moved to top level


    map.setView([0, 0], 2);
    
    // Invalidate size to ensure it knows its dimensions
    map.invalidateSize();

    // Debug Listeners: Update zoom and coordinates
    currentZoom.value = map.getZoom();
    map.on('zoomend', () => {
        currentZoom.value = map.getZoom();
    });
    map.on('mousemove', (e) => {
        // Wrap for infinite scrolling
        const wrapped = e.latlng.wrap();
        
        // Strict Display Bounds
        if (Math.abs(wrapped.lat) <= 90 && Math.abs(wrapped.lng) <= 180) {
            cursorCoords.value = wrapped;
            // Also update validity flag if implemented, or just use null checks in template
        } else {
             // Out of bounds - maybe set to null or a specific flag to hide?
             // Since we use cursorCoords for display, let's keep it but template will hide it?
             // Or set a flag "coordsValid"
             // Simplest: Check bounds in template or set to null? 
             // Ref is object, cannot set to null easily without breaking prop access.
             // Let's set lat/lng to > 999 to indicate invalid? 
             // Or better:
             cursorCoords.value = { lat: 999, lng: 999 };
        }
    });
    map.on('click', (e) => {
        const wrapped = e.latlng.wrap();
        cursorCoords.value = wrapped;
    });
    
    // Right-click / Long-press Context Menu for Copying
    map.on('contextmenu', (e) => {
        const lat = e.latlng.lat.toFixed(5);
        const lng = e.latlng.lng.toFixed(5);
        const zoom = map.getZoom();
        
        L.popup()
            .setLatLng(e.latlng)
            .setContent(`
                <div style="text-align:center; font-size:12px;">
                    <b>Lat:</b> ${lat}<br>
                    <b>Lon:</b> ${lng}<br>
                    <div style="display:flex; flex-direction:column; gap:5px; margin-top:5px;">
                        <button onclick="window.toggleDebugInfo()" 
                            class="button button--flat" 
                            style="padding:2px 8px; font-size:11px; cursor:pointer;">
                            Toggle Debug Window
                        </button>
                        <button onclick="window.copyLeafletCoords('${lat}', '${lng}')" 
                            class="button button--flat" 
                            style="padding:2px 8px; font-size:11px; cursor:pointer;">
                            Copy Coordinates
                        </button>
                        <button onclick="window.copyText('${lat}')" 
                            class="button button--flat" 
                            style="padding:2px 8px; font-size:11px; cursor:pointer;">
                            Copy Latitude
                        </button>
                        <button onclick="window.copyText('${lng}')" 
                            class="button button--flat" 
                            style="padding:2px 8px; font-size:11px; cursor:pointer;">
                            Copy Longitude
                        </button>
                    </div>
                    <div id="copy-status-${lat.replace('.','-')}" style="color:green; display:none; font-size:10px; margin-top:2px;">Copied!</div>
                </div>
            `)
            .openOn(map);
    });

    // Global helper for the popup button
    window.toggleDebugInfo = () => {
        showDebugInfo.value = !showDebugInfo.value;
        // Close popup? Maybe keep it open.
    };

    window.copyLeafletCoords = (lat, lng) => {
        const text = `${lat}, ${lng}`;
        window.copyText(text, lat); // Reuse helper
    };

    window.copyText = (text, latKey) => {
        navigator.clipboard.writeText(text).then(() => {
            // Try to find status element. If latKey is provided, use it for ID (legacy support for copyLeafletCoords)
            // Otherwise we might need a more generic way to show status in the popup.
            // But popups are transient.
            // The existing ID logic used lat.replace('.','-').
            // Let's rely on finding any copy-status element in the current popup if we can?
            // Actually, for simplicity, Copy Latitude/Longitude buttons in the SAME popup can share the SAME status message div.
            // The ID is based on ${lat} which is constant for this popup instance.
            // So we need to pass that 'lat' value to finding the element.
            
            // Hack: search for the status element in the document (it's in the leaflet popup pane)
            const statusEls = document.querySelectorAll('[id^="copy-status-"]');
            statusEls.forEach(el => {
                if (el.offsetParent !== null) { // visible-ish
                     el.style.display = 'block';
                     el.innerText = "Copied: " + (text.length > 20 ? text.substring(0,17)+"..." : text);
                     setTimeout(() => { el.style.display = 'none'; }, 2000);
                }
            });

        }).catch(err => console.error('Failed to copy', err));
    };

    // Global helper for inspecting location
    window.inspectLocationGlobal = (path, source) => {
        inspectLocation(path, source);
    };
    
    window._tempInspectItems = [];
    window.inspectLocationDirect = () => {
        if (window._tempInspectItems && window._tempInspectItems.length > 0) {
            inspectLocation(null, null, window._tempInspectItems);
        }
    };
    
    // Global helper for Coords Inspection (Backend)
    window.inspectLocationByCoords = (lat, lon, path, source, bounds) => {
        let extra = null;
        if (bounds) {
            extra = { lat: parseFloat(lat), lon: parseFloat(lon), ...bounds };
        } else {
             extra = { lat: parseFloat(lat), lon: parseFloat(lon) };
        }
        inspectLocation(path, source, null, extra);
    };


    const osm = L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', { 
        attribution: '&copy; OpenStreetMap contributors',
        noWrap: false
    });
    const satellite = L.tileLayer('https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}', { 
        attribution: 'Tiles &copy; Esri',
        noWrap: false
    });
    const topo = L.tileLayer('https://server.arcgisonline.com/ArcGIS/rest/services/World_Topo_Map/MapServer/tile/{z}/{y}/{x}', { 
        attribution: 'Tiles &copy; Esri',
        noWrap: false
    });
    const dark = L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', { 
        attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>', 
        subdomains: 'abcd', 
        maxZoom: 20,
        noWrap: false
    });
    
    // Google Basemaps
    const googleStreets = L.tileLayer('https://mt1.google.com/vt/lyrs=m&x={x}&y={y}&z={z}', {
        maxZoom: 20,
        attribution: 'Google',
        noWrap: false
    });
    const googleHybrid = L.tileLayer('https://mt1.google.com/vt/lyrs=y&x={x}&y={y}&z={z}', {
        maxZoom: 20,
        attribution: 'Google',
        noWrap: false
    });

    const baseMaps = {
        "Standard (OSM)": osm,
        "Satellite (Esri)": satellite,
        "Hybrid (Esri)": topo,
        "Dark Mode": dark,
        "Google Streets": googleStreets,
        "Google Hybrid": googleHybrid
    };
    
    osm.addTo(map);


    // Custom Browser Fullscreen Control
    L.Control.BrowserFullscreen = L.Control.extend({
        onAdd: function(map) {
            var container = L.DomUtil.create('div', 'leaflet-bar leaflet-control');
            var className = 'leaflet-control-fullscreen-button';
            if (!document.fullscreenElement) {
                className += ' flash-on-load';
            }
            var button = L.DomUtil.create('a', className, container);
            button.href = '#';
            button.title = 'Full Screen';
            button.innerHTML = '<i class="material-icons" style="font-size:18px; line-height:30px;">fullscreen</i>';
            button.style.width = '30px';
            button.style.height = '30px';
            button.style.textAlign = 'center';
            button.style.backgroundColor = 'white';
            button.style.cursor = 'pointer';
            button.style.display = 'block';

            L.DomEvent.on(button, 'click', function(e) {
                L.DomEvent.preventDefault(e);
                if (!document.fullscreenElement) {
                    document.documentElement.requestFullscreen();
                    button.innerHTML = '<i class="material-icons" style="font-size:18px; line-height:30px;">fullscreen_exit</i>';
                    button.title = 'Exit Full Screen';
                } else {
                    if (document.exitFullscreen) {
                        document.exitFullscreen();
                        button.innerHTML = '<i class="material-icons" style="font-size:18px; line-height:30px;">fullscreen</i>';
                        button.title = 'Full Screen';
                    }
                }
            });

            return container;
        },
        onRemove: function(map) {}
    });

    // Add custom control to map
    new L.Control.BrowserFullscreen({ position: 'topleft' }).addTo(map);

    // Basemaps (Added LAST to be at the bottom of the stack)
    L.control.layers(baseMaps, null, { position: 'topleft' }).addTo(map);

    // Use MarkerClusterGroup to enable Spiderfy effect
    markers = L.markerClusterGroup({
        spiderfyOnMaxZoom: false, // CHANGED: Disable spiderfy at max zoom to allow direct inspection
        showCoverageOnHover: true, // Enable coverage (blue area) on hover
        zoomToBoundsOnClick: false, // CHANGED: Manual control in clusterclick
        maxClusterRadius: 50, // Slightly larger radius to catch overlaps
        disableClusteringAtZoom: 20, // CHANGED: Always cluster overlaps (until max zoom + 1)
        spiderfyDistanceMultiplier: 2, 
        spiderLegPolylineOptions: { weight: 1.5, color: '#222', opacity: 0.5 },
        iconCreateFunction: function(cluster) {
            var childCount = cluster.getChildCount();
            var children = cluster.getAllChildMarkers();
            
            // Find a representative thumbnail
            var thumbPath = null;
            var thumbSource = null;
            
            // Try to find a valid thumb path in children
            for (var i = 0; i < children.length; i++) {
                if (children[i].options.thumbPath) {
                    thumbPath = children[i].options.thumbPath;
                    thumbSource = children[i].options.thumbSource;
                    break;
                }
            }
            
            // Count Aggregation
            var totalCount = 0;
            children.forEach(m => {
                totalCount += (m.options.photoCount || 1);
            });

            // Calculate Border Color based on Folder (using representative path)
            let borderColor = '#555';
            if (thumbPath) {
                 let folderPath = "/";
                const lastSlash = thumbPath.lastIndexOf('/');
                if (lastSlash > 0) {
                    folderPath = thumbPath.substring(0, lastSlash);
                }
                borderColor = getClusterColor(folderPath); // Use existing helper
            }

            // Generate HTML
            let innerHtml = '';
            if (thumbPath) {
                 // Force small size preview with cache bust
                const url = '/api/preview?path=' + encodeURIComponent(thumbPath) + '&source=' + encodeURIComponent(thumbSource || '') + '&size=small';
                
                innerHtml = `<div class="cluster-thumb-container" style="border:3px solid ${borderColor} !important; box-shadow:0 2px 5px rgba(0,0,0,0.5); background-color: #555; width:48px !important; height:48px !important; border-radius:50%; overflow:hidden; position:relative; box-sizing:border-box;">
                    <img src="${url}" class="fan-thumb-img" style="width:100% !important; height:100% !important; max-width:100% !important; max-height:100% !important; object-fit:cover !important; border-radius:50%; display:block;" onerror="window.fileBrowserHeatmapImageError(this, '${thumbPath.replace(/\'/g, "\\\'")}')" />
                    <span style="position:absolute; top:50%; left:50%; transform:translate(-50%, -50%); background:rgba(0,0,0,0.7); border-radius:10px; padding:1px 5px; color:white; font-size:11px; font-weight:bold; white-space:nowrap;">${totalCount}</span>
                </div>`;
            } else {
                // Fallback (Generic Number)
                 innerHtml = `<div style="background-color:rgba(100,100,100,0.8);border-radius:50%;width:30px;height:30px;display:flex;align-items:center;justify-content:center;border:2px solid white;"><span style="color:white;text-shadow:0 0 2px black;font-weight:bold;">${totalCount}</span></div>`;
            }

            return new L.DivIcon({ 
                html: innerHtml, 
                className: 'custom-cluster-marker', 
                iconSize: new L.Point(48, 48),
                iconAnchor: [24, 24]
            });
        }
    });

    // Custom Handle for Max Zoom Click (instead of Spiderfy)
    markers.on('clusterclick', function (a) {
        // Fix: Use map.getZoom() instead of a.layer.getZoom()
        // OR if the cluster is fully contained in a small area (same location)
        // a.layer.getBounds() might be a single point.
        // We generally want "Inspection" if we are at max zoom OR if zooming won't help.
        
        const zoom = map.getZoom();
        const cluster = a.layer;
        // Check if zooming in would actually help
        // If current zoom is max, OR if the cluster bounds are effectively a point
        // (But MarkerClusterGroup handles "spiderfy" for same-location points usually)
        // Since we disabled spiderfyOnMaxZoom, we catch it here.
        
        if (zoom >= 18) { // Max Zoom Logic
             const children = cluster.getAllChildMarkers();
             
             // Collect items for inspection
             const directItems = [];
             children.forEach(m => {
                 let p = m.options.thumbPath || m.options.clusterPath; 
                 let s = m.options.thumbSource || m.options.clusterSource;
                 let c = m.options.photoCount || 1;
                 let cid = m.options.clusterID || "";
                 
                 if (p) {
                     directItems.push({ path: p, source: s, name: p.split('/').pop(), count: c, clusterID: cid });
                 }
             });
             
             if (directItems.length > 0) {
                 // Open Inspection Panel DIRECTLY
                 // Do not use inspectLocationDirect (helper ignores args)
                 inspectLocation(null, null, directItems);
             }
        } else {
             // Standard behavior: Zoom to cluster bounds
             a.layer.zoomToBounds();
        }
    });
    
    markers.on('click', function (a) {
        // console.log("DEBUG: marker click fired. Is it a cluster?", false);
    });

    // CRITICAL: Ensure context menu bubbles up if individual binding fails (common in MarkerCluster)
    // This catches right-clicks on any marker (including our custom badges) inside the group
    markers.on('contextmenu', function (e) {
        // e.layer is the marker that was clicked
        if (e.layer && e.layer.options && e.layer.options.photoCount) {
             // It's one of our badges
             // Re-use the handler if we can, or just inspect
             // We need path and source from options?
             // We didn't save path/source to marker options explicitly in createTile, 
             // we bound it via closure in on('contextmenu').
             // But if that failed, we need data here.
             // We should attach path/source to marker options in createTile to be safe.
             
             // For now, let's rely on the direct bind working, but if MCG swallows it...
             // MCG docs say it propagates.
             // Let's force it on the group just in case.
        }
    });

    // CRITICAL: Handle interactions on CLUSTERS (the "Grey Circles")
    // When many items are at the EXACT same location, they cluster even at max zoom.
    // We need to allow right-click on these too.
    markers.on('clustercontextmenu', function (e) {
        console.log("Cluster Context Menu", e);
        const cluster = e.layer;
        const children = cluster.getAllChildMarkers();
        if (children.length > 0) {
            // Collect all items in this cluster
            const directItems = [];
            children.forEach(m => {
                 let p = m.options.thumbPath || m.options.clusterPath; // clusterPath from badge, thumbPath from fan leaf
                 let s = m.options.thumbSource || m.options.clusterSource;
                 let c = m.options.photoCount || 1;
                 // Capture Cluster ID from options (added in addMarkersForTile)
                 let cid = m.options.clusterID || ""; // Ensure string
                 
                 if (p) {
                     directItems.push({ path: p, source: s, name: p.split('/').pop(), count: c, clusterID: cid });
                 }
            });
            
            // Use cluster bounds for context
            const bounds = cluster.getBounds();
            
            // Use first child for reference coords key
            const lat = e.latlng.lat.toFixed(5);
            const lng = e.latlng.lng.toFixed(5);
             
             let content = `
                <div style="text-align:center; font-size:12px;">
                    <b>Lat:</b> ${lat}<br>
                    <b>Lon:</b> ${lng}<br>
                    <div style="margin-top:5px; display:flex; flex-direction:column; gap:4px;">
                        <button onclick="window.copyLeafletCoords('${lat}', '${lng}')" class="button button--flat" style="font-size:11px; cursor:pointer;">Copy Coordinates</button>
                    `;
             
             // If we have items, we can inspect
             if (directItems.length > 0) {
                 // Client-side cluster with items ready
                 window._tempInspectItems = directItems;
                 content += `<button onclick="window.inspectLocationDirect()" class="button button--flat" style="font-size:11px; cursor:pointer; background:rgba(0,100,200,0.3);">Inspect Files (${directItems.length})</button>`;
             }
             
             content += `</div><div id="copy-status-${lat.replace('.','-')}" style="color:green; display:none; font-size:10px; margin-top:2px;">Copied!</div></div>`;

             L.popup().setLatLng(e.latlng).setContent(content).openOn(map);
        }
    });

    map.addLayer(markers);
};

const loadData = async () => {
    // wait for map
    if (!map) { 
        await initMap(); 
        if (!map) return;
    }

    // Cancel any ongoing requests from previous load
    if (abortController) {
        abortController.abort();
    }
    abortController = new AbortController();

    // Clean up existing layers
    markers.clearLayers();
    if (heatLayer) {
        map.removeLayer(heatLayer);
        heatLayer = null;
    }
    if (tileLayer) {
        map.removeLayer(tileLayer);
        tileLayer = null;
    }
    
    // Event Listeners
    map.on('move', () => {
         updateVisibleMarkers();
         updateLeaderLine(); // Keep line attached during pan/zoom
    });
    map.on('zoomend', () => {
        updateVisibleMarkers();
        updateLeaderLine();
    });
    map.on('resize', () => {
        updateLeaderLine();
    });

    // Clear stores
    clusterDataStore.clear();
    renderedMarkerStore.clear();

    const source = recursiveDecode(route.query.source || "");
    const path = recursiveDecode(route.query.path || "");
    
    // Determine if folder or global
    if (path) {
        isFolderMode.value = true;
    } else {
        isFolderMode.value = false;
    }

    debugStatus.value = "Loading tiles...";
    
    // Instead of loading all data at once, we'll load tiles as needed
    // Create a custom tile layer that fetches our JSON tiles
    tileLayer = L.gridLayer({
        tileSize: 256,
        minZoom: 1,
        maxZoom: 18,
        updateWhenIdle: true, // Only update tiles after zoom/pan completes
        updateWhenZooming: false, // Don't update during zoom animation
        keepBuffer: 0, // Don't keep extra tiles in buffer
        noWrap: true // Fix duplicate markers on world wrap
    });

    // Store loaded tiles to avoid reloading
    const loadedTiles = new Map();
    const tileMarkers = new Map(); // Map of tile key -> markers
    const renderedPaths = new Set(); // Global registry of rendered paths to prevent duplication
    
    // Aggressive cleanup on zoom end to prevent doubling
    // MODIFIED: Keep tiles from adjacent zoom levels (±1) for smoother transitions
    // Marker Management
    // We bind markers to tiles. When a tile unloads (panned out/zoomed out), we remove its markers.
    tileLayer.on('tileunload', (e) => {
        const key = `${e.coords.z}-${e.coords.x}-${e.coords.y}`;
        if (tileMarkers.has(key)) {
            const layers = tileMarkers.get(key);
            // Cleanup markers
            layers.forEach(m => {
                 if (m.options.clusterPath) {
                     const renderKey = `${m.options.clusterPath}:${e.coords.z}`;
                     renderedPaths.delete(renderKey);
                 }
            });
            markers.removeLayers(layers);
            tileMarkers.delete(key);
        }
    });

    // Helper to render markers for a tile
    const addMarkersForTile = (data, coords, tileKey) => {
        // Strict Zoom Guard: Don't add markers if map has zoomed away during load
        if (map && Math.round(map.getZoom()) !== coords.z) {
            return;
        }

        const tileMarkerList = [];
        let clusters = data.clusters || [];
        
        clusters.forEach(c => {
            // STRICT DEDUPLICATION (Scoped by Zoom):
            const renderKey = `${c.path}:${coords.z}`;
            if (renderedPaths.has(renderKey)) {
                return; 
            }
            renderedPaths.add(renderKey);

            try {
                // UNIFIED LOGIC: Always create a marker with thumbnail metadata
                // This allows L.markerCluster to handle everything.
                
                // 1. Determine Preview Path
                // Use c.previewID if available (Server Cluster), else c.path (Leaf)
                let previewPath = c.path; 
                // NOTE: We rely on c.path being the representative path.
                
                if (c.points && c.points.length > 0) {
                     // Add individual markers for points
                    c.points.forEach(p => {
                         const marker = L.marker([p.lat, p.lon], { 
                             icon: generateMarkerIcon(p.path, c.source || source, 1), // Single item icon (Fan Leaf Style)
                             thumbPath: p.path,
                             thumbSource: c.source || source,
                             clusterID: c.id,
                             photoCount: 1
                         });
                         
                         // CHANGED: No popup, direct click inspect
                         // marker.bindPopup(generatePopupHtml(p.path, c.source || source, 1));
                         marker.on('click', (e) => {
                             L.DomEvent.stopPropagation(e);
                             const safePath = (p.path || "");
                             const safeSource = (c.source || source);
                             const boundsObj = {minLat: p.lat, minLon: p.lon, maxLat: p.lat, maxLon: p.lon, clusterID: c.id};
                             // Direct open
                             inspectLocationByCoords(p.lat, p.lon, safePath, safeSource, boundsObj);
                         });
                         
                         // Context Menu
                         marker.on('contextmenu', (e) => {
                             L.DomEvent.stopPropagation(e);
                              const lat = e.latlng.lat.toFixed(5);
                              const lng = e.latlng.lng.toFixed(5);
                              const safePath = (p.path || "").replace(/'/g, "\\'");
                              const safeSource = (c.source || source).replace(/'/g, "\\'");
                              // Bounds for single item are just its point
                              const boundsObj = {minLat: p.lat, minLon: p.lon, maxLat: p.lat, maxLon: p.lon, clusterID: c.id};
                              const boundsJson = JSON.stringify(boundsObj).replace(/"/g, "&quot;");
                              
                              L.popup().setLatLng(e.latlng).setContent(`
                                <div style="text-align:center; font-size:12px;">
                                    <b>Lat:</b> ${lat}<br><b>Lon:</b> ${lng}<br>
                                    <div style="margin-top:5px; display:flex; flex-direction:column; gap:4px;">
                                        <button onclick="window.copyLeafletCoords('${lat}', '${lng}')" class="button button--flat" style="font-size:11px; cursor:pointer;">Copy</button>
                                        <button onclick='window.inspectLocationByCoords("${lat}", "${lng}", "${safePath}", "${safeSource}", ${boundsJson})' class="button button--flat" style="font-size:11px; cursor:pointer; background:rgba(0,100,200,0.3);">Inspect File</button>
                                    </div>
                                </div>`).openOn(map);
                         });

                         tileMarkerList.push(marker);
                    });
                } else {
                    // Add single marker for the Cluster itself (Server Aggregation)
                    
                    const marker = L.marker([c.lat, c.lon], {
                        icon: generateMarkerIcon(c.path, c.source || source, c.count),
                        thumbPath: c.path,
                        thumbSource: c.source || source,
                        clusterID: c.id,
                        photoCount: c.count,
                        totalCount: c.count, // Ensure totalCount is set for selection logic
                        clusterPath: c.path,
                        clusterSource: c.source || source
                    });
                     
                     // CHANGED: No popup, direct click inspect (if count is small or user clicks it)
                     // If it's a cluster, the ClusterGroup usually handles click (zoom or spiderfy).
                     // BUT if it's a single item cluster (count=1), it behaves like a marker.
                     // Or if max zoom. 
                     // We add the click handler here to be safe. 
                     // If L.markerClusterGroup captures it, this might not fire unless spiderfy is off?
                     // Actually, individual markers inside a cluster don't receive click events if they are clustered.
                     // But if they are NOT clustered (e.g. single item in that area), they do.
                     marker.on('click', (e) => {
                         // Only if not clustered? 
                         // No, if it's visible as a marker, we want this.
                         L.DomEvent.stopPropagation(e);
                         const safePath = (c.path || "");
                         const safeSource = (c.source || source);
                         const boundsObj = {minLat: c.min ? c.min[0] : c.lat, minLon: c.min ? c.min[1] : c.lon, maxLat: c.max ? c.max[0] : c.lat, maxLon: c.max ? c.max[1] : c.lon, clusterID: c.id};
                         inspectLocationByCoords(c.lat, c.lon, safePath, safeSource, boundsObj);
                     });

                     // Context Menu for Cluster Marker
                      marker.on('contextmenu', (e) => {
                         L.DomEvent.stopPropagation(e);
                          const lat = e.latlng.lat.toFixed(5);
                          const lng = e.latlng.lng.toFixed(5);
                          const safePath = (c.path || "").replace(/'/g, "\\'");
                          const safeSource = (c.source || source).replace(/'/g, "\\'");
                          const boundsObj = {minLat: c.min ? c.min[0] : c.lat, minLon: c.min ? c.min[1] : c.lon, maxLat: c.max ? c.max[0] : c.lat, maxLon: c.max ? c.max[1] : c.lon, clusterID: c.id};
                          const boundsJson = JSON.stringify(boundsObj).replace(/"/g, "&quot;");
                          
                          L.popup().setLatLng(e.latlng).setContent(`
                            <div style="text-align:center; font-size:12px;">
                                <b>Lat:</b> ${lat}<br><b>Lon:</b> ${lng}<br>
                                <div style="margin-top:5px; display:flex; flex-direction:column; gap:4px;">
                                    <button onclick="window.copyLeafletCoords('${lat}', '${lng}')" class="button button--flat" style="font-size:11px; cursor:pointer;">Copy</button>
                                    <button onclick='window.inspectLocationByCoords("${lat}", "${lng}", "${safePath}", "${safeSource}", ${boundsJson})' class="button button--flat" style="font-size:11px; cursor:pointer; background:rgba(0,100,200,0.3);">Inspect Files</button>
                                </div>
                            </div>`).openOn(map);
                      });

                    tileMarkerList.push(marker);
                }

            } catch (err) {
                console.error("Error creating marker for tile:", err);
            }
        });
        
        tileMarkers.set(tileKey, tileMarkerList);
        markers.addLayers(tileMarkerList);
    };

    tileLayer.createTile = function(coords, done) {
        const tile = document.createElement('div');
        const tileKey = `${coords.z}-${coords.x}-${coords.y}`;
        
        if (loadedTiles.has(tileKey)) {
            if (!tileMarkers.has(tileKey)) {
                addMarkersForTile(loadedTiles.get(tileKey), coords, tileKey);
            }
            done(null, tile);
            return tile;
        }
        
        // Build tile URL
        let tileUrl = `/api/heatmap/tiles/${coords.z}/${coords.x}/${coords.y}`;
        if (source || path) {
            const params = new URLSearchParams();
            if (source) params.append('source', source);
            if (path) params.append('path', path);
            tileUrl += '?' + params.toString();
        }

        // Load tile data with abort signal
        fetch(tileUrl, { signal: abortController.signal })
            .then(r => r.ok ? r.json() : {clusters:[]})
            .then(data => {
                loadedTiles.set(tileKey, data);
                addMarkersForTile(data, coords, tileKey);
                // Trigger heatmap update (debounced) to include this new tile's data
                updateHeatmap();
                done(null, tile);
            })
            .catch(() => done(null, tile));

        return tile;
    };

    /* Deprecated Logic Block (Ghost Code from Refactor) */
    const _deprecated_logic = () => {
            const tileMarkerList = [];
            // Sort clusters by count (descending) so we show the most important ones if we limit
            let clusters = data.clusters || [];
            
            // PERFORMANCE SAFETY: Limit total markers per tile?
            // User requested to see ALL data ("Inspect matches cache"). 
            // Arbitrary limiting causes "disappearing" data inconsistency.
            // We now rely on backend aggregation (SimpifyClustersByZoom) or just render them.
            // Removing the limit to ensure 1:1 match with data.
            /*
            if (clusters.length > 500) {
                 clusters.sort((a, b) => b.count - a.count);
                 clusters = clusters.slice(0, 500);
            }
            */
            
            clusters.forEach(c => {
                try {
                    // Determine how many markers to show based on zoom
                    const zoom = coords.z;
                    
                    // AGGRESSIVE optimization: very few thumbnails at all zoom levels
                    if (zoom < 15) {
                        // Low/Medium zoom: show cluster badge only (NO thumbnails)
                        // Also add to heatmap data
                        const marker = L.marker([c.lat, c.lon], {
                            icon: L.divIcon({
                                html: `<div style="background:rgba(0,0,0,0.7);color:white;border-radius:50%;width:30px;height:30px;display:flex;align-items:center;justify-content:center;font-size:11px;font-weight:bold;border:2px solid white;">${c.count}</div>`,
                                className: 'tile-cluster-badge',
                                iconSize: [30, 30]
                            }),
                            photoCount: c.count,
                            clusterPath: c.path,
                            clusterSource: c.source || source
                        });
                        // Shared Context Menu Handler for Badges (Backend Tiles)
                        const onBadgeContextMenu = (e, clusterPath, clusterSource, minLat, minLon, maxLat, maxLon, clusterID) => {
                             L.DomEvent.stopPropagation(e); // Prevent map context menu
                             const lat = e.latlng.lat.toFixed(5);
                             const lng = e.latlng.lng.toFixed(5);
                             const safePath = (clusterPath || "").replace(/'/g, "\\'");
                             const safeSource = (clusterSource || source).replace(/'/g, "\\'");
                             const cID = clusterID || "";
                             
                             // Construct bounds object for the button call
                             const boundsObj = {minLat, minLon, maxLat, maxLon, clusterID: cID};
                             const boundsJson = JSON.stringify(boundsObj).replace(/"/g, "&quot;");
                             
                             L.popup()
                                .setLatLng(e.latlng)
                                .setContent(`
                                    <div style="text-align:center; font-size:12px;">
                                        <b>Lat:</b> ${lat}<br>
                                        <b>Lon:</b> ${lng}<br>
                                        <div style="margin-top:5px; display:flex; flex-direction:column; gap:4px;">
                                            <button onclick="window.copyLeafletCoords('${lat}', '${lng}')" class="button button--flat" style="font-size:11px; cursor:pointer;">Copy Coordinates</button>
                                            <button onclick='window.inspectLocationByCoords("${lat}", "${lng}", "${safePath}", "${safeSource}", ${boundsJson})' class="button button--flat" style="font-size:11px; cursor:pointer; background:rgba(0,100,200,0.3);">Inspect Files</button>
                                        </div>
                                        <div id="copy-status-${lat.replace('.','-')}" style="color:green; display:none; font-size:10px; margin-top:2px;">Copied!</div>
                                    </div>
                                `)
                                .openOn(map);
                        };

                        marker.bindPopup(`<b>${c.count} photos</b><br>Zoom in for details<br><span style="font-size:10px;color:#aaa">Right-click for Options</span>`);
                        
                        // Pass bounds (Min/Max) AND ID from backend cluster 'c'
                        if (c.min && c.max) {
                            marker.on('contextmenu', (e) => onBadgeContextMenu(e, c.path, c.source || source, c.min[0], c.min[1], c.max[0], c.max[1], c.id));
                        } else {
                             marker.on('contextmenu', (e) => onBadgeContextMenu(e, c.path, c.source || source, 0, 0, 0, 0, c.id));
                        }
                        
                        markers.addLayer(marker);
                        tileMarkerList.push(marker);
                    } else if (c.count > 15) {
                        // Large cluster badge
                        const marker = L.marker([c.lat, c.lon], {
                            icon: L.divIcon({
                                html: `<div style="background:rgba(255,100,0,0.8);color:white;border-radius:50%;width:36px;height:36px;display:flex;align-items:center;justify-content:center;font-size:12px;font-weight:bold;border:2px solid white;box-shadow:0 2px 5px rgba(0,0,0,0.3);">${c.count}</div>`,
                                className: 'tile-cluster-badge-large',
                                iconSize: [36, 36]
                            }),
                            photoCount: c.count,
                            totalCount: c.count, 
                            clusterID: c.id
                        });
                        
                        marker.bindPopup(`<b>${c.count} photos</b><br>Location: ${c.path}<br><span style="font-size:10px;color:#aaa">Right-click for Options</span>`);
                        
                        marker.bindPopup(`<b>${c.count} photos</b><br>Location: ${c.path}<br><span style="font-size:10px;color:#aaa">Right-click for Options</span>`);
                        
                        // Attach context menu handler for specific badge
                        // Attach context menu handler for specific badge
                        if (c.min && c.max) {
                            marker.on('contextmenu', (e) => onBadgeContextMenu(e, c.path, c.source || source, c.min[0], c.min[1], c.max[0], c.max[1], c.id));
                        } else {
                             marker.on('contextmenu', (e) => onBadgeContextMenu(e, c.path, c.source || source, 0, 0, 0, 0, c.id));
                        }

                        // Remove "Direct Click to Inspect" to match request "Right click... should not immediately open inspect"
                        // But wait, user said "Right click and long press should not immediately open...".
                        // Left click is fine to open Popup.
                        // I will REMOVE the left-click override I added.
                        
                        markers.addLayer(marker);
                        tileMarkerList.push(marker);
                    } else {
                        // Small clusters: show thumbnails in FAN LAYOUT (Zoom 15+)
                        let maxFanSize = 3; // Default: 3 thumbnails
                        if (zoom >= 16 && zoom < 17) {
                            maxFanSize = 5; // Zoom 16: 5 thumbnails
                        } else if (zoom >= 17) {
                            maxFanSize = 12; // Zoom 17+: 12 thumbnails (Spiderfy will handle clutter)
                        }
                        
                        if (c.points && c.points.length > 0) {
                            // Show individual points
                            const pointsToShow = c.points.slice(0, maxFanSize);
                            
                            // FORCE FAN LAYOUT: Always apply offset if count is small, to ensure visibility
                            // Instead of checking zoom >= 17, we check if we *should* spiderfy.
                            // But user wants to SEE them.
                            // If we use 0 radius at zoom 17+, they stack and need click to spiderfy.
                            // If we use 0.0002, they are separate. 
                            // User request: "I do not see the photo thumbnail fan... when zoomed in at zoom level 18".
                            // So we MUST use offset.
                            const fanRadius = 0.0002; 
                            const angleStep = (2 * Math.PI) / pointsToShow.length;
                            
                            pointsToShow.forEach((p, idx) => {
                                // Calculate position in circle around cluster center
                                const angle = idx * angleStep;
                                const offsetLat = fanRadius * Math.cos(angle);
                                const offsetLon = fanRadius * Math.sin(angle);
                                
                                const marker = L.marker([c.lat + offsetLat, c.lon + offsetLon], {
                                    icon: generateMarkerIcon(p.path, c.source || source, 1),
                                    thumbPath: p.path,
                                    thumbSource: c.source || source,
                                    clusterID: c.id, 
                                    totalCount: c.count, // Capture total count of the cluster this leaf belongs to
                                    riseOnHover: true, 
                                });
                                marker.bindPopup(generatePopupHtml(p.path, c.source || source, 1));
                                
                                // Add context menu handler to marker
                                marker.on('contextmenu', () => {
                                    // Open side panel for this file's folder
                                    let parent = p.path.substring(0, p.path.lastIndexOf('/'));
                                    inspectLocation(parent, c.source || source);
                                });

                                markers.addLayer(marker);
                                tileMarkerList.push(marker);
                            });
                        } else {
                            // Single cluster marker
                            const marker = L.marker([c.lat, c.lon], {
                                 icon: generateMarkerIcon(c.path, c.source || source, c.count),
                                 thumbPath: c.path,
                                 thumbSource: c.source || source,
                                 clusterID: c.id, // Explicitly Store Cluster ID
                                 totalCount: c.count // Capture count
                            });
                            marker.bindPopup(generatePopupHtml(c.path, c.source || source, c.count));
                            markers.addLayer(marker);
                            tileMarkerList.push(marker);
                        }
                    }
                } catch (err) {
                    console.warn('Failed to create marker:', err);
                }
            });
            
            tileMarkers.set(tileKey, tileMarkerList);
            debugClusterCount.value = loadedTiles.size;
            debugStatus.value = `Loaded ${loadedTiles.size} tiles`;
            
            done(null, tile);
    };
    // End of deprecated logic

    // Remove tile markers when tile is removed
    tileLayer.on('tileunload', (e) => {
        const coords = e.coords;
        const tileKey = `${coords.z}-${coords.x}-${coords.y}`;
        const layers = tileMarkers.get(tileKey);
        if (layers) {
            layers.forEach(marker => {
                 if (marker.options.clusterPath) renderedPaths.delete(marker.options.clusterPath);
                 try { markers.removeLayer(marker); } catch(e){}
            });
            tileMarkers.delete(tileKey);
        }
        // NOTE: We do NOT delete from loadedTiles here, to allow cache hits (re-render) later.
        // But we MUST allow re-rendering in addMarkersForTile by clearing renderedPaths above.
    });

    // Add tile layer to map (this will trigger tile loading)
    map.addLayer(tileLayer);
    
    // Add heatmap layer for low zoom levels
    const heatPoints = [];
    
    // Function to collect all cluster points for heatmap (Debounced)
    let heatmapTimeout;
    const updateHeatmap = () => {
        if (heatmapTimeout) clearTimeout(heatmapTimeout);
        heatmapTimeout = setTimeout(() => {
            // Only update if at desired zoom
            if (map.getZoom() > 17) {
                if (heatLayer && map.hasLayer(heatLayer)) {
                    map.removeLayer(heatLayer);
                }
                return;
            }

            heatPoints.length = 0; // Clear array
            heatPoints.length = 0; // Clear array
            const currentZoom = Math.round(map.getZoom());
            
            loadedTiles.forEach((tileData, key) => {
                // Ensure we only use tiles from current zoom level
                const tileZ = parseInt(key.split('-')[0]);
                if (tileZ !== currentZoom) return;

                if (tileData.clusters) {
                    tileData.clusters.forEach(c => {
                        // Intensity Logic:
                        // Gradient starts at 0.4.
                        // We want single items to be VISIBLE (e.g. 0.5).
                        // Large clusters should hit 1.0 quickly.
                        // Formula: 0.4 (base) + (count / 10) * 0.6
                        // Count 1 => 0.4 + 0.06 = 0.46 (Blue)
                        // Count 5 => 0.4 + 0.3 = 0.7 (Lime)
                        // Count 10 => 1.0 (Red)
                        let intensity = 0.4 + (c.count / 10) * 0.6;
                        if (intensity > 1.0) intensity = 1.0;
                         
                        // Sanity check to prevent canvas glitches
                        if (!isNaN(c.lat) && !isNaN(c.lon)) {
                            heatPoints.push([c.lat, c.lon, intensity]);
                        }
                    });
                }
            });
            
            const zoom = map.getZoom();
            // REQUESTED VALUES (User Specified):
            // Low Zoom (< 10): Radius 80 (Blur 50)
            // Mid Zoom (< 13): Radius 75 (Blur 40)
            // High Zoom (>= 13): Radius 50 (Blur 35)
            const radius = zoom < 10 ? 80 : (zoom < 13 ? 75 : 50);
            const blur = zoom < 10 ? 50 : (zoom < 13 ? 40 : 35);

            if (!heatLayer) {
                // Initialize if missing
                heatLayer = L.heatLayer(heatPoints, {
                    radius: radius,
                    blur: blur,
                    maxZoom: 17, // Enable heatmap up to zoom 17 (inclusive)
                    max: 1.0, 
                    gradient: {0.4: 'blue', 0.6: 'cyan', 0.7: 'lime', 0.8: 'yellow', 1.0: 'red'}
                });
                
                // Apply CSS class for GPU stability
                if (heatLayer._canvas) {
                    heatLayer._canvas.classList.add('heatmap-canvas');
                }
                
                map.addLayer(heatLayer);
            } else {
                // EFFICIENT UPDATE: Reuse layer
                // FIX: Only set options if layer is on map to avoid _animating null error
                if (map.hasLayer(heatLayer)) {
                     heatLayer.setOptions({
                        radius: radius,
                        blur: blur
                    });
                } else {
                    map.addLayer(heatLayer); 
                    // Re-adding might reset options, so set them again safely
                    heatLayer.setOptions({
                        radius: radius,
                        blur: blur
                    });
                }
                
                // Ensure canvas class is set
                if (heatLayer._canvas && !heatLayer._canvas.classList.contains('heatmap-canvas')) {
                    heatLayer._canvas.classList.add('heatmap-canvas');
                }

                heatLayer.setLatLngs(heatPoints);
                
                if (!map.hasLayer(heatLayer)) {
                    map.addLayer(heatLayer);
                }
            }
        }, 50); // 50ms debounce for better responsiveness
    };
    
    // Update heatmap when zoom changes
    map.on('zoomend', () => {
        updateHeatmap(); // Trigger refresh to update radius/data
    });
    
    // Update heatmap when tiles load
    tileLayer.on('load', () => {
        updateHeatmap();
    });

    // Set initial view
    if (path) {
        // For folder view, try to fit bounds if we have data
        // For now, just use a default view
        map.setView([0, 0], 2);
    } else {
        map.setView([0, 0], 2);
    }

    debugStatus.value = "Tile-based loading active";
};


// Box Selection Logic
const toggleSelectionMode = () => {
    isBoxSelectMode.value = !isBoxSelectMode.value;
    if (isBoxSelectMode.value) {
        // Disable map interaction? Leaflet usually handles this if we consume events.
        // The overlay has touch-action: none.
        map.dragging.disable();
    } else {
        map.dragging.enable();
        selectionBox.value.visible = false;
    }
};

const startSelection = (e) => {
    if (!isBoxSelectMode.value) return;
    
    // Get coords relative to overlay itself for perfect visual alignment
    const rect = e.currentTarget.getBoundingClientRect();
    
    let clientX, clientY;
    if (e.type.startsWith('touch')) {
        clientX = e.touches[0].clientX;
        clientY = e.touches[0].clientY;
    } else {
        clientX = e.clientX;
        clientY = e.clientY;
    }
    
    const x = clientX - rect.left;
    const y = clientY - rect.top;
    
    selectionBox.value.startX = x;
    selectionBox.value.startY = y;
    selectionBox.value.currentX = x;
    selectionBox.value.currentY = y;
    selectionBox.value.visible = true;
    selectionBox.value.style = {
        left: x + 'px',
        top: y + 'px',
        width: '0px',
        height: '0px'
    };
};

const updateSelection = (e) => {
    if (!isBoxSelectMode.value || !selectionBox.value.visible) return;
    
    // Prevent scrolling on touch
    if (e.type.startsWith('touch')) e.preventDefault();
    
    const rect = e.currentTarget.getBoundingClientRect();
    
    let clientX, clientY;
    if (e.type.startsWith('touch')) {
        clientX = e.touches[0].clientX;
        clientY = e.touches[0].clientY;
    } else {
        clientX = e.clientX;
        clientY = e.clientY;
    }
    
    const x = clientX - rect.left;
    const y = clientY - rect.top;
    
    selectionBox.value.currentX = x;
    selectionBox.value.currentY = y;
    
    const minX = Math.min(selectionBox.value.startX, x);
    const maxX = Math.max(selectionBox.value.startX, x);
    const minY = Math.min(selectionBox.value.startY, y);
    const maxY = Math.max(selectionBox.value.startY, y);
    
    selectionBox.value.style = {
        left: minX + 'px',
        top: minY + 'px',
        width: (maxX - minX) + 'px',
        height: (maxY - minY) + 'px'
    };
};

const endSelection = (e) => {
    if (!isBoxSelectMode.value || !selectionBox.value.visible) return;
    
    selectionBox.value.visible = false;
    
    // Calculate Bounds
    const overlay = e.currentTarget;
    const overlayRect = overlay.getBoundingClientRect();
    
    const mapContainer = map.getContainer();
    const mapRect = mapContainer.getBoundingClientRect();
    
    // Calculate Offset: Overlay Relative -> Map Relative
    // MapPoint = OverlayPoint + OverlayAbs - MapAbs
    const offsetX = overlayRect.left - mapRect.left;
    const offsetY = overlayRect.top - mapRect.top;
    
    // We already calculated relative x/y in updateSelection (which are Overlay Relative)
    // Apply offset to make them Map Relative
    const startPt = L.point(
        selectionBox.value.startX + offsetX, 
        selectionBox.value.startY + offsetY
    );
    const endPt = L.point(
        selectionBox.value.currentX + offsetX, 
        selectionBox.value.currentY + offsetY
    );
    
    // Convert to LatLng (Leaflet expects Map Relative points)
    const startLatLng = map.containerPointToLatLng(startPt);
    const endLatLng = map.containerPointToLatLng(endPt);
    
    const bounds = L.latLngBounds(startLatLng, endLatLng);
    
    // Query Markers
    const selectedItems = [];
    
    // Iterate ALL layers in the cluster group
    // Note: markers.eachLayer iterates specific markers (leaves), not clusters.
    markers.eachLayer(layer => {
        if (bounds.contains(layer.getLatLng())) {
             let p = layer.options.thumbPath || layer.options.clusterPath;
             let s = layer.options.thumbSource || layer.options.clusterSource;
             // Count: Prefer totalCount (from Cluster data) over photoCount (Badge) over 1
             let c = layer.options.totalCount || layer.options.photoCount || 1;
             let cid = layer.options.clusterID || ""; // Now populated for all types
             
             if (p) {
                 selectedItems.push({ 
                     path: p, 
                     source: s, 
                     name: p.split('/').pop(), 
                     count: c, 
                     clusterID: cid 
                 });
             }
        }
    });
    
    // Auto-disable mode
    toggleSelectionMode();
    
    if (selectedItems.length > 0) {
        // Feed to Inspection Panel
        // Use inspectLocation with directItems. 
        // The items now have valid 'clusterID', so openFolderView will work correctly.
        inspectLocation(null, null, selectedItems);
    } else {
        notify.showInfo("No items selected in area");
    }
};


onMounted(() => {
    // Setup global error handler
    window.fileBrowserHeatmapImageError = (img, path) => {
        img.style.display='none'; 
        img.parentElement.style.backgroundColor='#666'; 
        img.parentElement.innerHTML='<span style=\'color:white;line-height:48px;display:block;text-align:center;font-weight:bold;font-size:20px;\'>!</span>';
        window.dispatchEvent(new CustomEvent('heatmap-image-error', { detail: { path: path } }));
    };

    window.addEventListener('heatmap-image-error', (e) => {
        if (!failedImages.value.has(e.detail.path)) {
            failedImages.value.add(e.detail.path);
            failedImageCount.value++;
            showErrorNotification();
        }
    });

    // Expose navigation function globally for Leaflet popups
    window.heatmapNavigate = (targetPath, sourceName, isFolder) => {
        console.log(`[Heatmap] heatmapNavigate called: targetPath='${targetPath}', sourceName='${sourceName}', isFolder=${isFolder}`);
        
        // Ensure path doesn't have double slashes
        if (targetPath.startsWith('//')) {
            console.log(`[Heatmap] Removing double slash from path`);
            targetPath = targetPath.substring(1);
        }
        
        if (isFolder) {
            console.log(`[Heatmap] Navigating to heatmap folder: ${targetPath}`);
            router.push({ path: '/heatmap', query: { path: targetPath, source: sourceName } })
              .catch(err => console.error("Router push error:", err));
        } else {
            // For file browsing, we want /files/{path}
            // Paths from heatmap.json are already correct absolute paths from source root
            // Example: /PHOTOCOLLECTIONS/POWELL-COLLECTION/BillPowellCollection/A01-BillPowell-1995/IMG_20200604_0004.jpg
            
            // Remove leading slash since we'll add /files/SOURCE/
            let cleanPath = targetPath.startsWith('/') ? targetPath.substring(1) : targetPath;
            
            // Handle multiple sources logic
            let navPath = '/files/';
            if (state.serverHasMultipleSources) {
                // Ensure source is present
                 if (!sourceName) {
                    console.warn("Source missing for file navigation, attempting fallback or root");
                } else {
                    navPath += sourceName + '/';
                }
            }
            navPath += cleanPath;
            
            console.log(`[Heatmap] Final navigation path: ${navPath} (original: ${targetPath})`);
            router.push({ path: navPath })
              .catch(err => console.error("[Heatmap] Router push error:", err));
        }
    };

    window.inspectLocationByCoords = (lat, lng, path, source, bounds) => {
        console.log(`[Heatmap] inspectLocationByCoords: lat=${lat}, lng=${lng}, path=${path}, source=${source}, bounds=`, bounds);
        inspectLocation(path, source, null, {
            lat: parseFloat(lat),
            lon: parseFloat(lng),
            minLat: bounds.minLat,
            maxLat: bounds.maxLat,
            minLon: bounds.minLon,
            maxLon: bounds.maxLon,
            clusterID: bounds.clusterID
        });
    };

    window.inspectLocationDirect = (items, source) => {
        // Fallback to global temp items if not passed (e.g. from onclick)
        const finalItems = items || window._tempInspectItems || [];
        const finalSource = source || (finalItems.length > 0 ? finalItems[0].source : "");
        
        console.log(`[Heatmap] inspectLocationDirect called with ${finalItems.length} items`);
        inspectLocation(null, finalSource, finalItems);
        
        // Cleanup
        if (!items) window._tempInspectItems = null;
    };

    initMap().then(() => {
        loadData();
        
        // Ensure overlays list is fetched immediately, especially for shared links
        fetchOverlays();

        // Auto-inspect if path is provided (e.g. from a shared link)
        if (route.query.path) {
            const p = recursiveDecode(route.query.path);
            const s = recursiveDecode(route.query.source);
            console.log("[Heatmap] Auto-inspecting path:", p);
            inspectLocation(p, s);
        }

        // Now it's safe to load overlay if present
        if (route.query.overlay) {
            // Switch to overlays tab to show the list
            activeTab.value = 'overlays';
            showSidePanel.value = true;
            loadOverlay();
        }
    });
});

// CRITICAL: Cleanup when component unmounts to STOP requests
onBeforeUnmount(() => {
    console.log('Heatmap component unmounting - stopping all requests...');
    
    // 0. Prevent error notifications from firing
    if (errorNotificationTimeout) {
        clearTimeout(errorNotificationTimeout);
        errorNotificationTimeout = null;
    }
    // Remove global event listener to stop processing new errors
    window.removeEventListener('heatmap-image-error', showErrorNotification); // We need to name the handler to remove it properly, or just set failedImageCount to 0 and ignore.
    // Easier: Just nullify the global handler function so dispatchEvent does nothing useful or check unmount state
    window.fileBrowserHeatmapImageError = () => {}; 
    
    // 0. AGGRESSIVE: Stop all image loading immediately by clearing src
    // This forces the browser to cancel pending requests
    const container = document.getElementById('heatmap-container');
    if (container) {
        const images = container.getElementsByTagName('img');
        for (let i = 0; i < images.length; i++) {
            // Remove onerror to prevent triggering our handler
            images[i].onerror = null;
            images[i].src = '';
            // Determine if we can remove them from DOM to be sure
            images[i].style.display = 'none';
        }
    }
    
    // 1. Cancel all ongoing fetch requests
    if (abortController) {
        abortController.abort();
        abortController = null;
    }
    
    // 2. Remove tile layer immediately
    if (map && tileLayer) {
        map.removeLayer(tileLayer);
        tileLayer = null;
    }

    // 3. Remove heatmap layer
    if (map && heatLayer) {
        map.removeLayer(heatLayer);
        heatLayer = null;
    }
    
    if (markers) {
        markers.clearLayers();
        if (map) map.removeLayer(markers);
        markers = null;
    }
    


    // 5. Destroy map
    if (map) {
        map.remove();
        map = null;
    }
    
    // 6. Clear stores
    clusterDataStore.clear();
    renderedMarkerStore.clear();
    
    // 7. Remove global extensions
    delete window.heatmapNavigate;
    delete window.copyLeafletCoords;
    delete window.inspectLocationGlobal;
    delete window.inspectLocationDirect;
    delete window.inspectLocationByCoords;
    delete window._tempInspectItems;
    // CRITICAL: NEVER delete fileBrowserHeatmapImageError. 
    // Images might still be loading or retrying after unmount, causing "is not a function" errors.
    
    console.log('Heatmap cleanup complete - background loading stopped.');
});

watch(() => route.query, (newQ, oldQ) => {
    // Only reload data if PATH or SOURCE changes.
    // Overlay changes are handled by their own independent watcher and shouldn't trigger a marker reload.
    if (newQ.path === oldQ?.path && newQ.source === oldQ?.source) {
         return;
    }
    loadData();
});

// Helper for UI
const openOverlaysTab = () => {
    showSidePanel.value = true;
    isPanelCollapsed.value = false;
    activeTab.value = 'overlays';
};
</script>

<style scoped>
#heatmap-container {
  width: 100%;
  height: 100%;
  min-height: calc(100vh - 4em); /* Adjust for header/sidebar */
  overflow-x: hidden; /* Fix side panel scrolling */
  position: relative; /* Ensure side panel absolute positioning is relative to this */
}

/* Side Panel Styles */
/* Side Panel Styles */
.heatmap-side-panel {
    position: fixed; /* Fixed to viewport to prevent scroll issues */
    top: 64px; /* Offset for main header */
    right: 0 !important; /* Force alignment to right edge */
    bottom: 0;
    width: 350px; /* Slightly wider */
    max-width: 100vw; /* Prevent overflow on small screens */
    height: auto; /* Defined by top/bottom */
    background: rgba(30, 30, 30, 0.98); /* Higher opacity */
    color: #ffffff;
    backdrop-filter: blur(15px);
    z-index: 9999; /* Max z-index */
    display: flex;
    flex-direction: column;
    border-left: 1px solid rgba(255,255,255,0.15);
    box-shadow: -5px 0 20px rgba(0,0,0,0.5);
    transition: transform 0.3s ease;
    box-sizing: border-box; /* Ensure padding/border doesn't add to width */
}
.panel-header {
    padding: 15px;
    background: rgba(0,0,0,0.4);
    display: flex;
    justify-content: flex-start; /* Align left */
    align-items: center;
    gap: 10px; /* Add spacing */
    font-weight: bold;
    color: white;
    border-bottom: 1px solid rgba(255,255,255,0.1);
}
.panel-header span {
    flex: 1; /* Title takes remaining space */
    text-align: right; /* Keep title on right? Or User just said close button on left. */
    padding-right: 10px;
}
.close-btn {
    order: -1; /* Move to first position (Left) using flexbox order */
    background: none;
    border: none;
    color: white;
    cursor: pointer;
}
.panel-content {
    flex: 1;
    overflow-y: auto;
    padding: 10px;
}
.panel-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 5px;
}
.panel-item {
    position: relative;
    aspect-ratio: 1;
    cursor: pointer;
    border-radius: 4px;
    overflow: hidden;
    width: calc(50% - 3px); /* 2 per row with small gap */
    border: 2px solid transparent; /* Reserve space for border */
    transition: border-color 0.2s;
}
.panel-item.active-preview {
    border-color: #ffeb3b !important; /* Bright Yellow highlight */
    box-shadow: 0 0 8px rgba(255, 235, 59, 0.6);
    z-index: 10;
}
.panel-thumb {
    width: 100%;
    height: 100%;
    object-fit: cover;
}
.panel-actions {
    position: absolute;
    bottom: 0;
    left: 0;
    width: 100%;
    background: rgba(0,0,0,0.7);
    display: flex;
    justify-content: space-around;
    padding: 4px 0;
    opacity: 0;
    transition: opacity 0.2s;
}
.panel-item:hover .panel-actions {
    opacity: 1;
}
.panel-actions button {
    background: none;
    border: none;
    color: white;
    cursor: pointer;
    padding: 2px;
}
.panel-actions button i {
    font-size: 16px;
}
.quick-view-modal {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    background: rgba(0,0,0,0.9);
    z-index: 4000;
    display: flex;
    justify-content: center;
    align-items: center;
}
.quick-view-content {
    position: relative;
    max-width: 90%;
    max-height: 90%;
    display: flex;
    flex-direction: column;
    align-items: center;
}
.quick-view-img {
    max-width: 100%;
    max-height: 80vh;
    border-radius: 4px;
    box-shadow: 0 0 20px rgba(0,0,0,0.5);
    user-select: none; /* Prevent selection */
}
/* Navigation Zones */
.nav-zone {
    position: absolute;
    top: 0;
    height: 100%;
    width: 33%;
    z-index: 10;
    cursor: pointer;
    /* border: 1px solid red; /* Debug */ 
}
.nav-zone-left {
    left: 0;
}
.nav-zone-right {
    right: 0;
}
.nav-zone:hover {
    background: rgba(255,255,255,0.05); /* Subtle hint */
}
.quick-view-close {
    position: absolute;
    top: -40px;
    right: 0;
    background: none;
    border: none;
    color: white;
    font-size: 30px;
    cursor: pointer;
    z-index: 20; /* Above nav zones */
}
/* Metadata Overlay */
.metadata-overlay {
    position: absolute;
    bottom: 0px; /* Above bottom edge of image? No, overlay ON image */
    left: 0;
    width: 100%;
    background: rgba(0,0,0,0.7);
    color: white;
    padding: 10px;
    box-sizing: border-box;
    pointer-events: none; /* Let clicks pass through to Nav Zones */
    text-align: left;
    border-bottom-left-radius: 4px;
    border-bottom-right-radius: 4px;
}
.meta-instructions {
    font-size: 15px; /* Increased from 13px */
    color: #ffca28; /* Amber for importance */
    margin-bottom: 4px;
    font-weight: bold;
    text-shadow: 0 1px 2px black;
}
.meta-iptc {
    font-size: 13px; /* Increased from 11px */
    color: #ddd;
    margin-bottom: 4px;
}
.meta-path {
    font-size: 10px;
    color: #aaa;
    word-break: break-all;
}
/* Folder List Styles */
.panel-folder-list {
    display: flex;
    flex-direction: column;
    gap: 5px;
}
.folder-item {
    display: flex;
    align-items: center;
    padding: 10px;
    background: rgba(255,255,255,0.05);
    border-radius: 4px;
    cursor: pointer;
    transition: background 0.2s;
}
.folder-item:hover {
    background: rgba(255,255,255,0.1);
}
.folder-item i {
    margin-right: 10px;
    color: #ffca28;
}
.folder-info {
    flex: 1;
    display: flex;
    flex-direction: column;
}
.folder-name {
    font-weight: bold;
    font-size: 13px;
    word-break: break-all;
}
.folder-count {
    font-size: 11px;
    opacity: 0.7;
}
.folder-item .chevron {
    color: white;
    opacity: 0.5;
}
.panel-sub-header {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 10px;
    font-weight: bold;
    border-bottom: 1px solid rgba(255,255,255,0.1);
    padding-bottom: 5px;
}
.back-btn {
    background: rgba(255,255,255,0.1);
    border: 1px solid rgba(255,255,255,0.2);
    border-radius: 4px;
    color: white;
    cursor: pointer;
    padding: 2px 5px;
    margin-right: 5px;
    transition: background 0.2s;
}
.back-btn:hover {
    background: rgba(255,255,255,0.2);
}
.quick-view-actions {
    margin-top: 15px;
    display: flex;
    gap: 10px;
}
/* Overlay List Styles */
.overlay-section {
    padding: 0 0 10px 0;
    border-bottom: 1px solid rgba(255,255,255,0.1);
    margin-bottom: 10px;
}
.overlay-list-container {
    display: flex;
    flex-direction: column;
    gap: 5px;
    max-height: 250px; /* Limit height */
    overflow-y: auto;
}
.overlay-item {
    display: flex;
    align-items: center;
    padding: 5px;
    background: rgba(255,255,255,0.05);
    border-radius: 4px;
    cursor: pointer;
    transition: background 0.2s;
    border: 1px solid transparent;
}
.overlay-item:hover {
    background: rgba(255,255,255,0.1);
}
.overlay-item.active {
    background: rgba(0, 100, 200, 0.3);
    border-color: rgba(0, 150, 255, 0.5);
}
.overlay-thumb-wrapper {
    width: 32px;
    height: 32px;
    min-width: 32px; /* Prevent shrink */
    margin-right: 10px;
    border-radius: 2px;
    overflow: hidden;
    background: #000;
    display: flex;
    justify-content: center;
    align-items: center;
}
.overlay-thumb-img {
    width: 100%;
    height: 100%;
    object-fit: cover;
}
.overlay-icon {
    font-size: 20px;
    color: #aaa;
}
.overlay-name {
    flex: 1;
    font-size: 13px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}
</style>

<style>
/* GLOBAL STYLES FOR LEAFLET MARKERS (Cannot be scoped) */

@keyframes flashButton {
    0% { transform: scale(1); box-shadow: 0 0 0 0 rgba(255, 69, 0, 0.7); }
    50% { transform: scale(1.2); box-shadow: 0 0 20px 0 rgba(255, 69, 0, 0.7); background: #ffeb3b !important; border-color: red !important; }
    100% { transform: scale(1); box-shadow: 0 0 0 0 rgba(255, 69, 0, 0); }
}
.flash-on-load {
    animation: flashButton 1s ease-in-out 3; /* Flash 3 times */
    z-index: 10000;
}

/* Cluster flare styles */
.marker-cluster {
    background-color: transparent !important; /* Allow our custom flare to show */
}

/* Force overrides for default plugin styles */
.marker-cluster-small, .marker-cluster-medium, .marker-cluster-large {
    width: 24px !important;
    height: 24px !important;
    margin-left: -12px !important; /* Center alignment adjustment matches half width */
    margin-top: -12px !important;
}

.marker-cluster div {
    /* Base styles, specific overrides handle sizes */
    font-size: 10px !important; 
    margin: 0 !important; 
}

/* Styles for the Fan/Single markers */
.custom-fan-marker {
    background: none !important;
    border: none !important;
    pointer-events: auto !important; /* Ensure clicks pass through to content */
}

/* The grid inside the cluster */
.cluster-thumb-grid {
    display: flex;
    flex-wrap: wrap;
    width: 24px !important;
    height: 24px !important;
    overflow: hidden;
    border-radius: 50%;
    background-color: rgba(0,0,0,0.6);
    border: 1px solid white; 
    justify-content: center;
    align-items: center;
    box-shadow: 0 2px 4px rgba(0,0,0,0.3);
    box-sizing: border-box; /* Crucial for border calc */
}

.cluster-thumb {
    width: 11px !important; /* Slightly less than 12 to avoid subpixel rounding wrap */
    height: 11px !important;
    object-fit: cover;
    display: block;
    margin: 0 !important;
    padding: 0 !important;
}

/* Single item inside grid (1 item cluster) */
.cluster-thumb-single-in-grid {
    width: 24px !important; 
    height: 24px !important;
    object-fit: cover;
    display: block;
}

/* The large fan images - STRICTLY SIZED */
.fan-thumb-img {
    width: 48px !important;
    height: 48px !important;
    max-width: 48px !important;
    max-height: 48px !important;
    object-fit: cover;
    border-radius: 50%;
}

.marker-cluster span {
    line-height: 24px !important;
    font-size: 10px !important;
}

/* Flash Animation for Zoom */
.zoom-flash-marker {
    display: flex;
    align-items: center;
    justify-content: center;
    pointer-events: none; /* Let clicks pass through */
}

.zoom-flash-circle {
    width: 20px;
    height: 20px;
    background-color: rgba(255, 235, 59, 0.8); /* Bright Yellow */
    border: 3px solid #fff;
    border-radius: 50%;
    box-shadow: 0 0 15px 5px rgba(255, 235, 59, 0.8);
    animation: zoomFlash 1s ease-out forwards;
}

@keyframes zoomFlash {
    0% {
        transform: scale(0.5);
        opacity: 0;
    }
    10% {
        transform: scale(1.5);
        opacity: 1;
    }
    50% {
        transform: scale(2.5);
        opacity: 0.5;
    }
    100% {
        transform: scale(3.5);
        opacity: 0;
    }
}

/* Grouped Cluster Layout */
.panel-grid-grouped {
    display: flex;
    flex-direction: column;
    gap: 15px;
    padding: 10px;
}

.cluster-group {
    display: flex;
    flex-direction: row;
    background: rgba(0,0,0,0.05); /* Light background for contrast */
    border-radius: 8px;
    overflow: hidden;
}

.cluster-sidebar {
    width: 40px;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: filter 0.2s;
}

.cluster-sidebar:hover {
    filter: brightness(1.2);
}

.cluster-target-icon {
    color: white;
    text-shadow: 0 1px 3px rgba(0,0,0,0.5);
    font-size: 20px;
}

.cluster-items-grid {
    flex: 1;
    display: flex;
    flex-wrap: wrap;
    gap: 5px;
    padding: 5px;
}
</style>



<style scoped>
/* Collapsible Panel Styles */
.heatmap-side-panel {
    transition: transform 0.3s ease;
}
.heatmap-side-panel.collapsed {
    transform: translateX(100%);
    pointer-events: none; /* Let clicks pass through when hidden */
}
/* Ensure the tab is clickable */
.panel-expand-tab {
    pointer-events: auto;
}
</style>
