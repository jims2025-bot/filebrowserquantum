<template>
  <div id="heatmap-container" @mouseleave="cursorCoords = {lat: 999, lng: 999}"></div>
  
  <!-- Back Button Overlay -->
  <div v-if="isFolderMode" style="position: absolute; top: 10px; left: 60px; z-index: 2000;">
      <button @click="goBack" class="button button--flat" style="background: rgba(0,0,0,0.6); color: white; border: 1px solid rgba(255,255,255,0.3); backdrop-filter: blur(4px);">
          <i class="material-icons">arrow_back</i> Back to Folder
      </button>
  </div>
  <div style="position: absolute; bottom: 10px; left: 10px; background: rgba(0,0,0,0.7); color: white; padding: 10px; z-index: 9999; font-size: 12px; max-width: 300px; border-radius: 4px;">
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
  <!-- Side Panel for Inspection -->
  <div v-if="showSidePanel" class="heatmap-side-panel">
      <div class="panel-header">
          <span>{{ sidePanelTitle }}</span>
          <button @click="closeSidePanel" class="close-btn"><i class="material-icons">close</i></button>
      </div>
      <div class="panel-content">
          <div v-if="sidePanelLoading" class="panel-loading">Loading...</div>
          <div v-else-if="sidePanelData.length === 0" class="panel-empty">No files found.</div>
          
          <!-- Folder List View -->
          <div v-else-if="!currentInspectionFolder" class="panel-folder-list">
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
          <div v-else class="panel-grid-container">
              <div class="panel-sub-header">
                  <button @click="backToFolders" v-if="formattedSidePanelData.length > 1" class="back-btn"><i class="material-icons">arrow_back</i></button>
                  <span>{{ currentInspectionFolder.path.split('/').pop() }}</span>
              </div>
              <div class="panel-grid">
                  <div v-for="file in displayedItems" :key="file.path" class="panel-item" @click="openQuickView(file)">
                      <img :src="file.thumbUrl" class="panel-thumb" loading="lazy">
                      <div class="panel-actions">
                          <button @click.stop="openQuickView(file)" title="Quick View"><i class="material-icons">visibility</i></button>
                          <button @click.stop="navToFolder(file)" title="Open Folder"><i class="material-icons">folder</i></button>
                      </div>
                  </div>
              </div>
              <div v-if="displayedCount < currentInspectionFolder.items.length" style="padding: 10px; text-align: center;">
                  <button @click="loadMore" class="button button--flat" style="width: 100%; justify-content: center;">
                      Load More ({{ currentInspectionFolder.items.length - displayedCount }} remaining)
                  </button>
              </div>
          </div>
      </div>
  </div>

  <!-- Quick View Modal -->
  <div v-if="quickViewFile" class="quick-view-modal" @click="closeQuickView">
      <div class="quick-view-content" @click.stop>
          <img :src="quickViewFile.previewUrl" class="quick-view-img">
          <button class="quick-view-close" @click="closeQuickView"><i class="material-icons">close</i></button>
          <div class="quick-view-actions">
               <button @click="navToFolder(quickViewFile)" class="button button--flat">Open Folder</button>
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

const route = useRoute();
const router = useRouter();
let map = null;
let markers = null;
let heatLayer = null;
let tileLayer = null; // Store reference to tile layer
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
const sidePanelLoading = ref(false);

// Pagination
const itemsPerPage = 50;
const displayedCount = ref(itemsPerPage);

const displayedItems = computed(() => {
    // If viewing a folder, show its items
    if (currentInspectionFolder.value) {
        return currentInspectionFolder.value.items.slice(0, displayedCount.value);
    }
    // Otherwise show folders list? No, "groups" are needed in the template.
    return [];
});

const displayedFolders = computed(() => {
    // Top level list of folders
    return formattedSidePanelData.value; 
});

