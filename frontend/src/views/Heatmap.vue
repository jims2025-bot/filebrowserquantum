<template>
  <div id="heatmap-container"></div>
  
  <!-- Back Button Overlay -->
  <div v-if="isFolderMode" style="position: absolute; top: 10px; left: 60px; z-index: 2000;">
      <button @click="goBack" class="button button--flat" style="background: rgba(0,0,0,0.6); color: white; border: 1px solid rgba(255,255,255,0.3); backdrop-filter: blur(4px);">
          <i class="material-icons">arrow_back</i> Back to Folder
      </button>
  </div>
  <div style="position: absolute; bottom: 10px; left: 10px; background: rgba(0,0,0,0.7); color: white; padding: 10px; z-index: 9999; font-size: 12px; max-width: 300px;">
      <b>Debug Info:</b><br>
      Path: {{ route.query.path || "(Global)" }}<br>
      Source: {{ route.query.source || "None" }}<br>
      Total Clusters: {{ debugClusterCount }}<br>
      Status: {{ debugStatus }}
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
import { ref } from 'vue'; // Import ref

import { notify } from "@/notify";

const route = useRoute();
const router = useRouter();
let map = null;
let markers = null;
let heatLayer = null;
const isFolderMode = ref(false);

const debugClusterCount = ref(0);
const debugStatus = ref("Initializing...");

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
        }
        const borderColor = stringToColor(folderPath);

        // Check global error handler
        if (!window.fileBrowserHeatmapImageError) {
             window.fileBrowserHeatmapImageError = (img, path) => {
                 img.style.display='none'; 
                 img.parentElement.style.backgroundColor='#888'; 
                 img.parentElement.innerHTML='<span style=\'color:white;line-height:48px;display:block;text-align:center;font-weight:bold;\'>?</span>';
                 
                 // Dispatch custom event or handle notification
                 // We can use a custom event on window to communicate back to Vue component
                 window.dispatchEvent(new CustomEvent('heatmap-image-error', { detail: { path: path } }));
             };
        }

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

