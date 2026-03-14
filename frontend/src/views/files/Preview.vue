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
                <div class="rotation-wrapper" :style="{ transform: `rotate(${rotation}deg)`, transformOrigin: 'center', transition: 'transform 0.3s ease', display: 'inline-block', position: 'relative' }">
                  <img 
                    ref="image" 
                    :src="raw" 
                    @load="updateImageDimensions" 
                    class="preview-image"
                    style="display: block; max-width: 100%; max-height: 100%;"
                  >
                  <div
                    v-if="faceRegions.length > 0 && activeTab === 'xmp'"
                    class="face-overlay"
                    style="position: absolute; top: 0; left: 0; width: 100%; height: 100%; pointer-events: none; z-index: 100;"
                  >
                    <div
                      v-for="(region, index) in faceRegions"
                      :key="index"
                      class="face-box"
                      :style="getFaceBoxStyle(region)"
                    >
                      <span class="face-label" :style="{ fontSize: faceFontSize + 'px', fontWeight: 'bold', top: -faceFontSize * 1.5 + 'px', left: '-4px', padding: '2px 8px', borderRadius: '4px 4px 0 0', backgroundColor: getFaceBoxColor(region, 0.95), color: '#000', whiteSpace: 'nowrap' }">
                        {{ region.DisplayName || region.Name || 'Unknown' }}
                        <span v-if="region.Source === 'ml' && region.Name && region.Name !== 'Unknown'" style="font-size: 0.7em; margin-left: 4px;"> ({{ (region.Confidence * 100).toFixed(1) }}%)</span>
                      </span>
                    </div>
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

          <div v-else-if="previewType == 'text' && isJsonFile" class="json-preview">
            <div class="json-toolbar">
              <span class="json-label"><i class="material-icons">data_object</i> JSON</span>
              <button class="json-copy-btn" @click="copyJson" :title="jsonCopied ? 'Copied!' : 'Copy raw JSON'">
                <i class="material-icons">{{ jsonCopied ? 'check' : 'content_copy' }}</i>
              </button>
            </div>
            <pre class="json-body"><code v-html="prettyJsonContent"></code></pre>
          </div>

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
          <h3 :title="'Machine Learning Face Data'">
            {{ canViewACDSee ? 'ACDSee and ML Face Data' : 'ML Face Data' }}
          </h3>
          
          <div style="display: flex; gap: 0.5rem; margin-bottom: 1rem;">
            <button 
              v-if="hasAnyFaceData && canViewACDSee"
              @click="showACDSeeFaces = !showACDSeeFaces; if (showACDSeeFaces) showMLFaces = false;" 
              class="button button--flat" 
              :style="showACDSeeFaces ? 'color: var(--accent-green); background: rgba(66, 185, 131, 0.1); border: 1px solid var(--accent-green);' : 'opacity: 0.5; border: 1px solid transparent;'"
              style="padding: 0.2rem 0.5rem; min-height: unset; margin:0; display: flex; align-items: center; gap: 4px;" 
              :title="showACDSeeFaces ? 'ACDSee Faces ON' : 'ACDSee Faces OFF'"
            >
              <i class="material-icons" style="font-size: 18px;">recent_actors</i>
              <span style="font-size: 11px; font-weight: bold;">{{ acdseeFaceCount }}</span>
            </button>
            <button 
              v-if="hasAnyFaceData"
              @click="showMLFaces = !showMLFaces; if (showMLFaces) showACDSeeFaces = false;" 
              class="button button--flat" 
              :style="showMLFaces ? 'color: var(--accent-yellow); background: rgba(255, 235, 59, 0.1); border: 1px solid var(--accent-yellow);' : 'opacity: 0.5; border: 1px solid transparent;'"
              style="padding: 0.2rem 0.5rem; min-height: unset; margin:0; display: flex; align-items: center; gap: 4px;" 
              :title="showMLFaces ? 'ML Faces ON' : 'ML Faces OFF'"
            >
              <i class="material-icons" style="font-size: 18px;">psychology</i>
              <span style="font-size: 11px; font-weight: bold;">{{ mlFaceCount }}</span>
            </button>
            <button v-if="canRunFaceScan" @click="scanFacesForThisImage" :disabled="isScanningFaces" class="button button--flat" style="padding: 0.2rem 0.5rem; min-height: unset; margin:0;" :title="isScanningFaces ? 'Scanning...' : 'Scan image for faces'">
              <i class="material-icons" :class="{ 'spin': isScanningFaces }" style="font-size: 18px;">face</i>
            </button>
          </div>

          <!-- Font Size Control for Face Boxes -->
          <div v-if="faceRegions.length > 0" style="margin-bottom: 1rem; padding: 10px; background: rgba(255,255,255,0.05); border-radius: 4px;">
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
          <!-- Face List Panel -->
          <div v-if="faceRegions.length > 0">
            <!-- Toolbar: All / None / Remove Selected -->
            <div v-if="canManageFaces" style="display: flex; gap: 6px; align-items: center; margin-bottom: 8px; padding: 4px 0;">
              <button @click.stop="selectAllFaces" class="button button--flat" style="font-size: 10px; padding: 2px 6px; min-height: unset;">All</button>
              <button @click.stop="unselectAllFaces" class="button button--flat" style="font-size: 10px; padding: 2px 6px; min-height: unset;">None</button>
              <button v-if="selectedFaceIndices.length > 0"
                @click.stop="removeSelectedFaces"
                class="button button--flat"
                style="font-size: 10px; padding: 2px 6px; min-height: unset; color: #f44336; margin-left: auto;">
                <i class="material-icons" style="font-size: 13px; vertical-align: middle;">delete</i>
                Remove ({{ selectedFaceIndices.length }})
              </button>
            </div>

            <!-- Face rows -->
            <div v-for="(region, index) in faceRegions" :key="'fl-' + index"
                 style="border-bottom: 1px solid rgba(255,255,255,0.08); padding: 5px 0;">
              <div style="display: flex; align-items: center; gap: 6px;">
                <!-- Checkbox -->
                <input v-if="canManageFaces && (region.Source === 'ml' || (region.Source === 'acdsee' && region.Confidence >= 0.85))" type="checkbox" v-model="selectedFaceIndices" :value="index"
                       @click.stop
                       style="margin: 0; cursor: pointer; flex-shrink: 0;">
                <div v-else-if="canManageFaces" style="width: 13px; margin: 0; flex-shrink: 0;"></div>
                <!-- Color dot -->
                <span :style="{ color: getFaceBoxColor(region, 1.0), fontSize: '14px', flexShrink: 0 }">●</span>
                <!-- Name (click to start rename) -->
                <span v-if="renamingFaceIndex !== index"
                      :style="{ fontWeight: '500', flex: '1', cursor: (canManageFaces && (region.Source === 'ml' || (region.Source === 'acdsee' && region.Confidence >= 0.85))) ? 'pointer' : 'default', minWidth: '0', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }"
                      :title="region.DisplayName || region.Name"
                      @click.stop="canManageFaces && (region.Source === 'ml' || (region.Source === 'acdsee' && region.Confidence >= 0.85)) && startRename(index, region)">
                  {{ region.DisplayName || region.Name || 'Unknown' }}
                </span>
                <!-- Inline rename input -->
                <div v-else style="flex: 1; display: flex; gap: 4px; align-items: center; min-width: 0;">
                  <input type="text" list="rename-people-list-inline"
                         v-model="renameInput"
                         @keyup.enter="submitInlineRename(index, region)"
                         @keyup.escape="renamingFaceIndex = -1"
                         placeholder="Name..."
                         ref="inlineRenameInput"
                         class="input" style="flex: 1; height: 22px; font-size: 12px; padding: 1px 4px; min-width: 60px;">
                  <button @click.stop="submitInlineRename(index, region)" class="button button--flat" style="padding: 1px 4px; min-height: unset; font-size: 11px;">✓</button>
                  <button @click.stop="renamingFaceIndex = -1" class="button button--flat" style="padding: 1px 4px; min-height: unset; font-size: 11px;">✕</button>
                </div>
                <!-- Verify button (for unverified faces with a name) -->
                <button
                  v-if="canManageFaces && renamingFaceIndex !== index && region.Confidence < 1.0 && region.Name && region.Name !== 'Unknown' && region.Source === 'ml'"
                  @click.stop="quickVerifyFace(region)"
                  class="button button--flat"
                  title="Verify this face"
                  style="padding: 1px; min-height: unset; color: #4CAF50; flex-shrink: 0;">
                  <i class="material-icons" style="font-size: 16px;">check_circle</i>
                </button>
                <!-- Source + confidence -->
                <span style="font-size: 10px; opacity: 0.5; flex-shrink: 0; white-space: nowrap;"
                >{{ region.Source }} {{ region.Confidence ? (region.Confidence * 100).toFixed(0) + '%' : '' }}</span>
              </div>
            </div>
            <!-- Shared datalist for inline rename autocomplete -->
            <datalist id="rename-people-list-inline" v-if="peopleList && peopleList.length">
              <option v-for="person in peopleList" :value="person.name" :key="'rl-' + person.name"></option>
            </datalist>
          </div>
          <p v-else-if="hasAnyFaceData">Toggle ACDSee or ML faces above to see face data.</p>
          <p v-else-if="metadata && (!metadata.xmp || Object.keys(metadata.xmp).length === 0)">No face data found. Use the scan button to detect faces.</p>
          <p v-else>Loading...</p>
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
                                  <button v-if="canEditCoordinates" @click="handleClearLocation" class="button button--flat" title="Clear embedded location" style="padding: 6px; height: 32px; line-height: 1; min-width: 32px;">
                                      <i class="material-icons" style="font-size: 20px; color: #f44336;">delete</i>
                                  </button>
                                 
                                 <button @click="copyCoordinates" class="button button--flat" title="Copy" style="padding: 6px; height: 32px; line-height: 1; min-width: 32px;">
                                     <i class="material-icons" style="font-size: 14px;">content_copy</i>
                                 </button>
                                  <button v-if="canEditCoordinates" @click="saveEmbeddedLocationToProfile" class="button button--flat" title="Save this location to My Locations" style="padding: 6px; height: 32px; line-height: 1; min-width: 32px;">
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
                           <h3 style="margin: 0; font-size: 0.75em; color: #2196f3; display: flex; align-items: center; font-weight: 600;">
                                <div v-if="canEditCoordinates" 
                                     @click.stop="togglePin"
                                     style="display: flex; align-items: center; cursor: pointer; margin-right: 10px;"
                                     :title="isPinned ? 'Unpin location to follow image' : 'Pin location to copy to next image'"
                                >
                                     <i class="material-icons" 
                                        style="font-size: 18px; transition: all 0.2s;"
                                        :style="isPinned ? 'color: #2196f3; transform: rotate(0deg);' : 'color: #ccc; transform: rotate(45deg);'"
                                     >push_pin</i>
                                     <span v-if="isPinned" style="font-size: 0.8em; margin-left: 4px; color: #2196f3; font-weight: bold;">
                                         HOLDING COORDS
                                     </span>
                                </div>

                                <span v-if="canEditCoordinates">EDITABLE LOCATION (PROPOSED)</span>
                                <span v-else>MAP PREVIEW</span>
                                

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
                      <div v-if="canEditCoordinates" style="display: flex; gap: 10px; align-items: center;">
                           <div style="display: flex; gap: 5px; align-items: center;">
                               <label style="font-size: 0.85em; color: #666; font-weight: bold;">Lat:</label>
                               <input type="number" step="any" v-model.lazy.number="editLat" :disabled="!canEditCoordinates" class="input" :style="{ backgroundColor: isPinned ? '#2e7d32' : '', color: isPinned ? 'white' : '' }" style="width: 110px; height: 26px; font-size: 0.9em; padding: 2px 5px;">
                           </div>
                           <div style="display: flex; gap: 5px; align-items: center;">
                               <label style="font-size: 0.85em; color: #666; font-weight: bold;">Lon:</label>
                               <input type="number" step="any" v-model.lazy.number="editLon" :disabled="!canEditCoordinates" class="input" :style="{ backgroundColor: isPinned ? '#2e7d32' : '', color: isPinned ? 'white' : '' }" style="width: 110px; height: 26px; font-size: 0.9em; padding: 2px 5px;">
                           </div>
                           
                           <!-- Search Box MOVED from here -->
                      </div>
                      
                      <!-- Proposed Matches Status -->
                      <div v-if="canEditCoordinates" style="margin-top: 6px; font-size: 0.8em; color: #555; display: flex; align-items: center;">
                          <span style="font-weight: bold; margin-right: 4px;">Proposed Matches:</span>
                          <span v-if="matchedLocationName || gpsMatchedLocationName" 
                                @click.stop="applyMatchedLocation(matchedLocationName || gpsMatchedLocationName)"
                                style="color: #1565c0; background: #e3f2fd; padding: 0 4px; border-radius: 3px; cursor: pointer; text-decoration: underline;"
                                title="Click to apply this location">
                               {{ matchedLocationName || gpsMatchedLocationName }}
                          </span>
                          <span v-else style="font-style: italic; color: #777;">None</span>
                      </div>
                 </div>

                 <!-- Saved Locations Footer (Stretches to bottom) -->
                 <div v-if="canEditCoordinates" style="flex: 1; display: flex; flex-direction: column; background: white; color: #333; overflow: hidden; border-top: 1px solid #ddd; min-height: 0;">
                      <div @click.stop="editTab = editTab === 'locations' ? 'location' : 'locations'" style="flex: 0 0 auto; padding: 6px 10px; cursor: pointer; display: flex; justify-content: space-between; align-items: center; background: #f5f5f5; border-bottom: 1px solid #eee;">
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
                                @click.stop="editLat = loc.lat; editLon = loc.lon; updateMapMarker(loc.lat, loc.lon);"
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
import { FaceEvents } from "@/utils/FaceEvents";
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
      // isEditingCoordinates removed
      // isLocationPinned removed (moved to user profile)
      searchQuery: "",
      mapStatus: "", // Text status is fine to be reactive
      
      // Bulletproof Init: Read localStorage directly in data()
      // This ensures values are ready before ANY watchers or hooks run.
      localPinValue: (() => {
          try {
             const val = localStorage.getItem("pinnedLocation");
             return val ? JSON.parse(val) : null;
          } catch(e) { return null; }
      })(),
      
      // Init edits from localPinValue if available
      editLat: (() => {
          try {
             const val = localStorage.getItem("pinnedLocation");
             return val ? JSON.parse(val).lat : 0;
          } catch(e) { return 0; }
      })(),
      editLon: (() => {
          try {
             const val = localStorage.getItem("pinnedLocation");
             return val ? JSON.parse(val).lon : 0;
          } catch(e) { return 0; }
      })(),
      
      // Edit Settings Tabs
      
      // Edit Settings Tabs
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
      jsonCopied: false, // copy-button flash state
      isPreFetching: false, // background scan status
      facesData: [], // Store JSON parsed faces
      showACDSeeFaces: false, // Toggle ACDSee faces (off by default, ML is primary)
      showMLFaces: true,     // Toggle ML faces (default TRUE)
      selectedFaceIndices: [], // For bulk face selection
      renamingFaceIndex: -1,   // Index of face being renamed inline (-1 = none)
      renameInput: '',         // Current inline rename text
      faceContextMenu: {
        show: false,
        top: 0,
        left: 0,
        region: null,
        editName: ''
      },
      peopleList: [],
      isScanningFaces: false,
      scanInterval: null
    };
  },
  computed: {
    isMobile() {
        return state.isMobile;
    },
    hasAnyFaceData() {
      if (this.facesData && this.facesData.length > 0) return true;
      if (this.metadata && this.metadata.xmp) {
        if (this.metadata.xmp.Regions || this.metadata.xmp.RegionInfo || this.metadata.xmp.RegionInfoACDSee) return true;
      }
      return false;
    },
    faceRegions() {
      let combinedFaces = [];

      // Prioritize facesData (from faces.json + ML)
      if (this.facesData && this.facesData.length > 0) {
        let mapped = this.facesData.map(f => {
          let Area = f.Area || null;
          if (!Area && f.box && f.box.length === 4 && this.imageDimensions.naturalWidth > 0) {
            const y1 = f.box[0];
            const x2 = f.box[1];
            const y2 = f.box[2];
            const x1 = f.box[3];
            
            const w = x2 - x1;
            const h = y2 - y1;
            const cx = x1 + (w / 2);
            const cy = y1 + (h / 2);

            Area = {
              X: cx / this.imageDimensions.naturalWidth,
              Y: cy / this.imageDimensions.naturalHeight,
              W: w / this.imageDimensions.naturalWidth,
              H: h / this.imageDimensions.naturalHeight
            };
          }

          return {
            Name: f.name,
            Confidence: f.confidence,
            Source: f.source,
            Area: Area,
            _rawBox: f.box // Keep the raw coords for update/delete API calls
          }
        });

        // Filter out ml_scanned markers and explicitly rejected faces
        mapped = mapped.filter(f => f.Source !== 'ml_scanned');

        // Number unknown faces with compact display names
        let unknownCounter = 0;
        mapped.forEach(f => {
          if (!f.Name || f.Name === 'Unknown') {
            unknownCounter++;
            f.DisplayName = `#${unknownCounter}`;
          } else {
            f.DisplayName = f.Name;
          }
        });

        // Ignore explicitly rejected faces
        mapped = mapped.filter(f => f.Confidence === undefined || f.Confidence >= 0);

        // Filter based on toggles
        if (!this.showACDSeeFaces && !this.showMLFaces) {
            mapped = [];
        } else if (!this.showACDSeeFaces) {
            // Show ML faces, PLUS verified ACDSee faces since they are pulled into ML data
            mapped = mapped.filter(f => f.Source === "ml" || (f.Source === "acdsee" && f.Confidence >= 0.85));
        } else if (!this.showMLFaces) {
            // Show ONLY ACDSee faces
            mapped = mapped.filter(f => f.Source === "acdsee");
        }
        combinedFaces.push(...mapped);
      }

      // If no facesData exists yet, fallback to reading XMP natively (only if ACDSee faces are toggled ON)
      if (combinedFaces.length === 0 && this.showACDSeeFaces && this.metadata && this.metadata.xmp) {
        const container = this.metadata.xmp.Regions || 
                          this.metadata.xmp.RegionInfo || 
                          this.metadata.xmp.RegionInfoACDSee;

        let rawItems = [];
        if (container) {
          if (Array.isArray(container)) rawItems.push(...container);
          else if (container.RegionList && Array.isArray(container.RegionList)) rawItems.push(...container.RegionList);
          else if (container.RegionList && typeof container.RegionList === 'object') rawItems.push(container.RegionList);
        }

        combinedFaces = rawItems.map(item => {
           // Handle MWG RegionInfo structure (standard for ACDSee/others)
           let area = item.Area || item.area || item.ALGArea || item.DLYArea || item;
           let name = item.Name || item.name || 'Unknown';
           return {
             Name: name,
             DisplayName: name,
             Source: 'acdsee',
             Confidence: 1.0,
             Area: {
               X: area.X ?? area.x ?? area['stArea:x'],
               Y: area.Y ?? area.y ?? area['stArea:y'],
               W: area.W ?? area.w ?? area['stArea:w'],
               H: area.H ?? area.h ?? area['stArea:h']
             }
           }
        });
      }

      return combinedFaces;
    },
    canRunFaceScan() {
      // In Preview.vue, we check File Management permission for single file scans
      return state.user?.permissions?.manageFaces === true && this.previewType === 'image';
    },
    acdseeFaceCount() {
       if (!this.facesData) return 0;
       return this.facesData.filter(f => f.source === 'acdsee').length;
    },
    mlFaceCount() {
       if (!this.facesData) return 0;
       return this.facesData.filter(f => f.source === 'ml' || (f.source === 'acdsee' && f.confidence >= 0.85)).length;
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
        return state.user?.permissions?.updateMap === true;
    },
    hasUnsavedCoordinates() {
        if (this.isPinned) return true; // Always allow saving if explicit pin is set
        if (!this.gpsCoordinates) {
            return this.editLat !== 0 || this.editLon !== 0; // If no original, any change from 0 is "unsaved"
        }
        // Use a small epsilon for float comparison or just direct
        return Math.abs(this.editLat - this.gpsCoordinates.lat) > 0.000001 || 
               Math.abs(this.editLon - this.gpsCoordinates.lon) > 0.000001;
    },
    canManageFaces() {
        return state.user?.permissions?.manageFaces === true;
    },
    canViewACDSee() {
        return state.user?.permissions?.viewACDSee === true || state.user?.permissions?.admin === true;
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
    isJsonFile() {
      const name = (state.req && state.req.name) ? state.req.name.toLowerCase() : '';
      return name.endsWith('.json');
    },
    prettyJsonContent() {
      if (!this.textContent) return '';
      let textToColor = this.textContent;
      try {
        // Clean leading BOMs and trim
        const cleanText = this.textContent.replace(/^\uFEFF/, '').trim();
        const parsed = JSON.parse(cleanText);
        textToColor = JSON.stringify(parsed, null, 2);
      } catch (e) {
        // Not valid strict JSON (e.g. contains comments).
        // Fall back to original raw text to at least show the content.
        console.warn('JSON parsing failed, falling back to raw text for syntax highlighting');
      }

      // Escape HTML entities to prevent injection
      textToColor = textToColor.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

      // Syntax highlight via a single pass regex
      return textToColor.replace(/("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g, function (match) {
        let cls = 'json-num';
        if (/^"/.test(match)) {
          if (/:$/.test(match)) {
            cls = 'json-key';
            const colonIndex = match.lastIndexOf(':');
            return '<span class="' + cls + '">' + match.substring(0, colonIndex).trim() + '</span>:';
          } else {
            cls = 'json-str';
          }
        } else if (/true|false/.test(match)) {
          cls = 'json-bool';
        } else if (/null/.test(match)) {
          cls = 'json-null';
        }
        return '<span class="' + cls + '">' + match + '</span>';
      });
    },
    pinnedLocation() {
      return state.user ? state.user.pinnedLocation : null;
    },
    activePin() {
      return this.pinnedLocation || this.localPinValue;
    },
    isPinned() {
      return !!this.activePin;
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
    rotation() {
      return state.previewRotation;
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
    async raw() {
      // console.log('🔥 RAW WATCHER TRIGGERED!', this.raw);
      
      mutations.resetPreviewRotation();
      
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
        await this.fetchMetadata();
      } catch (e) {
        console.error('fetchTextContent/metadata error:', e);
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
        }
        
        // Final safety check: if pinned, ensure inputs match pin
        // This fixes the "0,0" bug on folder change
        // Final safety check: if pinned, ensure inputs match pin
        // This fixes the "0,0" bug even if promises fail
        // Using finally ensures this runs regardless of 500 errors (e.g. heatmap)
      }).finally(() => {
        if (this.isPinned) {
            this.reapplyPinnedLocation();
        }
      });
    },

    gpsCoordinates(newVal) {
       // console.log("LOG_TRACE: gpsCoordinates watcher triggered. Lat/Lon:", newVal?.lat, newVal?.lon);
       
       // Priority 1: Update Inputs (Always visible in header)
       if (newVal) {
           // Update inputs if not pinned
           if (!this.isPinned && this.canEditCoordinates) {
               this.editLat = parseFloat(newVal.lat.toFixed(6));
               this.editLon = parseFloat(newVal.lon.toFixed(6));
               if (typeof this.updateMapMarker === 'function') {
                   this.updateMapMarker(this.editLat, this.editLon);
               }
           } else {
               // console.log("LOG_TRACE: gpsCoordinates SKIPPING update. Pinned:", this.isPinned);
           }
       } else {
           // Clear inputs if not pinned
           if (!this.isPinned && this.canEditCoordinates) {
               this.editLat = 0;
               this.editLon = 0;
               if (typeof this.updateMapMarker === 'function') {
                   this.updateMapMarker(0, 0);
               }
           }
       }

       // Priority 2: Map & Marker Updates (Only if map ready)
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
       } else {
           if (this.embeddedMarker) {
               this.mapInstance.removeLayer(this.embeddedMarker);
               this.embeddedMarker = null;
           }
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
      },
      activePin: {
        handler(newVal) {
            
            // Sync activePin back to localPinValue if it came from backend (newVal is truthy, local might be null)
            // But activePin returns (pinnedLocation || localPinValue).
            // We just need to ensure that if pinnedLocation is what's valid, localPin gets updated?
            // Actually, if activePin is valid, use it.
            
            if (newVal) {
                this.editLat = newVal.lat;
                this.editLon = newVal.lon;
                if (typeof this.updateMapMarker === 'function') {
                     this.$nextTick(() => {
                        this.updateMapMarker(this.editLat, this.editLon);
                     });
                }
            } else {
               // Unpinned: Revert to image's GPS coordinates if available
               if (this.gpsCoordinates) {
                   this.editLat = parseFloat(Number(this.gpsCoordinates.lat).toFixed(6));
                   this.editLon = parseFloat(Number(this.gpsCoordinates.lon).toFixed(6));
               } else {
                   // No GPS on image, reset to 0
                   this.editLat = 0;
                   this.editLon = 0;
               }
               
               if (typeof this.updateMapMarker === 'function') {
                    this.$nextTick(() => {
                        this.updateMapMarker(this.editLat, this.editLon);
                    });
               }
            }
        },
        immediate: true 
      },
    pinnedLocation: {
          handler(newVal) {
            // Keep local backup in sync
            // Only update if truthy, or if we need to clear local because of explicit unpin?
            // Since we handle explicit unpin in togglePin, 
            // Here we just grab valid values to ensure polyfill is ready for next reload.
            if (newVal) {
                this.localPinValue = newVal;
            }
            // Note: we do NOT clear localPinValue here if null, 
            // to allow local backup to survive transient backend nulls.
          },
          immediate: true
      },
    req() {
        mutations.resetPreviewRotation();
    }
  },
  created() {
      // Non-reactive properties for Leaflet
      this.mapInstance = null;
      this.mapMarker = null;
  },
  async mounted() {
    // Check for GeoJSON redirect
    const low = this.req.name.toLowerCase();
    if (this.req && this.req.name && (low.endsWith('.geojson') || low.endsWith('.pmtiles') || low.endsWith('.pmtile'))) {
        const path = this.req.path;
        const source = this.req.source;
        this.$router.replace({ 
            path: '/heatmap', 
            query: { 
                overlay: path,
                source: source
            } 
        });
        return;
    }
    this.checkScanStatus();
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
    
    // Initialize sticky coords
    // console.log("Preview mounted. activePin:", this.activePin);
    if (this.activePin) {
        // console.log("Applying pinned coords on mount:", this.activePin);
        this.editLat = this.activePin.lat;
        this.editLon = this.activePin.lon;
        // Don't update map marker yet as map not init
    }

	this.retrieveSavedLocations();
    await this.fetchMetadata();
    await this.fetchTextContent();
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
    if (this.scanInterval) {
        clearInterval(this.scanInterval);
        this.scanInterval = null;
    }
  },
  methods: {
    updateSearchThumbnailUrl(boxArray) {
      if (!this.req || typeof this.req.thumbnailUrl !== 'string') return;
      if (!Array.isArray(boxArray) || boxArray.length < 4) return;
      
      const newBoxStr = boxArray.join(',');
      let url = this.req.thumbnailUrl;
      
      if (url.includes('&box=')) {
         url = url.replace(/&box=[^&]*/, '&box=' + newBoxStr);
      } else if (url.includes('?box=')) {
         url = url.replace(/\?box=[^&]*/, '?box=' + newBoxStr);
      } else {
         url += (url.includes('?') ? '&' : '?') + 'box=' + newBoxStr;
      }
      
      // Update the known box so it naturally follows
      this.req.box = newBoxStr;
      
      // Cache-bust to force Icon.vue to reload image
      url = url.replace(/&_t=\d+/, ''); 
      url += '&_t=' + Date.now();
      
      this.req.thumbnailUrl = url;
      
      // Reactively mutate the matching item in the parent listing (search results)
      if (this.listing && this.listing.length > 0) {
        // Find the corresponding item in the list
        const listIndex = this.listing.findIndex(item => item.name === this.req.name && item.path === this.req.path);
        if (listIndex !== -1) {
          this.listing[listIndex].box = newBoxStr;
          this.listing[listIndex].thumbnailUrl = url;
        }
      }
    },
    async checkScanStatus() {
        try {
            const { getFaceScanStatus } = await import('@/api/files');
            const status = await getFaceScanStatus();
            this.isScanningFaces = status.isScanning;
            
            if (this.isScanningFaces && !this.scanInterval) {
                this.scanInterval = setInterval(async () => {
                    const s = await getFaceScanStatus();
                    this.isScanningFaces = s.isScanning;
                    if (!this.isScanningFaces) {
                        clearInterval(this.scanInterval);
                        this.scanInterval = null;
                        notify.showSuccess(`Finished scanning faces`);
                        await this.fetchFacesData(); // reload
                        
                        // Smart thumbnail updater: if we are in a face search result context
                        if (this.req && typeof this.req.thumbnailUrl === 'string' && this.req.thumbnailUrl.includes('box=')) {
                            // Try to find the best ML or verified ACDSee face to update the thumbnail with
                            const validFaces = (this.faceRegions || []).filter(f => (f.Source === 'ml' || (f.Source === 'acdsee' && f.Confidence === 1.0)) && f._rawBox);
                            if (validFaces.length > 0) {
                                // Prefer a face that was recognized with a name, else just the first one
                                const namedFace = validFaces.find(f => f.Name && f.Name !== 'Unknown');
                                this.updateSearchThumbnailUrl(namedFace ? namedFace._rawBox : validFaces[0]._rawBox);
                            }
                        }
                    }
                }, 1000); // 1 second polling
            }
        } catch (e) {
            this.isScanningFaces = false;
            if (this.scanInterval) {
                clearInterval(this.scanInterval);
                this.scanInterval = null;
            }
        }
    },
    async fetchTextContent() {
      // console.log('fetchTextContent called, previewType:', this.previewType, 'req.type:', state.req.type);
      
      // Only fetch for text files
      if (this.previewType !== 'text') {
        this.textContent = '';
        return;
      }

      try {
        // console.log('Fetching text content from:', this.raw);
        const response = await fetch(this.raw);
        if (!response.ok) {
          throw new Error('Failed to fetch text content');
        }
        let text = await response.text();
        // console.log('Fetched text, length:', text.length);

        // Pretty-print JSON
        if (state.req.type === 'application/json') {
          try {
            const json = JSON.parse(text);
            text = JSON.stringify(json, null, 2);
          } catch (e) {
            console.warn('Failed to parse/stringify JSON:', e);
          }
        }

        this.textContent = text;
      } catch (error) {
        console.error('Error fetching text content:', error);
        this.textContent = 'Error loading file content';
      }
    },

    async fetchFacesData() {
      this.facesData = [];
      this.selectedFaceIndices = [];
      this.renamingFaceIndex = -1;
      if (this.previewType !== 'image') return;
      
      try {
        const { getApiPath, encodePath } = await import('@/utils/url.js');
        const { fetchURL } = await import('@/api/utils');
        
        const dirPath = encodePath(this.req.path.substring(0, this.req.path.lastIndexOf('/')));
        const url = getApiPath(`/api/raw${dirPath}/faces.json`, { source: this.req.source, _t: Date.now() });
        
        const res = await fetchURL(url);
        const json = await res.json();
        if (json && json[this.req.name]) {
           this.facesData = json[this.req.name];
        }
      } catch (e) {
        // file might not exist or auth failed, silently ignore
      }
    },
    hasMatchingMLFace(acdseeRegion) {
      if (!this.faceRegions || !acdseeRegion._rawBox) return false;
      const b1 = acdseeRegion._rawBox;
      
      return this.faceRegions.some(r => {
        if (r.Source !== 'ml') return false;
        const b2 = r._rawBox;
        if (!b2) return false;
        // Check if raw bounding boxes match exactly
        return b1.length === 4 && b2.length === 4 && 
               b1[0] === b2[0] && b1[1] === b2[1] &&
               b1[2] === b2[2] && b1[3] === b2[3];
      });
    },
    async scanFacesForThisImage() {
      if (this.isScanningFaces) return;
      this.isScanningFaces = true;
      try {
        notify.showSuccess(`Scanning ${this.req.name} for faces...`);
        const { scanFacesFile } = await import('@/api/files');
        await scanFacesFile(this.req.url);
        // Start polling the global status
        this.checkScanStatus();
      } catch (e) {
        notify.showError(`Error scanning faces: ${e.message}`);
        this.isScanningFaces = false;
      }
    },
    async fetchPeopleList() {
      if (this.peopleList && this.peopleList.length > 0) return; // already fetched
      try {
        const { fetchURL } = await import('@/api/utils');
        const source = this.req.source || state.sources?.current || "";
        const res = await fetchURL("/api/facerec/people?source=" + encodeURIComponent(source));
        if (res.ok) {
          const data = await res.json();
          this.peopleList = data || [];
        }
      } catch (e) {
        console.error("Error fetching people list:", e);
      }
    },

    // --- Bulk selection methods ---
    selectAllFaces() {
      this.selectedFaceIndices = this.faceRegions
        .map((r, i) => (r.Source === 'ml' || (r.Source === 'acdsee' && r.Confidence >= 0.85)) ? i : -1)
        .filter(i => i !== -1);
    },
    unselectAllFaces() {
      this.selectedFaceIndices = [];
    },
    async removeSelectedFaces() {
      const faces = this.selectedFaceIndices
        .sort((a, b) => b - a) // Process in reverse order
        .map(i => this.faceRegions[i])
        .filter(r => r && r._rawBox);
      if (!faces.length) return;
      if (!confirm(`Remove ${faces.length} face(s)?`)) return;

      try {
        const { removeFaceBox } = await import('@/api/files');
        for (const face of faces) {
          await removeFaceBox(this.req.url, face.Name || 'Unknown', face._rawBox);
        }
        notify.showSuccess(`Removed ${faces.length} face(s)`);
        this.selectedFaceIndices = [];
        this.fetchFacesData();
        FaceEvents.emit(FaceEvents.FACE_UPDATED, { type: 'remove', path: this.req.path });
      } catch (err) {
        notify.showError(`Error removing faces: ${err.message}`);
      }
    },
    // --- Inline rename methods ---
    startRename(index, region) {
      this.fetchPeopleList();
      this.renamingFaceIndex = index;
      this.renameInput = (region.Name && region.Name !== 'Unknown') ? region.Name : '';
      this.$nextTick(() => {
        const inputs = this.$refs.inlineRenameInput;
        if (inputs) {
          // refs with v-for return an array
          const input = Array.isArray(inputs) ? inputs[0] : inputs;
          if (input) input.focus();
        }
      });
    },
    async submitInlineRename(index, region) {
      const newName = this.renameInput.trim();
      if (!newName) return;

      const oldName = region.Name;
      const box = region._rawBox;
      if (!box) {
        notify.showError("Missing raw box data for rename.");
        return;
      }

      try {
        const { updateFaceBox } = await import('@/api/files');
        await updateFaceBox(this.req.url, oldName, newName, box);
        notify.showSuccess(`Renamed ${region.DisplayName} → ${newName}`);
        this.renamingFaceIndex = -1;
        this.renameInput = '';
        this.updateSearchThumbnailUrl(box);
        this.fetchFacesData();
        FaceEvents.emit(FaceEvents.FACE_UPDATED, { type: 'rename', oldName, newName, path: this.req.path });
      } catch (err) {
        notify.showError(`Error renaming face: ${err.message}`);
      }
    },
    async quickVerifyFace(region) {
       const name = region.Name;
       if (!name || name === 'Unknown') return;
       
       const box = region._rawBox;
       if (!box) {
         notify.showError("Missing raw box data for update.");
         return;
       }

       try {
         const { updateFaceBox } = await import('@/api/files');
         notify.showSuccess(`Verifying ${name}...`);
         await updateFaceBox(this.req.url, name, name, box); // Same name acts as manual verification
         notify.showSuccess(`Verified ${name} and saved to ML DB.`);
         this.updateSearchThumbnailUrl(box);
         this.fetchFacesData(); // Refresh UI
         FaceEvents.emit(FaceEvents.FACE_UPDATED, { type: 'verify', name: name, path: this.req.path });
       } catch (err) {
         notify.showError(`Error verifying face: ${err.message}`);
       }
    },
    hasMatchingMLFace(region) {
       if (!region._rawBox || region._rawBox.length < 4) return false;
       return this.faceRegions.some(f => 
          f.Source === 'ml' && 
          f._rawBox && 
          f._rawBox.length >= 4 &&
          f._rawBox[0] === region._rawBox[0] && 
          f._rawBox[1] === region._rawBox[1]
       );
    },
    async removeFace() {
       const oldName = this.faceContextMenu.region.Name;
       const box = this.faceContextMenu.region._rawBox;
       if (!box) return;

       if (!confirm("Are you sure you want to remove this face box?")) return;

       try {
         const { removeFaceBox } = await import('@/api/files');
         await removeFaceBox(this.req.url, oldName, box);
         notify.showSuccess(`Removed face box for ${oldName || 'Unknown'}`);
         this.closeFaceContextMenu();
         this.fetchFacesData(); // Refresh UI
         FaceEvents.emit(FaceEvents.FACE_UPDATED, { type: 'remove', path: this.req.path });
       } catch (err) {
         notify.showError(`Error removing face: ${err.message}`);
       }
    },
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
            notify.showSuccess('Coordinates Updated');
            
            // Reload metadata to ensure UI is in sync
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
        // BUT respecting the PIN if active!
        if (!this.isPinned) {
            if (this.gpsCoordinates) {
                this.editLat = parseFloat(this.gpsCoordinates.lat.toFixed(6));
                this.editLon = parseFloat(this.gpsCoordinates.lon.toFixed(6));
            } else {
                 this.editLat = 0;
                 this.editLon = 0;
            }
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

        const satellite = L.tileLayer('https://mt1.google.com/vt/lyrs=s&x={x}&y={y}&z={z}',{
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
        
        // Custom Browser Fullscreen Control
        L.Control.BrowserFullscreen = L.Control.extend({
            onAdd: function(map) {
                var container = L.DomUtil.create('div', 'leaflet-bar leaflet-control');
                var button = L.DomUtil.create('a', 'leaflet-control-fullscreen-button', container);
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
        new L.Control.BrowserFullscreen({ position: 'topleft' }).addTo(this.mapInstance);

        // Add Layer Control
        const baseMaps = {
            "Satellite (Hybrid)": hybrid,
            "Satellite (Pure)": satellite,
            "Standard (Streets)": streets,
            "OpenStreetMap": osm,
            "Dark Mode": dark
        };
        L.control.layers(baseMaps, null, { position: 'topleft' }).addTo(this.mapInstance);

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

        // If no permission OR invalid coords, remove marker
        if (!this.canEditCoordinates || (lat === 0 && lon === 0)) {
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
      
      // Update pre-fetching status from backend
      this.isPreFetching = data.isPreFetching || false;

      // Clone to ensure we don't mutate cache directly if we don't want to, 
      // OR mostly we do want to cache the parsed result. 
      // For now, let's just assign.
      this.metadata = data;

      if (!this.metadata.xmp) {
        this.metadata.xmp = {};
      }
      this.parsePhotoshopInstructions();
      this.fetchFacesData();
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
        this.imageDimensions = { 
            width: rect.width, 
            height: rect.height,
            naturalWidth: imgElement.naturalWidth,
            naturalHeight: imgElement.naturalHeight
        };
        this.imageOffset = { top: rect.top, left: rect.left };
        
        // Initialize panzoom here once image is laid out
        this.initPanzoom();
    },
	
    copyJson() {
      if (!this.textContent) return;
      navigator.clipboard.writeText(this.textContent).then(() => {
        this.jsonCopied = true;
        setTimeout(() => { this.jsonCopied = false; }, 2000);
      });
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
    
    getFaceBoxColor(region, opacity = 0.2) {
      if (region.Source === 'acdsee') {
         if (region.Confidence === 1.0) {
            return `rgba(66, 185, 131, ${opacity})`; // Green for Verified ACDSee
         } else {
            return `rgba(156, 39, 176, ${opacity})`; // Purple for Unverified ACDSee
         }
      }
      
      if (region.Source === 'ml') {
         if (region.Confidence === 1.0) {
            return `rgba(33, 150, 243, ${opacity})`; // Blue for Verified ML Face (Renamed)
         } else if (region.Confidence > 0.90) {
            return `rgba(255, 255, 0, ${opacity})`; // Yellow for High-Confidence ML
         } else {
            return `rgba(255, 152, 0, ${opacity})`; // Orange for Med/Low Confidence ML
         }
      }
      
      // Fallback
      return `rgba(255, 0, 0, ${opacity})`;
    },

    getFaceBoxStyle(region) {
      const area = region.Area || region.area || region.ALGArea || region.DLYArea || region;
      if (!area || 
          (area.X === undefined && area.x === undefined && area['stArea:x'] === undefined) || 
          (area.Y === undefined && area.y === undefined && area['stArea:y'] === undefined)) {
        return { display: "none" };
      }

      let X = parseFloat(area.X ?? area.x ?? area['stArea:x'] ?? 0);
      let Y = parseFloat(area.Y ?? area.y ?? area['stArea:y'] ?? 0);
      let W = parseFloat(area.W ?? area.w ?? area['stArea:w'] ?? 0);
      let H = parseFloat(area.H ?? area.h ?? area['stArea:h'] ?? 0);
      
      if (W === 0 || H === 0) return { display: "none" };
      
      // AUTO-NORMALIZE: If values are > 1, they are likely in pixels, not 0-1 range
      if ((X > 1 || Y > 1) && this.imageDimensions.naturalWidth > 0) {
          X = X / this.imageDimensions.naturalWidth;
          Y = Y / this.imageDimensions.naturalHeight;
          W = W / this.imageDimensions.naturalWidth;
          H = H / this.imageDimensions.naturalHeight;
      }

      const left = (X - W / 2) * 100;
      const top = (Y - H / 2) * 100;
      const width = W * 100;
      const height = H * 100;

      const backgroundColor = this.getFaceBoxColor(region, 0.4); // slightly more opaque
      const borderColor = this.getFaceBoxColor(region, 1.0);

      return {
          left: `${left}%`,
          top: `${top}%`,
          width: `${width}%`,
          height: `${height}%`,
          border: `3px solid ${borderColor}`,
          outline: `2px solid #000`, // High-contrast border outline
          boxSizing: "border-box",
          background: backgroundColor,
          boxShadow: "0 0 6px rgba(0,0,0,0.8)",
          position: "absolute",
          zIndex: 999, // Way above image
          pointerEvents: "auto",
          cursor: "context-menu"
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
        await filesApi.updateXMPInstructions(state.req.source, metadataPath, this.photoshopInstructions);
        
        // Update original to prevent re-saving
        this.originalInstructions = this.photoshopInstructions;
        notify.showSuccess("Image Notes saved");
      } catch (err) {
        console.error("Failed to save Photoshop instructions:", err);
        notify.showError("Failed to save Image Notes");
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



    
    
    reapplyPinnedLocation() {
        if (this.activePin) {
             // Force inputs to match pinned location
             this.editLat = this.activePin.lat;
             this.editLon = this.activePin.lon;
        }
    },

    async togglePin() {
        if (!state.user) return;
        
        let newPin = null;
        if (!this.activePin) {
            // Setup new pin from current edit coords
            let latToPin = this.editLat;
            let lonToPin = this.editLon;
            
            // Safety: If inputs are 0,0 but we have valid GPS, prefer GPS. 
            // This prevents accidental pinning of "empty" state.
            if (latToPin === 0 && lonToPin === 0 && this.gpsCoordinates) {
                latToPin = this.gpsCoordinates.lat;
                lonToPin = this.gpsCoordinates.lon;
            }

            newPin = {
                name: "Pinned Location", // Placeholder name
                lat: latToPin,
                lon: lonToPin
            };
        } else {
            // Turn off (set to null)
            newPin = null;
        }
        
        // Update local polyfill
        this.localPinValue = newPin;
        
        // Explicitly handle localStorage here because we disabled auto-wiping in mutations.js
        if (newPin) {
             localStorage.setItem("pinnedLocation", JSON.stringify(newPin));
        } else {
             localStorage.removeItem("pinnedLocation");
        }

        // Update user profile
        try {
             // We construct a partial user update. The backend might need explicit null handling? 
             // Usually Go JSON unmarshalling handles null ptrs fine if we send null.
             const userUpdate = { ...state.user, pinnedLocation: newPin };
             
             // Optimistic update
             mutations.updateCurrentUser({ pinnedLocation: newPin });
             
             await usersApi.update(userUpdate, ['pinnedLocation']);
        } catch (e) {
             console.error("Failed to toggle pin", e);
             notify.showError("Failed to update pinned location");
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
      if (state.showSearchSidebar && state.searchResults.length > 0) {
        this.listing = state.searchResults;
      } else if (!this.listing) {
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
          if (getTypeInfo(composedListing.type).simpleType == "image") {
            composedListing.path = directoryPath + "/" + composedListing.name;
            this.previousLink = composedListing.url;
            this.previousRaw = this.prefetchUrl(composedListing);
            // Prefetch metadata for previous image
            this.getMetadata(state.req.source, composedListing.path);
            break;
          }
        }
        for (let j = i + 1; j < this.listing.length; j++) {
          let composedListing = this.listing[j];
          if (getTypeInfo(composedListing.type).simpleType == "image") {
            composedListing.path = directoryPath + "/" + composedListing.name;
            this.nextLink = composedListing.url;
            this.nextRaw = this.prefetchUrl(composedListing);
            // Prefetch metadata for next image
            this.getMetadata(state.req.source, composedListing.path);
            break;
          }
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

  .face-context-menu {
    background: var(--surfacePrimary, #fff);
    border-radius: 4px;
    box-shadow: 0 4px 12px rgba(0,0,0,0.3);
    padding: 1rem;
    min-width: 250px;
    position: fixed; /* Make it float */
    z-index: 2000; /* Ensure it's on top */
    
    .face-menu-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 0.5rem;
      border-bottom: 1px solid var(--divider, #eee);
      padding-bottom: 0.5rem;
    }

    .face-menu-body {
      input { margin-bottom: 0.5rem; }
      .button-row { display: flex; gap: 0.5rem; justify-content: flex-end; }
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

.spin {
  animation: spin 2s linear infinite;
}

@keyframes spin {
  100% {
    transform: rotate(360deg);
  }
}

.prefetching-indicator {
  display: flex;
  align-items: center;
  justify-content: center;
  margin-left: 10px;
}

/* -- JSON Pretty Viewer ----------------------------------------- */
.json-preview {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: #1a1b2e;
  color: #cdd6f4;
  font-family: 'Fira Code', 'Cascadia Code', monospace;
}
.json-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 14px;
  background: rgba(255,255,255,0.05);
  border-bottom: 1px solid rgba(255,255,255,0.08);
  flex-shrink: 0;
}
.json-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.8rem;
  font-weight: 600;
  color: #7c8cf8;
  text-transform: uppercase;
  letter-spacing: 0.06em;
}
.json-label .material-icons { font-size: 16px; }
.json-copy-btn {
  background: none;
  border: 1px solid rgba(255,255,255,0.12);
  border-radius: 6px;
  color: #aaa;
  cursor: pointer;
  padding: 3px 8px;
  display: flex;
  align-items: center;
  transition: background 0.15s, color 0.15s;
}
.json-copy-btn:hover { background: rgba(255,255,255,0.1); color: #fff; }
.json-copy-btn .material-icons { font-size: 16px; }
.json-body {
  margin: 0;
  padding: 16px 20px;
  overflow: auto;
  flex: 1;
  font-size: 0.82rem;
  line-height: 1.6;
  white-space: pre-wrap !important;
  word-wrap: break-word;
}
.json-body code {
  white-space: pre-wrap !important;
  font-family: inherit;
}
.json-key  { color: #89dceb; }
.json-str  { color: #a6e3a1; }
.json-num  { color: #fab387; }
.json-bool { color: #cba6f7; }
.json-null { color: #6c7086; }
</style>