const openFolderView = (folderObj) => {
    // Drill-Down Support:
    // If this is a "Virtual Folder" returned from a parent cluster,
    // clicking it should trigger inspection of THAT folder's specific cluster.
    if (folderObj.isVirtual && folderObj.clusterID) {
        inspectLocation(folderObj.path, folderObj.source, null, { clusterID: folderObj.clusterID });
        return;
    }

    currentInspectionFolder.value = folderObj;
    displayedCount.value = itemsPerPage;
};

const backToFolders = () => {
    currentInspectionFolder.value = null;
};

const loadMore = () => {
    displayedCount.value += itemsPerPage;
};

const closeSidePanel = () => {
    showSidePanel.value = false;
    displayedCount.value = itemsPerPage; // Reset logic check
};

// Quick View State
const quickViewFile = ref(null);

const openQuickView = (file) => {
    quickViewFile.value = {
        ...file,
        previewUrl: getPreviewUrl(file.path, file.source, 'large')
    };
};

const closeQuickView = () => {
    quickViewFile.value = null;
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
    window.heatmapNavigate(file.parentPath || "/", file.source, false); // Reuse existing helper logic for folder
};

// Helper to get preview URL
const getPreviewUrl = (path, source, size) => {
     const params = new URLSearchParams();
     params.append('path', path);
     if (source) params.append('source', source);
     params.append('size', size);
     const url = '/api/preview?' + params.toString();
     console.log(`[Heatmap] getPreviewUrl: path='${path}', source='${source}', size='${size}' => ${url}`);
     return url;
};

const isImageFile = (filename) => {
    if (!filename) return false;
    const ext = filename.split('.').pop().toLowerCase();
    return ['jpg', 'jpeg', 'png', 'gif', 'webp', 'bmp', 'heic'].includes(ext);
};