const generatePopupHtml = (markerPath, sourceArg, count) => {
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
            
            popupHtml += `<div style="display:flex; flex-direction:column; gap:5px; margin-top:5px;">`;
            popupHtml += `<button onclick="${viewAction}" class="button button--flat" style="width:100%;cursor:pointer;">View Image</button>`;
            popupHtml += `<button onclick="${folderAction}" class="button button--flat" style="width:100%;cursor:pointer;">Open Folder</button>`;
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

    map = L.map('heatmap-container').setView([0, 0], 2);
    
    // Invalidate size to ensure it knows its dimensions
    map.invalidateSize();

    const osm = L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', { attribution: '&copy; OpenStreetMap contributors' });
    const satellite = L.tileLayer('https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}', { attribution: 'Tiles &copy; Esri' });
    const topo = L.tileLayer('https://server.arcgisonline.com/ArcGIS/rest/services/World_Topo_Map/MapServer/tile/{z}/{y}/{x}', { attribution: 'Tiles &copy; Esri' });
    const dark = L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', { attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>', subdomains: 'abcd', maxZoom: 20 });

    const baseMaps = {
        "Standard": osm,
        "Satellite": satellite,
        "Hybrid": topo,
        "Dark Mode": dark
    };
    
    osm.addTo(map);
    L.control.layers(baseMaps).addTo(map);

    markers = L.markerClusterGroup({
        spiderfyOnMaxZoom: true,
        showCoverageOnHover: true,
        zoomToBoundsOnClick: true, // Restore default to ensure standard behavior works
        maxClusterRadius: 40, // Increased to clump more items together
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
                    
                    // Fallback handled by onerror
                    var style = "width:11px !important; height:11px !important; object-fit:cover !important; margin:0 !important; padding:0 !important; display:block !important;";
                    if (thumbs.length === 1) style = "width:24px !important; height:24px !important; object-fit:cover !important; display:block !important;";
                    
                    innerHtml += '<img src="' + url + '" class="' + cls + '" style="' + style + '" onerror="this.style.display=\'none\';" />';
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

    map.addLayer(markers);
};

const loadData = async () => {
    // wait for map
    if (!map) { 
        await initMap(); 
        if (!map) return;
    }

    markers.clearLayers();
    if (heatLayer) {
        map.removeLayer(heatLayer);
    }
    
    // Clear stores
    clusterDataStore.clear();
    renderedMarkerStore.clear();

    const source = route.query.source || "";
    const path = route.query.path || "";
    let url = '/api/heatmap/global';
    
    // Determine if folder or global
    if (path) {
        url = `/api/heatmap/folder?source=${encodeURIComponent(source)}&path=${encodeURIComponent(path)}`;
        isFolderMode.value = true;
    } else {
        isFolderMode.value = false;
    }

    try {
        const data = await fetchJSON(url);
        let clusters = data.clusters || [];
        
        // Removed shuffle/limit logic to allow full dataset

        const heatPoints = [];
        clusters.forEach(c => {
             // Leaflet.heat expects [lat, lng, intensity]
            heatPoints.push([c.lat, c.lon, Math.min(c.count * 10, 100)]); 

            // Helper to add a marker
            // isGhost = true means invisible marker for counting purposes
            const addMarker = (lat, lon, count, markerPath, sourceArg, isGhost) => {
                 let markerIcon;
                 
                 if (isGhost) {
                     markerIcon = L.divIcon({
                         html: '',
                         className: 'ghost-marker',
                         iconSize: new L.Point(0, 0)
                     });
                 } else {
                     markerIcon = generateMarkerIcon(markerPath, sourceArg, count);
                 }

                const marker = L.marker([lat, lon], { 
                    icon: markerIcon,
                    photoCount: count, 
                    thumbPath: markerPath,   
                    thumbSource: sourceArg
                });
                
                if (!isGhost) {
                    marker.bindPopup(generatePopupHtml(markerPath, sourceArg, count));
                }
                
                markers.addLayer(marker);
                return marker;
            };

            // Explode points if available (Folder View / Spiderfy support)
            // Let's rely on checking c.points
            if (c.points && c.points.length > 0) {
                 // Store all points for future shuffling
                 const key = `${c.lat},${c.lon}`;
                 clusterDataStore.set(key, c.points);
                 
                 // LIMIT FAN SIZE: Randomly select up to 15 items per cluster
                 let pointsToRender = [...c.points];
                 let ghostCount = 0;
                 
                 if (pointsToRender.length > 15) {
                     // Initial randomization
                     for (let i = pointsToRender.length - 1; i > 0; i--) {
                        const j = Math.floor(Math.random() * (i + 1));
                        [pointsToRender[i], pointsToRender[j]] = [pointsToRender[j], pointsToRender[i]];
                    }
                    // Calculate remainder
                    ghostCount = pointsToRender.length - 15;
                    pointsToRender = pointsToRender.slice(0, 15);
                 }

                 const rendered = [];
                 pointsToRender.forEach(p => {
                     // Pass count=1 for specific point
                     const m = addMarker(p.lat, p.lon, 1, p.path, c.source || source, false);
                     rendered.push(m);
                 });
                 
                 // Save render references for updates
                 renderedMarkerStore.set(key, rendered);
                 
                 // Add Ghost Marker if needed
                 if (ghostCount > 0) {
                     addMarker(c.lat, c.lon, ghostCount, null, null, true);
                 }
                 
            } else {
                // Fallback to aggregated marker
                addMarker(c.lat, c.lon, c.count, c.path, c.source || source, false);
            }
        });

        // Add heat layer with slight delay and ensure size
        map.invalidateSize();
        // Check if we have points
        if (heatPoints.length > 0) {
            heatLayer = L.heatLayer(heatPoints, { radius: 25, maxZoom: 18 }).addTo(map);
        }

        if (clusters.length > 0) {
            const bounds = L.latLngBounds(clusters.map(c => [c.lat, c.lon]));
            map.fitBounds(bounds);
        }
        console.log("Heatmap data loaded:", clusters.length, "clusters");
        debugClusterCount.value = clusters.length;
        debugStatus.value = "Loaded successfully";

    } catch (e) {
        console.error("Heatmap load error:", e);
        debugStatus.value = "Error: " + e.message;
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
        console.log("heatmapNavigate called:", { targetPath, sourceName, isFolder });
        
        // Ensure path doesn't have double slashes
        if (targetPath.startsWith('//')) {
            targetPath = targetPath.substring(1);
        }
        
        if (isFolder) {
            router.push({ path: '/heatmap', query: { path: targetPath, source: sourceName } })
              .catch(err => console.error("Router push error:", err));
        } else {
            // For file browsing, we want /files/{path}
            // Ensure we don't end up with /files//path
            // If targetPath starts with /, remove it because /files/ implies root
            let cleanPath = targetPath.startsWith('/') ? targetPath.substring(1) : targetPath;
            
            // Handle multiple sources logic
            let navPath = '/files/';
            if (state.serverHasMultipleSources) {
                // Ensure source is present
                 if (!sourceName) {
                    // Try to extract from path if possible, or warn
                    console.warn("Source missing for file navigation, attempting fallback or root");
                } else {
                    navPath += sourceName + '/';
                    
                    // PREVENT DOUBLE PATHING:
                    // If the provided path from heatmap data ALREADY starts with the source name, strip it.
                    // e.g. cleanPath = "MySource/Folder/File" and sourceName = "MySource"
                    // We don't want /files/MySource/MySource/Folder/File
                    if (cleanPath.startsWith(sourceName + '/')) {
                        cleanPath = cleanPath.substring(sourceName.length + 1);
                    }
                }
            }
            navPath += cleanPath;
            
            console.log("Navigating to files:", navPath);
            router.push({ path: navPath })
              .catch(err => console.error("Router push error:", err));
        }
    };

    initMap().then(loadData);
});

onBeforeUnmount(() => {
    delete window.heatmapNavigate;
});

watch(() => route.query, () => {
    loadData();
});
</script>

<style scoped>
#heatmap-container {
  width: 100%;
  height: 100%;
  min-height: calc(100vh - 4em); /* Adjust for header/sidebar */
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
