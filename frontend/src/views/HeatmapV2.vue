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
       style="position:absolute; top:0; left:0; z-index:11000; pointer-events:none; border: 1px solid rgba(255,0,0,0.2);">
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
  <div style="position:absolute; top:10px; right:10px; z-index:2000; display:flex; gap:10px; align-items:center;">
      <!-- Toggle Clusters Switch -->
      <div style="background:rgba(255,255,255,0.9); padding:5px 12px; border-radius:20px; box-shadow:0 2px 4px rgba(0,0,0,0.2); display:flex; align-items:center; gap:8px; cursor:pointer;" @click="showImageClusters = !showImageClusters">
          <i class="material-icons" :style="{ color: showImageClusters ? '#42a5f5' : '#777', fontSize: '20px' }">
              {{ showImageClusters ? 'layers' : 'layers_clear' }}
          </i>
          <span style="font-size:12px; font-weight:bold; color:#333; white-space:nowrap;">{{ showImageClusters ? 'Clusters On' : 'Clusters Off' }}</span>
          <div style="width:34px; height:18px; background:#ccc; border-radius:10px; position:relative; transition:background 0.3s;" :style="{ background: showImageClusters ? '#42a5f5' : '#ccc' }">
              <div style="width:14px; height:14px; background:white; border-radius:10px; position:absolute; top:2px; left:2px; transition:transform 0.3s;" :style="{ transform: showImageClusters ? 'translateX(16px)' : 'translateX(0)' }"></div>
          </div>
      </div>

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
              <div @click="currentInspectionFolder ? backToFolders() : activeTab = 'inspection'"
                   :title="currentInspectionFolder ? 'Back to Folder List' : 'Inspection'"
                   :style="{ borderBottom: activeTab === 'inspection' ? '3px solid #fbc02d' : '3px solid transparent', opacity: activeTab === 'inspection' ? 1 : 0.6, flex: 1, textAlign:'center', padding:'8px', cursor:'pointer', color:'white', position:'relative', fontSize: '12px' }">
                   <!-- When a folder is selected: overlay a back arrow on the folder icon -->
                   <span v-if="currentInspectionFolder" style="position:relative; display:inline-flex; align-items:center; justify-content:center;">
                     <i class="material-icons" style="font-size: 20px; color: #fbc02d;">folder</i>
                     <i class="material-icons" style="font-size: 12px; color: white; position:absolute; bottom:-2px; right:-4px; background:#fbc02d; border-radius:50%; padding:1px;">arrow_back</i>
                   </span>
                   <div v-else style="display:flex; flex-direction:column; align-items:center;">
                       <i class="material-icons" style="font-size: 20px; color: #fbc02d;">folder</i>
                       <b>Folders</b>
                   </div>
              </div>
              <div @click="activeTab = 'overlays'" title="File Overlays"
                   :style="{ borderBottom: activeTab === 'overlays' ? '3px solid #4f83cc' : '3px solid transparent', opacity: activeTab === 'overlays' ? 1 : 0.6, flex: 1, textAlign:'center', padding:'8px', cursor:'pointer', color:'white', display:'flex', flexDirection:'column', alignItems:'center', justifyContent:'center', fontSize: '12px' }">
                   <i class="material-icons" style="font-size: 20px; color: #42a5f5;">map</i>
                   <span style="font-weight: bold;">Overlays</span>
              </div>
              <div @click="activeTab = 'external'" title="External Maps"
                   :style="{ borderBottom: activeTab === 'external' ? '3px solid #66bb6a' : '3px solid transparent', opacity: activeTab === 'external' ? 1 : 0.6, flex: 1, textAlign:'center', padding:'8px', cursor:'pointer', color:'white', display:'flex', flexDirection:'column', alignItems:'center', justifyContent:'center', fontSize: '12px' }">
                   <i class="material-icons" style="font-size: 20px; color: #81c784;">public</i>
                   <span style="font-weight: bold;">External</span>
              </div>
              <div @click="activeTab = 'draw'" title="Drawing Tools"
                   :style="{ borderBottom: activeTab === 'draw' ? '3px solid #ef5350' : '3px solid transparent', opacity: activeTab === 'draw' ? 1 : 0.6, flex: 1, textAlign:'center', padding:'8px', cursor:'pointer', color:'white', display:'flex', flexDirection:'column', alignItems:'center', justifyContent:'center', fontSize: '12px' }">
                   <i class="material-icons" style="font-size: 20px; color: #ef5350;">edit</i>
                   <span style="font-weight: bold;">Draw</span>
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
                        :style="isOverlayActive(item.path, item.source) ? 'background:rgba(79, 131, 204, 0.3); border:1px solid #4f83cc;' : 'background:rgba(255,255,255,0.05);'"
                        @click="toggleOverlay(item.path, item.source)">
                                             <i class="material-icons" style="margin-right:10px; color:#42a5f5;">{{ isOverlayActive(item.path, item.source) ? 'check_box' : 'check_box_outline_blank' }}</i>
                      <div style="flex:1; overflow:hidden;">
                          <!-- Clean Name Display: Use Name if available, else filename -->
                          <div class="overlay-name" style="font-weight:500; font-size: 13px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;">
                             {{ item.name || item.path.split('/').pop() }}
                          </div>
                           <!-- Opacity Slider -->
                           <div v-if="isOverlayActive(item.path, item.source)" style="margin-top: 5px; display: flex; align-items: center; gap: 8px;" @click.stop>
                               <i class="material-icons" style="font-size: 14px; color: #aaa;">opacity</i>
                               <input type="range" min="0" max="1" step="0.1" 
                                      :value="getOverlayOpacity(item.path, item.source)" 
                                      @input="updateOverlayOpacity(item.path, $event.target.value, item.source)"
                                      style="flex: 1; height: 4px; accent-color: #42a5f5; cursor: pointer;">
                               <span style="font-size: 10px; min-width: 25px;">{{ Math.round(getOverlayOpacity(item.path, item.source) * 100) }}%</span>
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

           <!-- External Maps View -->
           <div v-else-if="activeTab === 'external'" class="overlay-section" style="padding:10px;">
               <div v-for="(group, categoryName) in Object.groupBy(externalMaps, m => m.category || 'Uncategorized')" :key="categoryName" style="margin-bottom:15px;">
                   <div @click="collapsedCategories.has(categoryName) ? collapsedCategories.delete(categoryName) : collapsedCategories.add(categoryName)"
                        style="font-weight:bold; color:#aaa; font-size:12px; margin-bottom:5px; border-bottom:1px solid #444; display:flex; align-items:center; cursor:pointer; padding:4px 0;">
                      <i class="material-icons" style="font-size:16px; margin-right:4px;">{{ collapsedCategories.has(categoryName) ? 'chevron_right' : 'expand_more' }}</i>
                      <i class="material-icons" style="font-size:14px; margin-right:4px;">folder</i>
                      {{ categoryName }}
                   </div>
                   <div v-if="!collapsedCategories.has(categoryName)">
                       <div v-for="mapItem in group" :key="mapItem.id" 
                            class="overlay-item" 
                            style="display:flex; align-items:center; padding:8px; cursor:pointer; border-radius:4px; margin-bottom:2px;"
                            :style="activeExternalMaps[mapItem.id] ? 'background:rgba(102, 187, 106, 0.3); border:1px solid #66bb6a;' : 'background:rgba(255,255,255,0.05);'"
                            @click="toggleExternalMap(mapItem)">
                           <i class="material-icons" style="margin-right:10px; color:#81c784;">{{ activeExternalMaps[mapItem.id] ? 'check_box' : 'check_box_outline_blank' }}</i>
                           <div style="flex:1; overflow:hidden;">
                               <div class="overlay-name" style="font-weight:500; font-size: 13px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;">
                                  {{ mapItem.name }}
                               </div>
                               <div v-if="mapItem.description" style="font-size:11px; opacity:0.7; word-break: break-word;">
                                   {{ mapItem.description }}
                               </div>
                           </div>
                       </div>
                   </div>
               </div>
               
               <div v-if="externalMaps.length === 0" style="text-align:center; opacity:0.6; margin-top:20px;">
                   No external maps configured.
               </div>

               <!-- Consolidated Temporal Range Slider -->
               <div v-if="activeExternalMapTimeSlider.visible" 
                    id="ext-range-slider"
                    style="margin-top:20px; padding:15px; background:rgba(255,255,255,0.05); border-radius:8px; border:1px solid #444;">
                   <div style="font-size:12px; font-weight:bold; color:#81c784; margin-bottom:12px; display:flex; justify-content:space-between; align-items:center;">
                       <span>Temporal Range Selector</span>
                       <i class="material-icons" style="font-size:16px;">date_range</i>
                   </div>
                   
                   <div class="range-slider-display" style="display:flex; justify-content:space-between; font-size:11px; margin-bottom:8px;">
                       <span>From: <b style="color:#eee;">{{ activeExternalMapTimeSlider.lowDisplay }}</b></span>
                       <span>To: <b style="color:#eee;">{{ activeExternalMapTimeSlider.highDisplay }}</b></span>
                   </div>

                   <div class="range-slider-wrapper">
                       <div class="range-slider-track"></div>
                       <div class="range-slider-highlight" :style="timeRangeStyle"></div>
                       <input type="range" 
                              class="range-handle"
                              :min="activeExternalMapTimeSlider.min" 
                              :max="activeExternalMapTimeSlider.max" 
                              step="86400000"
                              v-model.number="activeExternalMapTimeSlider.low">
                       <input type="range" 
                              class="range-handle"
                              :min="activeExternalMapTimeSlider.min" 
                              :max="activeExternalMapTimeSlider.max" 
                              step="86400000"
                              v-model.number="activeExternalMapTimeSlider.high">
                   </div>

                   <div style="margin-top:15px; text-align:center; font-size:10px; opacity:0.6;">
                       Use either handle to adjust the visible date range
                   </div>
               </div>
           </div>

           <!-- Drawing View -->
           <div v-else-if="activeTab === 'draw'" class="overlay-section" style="padding:15px; display:flex; flex-direction:column; gap:20px;">
                <div style="font-size:14px; font-weight:bold; color:#ef5350; border-bottom:1px solid #444; padding-bottom:8px; display:flex; align-items:center; gap:8px;">
                    <i class="material-icons" style="font-size:18px;">brush</i>
                    Annotate Map
                </div>

                <div class="tool-group">
                    <div style="font-size:11px; opacity:0.7; margin-bottom:8px; text-transform:uppercase; letter-spacing:1px;">1. Select Tool</div>
                    <div style="display:flex; gap:10px;">
                        <button @click="activeDrawTool = (activeDrawTool === 'point' ? null : 'point')" 
                                :style="{ background: activeDrawTool === 'point' ? '#ef5350' : '#444' }"
                                style="flex:1; padding:10px; border-radius:8px; border:none; color:white; cursor:pointer; display:flex; flex-direction:column; align-items:center; gap:5px; transition:0.2s;">
                            <i class="material-icons" style="font-size:20px;">place</i>
                            <span style="font-size:11px;">Point</span>
                        </button>
                        <button @click="activeDrawTool = (activeDrawTool === 'arrow' ? null : 'arrow')"
                                :style="{ background: activeDrawTool === 'arrow' ? '#ef5350' : '#444' }"
                                style="flex:1; padding:10px; border-radius:8px; border:none; color:white; cursor:pointer; display:flex; flex-direction:column; align-items:center; gap:5px; transition:0.2s;">
                            <i class="material-icons" style="font-size:20px;">trending_flat</i>
                            <span style="font-size:11px;">Arrow</span>
                        </button>
                    </div>
                    <div v-if="activeDrawTool" style="margin-top:10px; font-size:11px; color:#ef5350; text-align:center; min-height:14px;">
                        {{ activeDrawTool === 'point' ? 'Click on map to place a point' : (lastDrawCoords ? 'Click destination for arrow' : 'Click start of arrow') }}
                    </div>
                </div>

                <div class="tool-group">
                    <div style="font-size:11px; opacity:0.7; margin-bottom:8px; text-transform:uppercase; letter-spacing:1px;">2. Label (Optional)</div>
                    <input v-model="drawLabel" placeholder="Type label text..." 
                           style="width:100%; background:#333; border:1px solid #555; border-radius:4px; padding:10px; color:white; font-size:13px; box-sizing:border-box;">
                </div>

                <div class="tool-group">
                    <div style="font-size:11px; opacity:0.7; margin-bottom:8px; text-transform:uppercase; letter-spacing:1px;">3. Color Choice</div>
                    <div style="display:flex; justify-content:space-between; gap:5px;">
                        <div v-for="c in ['#ef5350', '#42a5f5', '#66bb6a', '#ffca28', '#ffffff', '#000000']" 
                             :key="c" @click="drawColor = c"
                             :style="{ background: c, border: drawColor === c ? '2px solid white' : '2px solid transparent' }"
                             style="width:28px; height:28px; border-radius:50%; cursor:pointer; box-shadow:0 2px 4px rgba(0,0,0,0.3); box-sizing:border-box;">
                        </div>
                    </div>
                </div>

                <div style="margin-top:auto; padding-top:20px;">
                    <button @click="clearDrawings" 
                            style="width:100%; padding:10px; background:rgba(255,255,255,0.05); border:1px solid #555; border-radius:4px; color:white; cursor:pointer; display:flex; align-items:center; justify-content:center; gap:8px;">
                        <i class="material-icons" style="font-size:18px;">delete_sweep</i>
                        Clear All Drawings
                    </button>
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
          
          <div v-else-if="!currentInspectionFolder && sidePanelData.length > 0" class="panel-folder-list">
             <div v-for="grp in displayedFolders" :key="grp.path" class="folder-item" @click="openFolderView(grp)"
                  @mouseenter="grp.lat && grp.lon ? drawLeaderLine($event, grp.lat, grp.lon) : null"
                  @mouseleave="clearLeaderLine">
                                 <img v-if="grp.thumbnailUrl" :src="grp.thumbnailUrl" class="folder-thumb-icon" loading="lazy">
                 <i v-else class="material-icons">folder</i>

                <div class="folder-info">
                    <span class="folder-name">{{ grp.displayName }}</span>
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
                               @click="file.type === 'folder' ? openFolderView(file) : openQuickView(file)">
                              <img :src="file.thumbUrl" class="panel-thumb" loading="lazy">
                              <span class="panel-item-name">{{ file.name }}</span>
                               <i v-if="file.type === 'folder'" class="material-icons folder-badge">folder</i>
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
import { onMounted, onBeforeUnmount, ref, computed, nextTick, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import maplibregl from 'maplibre-gl';
import 'maplibre-gl/dist/maplibre-gl.css';
import * as pmtiles from 'pmtiles';
import { state } from "@/store";
import { fetchJSON } from "@/api/utils";
import { notify } from "@/notify";
import { processDirectItems } from "@/utils/heatmapInspector";
import { usersApi } from "@/api";
import { staticURL } from "@/utils/constants";

// Register the global PMTiles protocol once (MapLibre GL v5 compatible)
if (!window._pmtilesProtocolRegistered) {
    const protocol = new pmtiles.Protocol();
    // MapLibre GL v5 uses a Promise-returning function for addProtocol
    maplibregl.addProtocol('pmtiles', (params) => protocol.tile(params));
    window._pmtilesProtocolRegistered = true;
}

const civilWarIcons = {
    'union': `
        <svg width="100" height="100" viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
            <radialGradient id="expGradU" cx="50%" cy="50%" r="50%">
                <stop offset="0%" style="stop-color:#fffce0;stop-opacity:1" />
                <stop offset="70%" style="stop-color:#ffcc00;stop-opacity:1" />
                <stop offset="100%" style="stop-color:#ff3300;stop-opacity:1" />
            </radialGradient>
            <path d="M50 25 L54 38 L68 33 L60 44 L78 50 L60 56 L68 67 L54 62 L50 75 L46 62 L32 67 L40 56 L22 50 L40 44 L32 33 L46 38 Z" fill="url(#expGradU)" stroke="#b32400" stroke-width="2.5"/>
            <g transform="translate(35, 12) scale(0.9)">
                <rect width="45" height="27" fill="#fff" stroke="#000" stroke-width="1"/>
                <rect width="45" height="2.1" y="0" fill="#B22234"/><rect width="45" height="2.1" y="4.2" fill="#B22234"/><rect width="45" height="2.1" y="8.4" fill="#B22234"/><rect width="45" height="2.1" y="12.6" fill="#B22234"/><rect width="45" height="2.1" y="16.8" fill="#B22234"/><rect width="45" height="2.1" y="21" fill="#B22234"/>
                <rect width="18" height="13" fill="#3C3B6E"/>
            </g>
        </svg>`,
    'confederate': `
        <svg width="100" height="100" viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
            <radialGradient id="expGradC" cx="50%" cy="50%" r="50%">
                <stop offset="0%" style="stop-color:#fffce0;stop-opacity:1" />
                <stop offset="70%" style="stop-color:#ffcc00;stop-opacity:1" />
                <stop offset="100%" style="stop-color:#ff3300;stop-opacity:1" />
            </radialGradient>
            <path d="M50 25 L54 38 L68 33 L60 44 L78 50 L60 56 L68 67 L54 62 L50 75 L46 62 L32 67 L40 56 L22 50 L40 44 L32 33 L46 38 Z" fill="url(#expGradC)" stroke="#b32400" stroke-width="2.5"/>
            <g transform="translate(35, 12) scale(0.9)">
                <rect width="45" height="27" fill="#B22234" stroke="#000" stroke-width="1"/>
                <path d="M0 0 L45 27 M45 0 L0 27" stroke="#fff" stroke-width="5"/>
                <path d="M0 0 L45 27 M45 0 L0 27" stroke="#3C3B6E" stroke-width="2.5"/>
            </g>
        </svg>`,
    'inconclusive': `
        <svg width="100" height="100" viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
            <radialGradient id="expGradI" cx="50%" cy="50%" r="50%">
                <stop offset="0%" style="stop-color:#fffce0;stop-opacity:1" />
                <stop offset="70%" style="stop-color:#ffcc00;stop-opacity:1" />
                <stop offset="100%" style="stop-color:#ff3300;stop-opacity:1" />
            </radialGradient>
            <path d="M50 25 L54 38 L68 33 L60 44 L78 50 L60 56 L68 67 L54 62 L50 75 L46 62 L32 67 L40 56 L22 50 L40 44 L32 33 L46 38 Z" fill="url(#expGradI)" stroke="#b32400" stroke-width="2.5"/>
        </svg>`
};

const route = useRoute();
const router = useRouter();
let map = null;
let initMapPromise = null; // Persistent promise for initialization
let overlayLayer = null; // MapLibre source for overlays
const isFolderMode = ref(false);

// Abort controller for canceling tile requests
let abortController = null;
let clusterMarkers = {}; // Keep track of rendered DOM markers

const debugStatus = ref("");
const debugClusterCount = ref(0);
const isDesktop = ref(window.innerWidth > 1024);
const showDebugInfo = ref(isDesktop.value); 
const currentZoom = ref(0);
const cursorCoords = ref({ lat: 999, lng: 999 });
const showImageClusters = ref(true); // Toggle for image bubbles/clusters

// Debounce timer for viewport updates
let updateDebounceTimer = null;
const currentSource = ref("");
const currentPath = ref("");

// Side Panel State
const showSidePanel = ref(false);
const sidePanelTitle = ref("Location Inspector");
const sidePanelData = ref([]);
const formattedSidePanelData = ref([]); // Array of {path, count, items}
const currentInspectionFolder = ref(null); // Reference to currently viewing folder object
const overlaySiblings = ref([]); // Sibling GeoJSON files for overlay navigation
const sidePanelLoading = ref(false);
const isPanelCollapsed = ref(false);
// Overlay State
const activeTab = ref('inspection'); // Unified declaration at top
const overlayMode = ref('context'); // 'context' (current folder) or 'all' (global)
const availableOverlays = ref([]); // List of overlay objects
const activeOverlayLayers = ref({}); // Map path -> Leaflet Layer
const overlayOpacities = ref({}); // Map path -> Opacity (0-1)

// External Maps State
const externalMaps = ref([]);
const activeExternalMaps = ref({});
const activeExternalMapTimeSlider = ref({ 
    min: 0, 
    max: 100, 
    low: 0, 
    high: 100, 
    visible: false, 
    fieldScale: '', 
    minDisplay: '', 
    maxDisplay: '', 
    lowDisplay: '', 
    highDisplay: '',
    sourceId: ''
});
const collapsedCategories = ref(new Set()); // Sub-folders in External Maps tab

const isBoxSelectMode = ref(false); // Box Selection Mode State
const selectionBox = ref({ visible: false, startX: 0, startY: 0, currentX: 0, currentY: 0, style: {} });
const itemsPerPage = 50;
const displayedCount = ref(itemsPerPage);

// Drawing State
const activeDrawTool = ref(null); // 'point', 'arrow'
const drawColor = ref('#ef5350');
const drawLabel = ref('');
const drawnFeatures = ref({
    type: 'FeatureCollection',
    features: []
});
let lastDrawCoords = null;
const leaderLine = ref({ visible: false, x1: 0, y1: 0, x2: 0, y2: 0, targetLat: null, targetLon: null, startEl: null, svgWidth: 0, svgHeight: 0 });
const leaderLineSvg = ref(null);

const currentBasemap = ref('carto-light');

const mapStyles = {
    'google-hybrid': {
        name: 'Google Satellite Hybrid',
        tiles: ['https://mt1.google.com/vt/lyrs=y&x={x}&y={y}&z={z}'],
        maxZoom: 20
    },
    'google-roadmap': {
        name: 'Google Map',
        tiles: ['https://mt1.google.com/vt/lyrs=m&x={x}&y={y}&z={z}'],
        maxZoom: 20
    },
    'google-terrain': {
        name: 'Google Terrain',
        tiles: ['https://mt1.google.com/vt/lyrs=p&x={x}&y={y}&z={z}'],
        maxZoom: 20
    },
    'osm': {
        name: 'OpenStreetMap',
        tiles: ['https://a.tile.openstreetmap.org/{z}/{x}/{y}.png', 'https://b.tile.openstreetmap.org/{z}/{x}/{y}.png', 'https://c.tile.openstreetmap.org/{z}/{x}/{y}.png'],
        maxZoom: 19
    },
    'opentopo': {
        name: 'OpenTopoMap',
        tiles: ['https://a.tile.opentopomap.org/{z}/{x}/{y}.png', 'https://b.tile.opentopomap.org/{z}/{x}/{y}.png', 'https://c.tile.opentopomap.org/{z}/{x}/{y}.png'],
        maxZoom: 17
    },
    'esri-world': {
        name: 'Esri Satellite',
        tiles: ['https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}'],
        maxZoom: 18
    },
    'carto-light': {
         name: 'Carto Light',
         tiles: ['https://a.basemaps.cartocdn.com/light_all/{z}/{x}/{y}.png', 'https://b.basemaps.cartocdn.com/light_all/{z}/{x}/{y}.png'],
         maxZoom: 20
    },
    'carto-dark': {
         name: 'Carto Dark',
         tiles: ['https://a.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}.png', 'https://b.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}.png'],
         maxZoom: 20
    }
};

const setBasemap = (key) => {
    currentBasemap.value = key;
    
    // Save to user settings
    if (state.user) {
        state.user.mapBasemap = key;
        usersApi.update(state.user, ['mapBasemap']).catch(err => {
            console.error("Failed to save basemap preference", err);
        });
    }

    if (map) {
        Object.keys(mapStyles).forEach(k => {
            if (map.getLayer(k)) {
                map.setLayoutProperty(k, 'visibility', k === key ? 'visible' : 'none');
            }
        });
    }
};

class BasemapControl {
    onAdd(map) {
        this.container = document.createElement('div');
        this.container.className = 'maplibregl-ctrl maplibregl-ctrl-group';
        this.container.style.position = 'relative';

        const btn = document.createElement('button');
        btn.className = 'maplibregl-ctrl-icon';
        btn.type = 'button';
        btn.title = 'Change Basemap';
        btn.style.display = 'flex';
        btn.style.alignItems = 'center';
        btn.style.justifyContent = 'center';
        btn.innerHTML = '<span class="material-icons" style="font-size: 18px;">layers</span>';
        
        const dropdown = document.createElement('div');
        dropdown.style.display = 'none';
        dropdown.style.position = 'absolute';
        dropdown.style.top = '0';
        dropdown.style.left = '100%';
        dropdown.style.marginLeft = '5px';
        dropdown.style.background = '#333';
        dropdown.style.borderRadius = '4px';
        dropdown.style.padding = '5px 0';
        dropdown.style.minWidth = '170px';
        dropdown.style.boxShadow = '0 2px 10px rgba(0,0,0,0.5)';
        dropdown.style.zIndex = '1000';

        Object.keys(mapStyles).forEach(key => {
            const opt = document.createElement('div');
            opt.style.padding = '8px 16px';
            opt.style.cursor = 'pointer';
            opt.style.color = 'white';
            opt.style.display = 'flex';
            opt.style.alignItems = 'center';
            opt.style.gap = '8px';
            opt.style.fontSize = '13px';
            
            const updateOpt = () => {
                opt.style.background = currentBasemap.value === key ? '#555' : 'transparent';
                opt.innerHTML = `<i class="material-icons" style="opacity: ${currentBasemap.value === key ? 1 : 0}; font-size: 16px;">check</i> <span>${mapStyles[key].name}</span>`;
            };
            updateOpt();
            
            opt.onclick = (e) => {
                e.stopPropagation();
                setBasemap(key);
                dropdown.style.display = 'none';
                Array.from(dropdown.children).forEach(c => c._updateOpt && c._updateOpt());
            };
            opt._updateOpt = updateOpt;
            dropdown.appendChild(opt);
        });

        // Close when clicking outside
        const closeDropdown = (e) => {
             if (!this.container.contains(e.target)) {
                 dropdown.style.display = 'none';
             }
        };
        document.addEventListener('click', closeDropdown);
        this.closeDropdown = closeDropdown;

        btn.onclick = (e) => {
            e.stopPropagation();
            dropdown.style.display = dropdown.style.display === 'none' ? 'block' : 'none';
            Array.from(dropdown.children).forEach(c => c._updateOpt && c._updateOpt());
        };

        this.container.appendChild(btn);
        this.container.appendChild(dropdown);

        return this.container;
    }
    
    onRemove() {
        if (this.closeDropdown) {
            document.removeEventListener('click', this.closeDropdown);
        }
        this.container.parentNode.removeChild(this.container);
        this.map = undefined;
    }
}

class PrintControl {
    onAdd(map) {
        this._map = map;
        this.container = document.createElement('div');
        this.container.className = 'maplibregl-ctrl maplibregl-ctrl-group';

        const btn = document.createElement('button');
        btn.className = 'maplibregl-ctrl-icon';
        btn.type = 'button';
        btn.title = 'Print Map';
        btn.style.display = 'flex';
        btn.style.alignItems = 'center';
        btn.style.justifyContent = 'center';
        btn.innerHTML = '<span class="material-icons" style="font-size: 18px;">print</span>';
        
        btn.onclick = (e) => {
            e.stopPropagation();
            this.printMap();
        };

        this.container.appendChild(btn);
        return this.container;
    }

    printMap() {
        // Ask for a custom title
        const customTitle = window.prompt("Enter a title for this map print:", document.title || 'Map Export');
        if (customTitle === null) return; // User cancelled

        // Force a render frame
        this._map.triggerRepaint();
        
        requestAnimationFrame(() => {
            const canvas = this._map.getCanvas();
            const dataUrl = canvas.toDataURL('image/png');
            
            const printWindow = window.open('', '_blank');
            if (!printWindow) {
                alert("Popup blocked! Please allow popups for printing.");
                return;
            }

            const date = new Date().toLocaleString();

            printWindow.document.write(`
                <html>
                    <head>
                        <title>${customTitle}</title>
                        <style>
                            body { margin: 0; padding: 20px; font-family: sans-serif; display: flex; flex-direction: column; align-items: center; background: #fff; }
                            .map-image { max-width: 100%; height: auto; box-shadow: 0 0 10px rgba(0,0,0,0.1); border: 1px solid #ccc; }
                            .header { width: 100%; display: flex; justify-content: space-between; margin-bottom: 20px; border-bottom: 2px solid #333; padding-bottom: 10px; }
                            h1 { font-size: 24px; margin: 0; }
                            .info { font-size: 12px; color: #666; text-align: right; }
                            @media print {
                                body { padding: 0; }
                                .map-image { border: none; box-shadow: none; border-radius: 0; }
                                .header { border-bottom-color: #000; }
                            }
                        </style>
                    </head>
                    <body>
                        <div class="header">
                            <h1>${customTitle}</h1>
                            <div class="info">
                                Printed on: ${date}
                            </div>
                        </div>
                        <img src="${dataUrl}" class="map-image" id="mapImage" />
                        <script>
                            const img = document.getElementById('mapImage');
                            img.onload = () => {
                                setTimeout(() => {
                                    window.print();
                                }, 300);
                            };
                            // Fallback if onload doesn't fire
                            setTimeout(() => {
                                if (img.complete) return;
                                window.print();
                            }, 5000);
                        <\/script>
                    </body>
                </html>
            `);
            printWindow.document.close();
        });
    }

    onRemove() {
        this.container.parentNode.removeChild(this.container);
        this._map = undefined;
    }
}

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

// Computed style for the temporal range slider track
const timeRangeStyle = computed(() => {
    const s = activeExternalMapTimeSlider.value;
    if (!s.visible || s.max === s.min) return { left: '0%', width: '100%' };
    
    const leftValue = ((s.low - s.min) / (s.max - s.min)) * 100;
    const rightValue = ((s.high - s.min) / (s.max - s.min)) * 100;
    
    const left = Math.min(leftValue, rightValue);
    const width = Math.abs(rightValue - leftValue);
    
    return {
        left: left + '%',
        width: width + '%'
    };
});

// Dynamic Leader Line Logic
const updateVisibleMarkers = () => {
    // Placeholder to prevent ReferenceError. 
    // Logic for filtering sidebar items by map bounds can be added here if needed.
};

const drawLeaderLine = (event, lat, lon) => {
    if (!map || !lat || !lon) return;
    console.log(`[Heatmap] drawLeaderLine: lat=${lat}, lon=${lon}`);

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
   const mapContainer = map.getContainer();
   const mapRect = mapContainer.getBoundingClientRect();
   leaderLine.value.svgWidth = mapRect.width;
   leaderLine.value.svgHeight = mapRect.height;

   // 2. Get Button and SVG Rects
   const buttonRect = leaderLine.value.startEl.getBoundingClientRect();
   const svgEl = leaderLineSvg.value;
   if (!svgEl) return;
   
   const svgRect = svgEl.getBoundingClientRect();

   // 3. Start Point (Side Panel Button) - Relative to SVG
   // buttonRect.right is screen space, svgRect.left is screen space. x1 is relative to SVG.
   const x1 = buttonRect.right - svgRect.left; 
   const y1 = (buttonRect.top + (buttonRect.height / 2)) - svgRect.top;

   // 4. Get Map Point logic
   // map.project([lng, lat]) provides x/y relative to the map container's top-left.
   const point = map.project([leaderLine.value.targetLon, leaderLine.value.targetLat]);
   
    // If SVG is exactly on top of map container, mapRect.left - svgRect.left is 0.
    // If not, this math corrects for it.
    leaderLine.value.x1 = x1;
    leaderLine.value.y1 = y1;
    leaderLine.value.x2 = (point.x + mapRect.left) - svgRect.left;
    leaderLine.value.y2 = (point.y + mapRect.top) - svgRect.top;
    
    console.log(`[Heatmap] updateLeaderLine: SVG@(${svgRect.left},${svgRect.top}) Map@(${mapRect.left},${mapRect.top})`);
    console.log(`[Heatmap] updateLeaderLine: Start=(${x1},${y1}) End=(${leaderLine.value.x2},${leaderLine.value.y2})`);
};

const clearLeaderLine = () => {
    leaderLine.value.visible = false;
    leaderLine.value.startEl = null;
    leaderLine.value.targetLat = null;
    leaderLine.value.targetLon = null;
};

// Drawing Helpers
const updateDrawSource = () => {
    if (map && map.getSource('draw-source')) {
        map.getSource('draw-source').setData(drawnFeatures.value);
    }
};

const clearDrawings = () => {
    drawnFeatures.value.features = [];
    lastDrawCoords = null;
    updateDrawSource();
};

const handleDrawClick = (e) => {
    const coords = [e.lngLat.lng, e.lngLat.lat];
    
    if (activeDrawTool.value === 'point') {
        const feature = {
            type: 'Feature',
            geometry: { type: 'Point', coordinates: coords },
            properties: {
                color: drawColor.value,
                label: drawLabel.value || ''
            }
        };
        drawnFeatures.value.features.push(feature);
        drawLabel.value = ''; // Reset label after use
        updateDrawSource();
    } 
    else if (activeDrawTool.value === 'arrow') {
        if (!lastDrawCoords) {
            lastDrawCoords = coords;
            notify.showSuccess("Start point set. Click destination.");
        } else {
            // Calculate Bearing for headache
            const start = lastDrawCoords;
            const end = coords;
            
            // Approximate bearing
            const y = Math.sin((end[0] - start[0]) * Math.PI / 180) * Math.cos(end[1] * Math.PI / 180);
            const x = Math.cos(start[1] * Math.PI / 180) * Math.sin(end[1] * Math.PI / 180) -
                      Math.sin(start[1] * Math.PI / 180) * Math.cos(end[1] * Math.PI / 180) * Math.cos((end[0] - start[0]) * Math.PI / 180);
            const bearing = Math.atan2(y, x) * 180 / Math.PI;

            // Add Line
            drawnFeatures.value.features.push({
                type: 'Feature',
                geometry: { type: 'LineString', coordinates: [start, end] },
                properties: { color: drawColor.value }
            });

            // Add Head (Point with icon)
            drawnFeatures.value.features.push({
                type: 'Feature',
                geometry: { type: 'Point', coordinates: end },
                properties: { 
                    color: drawColor.value, 
                    label: drawLabel.value || '',
                    bearing: bearing,
                    icon: 'triangle' // We'll need to define this icon
                }
            });

            drawLabel.value = '';
            lastDrawCoords = null;
            updateDrawSource();
        }
    }
};

// Box Selection Logic (MapLibre Version)
const toggleSelectionMode = () => {
    isBoxSelectMode.value = !isBoxSelectMode.value;
    if (isBoxSelectMode.value) {
        if (map) {
            map.dragPan.disable();
            map.getCanvas().style.cursor = 'crosshair';
        }
    } else {
        if (map) {
            map.dragPan.enable();
            map.getCanvas().style.cursor = '';
        }
        selectionBox.value.visible = false;
    }
};

const startSelection = (e) => {
    if (!isBoxSelectMode.value) return;
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
    
    if (!map) return;

    const overlay = e.currentTarget;
    const overlayRect = overlay.getBoundingClientRect();
    const mapContainer = map.getContainer();
    const mapRect = mapContainer.getBoundingClientRect();
    
    // Calculate Offset: Overlay Relative -> Map Relative
    const offsetX = overlayRect.left - mapRect.left;
    const offsetY = overlayRect.top - mapRect.top;
    
    const startX = selectionBox.value.startX + offsetX;
    const startY = selectionBox.value.startY + offsetY;
    const endX = selectionBox.value.currentX + offsetX;
    const endY = selectionBox.value.currentY + offsetY;
    
    const p1 = [Math.min(startX, endX), Math.min(startY, endY)];
    const p2 = [Math.max(startX, endX), Math.max(startY, endY)];
    
    // Convert screen coordinates to LngLat bounds
    // sw (SouthWest) = minX, maxY (lowest Lng/Lat)
    // ne (NorthEast) = maxX, minY (highest Lng/Lat)
    const sw = map.unproject([p1[0], p2[1]]);
    const ne = map.unproject([p2[0], p1[1]]);

    console.log("[Heatmap] REBUILT endSelection: Requesting backend cluster search for bbox:", { sw, ne });

    toggleSelectionMode();
    
    // Perform robust backend-driven bounding box inspection
    // This handles all underlying clustered folders correctly, matching Leaflet behavior.
    inspectLocation(currentPath.value, currentSource.value, null, { 
        lat: (sw.lat + ne.lat) / 2, 
        lon: (sw.lng + ne.lng) / 2, 
        minLat: sw.lat, 
        maxLat: ne.lat, 
        minLon: sw.lng, 
        maxLon: ne.lng 
    });
};


const zoomToLocation = (lat, lon) => {
    if (map && lat && lon) {
        map.flyTo({
            center: [lon, lat],
            zoom: 16,
            essential: true
        });

        // Flash Effect (Simulated with a temporary layer)
        const flashId = `zoom-flash-${Date.now()}`;
        map.addSource(flashId, {
            type: 'geojson',
            data: {
                type: 'Feature',
                geometry: {
                    type: 'Point',
                    coordinates: [lon, lat]
                }
            }
        });

        map.addLayer({
            id: flashId,
            type: 'circle',
            source: flashId,
            paint: {
                'circle-radius': 0,
                'circle-color': '#f1c40f',
                'circle-opacity': 0.8,
                'circle-stroke-width': 2,
                'circle-stroke-color': '#fff'
            }
        });

        // Animate the flash
        let radius = 0;
        const animate = () => {
            radius += 2;
            if (radius <= 40) {
                map.setPaintProperty(flashId, 'circle-radius', radius);
                requestAnimationFrame(animate);
            } else {
                map.removeLayer(flashId);
                map.removeSource(flashId);
            }
        };
        setTimeout(animate, 1500); // Start after flyTo duration
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

const getOverlayKey = (path, sourceArg = null) => {
    const s = sourceArg || route.query.source || "";
    return `${recursiveDecode(s)}::${recursiveDecode(path)}`;
};

const sanitizeId = (str) => str.replace(/[^a-zA-Z0-9_-]/g, '_');

const toggleOverlay = async (path, providedSource = null) => {
    const key = getOverlayKey(path, providedSource);
    // Use a safe short ID for MapLibre source/layer IDs (slashes crash MapLibre internals)
    const safeId = sanitizeId(key).slice(-40);

    if (activeOverlayLayers.value[key]) {
        if (map) {
            const layerEntry = activeOverlayLayers.value[key];
            if (layerEntry && layerEntry.layers) {
                layerEntry.layers.forEach(id => {
                    try { if (map.getLayer(id)) map.removeLayer(id); } catch(e) { console.warn("[Heatmap] Remove layer err", e); }
                });
            }
            if (layerEntry && layerEntry.sources) {
                layerEntry.sources.forEach(id => {
                    try { if (map.getSource(id)) map.removeSource(id); } catch(e) { console.warn("[Heatmap] Remove source err", e); }
                });
            } else {
                try { if (map.getSource(`src-${safeId}`)) map.removeSource(`src-${safeId}`); } catch(e) { console.warn("[Heatmap] Remove source err", e); }
            }
            
            // Clean up any custom PMTiles protocol we registered for this specific overlay
            if (window._pmtilesProtocols && window._pmtilesProtocols[key]) {
                try { maplibregl.removeProtocol(`pmt-${window._pmtilesProtocols[key]}`); } catch(e) { console.warn("[Heatmap] Remove protocol err", e); }
                delete window._pmtilesProtocols[key];
            }
        }
        delete activeOverlayLayers.value[key];
        delete overlayOpacities.value[key];
        return;
    }

    if (Object.keys(activeOverlayLayers.value).length >= 5) {
        notify.showError("Maximum 5 overlays allowed.");
        return;
    }

    let overlay = availableOverlays.value.find(o => o.path === path);
    if (!overlay && providedSource) {
        overlay = { path: path, source: providedSource };
    }
    if (!overlay) return;

    let rawPath = overlay.path;
    if (!rawPath.startsWith('/')) rawPath = '/' + rawPath;

    // Ensure map + style are ready
    if (!map || !map.isStyleLoaded()) {
        console.warn('[Heatmap] Map not ready yet, waiting...');
        await new Promise(resolve => map.once('idle', resolve));
    }

    try {
        const cleanSource = recursiveDecode(overlay.source || route.query.source || "");
        const cleanPath = recursiveDecode(rawPath);
        const fileSpec = `${cleanSource}::${cleanPath}`;
        const url = `/api/raw?files=${encodeURIComponent(fileSpec)}`;

        if (overlay.type === 'pmtiles' || cleanPath.toLowerCase().endsWith('.pmtiles')) {
            try {
                const p = new pmtiles.PMTiles(`${location.origin}${url}`);
                const header = await p.getHeader();
                const isVector = header.tileType === 1;
                console.log(`[Heatmap] MapLibre PMTiles: type=${header.tileType}, vector=${isVector}, path=${path}`);

                // Register unique per-overlay protocol so MapLibre can fetch tiles
                const protocolKey = `pk${Date.now()}`;
                maplibregl.addProtocol(`pmt-${protocolKey}`, async (params) => {
                    const clean = params.url.replace(`pmt-${protocolKey}://`, '');
                    const [z, x, y] = clean.split('/').map(Number);
                    const data = await p.getZxy(z, x, y);
                    return { data: data ? data.data : new Uint8Array(0) };
                });
                if (!window._pmtilesProtocols) window._pmtilesProtocols = {};
                window._pmtilesProtocols[key] = protocolKey;

                const tileUrl = `pmt-${protocolKey}://{z}/{x}/{y}`;

                try {
                    const b = [header.minLon, header.minLat, header.maxLon, header.maxLat];
                    if (b[0] !== 0 || b[2] !== 0) map.fitBounds(b, { padding: 50 });
                } catch (_) {}

                if (isVector) {
                    map.addSource(`src-${safeId}`, { type: 'vector', tiles: [tileUrl] });
                    let sourceLayer = '';
                    try {
                        const meta = await p.getMetadata();
                        const layers = meta?.vector_layers || meta?.tilestats?.layers;
                        if (layers && layers.length > 0) sourceLayer = layers[0].id || layers[0].layer;
                    } catch(_) {}
                    if (!sourceLayer) sourceLayer = cleanPath.split('/').pop().replace('.pmtiles', '');

                    map.addLayer({ id: `lyr-fill-${safeId}`, type: 'fill', source: `src-${safeId}`, 'source-layer': sourceLayer,
                        paint: { 'fill-color': '#ff7800', 'fill-opacity': 0.4 } });
                    map.addLayer({ id: `lyr-line-${safeId}`, type: 'line', source: `src-${safeId}`, 'source-layer': sourceLayer,
                        paint: { 'line-color': '#ff7800', 'line-width': 2 } });
                    map.addLayer({ id: `lyr-circle-${safeId}`, type: 'circle', source: `src-${safeId}`, 'source-layer': sourceLayer,
                        paint: { 'circle-radius': 4, 'circle-color': '#ff7800', 'circle-stroke-color': '#fff', 'circle-stroke-width': 1 } });

                    activeOverlayLayers.value[key] = { layers: [`lyr-fill-${safeId}`, `lyr-line-${safeId}`, `lyr-circle-${safeId}`] };
                } else {
                    map.addSource(`src-${safeId}`, { type: 'raster', tiles: [tileUrl], tileSize: 256 });
                    map.addLayer({ id: `lyr-raster-${safeId}`, type: 'raster', source: `src-${safeId}` });
                    // Explicitly track the raster layer so toggle off works
                    activeOverlayLayers.value[key] = { layers: [`lyr-raster-${safeId}`] };
                }

                overlayOpacities.value[key] = 1;
                nextTick(() => updateOverlayOpacity(path, 1, providedSource));
                return;
            } catch (e) {
                console.error("PMTiles load error", e);
                throw e;
            }
        }

        // GeoJSON
        const res = await fetch(url);
        if (!res.ok) throw new Error("Fetch failed: " + res.status);
        let geojson = await res.json();

        if (geojson.type === 'Feature') {
            geojson = { type: 'FeatureCollection', features: [geojson] };
        } else if (!geojson.features) {
            geojson = { type: 'FeatureCollection', features: [] };
        }

        console.log(`[Heatmap] GeoJSON features: ${geojson.features.length}, safeId=${safeId}`);

        // Split GeoJSON by geometry type to avoid MapLibre v5 layer filter bugs
        const points = [], lines = [], polys = [];
        
        const processGeometry = (geom, props) => {
            if (!geom) return;
            const t = geom.type;
            if (t === 'GeometryCollection') {
                (geom.geometries || []).forEach(g => processGeometry(g, props));
            } else if (t === 'Point' || t === 'MultiPoint') {
                points.push({ type: 'Feature', geometry: geom, properties: props });
            } else if (t === 'LineString' || t === 'MultiLineString') {
                lines.push({ type: 'Feature', geometry: geom, properties: props });
            } else if (t === 'Polygon' || t === 'MultiPolygon') {
                polys.push({ type: 'Feature', geometry: geom, properties: props });
            }
        };

        (geojson.features || []).forEach(f => {
            processGeometry(f.geometry, f.properties);
        });

        const activeLyrs = [];
        const activeSrcs = [];

        if (polys.length > 0) {
            map.addSource(`src-poly-${safeId}`, { type: 'geojson', data: { type: 'FeatureCollection', features: polys } });
            map.addLayer({ id: `lyr-fill-${safeId}`, type: 'fill', source: `src-poly-${safeId}`,
                paint: { 'fill-color': '#ff7800', 'fill-opacity': 0.4 } });
            activeLyrs.push(`lyr-fill-${safeId}`);
            activeSrcs.push(`src-poly-${safeId}`);
        }
        if (lines.length > 0) {
            map.addSource(`src-line-${safeId}`, { type: 'geojson', data: { type: 'FeatureCollection', features: lines } });
            map.addLayer({ id: `lyr-line-${safeId}`, type: 'line', source: `src-line-${safeId}`,
                paint: { 'line-color': '#ff7800', 'line-width': 3 } });
            activeLyrs.push(`lyr-line-${safeId}`);
            activeSrcs.push(`src-line-${safeId}`);
        }
        if (points.length > 0) {
            map.addSource(`src-point-${safeId}`, { type: 'geojson', data: { type: 'FeatureCollection', features: points } });
            map.addLayer({ id: `lyr-circle-${safeId}`, type: 'circle', source: `src-point-${safeId}`,
                paint: { 'circle-radius': 6, 'circle-color': '#ff7800', 'circle-stroke-color': '#fff', 'circle-stroke-width': 1 } });
            activeLyrs.push(`lyr-circle-${safeId}`);
            activeSrcs.push(`src-point-${safeId}`);
        }

        activeOverlayLayers.value[key] = { layers: activeLyrs, sources: activeSrcs };
        
        // Add Hover Handlers for cursor
        activeLyrs.forEach(id => {
            map.on('mouseenter', id, () => { if (map) map.getCanvas().style.cursor = 'pointer'; });
            map.on('mouseleave', id, () => { if (map) map.getCanvas().style.cursor = ''; });
        });

        overlayOpacities.value[key] = 1;
        nextTick(() => updateOverlayOpacity(path, 1, providedSource));

        try {
            const bounds = new maplibregl.LngLatBounds();
            const addCoords = (c) => {
                if (!c) return;
                if (typeof c[0] === 'number') { bounds.extend([c[0], c[1]]); }
                else { c.forEach(addCoords); }
            };
            (geojson.features || []).forEach(f => {
                if (f.geometry && f.geometry.coordinates) addCoords(f.geometry.coordinates);
            });
            if (!bounds.isEmpty()) map.fitBounds(bounds, { padding: 50 });
        } catch (boundsErr) {
            console.warn('[Heatmap] Could not fit bounds:', boundsErr);
        }

    } catch (e) {
        notify.showError("Overlay error: " + e.message);
    }
};

const loadExternalMapsConfig = async () => {
    try {
        const res = await fetch(`${staticURL}/external_maps.json`);
        if (res.ok) {
            externalMaps.value = await res.json();
            console.log("[Heatmap] Loaded external maps config:", externalMaps.value);
        } else {
            console.warn("[Heatmap] No external_maps.json found");
        }
    } catch(err) {
        console.warn("[Heatmap] Error loading external maps:", err);
    }
};

const showOverlayInfo = (item) => {
    if (!map) return;
    
    const center = map.getCenter();
    const props = {
        'Name': item.name,
        'Path': item.path,
        'Description': item.description || 'No description provided.',
        'Type': item.type || (item.path.endsWith('.pmtiles') ? 'PMTiles' : 'GeoJSON')
    };
    
    showFeaturePopup(center, props, 'Overlay Details', 'info');
};

const toggleExternalMap = async (mapItem) => {
    const layerId = `ext-${mapItem.id}`;
    
    // Toggle Off
    if (activeExternalMaps.value[mapItem.id]) {
        if (map) {
            const layers = activeExternalMaps.value[mapItem.id].layers || [];
            layers.forEach(l => {
                if (map.getLayer(l)) map.removeLayer(l);
            });
            if (map.getSource(layerId)) {
                map.removeSource(layerId);
            }
        }
        delete activeExternalMaps.value[mapItem.id];
        activeExternalMapTimeSlider.value.visible = false;
        return;
    }

    // Toggle On
    activeExternalMaps.value[mapItem.id] = { layers: [], sourceId: layerId };
    sidePanelLoading.value = true;
    notify.showSuccess(`Loading ${mapItem.name}...`);

    try {
        const res = await fetch(mapItem.url);
        if (!res.ok) throw new Error(`HTTP error! status: ${res.status}`);
        const geojson = await res.json();
        
        if (!map) return;
        
        map.addSource(layerId, { type: 'geojson', data: geojson });

        // Add Points
        if (mapItem.id === 'civil_war_battles') {
            // Register Icons
            const loadIcon = (id, svg) => {
                if (map.hasImage(id)) return;
                const img = new Image(60, 60);
                const blob = new Blob([svg], { type: 'image/svg+xml' });
                const url = URL.createObjectURL(blob);
                img.onload = () => {
                    map.addImage(id, img);
                    URL.revokeObjectURL(url);
                };
                img.src = url;
            };
            loadIcon('cw-union', civilWarIcons.union);
            loadIcon('cw-confederate', civilWarIcons.confederate);
            loadIcon('cw-inconclusive', civilWarIcons.inconclusive);

            map.addLayer({
                id: `${layerId}-points`,
                type: 'symbol',
                source: layerId,
                filter: ['any', ['==', ['geometry-type'], 'Point'], ['==', ['geometry-type'], 'MultiPoint']],
                layout: {
                    'icon-image': ['match', ['get', 'Result_s_'], 'Union victory', 'cw-union', 'Confederate victory', 'cw-confederate', 'cw-inconclusive'],
                    'icon-size': ['interpolate', ['linear'], ['get', 'Casualties_in_Integers'], 0, 0.4, 500, 0.6, 2000, 0.8, 10000, 1.2, 50000, 2.0],
                    'icon-allow-overlap': true,
                    'icon-ignore-placement': true
                }
            });
        } else {
            map.addLayer({
                id: `${layerId}-points`,
                type: 'circle',
                source: layerId,
                filter: ['any', ['==', ['geometry-type'], 'Point'], ['==', ['geometry-type'], 'MultiPoint']],
                paint: { 'circle-radius': 6, 'circle-color': '#e53935', 'circle-stroke-color': '#ffffff', 'circle-stroke-width': 1 }
            });
        }

        // Add Lines (for actual LineStrings)
        map.addLayer({
            id: `${layerId}-lines`,
            type: 'line',
            source: layerId,
            filter: ['any', ['==', ['geometry-type'], 'LineString'], ['==', ['geometry-type'], 'MultiLineString']],
            paint: { 'line-color': '#e53935', 'line-width': 3 }
        });

        // Add Fills (Polygons)
        map.addLayer({
            id: `${layerId}-fills`,
            type: 'fill',
            source: layerId,
            filter: ['any', ['==', ['geometry-type'], 'Polygon'], ['==', ['geometry-type'], 'MultiPolygon']],
            paint: {
                'fill-color': mapItem.id === 'civil_war_states' ? [
                    'match', ['get', 'CW_COUNTRY'], 'USA', '#2196F3', 'CSA', '#F44336', 'Border State - USA', '#90A4AE', 'rgba(0,0,0,0.1)'
                ] : 'rgba(0,0,0,0.1)',
                'fill-opacity': 0.5
            }
        });

        // Add Outlines (for Polygons)
        map.addLayer({
            id: `${layerId}-outlines`,
            type: 'line',
            source: layerId,
            filter: ['any', ['==', ['geometry-type'], 'Polygon'], ['==', ['geometry-type'], 'MultiPolygon']],
            paint: {
                'line-color': mapItem.id === 'civil_war_states' ? [
                    'match', ['get', 'CW_COUNTRY'], 'USA', '#0D47A1', 'CSA', '#B71C1C', 'Border State - USA', '#455A64', '#555'
                ] : '#555',
                'line-width': 1
            }
        });

        activeExternalMaps.value[mapItem.id].layers.push(`${layerId}-points`, `${layerId}-lines`, `${layerId}-fills`, `${layerId}-outlines`);
        
        // Add Hover Handlers for cursor
        activeExternalMaps.value[mapItem.id].layers.forEach(id => {
            map.on('mouseenter', id, () => { if (map) map.getCanvas().style.cursor = 'pointer'; });
            map.on('mouseleave', id, () => { if (map) map.getCanvas().style.cursor = ''; });
        });

        // Time Slider Setup
        if (mapItem.timeAware) {
            setupTimeSlider(geojson.features || [], mapItem, layerId);
        }

        // Fit Bounds
        try {
            const bounds = new maplibregl.LngLatBounds();
            const addCoords = (c) => {
                if (!c) return;
                if (typeof c[0] === 'number') { bounds.extend([c[0], c[1]]); }
                else { c.forEach(addCoords); }
            };
            (geojson.features || []).forEach(f => {
                if (f.geometry && f.geometry.coordinates) addCoords(f.geometry.coordinates);
            });
            if (!bounds.isEmpty()) map.fitBounds(bounds, { padding: 50 });
        } catch(e) {}
        
    } catch (e) {
        console.error("[Heatmap] Failed to load external map:", e);
        notify.showError(`Failed to load ${mapItem.name}`);
        delete activeExternalMaps.value[mapItem.id];
    } finally {
        sidePanelLoading.value = false;
    }
};

const setupTimeSlider = (features, mapItem, sourceId) => {
    let minTime = Infinity;
    let maxTime = -Infinity;
    
    // Find absolute bounds from features
    features.forEach(f => {
        let tStart = f.properties[mapItem.timeFieldStart];
        let tEnd = f.properties[mapItem.timeFieldEnd] || tStart; // fallback if no end
        
        if (tStart) {
            // ArcGIS dates are often epoch ms
            const ts = typeof tStart === 'number' ? tStart : new Date(tStart).getTime();
            if (ts < minTime) minTime = ts;
            if (ts > maxTime) maxTime = ts;
        }
        if (tEnd) {
            const te = typeof tEnd === 'number' ? tEnd : new Date(tEnd).getTime();
            if (te < minTime) minTime = te;
            if (te > maxTime) maxTime = te;
        }
    });

    if (minTime === Infinity) return; // No valid time fields found

    activeExternalMapTimeSlider.value = {
        visible: true,
        mapItem: mapItem,
        sourceId: sourceId,
        mapName: mapItem.name,
        min: minTime,
        max: maxTime,
        low: minTime,
        high: maxTime,
        fieldScale: mapItem.timeFieldStart,
        minDisplay: new Date(minTime).toLocaleDateString(),
        maxDisplay: new Date(maxTime).toLocaleDateString(),
        lowDisplay: new Date(minTime).toLocaleDateString(),
        highDisplay: new Date(maxTime).toLocaleDateString()
    };
    
    applyExternalMapTimeFilter();
};

watch([() => activeExternalMapTimeSlider.value.low, () => activeExternalMapTimeSlider.value.high], () => {
    if (!activeExternalMapTimeSlider.value.visible) return;
    
    const slider = activeExternalMapTimeSlider.value;
    
    // Ensure handles don't cross in a way that breaks logic (low > high)
    // Most browser range inputs handle this if the other is set as min/max, 
    // but with two full-range inputs, we want them to "push" each other or just cap.
    // For now, let's just update displays.
    
    slider.lowDisplay = new Date(Math.min(slider.low, slider.high)).toLocaleDateString();
    slider.highDisplay = new Date(Math.max(slider.low, slider.high)).toLocaleDateString();
    
    applyExternalMapTimeFilter();
});

const applyExternalMapTimeFilter = () => {
    if (!map || !activeExternalMapTimeSlider.value.visible) return;
    const item = activeExternalMapTimeSlider.value.mapItem;
    if (!item) return;

    const t1 = Math.min(activeExternalMapTimeSlider.value.low, activeExternalMapTimeSlider.value.high);
    const t2 = Math.max(activeExternalMapTimeSlider.value.low, activeExternalMapTimeSlider.value.high);
    
    // Create a range filter: StartDate >= min AND StartDate <= max
    const filter = ['all', 
        ['>=', ['get', item.timeFieldStart], t1],
        ['<=', ['get', item.timeFieldStart], t2]
    ];
    
    const layers = activeExternalMaps.value[item.id].layers || [];
    layers.forEach(l => {
        if (map.getLayer(l)) {
            map.setFilter(l, filter);
        }
    });
};

const isOverlayActive = (path, sourceArg = null) => !!activeOverlayLayers.value[getOverlayKey(path, sourceArg)];

const updateOverlayOpacity = (path, value, sourceArg = null) => {
    const key = getOverlayKey(path, sourceArg);
    const val = parseFloat(value);
    overlayOpacities.value[key] = val;
    
    if (!map || !activeOverlayLayers.value[key]) return;
    
    activeOverlayLayers.value[key].layers.forEach(id => {
        if (map.getLayer(id)) {
            const type = map.getLayer(id).type;
            if (type === 'raster') map.setPaintProperty(id, 'raster-opacity', val);
            else if (type === 'line') map.setPaintProperty(id, 'line-opacity', val);
            else if (type === 'fill') map.setPaintProperty(id, 'fill-opacity', val);
            else if (type === 'circle') {
                map.setPaintProperty(id, 'circle-opacity', val);
                map.setPaintProperty(id, 'circle-stroke-opacity', val);
            }
        }
    });
};

const getOverlayOpacity = (path, sourceArg = null) => {
    const key = getOverlayKey(path, sourceArg);
    return overlayOpacities.value[key] !== undefined ? overlayOpacities.value[key] : 1;
};

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

            console.log(`[Heatmap] REBUILT inspectLocation: Requesting cluster data from backend for path='${cleanPath}', source='${cleanSource}'`);
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
                        // GROUPING KEY CALCULATION:
                        // We want to consolidate all items (files or folder-aggregates) 
                        // that logically belong to the same directory.
                        let p = (item.type === 'folder') ? item.path : (item.parentPath || "Root");
                        
                        // If it's a folder aggregate but points to a filename, normalize to parent
                        if (item.type === 'folder' && (p.toLowerCase().endsWith('.jpg') || p.toLowerCase().endsWith('.jpeg') || p.toLowerCase().endsWith('.png'))) {
                            p = p.substring(0, p.lastIndexOf('/')) || "Root";
                        }

                        if (!groups[p]) {
                            groups[p] = { 
                                path: p, 
                                thumbnailPath: item.path, // Keep original path for thumbnail
                                thumbnailUrl: getPreviewUrl(item.path, item.source, 'small'),
                                displayName: "", 
                                count: 0, 
                                totalImageCount: 0,
                                items: [], 
                                isVirtual: (item.type === 'folder' || p !== item.path),
                                clusterIDs: new Set(),
                                source: item.source,
                                lat: item.lat,
                                lon: item.lon
                            };
                            
                            // DISPLAY NAME: Use the last segment of the FOLDER path
                            const segments = p.split('/').filter(s => s !== "");
                            groups[p].displayName = segments.length > 0 ? segments[segments.length - 1] : "Root";
                        }
                        
                        // Sum up counts
                        groups[p].count += (item.count && item.count > 1 ? item.count : 1);
                        
                        // Collect IDs
                        if (item.clusterID) {
                            groups[p].clusterIDs.add(item.clusterID);
                        }

                        // Add all items to the items list for grid display (files AND sub-clusters)
                        groups[p].items.push(item);

                        // Aggregate total image count (Take max/typical value, don't sum)
                        if (item.totalImageCount) {
                            groups[p].totalImageCount = Math.max(groups[p].totalImageCount || 0, item.totalImageCount);
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
                    
                    // If only one folder, manage navigation
                    if (formattedSidePanelData.value.length === 1) {
                        const group = formattedSidePanelData.value[0];
                        
                        // AUTO-DRILL: If we found exactly one folder, and it's not the one we are already looking AT, 
                        // drill into it to see the actual images (Solves "Folders in Boxes" and "Single Folder Clusters")
                        if (group.isVirtual && group.path !== path) {
                            console.log(`[Heatmap] Single folder result (${group.path}) detected. Auto-drilling...`);
                            openFolderView(group);
                            return;
                        }

                        // Otherwise, just show the grid for this group
                        console.log('[Heatmap] Single item result (Already drilled or File). Showing grid.');
                        currentInspectionFolder.value = group;
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

// Helper for Back Navigation
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
        
        const geoFiles = items.filter(f => {
            const low = f.name.toLowerCase();
            return low.endsWith('.geojson') || low.endsWith('.pmtiles') || low.endsWith('.pmtile');
        });
        
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
        if (overlayInput.includes('/') || overlayInput.endsWith('.geojson') || overlayInput.endsWith('.pmtiles') || overlayInput.endsWith('.pmtile')) {
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
    if (initMapPromise) return initMapPromise;
    await nextTick();
    
    // Ensure container exists
    const container = document.getElementById('heatmap-container');
    if (!container) return;

    initMapPromise = new Promise((resolve) => {
        map = new maplibregl.Map({
            container: 'heatmap-container',
            style: {
                version: 8,
                sources: {},
                layers: []
            },
            center: [-18.48507, 38.81225],
            zoom: 2,
            antialias: true,
            preserveDrawingBuffer: true, // Required for canvas.toDataURL()
            glyphs: 'https://demotiles.maplibre.org/font/{fontstack}/{range}.pbf'
        });

        map.addControl(new maplibregl.NavigationControl(), 'top-left');
        map.addControl(new maplibregl.FullscreenControl(), 'top-left');
        map.addControl(new BasemapControl(), 'top-left');
        map.addControl(new PrintControl(), 'top-left');

        map.on('load', () => {
            // Add all basemaps explicitly
            Object.keys(mapStyles).forEach(key => {
                map.addSource(key, {
                    type: 'raster',
                    tiles: mapStyles[key].tiles,
                    tileSize: 256,
                    maxzoom: mapStyles[key].maxZoom
                });
                
                map.addLayer({
                    id: key,
                    type: 'raster',
                    source: key,
                    minzoom: 0,
                    maxzoom: 22,
                    layout: {
                        visibility: key === currentBasemap.value ? 'visible' : 'none'
                    },
                    paint: {
                        'raster-fade-duration': 0
                    },
                    metadata: {
                        'mapbox:group': 'background'
                    }
                });
            });

            // Add Arrowhead Icon (Triangle)
            const triangle = new Uint8Array([
                0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,
                0,0,0,0,0,0,0,1,1,0,0,0,0,0,0,0,
                0,0,0,0,0,0,1,1,1,1,0,0,0,0,0,0,
                0,0,0,0,0,1,1,1,1,1,1,0,0,0,0,0,
                0,0,0,0,1,1,1,1,1,1,1,1,0,0,0,0,
                0,0,0,1,1,1,1,1,1,1,1,1,1,0,0,0,
                0,0,1,1,1,1,1,1,1,1,1,1,1,1,0,0,
                0,1,1,1,1,1,1,1,1,1,1,1,1,1,1,0,
                0,1,1,1,1,1,1,1,1,1,1,1,1,1,1,0,
                0,0,1,1,1,1,1,1,1,1,1,1,1,1,0,0,
                0,0,0,1,1,1,1,1,1,1,1,1,1,0,0,0,
                0,0,0,0,1,1,1,1,1,1,1,1,0,0,0,0,
                0,0,0,0,0,1,1,1,1,1,1,0,0,0,0,0,
                0,0,0,0,0,0,1,1,1,1,0,0,0,0,0,0,
                0,0,0,0,0,0,0,1,1,0,0,0,0,0,0,0,
                0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
            ].map(v => v ? 255 : 0));
            // Use a simple 16x16 alpha mask basically
            const rgba = new Uint8Array(16 * 16 * 4);
            for (let i = 0; i < 256; i++) {
                rgba[i * 4 + 0] = 255;
                rgba[i * 4 + 1] = 255;
                rgba[i * 4 + 2] = 255;
                rgba[i * 4 + 3] = triangle[i];
            }
            map.addImage('triangle', { width: 16, height: 16, data: rgba }, { sdf: true });

            debugStatus.value = "Map loaded";
            
            // Add Drawing Sources/Layers
            map.addSource('draw-source', {
                type: 'geojson',
                data: drawnFeatures.value
            });

            // Lines (Arrows)
            map.addLayer({
                id: 'draw-lines',
                type: 'line',
                source: 'draw-source',
                filter: ['==', '$type', 'LineString'],
                paint: {
                    'line-color': ['get', 'color'],
                    'line-width': 3
                }
            });

            // Points
            map.addLayer({
                id: 'draw-points',
                type: 'circle',
                source: 'draw-source',
                filter: ['==', '$type', 'Point'],
                paint: {
                    'circle-color': ['get', 'color'],
                    'circle-radius': 6,
                    'circle-stroke-width': 2,
                    'circle-stroke-color': '#fff'
                }
            });

            // Labels & Arrowheads
            map.addLayer({
                id: 'draw-labels',
                type: 'symbol',
                source: 'draw-source',
                layout: {
                    'text-field': ['get', 'label'],
                    'text-font': ['Open Sans Semibold'],
                    'text-size': 14,
                    'text-offset': [0, 1.2],
                    'text-anchor': 'top',
                    'icon-image': ['get', 'icon'],
                    'icon-allow-overlap': true,
                    'text-allow-overlap': true,
                    'icon-rotate': ['get', 'bearing'],
                    'icon-rotation-alignment': 'map'
                },
                paint: {
                    'text-color': ['get', 'color'],
                    'text-halo-color': '#fff',
                    'text-halo-width': 2
                }
            });

            setupMapEventHandlers();
            resolve();
        });
    });

    return initMapPromise;
};

const showFeaturePopup = (lngLat, props, title = 'Feature Details', icon = 'info', sourceLabel = 'Overlay Data') => {
    if (!map) return;
    console.log("[Popup] Showing with props:", props);

    let rows = Object.entries(props).map(([key, val], idx) => {
        // Format dates if they look like epoch timestamps
        if (typeof val === 'number' && val > 1000000000 && (key.toLowerCase().includes('date') || key.toLowerCase().includes('time'))) {
            val = new Date(val).toLocaleString();
        }
        // Handle boolean/object
        if (typeof val === 'boolean') val = val ? 'Yes' : 'No';
        if (val === null || val === undefined) val = '-';
        if (typeof val === 'object') val = JSON.stringify(val);

        return `
            <tr style="background: ${idx % 2 === 0 ? '#f8f9fa' : '#fff'}; border-bottom: 1px solid #edf2f7;">
                <td style="padding: 6px 10px; color: #718096; font-weight: 600; width: 35%; border-right: 1px solid #edf2f7; vertical-align: top; word-break: break-word;">${key}</td>
                <td style="padding: 6px 10px; color: #2d3748; font-weight: 400; word-break: break-word; line-height: 1.4;">${val}</td>
            </tr>
        `;
    }).join('');

    let popupContent = `
        <div class="premium-popup-container">
            <div class="premium-popup-header">
                <i class="material-icons" style="font-size: 18px; color: #81c784;">${icon}</i>
                <span class="premium-popup-title">${title}</span>
            </div>
            <div class="premium-popup-body" style="max-height: 350px; overflow-y: scroll !important; display: block; background: #fff;">
                <table class="premium-popup-table" style="width: 100%; border-collapse: collapse; table-layout: auto;">
                    ${rows}
                </table>
            </div>
            <div class="premium-popup-footer">
                Source: ${sourceLabel}
            </div>
        </div>
    `;

    new maplibregl.Popup({ maxWidth: '340px', className: 'premium-popup', offset: [0, -5] })
        .setLngLat(lngLat)
        .setHTML(popupContent)
        .addTo(map);
};

const setupMapEventHandlers = () => {
    if (!map) return;
    
    map.on('zoomend', () => {
        currentZoom.value = Math.round(map.getZoom());
    });

    map.on('moveend', updateViewportData);

    map.on('mousemove', (e) => {
        const wrapped = e.lngLat.wrap();
        if (Math.abs(wrapped.lat) <= 90 && Math.abs(wrapped.lng) <= 180) {
            cursorCoords.value = { lat: wrapped.lat, lng: wrapped.lng };
        } else {
            cursorCoords.value = { lat: 999, lng: 999 };
        }
    });

    map.on('click', (e) => {
        if (activeDrawTool.value) {
            handleDrawClick(e);
            return;
        }

        const wrapped = e.lngLat.wrap();
        cursorCoords.value = { lat: wrapped.lat, lng: wrapped.lng };

        // Check for clicks on external layers OR local overlays
        const features = map.queryRenderedFeatures(e.point);
        const feature = features.find(f => f.layer.id.startsWith('ext-') || f.layer.id.startsWith('lyr-'));
        
        if (feature) {
            const props = feature.properties;
            
            // Try to find a header title
            const titleField = Object.keys(props).find(k => 
                ['state_name', 'battle_name', 'unit_name', 'name', 'title', 'label'].includes(k.toLowerCase())
            );
            const title = titleField ? props[titleField] : 'Feature Details';
            const isExternal = feature.layer.id.startsWith('ext-');

            showFeaturePopup(e.lngLat, props, title, isExternal ? 'info' : 'layers', isExternal ? 'ArcGIS Feature Service' : 'GeoJSON Overlay');
        }
    });

    // Right-click Context Menu
    map.on('contextmenu', (e) => {
        const lat = e.lngLat.lat.toFixed(5);
        const lng = e.lngLat.lng.toFixed(5);
        
        // Show context menu using a popup (MapLibre style)
        new maplibregl.Popup()
            .setLngLat(e.lngLat)
            .setHTML(`
                <div style="text-align:center; font-size:12px; color: #333;">
                    <b>Lat:</b> ${lat}<br>
                    <b>Lon:</b> ${lng}<br>
                    <div style="display:flex; flex-direction:column; gap:5px; margin-top:5px;">
                        <button onclick="window.toggleDebugInfo()" 
                            class="button button--flat" 
                            style="padding:2px 8px; font-size:11px; cursor:pointer;">
                            Toggle Debug Window
                        </button>
                        <button onclick="window.copyText('${lat}, ${lng}')" 
                            class="button button--flat" 
                            style="padding:2px 8px; font-size:11px; cursor:pointer;">
                            Copy Coordinates
                        </button>
                    </div>
                </div>
            `)
            .addTo(map);
    });

    // Global helpers
    window.toggleDebugInfo = () => {
        showDebugInfo.value = !showDebugInfo.value;
    };

    window.copyText = (text) => {
        navigator.clipboard.writeText(text).then(() => {
            notify.showSuccess("Copied to clipboard");
        }).catch(err => console.error('Failed to copy', err));
    };

    window.inspectLocationGlobal = (path, source) => {
        inspectLocation(path, source);
    };

    window.inspectLocationDirect = (items, source) => {
        const finalItems = items || window._tempInspectItems || [];
        const finalSource = source || (finalItems.length > 0 ? finalItems[0].source : "");
        inspectLocation(null, finalSource, finalItems);
        if (!items) window._tempInspectItems = null;
    };

    window.inspectLocationByCoords = inspectLocationByCoords;
};

const inspectLocationByCoords = (lat, lon, path, source, bounds) => {
    let extra = { lat: parseFloat(lat), lon: parseFloat(lon) };
    if (bounds) extra = { ...extra, ...bounds };
    inspectLocation(path, source, null, extra);
};

// Function to fetch and update data based on current viewport
const updateViewportData = async () => {
    if (!map) return;

    // Debounce: Clear previous timer
    if (updateDebounceTimer) clearTimeout(updateDebounceTimer);
    
    updateDebounceTimer = setTimeout(async () => {
        await executeUpdateViewportData();
    }, 150);
};

const executeUpdateViewportData = async () => {
    if (!map) return;
    
    const bounds = map.getBounds();
    const zoom = map.getZoom();
    const source = currentSource.value;
    const path = currentPath.value;
    
    const center = map.getCenter();
    const lat2tile = (lat, z) => Math.floor((1 - Math.log(Math.tan(lat * Math.PI / 180) + 1 / Math.cos(lat * Math.PI / 180)) / Math.PI) / 2 * Math.pow(2, z));
    const lon2tile = (lon, z) => Math.floor((lon + 180) / 360 * Math.pow(2, z));
    
    const z = Math.floor(Math.min(zoom, 18));
    const x = lon2tile(center.lng, z);
    const y = lat2tile(center.lat, z);
    
    const features = [];
    const seenPaths = new Set();
    const fetchPromises = [];

    if (abortController) abortController.abort();
    abortController = new AbortController();

    for (let dx = -1; dx <= 1; dx++) {
        for (let dy = -1; dy <= 1; dy++) {
            const tx = x + dx;
            const ty = y + dy;
            if (tx < 0 || ty < 0) continue;
            
            const url = `/api/heatmap/tiles/${z}/${tx}/${ty}?source=${encodeURIComponent(source)}&path=${encodeURIComponent(path)}`;
            fetchPromises.push(fetch(url, { signal: abortController.signal })
                .then(res => res.json())
                .catch(() => null));
        }
    }
    
    const results = await Promise.all(fetchPromises);
    
    const rawClusters = [];
    results.forEach(data => {
        if (data && data.clusters) {
            data.clusters.forEach(c => {
                 if (typeof c.lon !== 'number' || typeof c.lat !== 'number') return;
                 rawClusters.push(c);
            });
        }
    });

    const processedClusters = [];
    const pixelRadius = 80; // Increased from 50 to spread them out more
    
    rawClusters.forEach(c => {
         const pt = map.project([c.lon, c.lat]);
         let merged = false;
         
         for (let current of processedClusters) {
             const dx = current.px.x - pt.x;
             const dy = current.px.y - pt.y;
             if (dx * dx + dy * dy < pixelRadius * pixelRadius) {
                 current.count += c.count;
                 current.ids.push(c.id); // Accumulate IDs for Backend Inspect
                 
                 const minLat = c.min ? c.min[0] : c.lat;
                 const minLon = c.min ? c.min[1] : c.lon;
                 const maxLat = c.max ? c.max[0] : c.lat,
                       maxLon = c.max ? c.max[1] : c.lon;
                 current.bounds.minLat = Math.min(current.bounds.minLat, minLat);
                 current.bounds.minLon = Math.min(current.bounds.minLon, minLon);
                 current.bounds.maxLat = Math.max(current.bounds.maxLat, maxLat);
                 current.bounds.maxLon = Math.max(current.bounds.maxLon, maxLon);
                 merged = true;
                 break;
             }
         }
         
         if (!merged) {
             processedClusters.push({
                 id: c.id,
                 ids: [c.id], // Initialize ID list
                 path: c.path,
                 source: c.source || source,
                 count: c.count,
                 lat: c.lat,
                 lon: c.lon,
                 px: pt,
                 bounds: {
                     minLat: c.min ? c.min[0] : c.lat,
                     minLon: c.min ? c.min[1] : c.lon,
                     maxLat: c.max ? c.max[0] : c.lat,
                     maxLon: c.max ? c.max[1] : c.lon,
                     clusterID: c.id
                 }
             });
         }
    });

    processedClusters.forEach(c => {
        const renderKey = `cluster_${c.id}_${z}`;
        if (seenPaths.has(renderKey)) return;
        seenPaths.add(renderKey);
        
        const isThumbMarker = zoom >= 4;
        
        features.push({
            type: 'Feature',
            geometry: { type: 'Point', coordinates: [c.lon, c.lat] },
            properties: {
                id: c.ids.join(','), // Send ALL merged IDs to GeoJSON
                path: c.path,
                source: c.source,
                count: c.count,
                count_str: c.count > 1 ? c.count.toString() : "",
                bounds: JSON.stringify(c.bounds)
            }
        });
        
        if (isThumbMarker && showImageClusters.value) {
            if (!clusterMarkers[renderKey]) {
                const el = document.createElement('div');
                
                let borderColor = '#f1c40f';
                let folderPath = "/";
                const lastSlash = c.path.lastIndexOf('/');
                if (lastSlash > 0) folderPath = c.path.substring(0, lastSlash);
                borderColor = getClusterColor(folderPath);
                
                let innerHtml = '';
                if (c.path) {
                    const url = '/api/preview?path=' + encodeURIComponent(c.path) + '&source=' + encodeURIComponent(c.source) + '&size=small';
                    innerHtml = `<div class="cluster-thumb-container" style="border:3px solid ${borderColor}; box-shadow:0 2px 5px rgba(0,0,0,0.5); background-color: #555; width:48px; height:48px; border-radius:50%; overflow:hidden; position:relative; box-sizing:border-box; cursor:pointer;">
                        <img src="${url}" class="fan-thumb-img" style="width:100%; height:100%; object-fit:cover; display:block;" onerror="window.fileBrowserHeatmapImageError(this, '${c.path.replace(/\'/g, "\\\'")}')" />
                        <span style="position:absolute; top:50%; left:50%; transform:translate(-50%, -50%); background:rgba(0,0,0,0.7); border-radius:10px; padding:1px 5px; color:white; font-size:11px; font-weight:bold; white-space:nowrap; pointer-events:none;">${c.count}</span>
                    </div>`;
                } else {
                    innerHtml = `<div style="background-color:rgba(100,100,100,0.8);border-radius:50%;width:30px;height:30px;display:flex;align-items:center;justify-content:center;border:2px solid ${borderColor}; cursor:pointer;"><span style="color:white;text-shadow:0 0 2px black;font-weight:bold;">${c.count}</span></div>`;
                }
                
                el.innerHTML = innerHtml;
                
                el.addEventListener('click', (e) => {
                    e.stopPropagation();
                    inspectLocationByCoords(c.lat, c.lon, c.path, c.source, c.bounds);
                });
                
                const marker = new maplibregl.Marker({ element: el })
                    .setLngLat([c.lon, c.lat])
                    .addTo(map);
                    
                clusterMarkers[renderKey] = marker;
            }
        }
    });
    
    // Cleanup old markers
    Object.keys(clusterMarkers).forEach(key => {
        if (!seenPaths.has(key) || zoom < 4 || !showImageClusters.value) {
            clusterMarkers[key].remove();
            delete clusterMarkers[key];
        }
    });
    
    if (map.getSource('heatmap-data')) {
        console.log(`[Heatmap] updateViewportData pushed ${features.length} features to heatmap-data source.`);
        map.getSource('heatmap-data').setData({
            type: 'FeatureCollection',
            features: features
        });
    }
};

const loadData = async () => {
    // Wait for map to be fully loaded before doing anything
    await initMap();
    if (!map) return;

    // Update refs so updateViewportData uses current context
    currentSource.value = recursiveDecode(route.query.source || "");
    currentPath.value = recursiveDecode(route.query.path || "");
    isFolderMode.value = !!currentPath.value;
    
    if (state.user && state.user.mapBasemap) {
        currentBasemap.value = state.user.mapBasemap;
    }

    debugStatus.value = "Loading data...";

    // Setup MapLibre Sources and Layers if not already present
    if (!map.getSource('heatmap-data')) {
        map.addSource('heatmap-data', {
            type: 'geojson',
            data: { type: 'FeatureCollection', features: [] }
        });

        // Heatmap Layer
        map.addLayer({
            id: 'heatmap-layer',
            type: 'heatmap',
            source: 'heatmap-data',
            maxzoom: 18,
            paint: {
                'heatmap-weight': ['interpolate', ['linear'], ['get', 'count'], 1, 1, 50, 10],
                'heatmap-intensity': ['interpolate', ['linear'], ['zoom'], 0, 1, 15, 3],
                'heatmap-color': [
                    'interpolate',
                    ['linear'],
                    ['heatmap-density'],
                    0, 'rgba(0,0,255,0)',
                    0.1, 'rgba(0,255,255,0.5)',
                    0.3, 'rgba(0,255,0,0.6)',
                    0.5, 'rgba(255,255,0,0.7)',
                    0.7, 'rgba(255,165,0,0.8)',
                    1.0, 'rgba(255,0,0,0.9)'
                ],
                'heatmap-radius': ['interpolate', ['linear'], ['zoom'], 0, 5, 8, 30],
                'heatmap-opacity': ['interpolate', ['linear'], ['zoom'], 6, 1, 15, 1, 18, 0]
            }
        });

        // Watch for cluster toggle to hide/show layer
        watch(showImageClusters, (val) => {
            if (map.getLayer('heatmap-layer')) {
                map.setPaintProperty('heatmap-layer', 'heatmap-opacity', val ? 1 : 0);
            }
            // Trigger marker cleanup/rerender
            executeUpdateViewportData();
        });
    }

    updateViewportData(); // Initial load
};


const handleResize = () => {
    isDesktop.value = window.innerWidth > 1024;
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
            console.log(`[Heatmap] Navigating to heatmap folder (MapLibre): ${targetPath}`);
            router.push({ path: '/heatmap-v2', query: { path: targetPath, source: sourceName } })
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
        // Mark map as used for admin usage tracking
        usersApi.markMapUsage();

        loadData();
        
        // Ensure overlays list is fetched immediately, especially for shared links
        fetchOverlays();

        // Load external maps configuration
        loadExternalMapsConfig();

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

    window.addEventListener('resize', handleResize);
    handleResize();
});

// CRITICAL: Cleanup when component unmounts to STOP requests
onBeforeUnmount(() => {
    console.log('Heatmap component unmounting - stopping all requests...');
    
    window.removeEventListener('resize', handleResize);
    
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
    
    // 2. Clear sources and layers (Optional but good for cleanliness)
    // Most resources are freed by map.remove()
    
    // 5. Destroy map
    if (map) {
        map.remove();
        map = null;
    }
    
    // 6. Cleanup stores removed (unused)
    
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
    scrollbar-width: thin;
    scrollbar-color: #555 transparent;
}
.panel-content::-webkit-scrollbar {
    width: 6px;
}
.panel-content::-webkit-scrollbar-thumb {
    background: #555;
    border-radius: 3px;
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
.folder-thumb-icon {
    width: 32px;
    height: 32px;
    object-fit: cover;
    border-radius: 4px;
    margin-right: 10px;
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
.folder-badge {
    position: absolute;
    top: 5px;
    right: 5px;
    color: #ffca28;
    background: rgba(0,0,0,0.6);
    border-radius: 50%;
    padding: 2px;
    font-size: 16px;
    z-index: 5;
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

/* Range Slider Styles */
.range-slider-wrapper {
    position: relative;
    height: 24px;
    width: 100%;
    margin: 10px 0;
    display: flex;
    align-items: center;
}

.range-slider-track {
    position: absolute;
    height: 6px;
    width: 100%;
    background-color: rgba(255, 255, 255, 0.1);
    border-radius: 3px;
    z-index: 1;
}

.range-slider-highlight {
    position: absolute;
    height: 6px;
    background-color: #81c784;
    border-radius: 3px;
    z-index: 2;
}

.range-handle {
    position: absolute;
    width: 100%;
    height: 6px;
    top: 50%;
    transform: translateY(-50%);
    background: none;
    pointer-events: none;
    -webkit-appearance: none;
    appearance: none;
    z-index: 3;
    margin: 0;
    cursor: pointer;
}

.range-handle::-webkit-slider-thumb {
    -webkit-appearance: none;
    appearance: none;
    width: 18px;
    height: 18px;
    border-radius: 50%;
    background: #81c784;
    border: 2px solid white;
    cursor: pointer;
    pointer-events: auto;
    box-shadow: 0 2px 4px rgba(0,0,0,0.3);
    transition: transform 0.15s ease-in-out;
}

.range-handle::-webkit-slider-thumb:hover {
    transform: scale(1.2);
}

.range-handle::-moz-range-thumb {
    width: 18px;
    height: 18px;
    border-radius: 50%;
    background: #81c784;
    border: 2px solid white;
    cursor: pointer;
    pointer-events: auto;
    box-shadow: 0 2px 4px rgba(0,0,0,0.3);
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

/* Premium Popup Styles */
.premium-popup .maplibregl-popup-content {
    background: transparent !important;
    padding: 0 !important;
    box-shadow: none !important;
    border: none !important;
    max-height: 420px !important; /* Total height cap */
    overflow-y: auto !important;
}

.premium-popup .maplibregl-popup-tip {
    border-top-color: #f7fafc !important; /* Matches footer */
}

.premium-popup-container {
    max-width: 320px;
    font-family: 'Inter', system-ui, -apple-system, sans-serif;
    overflow: hidden;
    border-radius: 12px;
    box-shadow: 0 10px 25px rgba(0,0,0,0.3);
    background: #fff;
    border: 1px solid #ddd;
}

.premium-popup-header {
    background: linear-gradient(135deg, #2c3e50, #34495e);
    color: white;
    padding: 12px 15px;
    display: flex;
    align-items: center;
    gap: 10px;
}

.premium-popup-title {
    font-weight: 600;
    font-size: 13px;
    letter-spacing: 0.5px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.premium-popup-body {
    max-height: 350px !important;
    min-height: 50px;
    overflow-y: scroll !important; /* Force it to ALWAYS show */
    overflow-x: hidden;
    background: #fff !important;
    padding: 2px;
    display: block;
    scrollbar-width: auto !important;
}

.premium-popup-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 11px;
    table-layout: auto;
}

.premium-popup-footer {
    background: #f7fafc;
    padding: 6px 12px;
    font-size: 9px;
    color: #a0aec0;
    text-align: right;
    border-top: 1px solid #edf2f7;
}

/* Custom Scrollbar for Popup Body */
.premium-popup-body::-webkit-scrollbar {
    width: 12px !important; /* Much wider for visibility */
    display: block !important;
}
.premium-popup-body::-webkit-scrollbar-track {
    background: #eeeeee !important;
    border-radius: 6px;
}
.premium-popup-body::-webkit-scrollbar-thumb {
    background: #888888 !important; /* Darker for contrast */
    border-radius: 6px;
    border: 2px solid #eeeeee;
}
.premium-popup-body::-webkit-scrollbar-thumb:hover {
    background: #555555 !important;
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
.tool-group {
    background: rgba(255,255,255,0.03);
    padding: 10px;
    border-radius: 8px;
    border: 1px solid rgba(255,255,255,0.05);
}
</style>