// Main Inspector Logic
const inspectLocation = async (path, source, directItems = null, coords = null) => {
    console.log(`[Heatmap] inspectLocation called: path='${path}', source='${source}', coords=`, coords);
    
    showSidePanel.value = true;
    sidePanelLoading.value = true;
    sidePanelData.value = [];
    currentInspectionFolder.value = null; // Reset folder view
    formattedSidePanelData.value = []; // Reset grouped data
    displayedCount.value = itemsPerPage; // Reset pagination
    
    // Exact location inspection via Backend API (Tile Clusters)
    if (coords) {
         try {
            // Include Zoom Level for radius search
            const currentZoom = map.getZoom();
            let url = `/api/heatmap/inspect?lat=${coords.lat}&lon=${coords.lon}&zoom=${currentZoom}&source=${encodeURIComponent(source||'')}&path=${encodeURIComponent(path||'')}`;
            console.log(`[Heatmap] Inspect API URL: ${url}`);
            
            // Add Bounds if available
            if (coords.minLat !== undefined) {
                url += `&minLat=${coords.minLat}&maxLat=${coords.maxLat}&minLon=${coords.minLon}&maxLon=${coords.maxLon}`;
            }
            
            // Add Cluster ID for exact match
            // DISABLED: Sending ID restricts result to single item if ID is shared/representative.
            // forcing spatial search ensures we get all items in the cluster area.
            /*
            if (coords.clusterID) {
                url += `&cluster_id=${encodeURIComponent(coords.clusterID)}`;
            }
            */

            const res = await fetch(url);
            if (res.ok) {
                const rawText = await res.text();
                console.log('[Heatmap] Inspect Raw Response:', rawText);
                const items = JSON.parse(rawText);
                if (items && items.length > 0) {
                     sidePanelData.value = items.map(item => ({
                        name: item.previewID || item.path.split('/').pop(),
                        path: item.path,
                        source: item.source || source,
                        parentPath: item.path.substring(0, item.path.lastIndexOf('/')), // Grouping Key
                        thumbUrl: getPreviewUrl(item.path, item.source || source, 'small'),
                        // Drill-down fields
                        type: item.type,
                        count: item.count,
                        clusterID: item.id
                    }));
                    
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
                                source: item.source 
                            };
                        } else {
                            const p = item.parentPath || "Root";
                            if (!groups[p]) groups[p] = { path: p, count: 0, items: [] };
                            // Robust count: If item has count > 1, use it. Otherwise count as 1 item.
                            groups[p].count += (item.count && item.count > 1 ? item.count : 1);
                            groups[p].items.push(item);
                        }
                    });
                    
                    formattedSidePanelData.value = Object.values(groups);
                    
                    // If only one folder, auto-open it?
                    // ONLY if it's NOT a virtual folder (virtual folders require click to drill)
                    if (formattedSidePanelData.value.length === 1 && !formattedSidePanelData.value[0].isVirtual) {
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
            thumbUrl: getPreviewUrl(item.path, item.source, 'small')
        }));
        
        // Grouping
        const groups = {};
        sidePanelData.value.forEach(item => {
             const p = item.parentPath || "Root";
             if (!groups[p]) groups[p] = { path: p, count: 0, items: [] };
             groups[p].count += (item.count && item.count > 1 ? item.count : 1);
             groups[p].items.push(item);
        });
        formattedSidePanelData.value = Object.values(groups);
        
        if (formattedSidePanelData.value.length === 1) {
            currentInspectionFolder.value = formattedSidePanelData.value[0];
        }

        sidePanelTitle.value = `Selected Items (${sidePanelData.value.length})`;
        sidePanelLoading.value = false;
        return;
    }

    sidePanelTitle.value = "Loading...";
    
    // Fallback logic for general folder inspection (no coordinates interaction)
    try {
        let targetPath = path;
        const apiUrl = `/api/resources?path=${encodeURIComponent(targetPath)}&source=${encodeURIComponent(source || "")}`;
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
                    source: item.source || source,
                    parentPath: targetPath,
                    thumbUrl: getPreviewUrl(item.path || (targetPath + "/" + item.name), item.source || source, 'small')
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





// Error tracking
const failedImageCount = ref(0);
const failedImages = ref(new Set());
let errorNotificationTimeout = null;

const showErrorNotification = () => {
    if (errorNotificationTimeout) clearTimeout(errorNotificationTimeout);
    errorNotificationTimeout = setTimeout(() => {
        if (failedImageCount.value > 0) {
            notify.showError(`Warning: ${failedImageCount.value} images failed to load (corrupted or unsupported).`);
        }
    }, 2000); // 2 second debounce
};

// Stores for dynamic reshuffling
const clusterDataStore = new Map(); // Key: `${lat},${lon}`, Value: Array of all points for that location
const renderedMarkerStore = new Map(); // Key: `${lat},${lon}`, Value: Array of L.Marker objects currently rendered for that location
const goBack = () => {
    if (route.query.source && route.query.path) {
         const src = route.query.source;
         const p = route.query.path;
         // Ensure no double slashes if path starts with /
         let cleanP = p.startsWith('/') ? p.substring(1) : p;
         // Prevent double source in path (e.g. PHOTOS/PHOTOS/...)
         if (cleanP.startsWith(src + '/')) {
             cleanP = cleanP.substring(src.length + 1);
         }
         router.push({ path: `/files/${src}/${cleanP}` }).catch(err => console.error(err));
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


const initMap = async () => {
    if (map) return;
    await nextTick();
    
    // Ensure container exists
    const container = document.getElementById('heatmap-container');
    if (!container) return;

    map = L.map('heatmap-container', {
        worldCopyJump: true // Enable infinite scrolling
    }).setView([0, 0], 2);
    
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
        
        // Try to find if we clicked on a known location/path context?
        // Hard to know without clicking a marker.
        // But we can offer "Inspect Global" or just coordinate copy.
        
        L.popup()
            .setLatLng(e.latlng)
            .setContent(`
                <div style="text-align:center; font-size:12px;">
                    <b>Lat:</b> ${lat}<br>
                    <b>Lon:</b> ${lng}<br>
                    <button onclick="window.copyLeafletCoords('${lat}', '${lng}')" 
                        class="button button--flat" 
                        style="margin-top:5px; padding:2px 8px; font-size:11px; cursor:pointer;">
                        Copy Coordinates
                    </button>
                    <div id="copy-status-${lat.replace('.','-')}" style="color:green; display:none; font-size:10px; margin-top:2px;">Copied!</div>
                </div>
            `)
            .openOn(map);
    });

    // Global helper for the popup button
    window.copyLeafletCoords = (lat, lng) => {
        const text = `${lat}, ${lng}`;
        navigator.clipboard.writeText(text).then(() => {
            const statusEl = document.getElementById(`copy-status-${lat.replace('.','-')}`);
            if (statusEl) {
                statusEl.style.display = 'block';
                setTimeout(() => { statusEl.style.display = 'none'; }, 2000);
            }
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
        noWrap: true
    });
    const satellite = L.tileLayer('https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}', { 
        attribution: 'Tiles &copy; Esri',
        noWrap: true
    });
    const topo = L.tileLayer('https://server.arcgisonline.com/ArcGIS/rest/services/World_Topo_Map/MapServer/tile/{z}/{y}/{x}', { 
        attribution: 'Tiles &copy; Esri',
        noWrap: true
    });
    const dark = L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', { 
        attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>', 
        subdomains: 'abcd', 
        maxZoom: 20,
        noWrap: true
    });

    const baseMaps = {
        "Standard": osm,
        "Satellite": satellite,
        "Hybrid": topo,
        "Dark Mode": dark
    };
    
    osm.addTo(map);
    L.control.layers(baseMaps).addTo(map);

    // Use MarkerClusterGroup to enable Spiderfy effect
    markers = L.markerClusterGroup({
        spiderfyOnMaxZoom: true, // Enable spiderfy
        showCoverageOnHover: false, // Disable hover for performance
        zoomToBoundsOnClick: true,
        maxClusterRadius: 35, // Small radius to only cluster very close items
        disableClusteringAtZoom: 18, // Stop frontend clustering at Z18 to stop "dancing" markers
        spiderfyDistanceMultiplier: 2, 
        spiderLegPolylineOptions: { weight: 1.5, color: '#222', opacity: 0.5 },
        iconCreateFunction: function(cluster) {
            var childCount = 0;
            var children = cluster.getAllChildMarkers();
            var thumbs = [];

            for (var i = 0; i < children.length; i++) {
                var m = children[i];
                // Sum up weights
                childCount += (m.options.photoCount || 1);
                
                // Collect potential thumbnails (up to 4)
                if (thumbs.length < 4 && m.options.thumbPath && m.options.thumbSource) {
                    thumbs.push({ path: m.options.thumbPath, source: m.options.thumbSource });
                }
            }

            var c = ' marker-cluster-';
            if (childCount < 10) {
                c += 'small';
            } else if (childCount < 100) {
                c += 'medium';
            } else {
                c += 'large';
            }

            // Generate HTML for thumbs (The Grid)
            var innerHtml = '';
            if (thumbs.length > 0) {
                innerHtml = '<div class="cluster-thumb-grid" style="width:24px !important; height:24px !important; display:flex !important; flex-wrap:wrap !important; justify-content:center; align-items:center; overflow:hidden; border-radius:50%; border:1px solid white; background-color:rgba(0,0,0,0.6); box-shadow:0 2px 4px rgba(0,0,0,0.3); box-sizing:border-box;">';
                thumbs.forEach(t => {
                    // Force small size preview with cache bust
                    var url = '/api/preview?path=' + encodeURIComponent(t.path) + '&source=' + encodeURIComponent(t.source) + '&size=small';
                    var cls = thumbs.length === 1 ? 'cluster-thumb cluster-thumb-single-in-grid' : 'cluster-thumb';
                    
                    // Fallback handled by onerror - wrap in try-catch for safety
                    var style = "width:11px !important; height:11px !important; object-fit:cover !important; margin:0 !important; padding:0 !important; display:block !important;";
                    if (thumbs.length === 1) style = "width:24px !important; height:24px !important; object-fit:cover !important; display:block !important;";
                    
                    try {
                        innerHtml += '<img src="' + url + '" class="' + cls + '" style="' + style + '" onerror="this.style.display=\'none\';" />';
                    } catch (e) {
                        console.warn('Failed to generate thumbnail HTML:', e);
                    }
                });
                
                // Overlay count
                innerHtml += '<span style="position:absolute; top:50%; left:50%; transform:translate(-50%, -50%); background:rgba(0,0,0,0.7); border-radius:10px; padding:0 4px; color:white; font-size:10px; font-weight:bold; white-space:nowrap;">' + childCount + '</span>';
                innerHtml += '</div>';
            } else {
                 innerHtml = '<div style="background-color:rgba(100,100,100,0.5);border-radius:50%;width:24px;height:24px;text-align:center;"><span style="color:white;text-shadow:0 0 2px black;line-height:24px;">' + childCount + '</span></div>';
            }

            return new L.DivIcon({ 
                html: innerHtml, 
                className: 'marker-cluster' + c, 
                iconSize: new L.Point(24, 24) 
            });
        }
    });

    // Debugging handlers
    markers.on('clusterclick', function (a) {
        // console.log("DEBUG: clusterclick fired. Child count:", a.layer.getAllChildMarkers().length);
        const cluster = a.layer;
        const children = cluster.getAllChildMarkers();
        if (children.length === 0) return;
        
        // Assume first child location is representative for the key
        // Note: All markers in a cluster have the same lat/lon for the purpose of clusterDataStore key
        const lat = children[0].getLatLng().lat;
        const lon = children[0].getLatLng().lng;
        const key = `${lat},${lon}`;
        
        // Check if we need to reshuffle
        if (clusterDataStore.has(key) && renderedMarkerStore.has(key)) {
            const allPoints = clusterDataStore.get(key);
            const rendered = renderedMarkerStore.get(key);
            
            if (allPoints.length > 15) {
                // Time to reshuffle!
                console.log("Reshuffling cluster on click:", key);
                
                // Pick 15 NEW randoms
                const newSelection = [];
                const tempPool = [...allPoints];
                 // Fisher-Yates shuffle for new selection
                for (let i = tempPool.length - 1; i > 0; i--) {
                    const j = Math.floor(Math.random() * (i + 1));
                    [tempPool[i], tempPool[j]] = [tempPool[j], tempPool[i]];
                }
                const newPoints = tempPool.slice(0, 15);
                
                // Update existing markers in-place
                // We iterate through the rendered *visible* markers (which are the first 15 in the array usually)
                // Note: The renderedMarkerStore array holds the marker objects.
                rendered.forEach((marker, index) => {
                    if (index < newPoints.length) {
                        const newP = newPoints[index];
                        const source = marker.options.thumbSource || newP.source || ""; // Fallback to existing source if newP.source is undefined
                        
                        // Update Options
                        marker.options.thumbPath = newP.path;
                        marker.options.thumbSource = source; 
                        
                        // Update Icon
                        marker.setIcon(generateMarkerIcon(newP.path, source, 1)); // Count is 1 for leaf
                        
                        // Update Popup
                        marker.bindPopup(generatePopupHtml(newP.path, source, 1));
                    }
                });
                // Note: The ghost marker (if it exists) is not in renderedMarkerStore, so it stays untouched (good).
            }
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
                 if (p) {
                     directItems.push({ path: p, source: s, name: p.split('/').pop(), count: c });
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
             } else {
                 // Fallback to Backend Inspection with Bounds?
                 // Typically markercluster has children on client.
                 // But if they are badges...
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
    
    // Clear stores
    clusterDataStore.clear();
    renderedMarkerStore.clear();

    const source = route.query.source || "";
    const path = route.query.path || "";
    
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
    const onZoomEnd = () => {
        const currentZoom = Math.round(map.getZoom());
        tileMarkers.forEach((layers, key) => {
            // key format: z-x-y
            const z = parseInt(key.split('-')[0]);
            // Keep current zoom level and adjacent levels (±1)
            const zoomDiff = Math.abs(z - currentZoom);
            if (zoomDiff > 1) {
                try {
                    // Remove checks from registry
                    layers.forEach(m => {
                        if (m.options.clusterPath) renderedPaths.delete(m.options.clusterPath);
                    });
                    markers.removeLayers(layers);
                } catch(e) {}
                tileMarkers.delete(key);
            }
        });
        
        // Safety: Clear registry of things that might have been removed implicitly?
        // No, rely on manual management for now.
    };
    map.on('zoomend', onZoomEnd);
    
    // Clean up listener if loadData is called again (handled by abortController logic implicitly? No, map events persist)
    // We should attach a cleanup to the abort signal or just rely on global cleanup?
    // Proper way: Remove old listener if exists. 
    // But `onZoomEnd` is local.
    // Hack: Attach it to the map object to track it?
    if (map._heatmapZoomHandler) {
        map.off('zoomend', map._heatmapZoomHandler);
    }
    map._heatmapZoomHandler = onZoomEnd;

    // Helper to render markers for a tile
    const addMarkersForTile = (data, coords, tileKey) => {
        // Strict Zoom Guard: Don't add markers if map has zoomed away
        if (map && Math.round(map.getZoom()) !== coords.z) {
            return;
        }

        const tileMarkerList = [];
        let clusters = data.clusters || [];
        
        clusters.forEach(c => {
            // STRICT DEDUPLICATION:
            if (renderedPaths.has(c.path)) {
                // Already rendered this exact cluster path
                return; 
            }
            renderedPaths.add(c.path);

            try {
                const zoom = coords.z;
                
                // Show badged markers for clusters (Zoom < 15)
                if (zoom < 15) {
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
                    
                    const onBadgeContextMenu = (e, clusterPath, clusterSource, minLat, minLon, maxLat, maxLon, clusterID) => {
                        L.DomEvent.stopPropagation(e);
                        const lat = e.latlng.lat.toFixed(5);
                        const lng = e.latlng.lng.toFixed(5);
                        const safePath = (clusterPath || "").replace(/'/g, "\\'");
                        const safeSource = (clusterSource || source).replace(/'/g, "\\'");
                        const cID = clusterID || "";
                        const boundsObj = {minLat, minLon, maxLat, maxLon, clusterID: cID};
                        const boundsJson = JSON.stringify(boundsObj).replace(/"/g, "&quot;");
                        
                        L.popup().setLatLng(e.latlng).setContent(`
                            <div style="text-align:center; font-size:12px;">
                                <b>Lat:</b> ${lat}<br><b>Lon:</b> ${lng}<br>
                                <div style="margin-top:5px; display:flex; flex-direction:column; gap:4px;">
                                    <button onclick="window.copyLeafletCoords('${lat}', '${lng}')" class="button button--flat" style="font-size:11px; cursor:pointer;">Copy</button>
                                    <button onclick='window.inspectLocationByCoords("${lat}", "${lng}", "${safePath}", "${safeSource}", ${boundsJson})' class="button button--flat" style="font-size:11px; cursor:pointer; background:rgba(0,100,200,0.3);">Inspect Files</button>
                                </div>
                            </div>
                        `).openOn(map);
                    };

                    if (c.min && c.max) {
                        marker.on('contextmenu', (e) => onBadgeContextMenu(e, c.path, c.source || source, c.min[0], c.min[1], c.max[0], c.max[1], c.id));
                    } else {
                        marker.on('contextmenu', (e) => onBadgeContextMenu(e, c.path, c.source || source, 0, 0, 0, 0, c.id));
                    }

                    marker.bindPopup(`<b>${c.count} photos</b><br>Zoom in for details`);
                    tileMarkerList.push(marker);

                } else if (c.count > 15) {
                    // Large Badge
                     const marker = L.marker([c.lat, c.lon], {
                        icon: L.divIcon({
                            html: `<div style="background:rgba(255,100,0,0.8);color:white;border-radius:50%;width:36px;height:36px;display:flex;align-items:center;justify-content:center;font-size:12px;font-weight:bold;border:2px solid white;box-shadow:0 2px 5px rgba(0,0,0,0.3);">${c.count}</div>`,
                            className: 'tile-cluster-badge-large',
                            iconSize: [36, 36]
                        }),
                        photoCount: c.count
                    });
                    marker.bindPopup(`<b>${c.count} photos</b>`);
                    tileMarkerList.push(marker);
                    
                    const onBadgeContextMenu = (e, clusterPath, clusterSource, minLat, minLon, maxLat, maxLon, clusterID) => {
                        L.DomEvent.stopPropagation(e);
                        const lat = e.latlng.lat.toFixed(5);
                        const lng = e.latlng.lng.toFixed(5);
                        const safePath = (clusterPath || "").replace(/'/g, "\\'");
                        const safeSource = (clusterSource || source).replace(/'/g, "\\'");
                        const cID = clusterID || "";
                        const boundsObj = {minLat, minLon, maxLat, maxLon, clusterID: cID};
                        const boundsJson = JSON.stringify(boundsObj).replace(/"/g, "&quot;");
                        
                        L.popup().setLatLng(e.latlng).setContent(`
                            <div style="text-align:center; font-size:12px;">
                                <b>Lat:</b> ${lat}<br><b>Lon:</b> ${lng}<br>
                                <div style="margin-top:5px; display:flex; flex-direction:column; gap:4px;">
                                    <button onclick="window.copyLeafletCoords('${lat}', '${lng}')" class="button button--flat" style="font-size:11px; cursor:pointer;">Copy</button>
                                    <button onclick='window.inspectLocationByCoords("${lat}", "${lng}", "${safePath}", "${safeSource}", ${boundsJson})' class="button button--flat" style="font-size:11px; cursor:pointer; background:rgba(0,100,200,0.3);">Inspect Files</button>
                                </div>
                            </div>
                        `).openOn(map);
                    };

                    if (c.min && c.max) {
                        marker.on('contextmenu', (e) => onBadgeContextMenu(e, c.path, c.source || source, c.min[0], c.min[1], c.max[0], c.max[1], c.id));
                    } else {
                        marker.on('contextmenu', (e) => onBadgeContextMenu(e, c.path, c.source || source, 0, 0, 0, 0, c.id));
                    }

                } else {
                    // Fan/Small Clusters
                    let maxFanSize = (zoom >= 17) ? 12 : (zoom === 16 ? 5 : 3);
                    
                    if (c.points && c.points.length > 0) {
                        // 1. Show Central Count Badge (User Request: "counts on all zoom levels")
                        // Use a smaller, lighter badge to avoid obscuring the photos
                        if (c.count > 1) {
                            const centerBadge = L.marker([c.lat, c.lon], {
                                icon: L.divIcon({
                                    html: `<div style="background:rgba(0,0,0,0.5);color:white;border-radius:50%;width:20px;height:20px;display:flex;align-items:center;justify-content:center;font-size:10px;border:1px solid white;">${c.count}</div>`,
                                    className: 'tile-cluster-badge-small',
                                    iconSize: [20, 20]
                                }),
                                interactive: false // Let clicks pass through? Or maybe strictly for info.
                            });
                             tileMarkerList.push(centerBadge);
                        }

                        // 2. Show individual points (Fan)
                        const pointsToShow = c.points.slice(0, maxFanSize);
                        const fanRadius = 0.0002; 
                        const angleStep = (2 * Math.PI) / pointsToShow.length;
                        // Context for precise inspection
                        const context = {
                            lat: c.lat, lon: c.lon, clusterID: c.id,
                            minLat: c.min ? c.min[0] : 0, minLon: c.min ? c.min[1] : 0,
                            maxLat: c.max ? c.max[0] : 0, maxLon: c.max ? c.max[1] : 0
                        };

                        pointsToShow.forEach((p, idx) => {
                            const angle = idx * angleStep;
                            const marker = L.marker([c.lat + fanRadius * Math.cos(angle), c.lon + fanRadius * Math.sin(angle)], {
                                icon: generateMarkerIcon(p.path, c.source || source, 1),
                            });
                            marker.bindPopup(generatePopupHtml(p.path, c.source || source, 1, context));
                            marker.on('contextmenu', () => {
                                let parent = p.path.substring(0, p.path.lastIndexOf('/'));
                                inspectLocation(parent, c.source || source);
                            });
                            tileMarkerList.push(marker);
                        });
                    } else {
                        // Single cluster marker
                        const context = {
                            lat: c.lat, lon: c.lon, clusterID: c.id,
                            minLat: c.min ? c.min[0] : 0, minLon: c.min ? c.min[1] : 0,
                            maxLat: c.max ? c.max[0] : 0, maxLon: c.max ? c.max[1] : 0
                        };
                        const marker = L.marker([c.lat, c.lon], {
                             icon: generateMarkerIcon(c.path, c.source || source, c.count),
                             thumbPath: c.path,
                             thumbSource: c.source || source
                        });
                        marker.bindPopup(generatePopupHtml(c.path, c.source || source, c.count, context));
                        tileMarkerList.push(marker);
                    }
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
                            photoCount: c.count
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
                                    thumbSource: c.source || source
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
                                thumbSource: c.source || source
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
            if (map.getZoom() > 17) return;

            heatPoints.length = 0; // Clear array
            loadedTiles.forEach(tileData => {
                if (tileData.clusters) {
                    tileData.clusters.forEach(c => {
                        // Add point with intensity based on count
                        heatPoints.push([c.lat, c.lon, Math.min(c.count / 5, 1.0)]); // Normalize intensity
                    });
                }
            });
            
            // Update or create heatmap layer
            if (heatLayer) {
                map.removeLayer(heatLayer);
            }
            if (heatPoints.length > 0) {
                const zoom = map.getZoom();
                // Boost visibility at low zoom
                // Boost visibility at low zoom
                const radius = zoom < 10 ? 60 : (zoom < 13 ? 40 : 25);
                const blur = zoom < 10 ? 40 : (zoom < 13 ? 35 : 25);

                heatLayer = L.heatLayer(heatPoints, {
                    radius: radius,
                    blur: blur,
                    maxZoom: 17, // Enable heatmap up to zoom 17
                    max: 0.8, // Slightly lower max intensity to avoid blowout at large radius
                    gradient: {0.4: 'blue', 0.6: 'cyan', 0.7: 'lime', 0.8: 'yellow', 1.0: 'red'}
                });
                
                if (map.getZoom() <= 17) {
                    map.addLayer(heatLayer);
                }
            }
        }, 50); // 50ms debounce for better responsiveness
    };

    
    // Update heatmap when zoom changes
    map.on('zoomend', () => {
        const currentZoom = map.getZoom();
        if (currentZoom <= 17 && heatLayer && !map.hasLayer(heatLayer)) {
            map.addLayer(heatLayer);
        } else if (currentZoom > 17 && heatLayer && map.hasLayer(heatLayer)) {
            map.removeLayer(heatLayer);
        }
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

    initMap().then(loadData);
});

// CRITICAL: Cleanup when component unmounts to STOP requests
onBeforeUnmount(() => {
    console.log('Heatmap component unmounting - stopping all requests...');
    
    // 0. AGGRESSIVE: Stop all image loading immediately by clearing src
    // This forces the browser to cancel pending requests
    const container = document.getElementById('heatmap-container');
    if (container) {
        const images = container.getElementsByTagName('img');
        for (let i = 0; i < images.length; i++) {
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

watch(() => route.query, () => {
    // Re-load data when query changes (but proper cleanup happens in loadData)
    loadData();
});
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
    background: none;
    border: none;
    color: white;
    cursor: pointer;
    padding: 0;
}
.quick-view-actions {
    margin-top: 15px;
    display: flex;
    gap: 10px;
}
</style>

<style>
/* GLOBAL STYLES FOR LEAFLET MARKERS (Cannot be scoped) */

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
</style>
