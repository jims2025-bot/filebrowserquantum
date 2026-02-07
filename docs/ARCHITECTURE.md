# FileBrowser Quantum - Architecture Documentation

## Table of Contents
1. [Heatmap Cluster ID Architecture](#heatmap-cluster-id-architecture)
2. [Backend Architecture](#backend-architecture)
3. [Frontend Architecture](#frontend-architecture)
4. [File Viewing](#file-viewing)
5. [Data Flow](#data-flow)
6. [Image Editing Features](#image-editing-features)
    - [Map Tab Details](#2-location-editing-map-tab)
    - [Rotation](#3-image-rotation-temporary)
7. [Server Permissions](#collection-folder-permissions)
8. [Administrative Functions](#administrative-functions)
9. [Documentation Notes](#documentation-notes)

---

## Heatmap Cluster ID Architecture

[↑ Back to Top](#table-of-contents)

### Overview
The heatmap system creates geographic clusters of images based on GPS coordinates and percolates them up through the folder hierarchy.

### Cluster ID Generation

**Location**: `backend/heatmap/scanner.go`

```go
genID := func() string {
    return fmt.Sprintf("%d-%d", time.Now().UnixNano(), len(points))
}
```

- **Random IDs**: Each cluster gets a unique random ID
- **Per Geographic Location**: Multiple clusters per folder based on GPS proximity
- **Cluster Radius**: 0.0005 degrees (~50 meters)

### Folder Hierarchy & Percolation

```
/photos/vacation/
├── heatmap.json (v3)
│   └── clusters: [
│       { id: "1738...-0", lat: 40.7, lon: -74.0, count: 15, path: "day1/IMG_001.jpg" },
│       { id: "1738...-1", lat: 40.8, lon: -74.1, count: 13, path: "day2/IMG_050.jpg" }
│   ]
├── day1/
│   └── heatmap.json
│       └── clusters: [
│           { id: "1738...-0", points: [15 images] }
│       ]
└── day2/
    └── heatmap.json
        └── clusters: [
            { id: "1738...-1", points: [13 images] }
        ]
```

### Percolation Process

**Location**: `backend/heatmap/manager.go`

1. **Leaf Level** (`GetLocalClusters`):
   - Scans folder for images
   - Extracts GPS coordinates
   - Creates clusters (merges nearby points)
   - Assigns random IDs
   - Saves to `heatmap.json`

2. **Aggregation** (`AggregateLevel`):
   - Reads child `heatmap.json` files
   - Strips `Points` array (keeps metadata)
   - **Preserves cluster ID from leaf level**
   - **Keeps preview image URL** (`PreviewID` and `Path`)
   - Merges with local clusters
   - Saves aggregated `heatmap.json`

3. **Percolate Up** (`PercolateUp`):
   - Recursively updates parent folders
   - **Clusters and their IDs percolate all the way up**
   - **Sample preview image URL maintained** throughout hierarchy
   - Continues to source root

### Inspection Flow

**Frontend → Backend**:
```
User clicks cluster badge "28 items"
  ↓
Frontend sends: /api/heatmap/inspect?lat=40.7&lon=-74.0&cluster_id=1738...-0
  ↓
Backend (heatmap_inspect.go):
  1. Loads folder heatmap.json
  2. Finds cluster by ID: lc.ID == c.ID
  3. Returns lc.Points (all images in that geographic cluster)
  ↓
Frontend displays 28 images in inspection panel
```

### Version Management

**Location**: `backend/heatmap/types.go`

```go
const HeatmapVersion = 4
```

- **Version 4**: Introduced Fix for missing IDs in frontend inspection.
- Incrementing this constant forces a complete re-scan of all heatmaps on server startup.
- Version check acts as a cache buster.

### Manual Regeneration

To allow users to fix missing or corrupted data without a full system re-scan:

**Endpoint**: `POST /api/heatmap/regenerate?path=/path/to/folder`

**Triggers**:
1.  **UI Button**: A refresh icon in the Heatmap Side Panel header allows regenerating the currently inspected folder.
2.  **Logic**: Bypasses the 7-day interval check and forces a rebuild of the `heatmap.json` for the target folder.


---

## Backend Architecture

[↑ Back to Top](#table-of-contents)

### Directory Structure

```
backend/
├── adapters/          # External integrations
│   └── fs/           # Filesystem operations
├── auth/             # Authentication & authorization
├── cmd/              # CLI commands
├── common/           # Shared utilities
├── database/         # Data persistence
│   ├── storage/      # Storage interface
│   ├── users/        # User management
│   └── settings/     # Settings management
├── events/           # Event system
├── heatmap/          # Heatmap generation
├── http/             # HTTP handlers & API
├── indexing/         # File indexing system
├── preview/          # Image/video preview generation
└── swagger/          # API documentation
```

### Core Components

#### 1. Database Layer
**Location**: `backend/database/`

- **Storage**: Bolt DB wrapper
- **Users**: User accounts, permissions, scopes
- **Settings**: Application configuration

#### 2. Indexing System
**Location**: `backend/indexing/`

- **Purpose**: Fast file metadata access
- **Index Structure**: In-memory tree of folders/files
- **Metadata**: Name, size, modified time, type
- **Sources**: Multiple root directories (e.g., PHOTOS, VIDEOS)

#### 3. Authentication
**Location**: `backend/auth/`

- JWT-based authentication
- User scopes (folder access control)
- Permission system (admin, create, rename, etc.)

#### 4. HTTP Layer
**Location**: `backend/http/`

- RESTful API endpoints
- File operations (upload, download, delete)
- Heatmap API (`heatmap_inspect.go`, `heatmap_tiles.go`)
- Preview API (thumbnails, EXIF)

#### 5. Preview System
**Location**: `backend/preview/`

- Thumbnail generation (small: 256x256, thumb: 64x64)
- EXIF metadata extraction
- Video frame extraction
- Caching

#### 6. Heatmap System
**Location**: `backend/heatmap/`

**Files**:
- `scanner.go`: GPS extraction, clustering
- `manager.go`: Recursive scanning, aggregation, percolation
- `tiles.go`: Tile coordinate conversion
- `types.go`: Data structures, version
- `monitor.go`: Active scan tracking

**Data Flow**:
```
StartJob (weekly cycle)
  ↓
ScanRecursive (for each user scope)
  ↓
GetLocalClusters (leaf folders)
  ↓
clusterPoints (merge nearby GPS)
  ↓
AggregateLevel (combine local + children)
  ↓
PercolateUp (update parents)
```

**Scan Interval Configuration**:

**Constant**: `ScanIntervalDays = 7` (in `backend/heatmap/manager.go`)

- **Full Scan**: Runs every 7 days (weekly)
- **Skip Logic**: Folders with heatmap.json < 7 days old are skipped during automatic scans
- **Manual Scans**: Override the interval, always regenerate, and percolate up immediately
- **Rationale**: Full scans take 6+ hours for large photo libraries, weekly is sufficient

**To Change Interval**: Modify `ScanIntervalDays` constant in `backend/heatmap/manager.go`

---

## Frontend Architecture

[↑ Back to Top](#table-of-contents)

### Directory Structure

```
frontend/src/
├── api/              # API client functions
├── assets/           # Static assets (images, icons)
├── components/       # Reusable Vue components
├── css/              # Global styles
├── i18n/             # Internationalization
├── notify/           # Notification system
├── router/           # Vue Router configuration
├── store/            # Vuex state management
├── utils/            # Helper functions
└── views/            # Page components
    ├── Heatmap.vue   # Heatmap view
    ├── Files.vue     # File browser
    └── Preview.vue   # Image/video preview
```

### Core Components

#### 1. State Management
**Location**: `frontend/src/store/`

- **state.ts**: Global application state
- **mutations.ts**: State mutations
- **getters.ts**: Computed state

**Key State**:
```typescript
{
  user: User,
  jwt: string,
  selected: FileInfo[],
  clipboard: FileInfo[],
  loading: boolean
}
```

#### 2. API Layer
**Location**: `frontend/src/api/`

- **utils.ts**: `fetchJSON`, `fetchURL` helpers
- **files.ts**: File operations
- **users.ts**: User management
- **preview.ts**: Preview/thumbnail requests

#### 3. Router
**Location**: `frontend/src/router/`

**Routes**:
```
/login           → Login.vue
/files/:source/* → Files.vue
/heatmap         → Heatmap.vue
/preview/*       → Preview.vue
/settings        → Settings.vue
```

#### 4. Heatmap View
**Location**: `frontend/src/views/Heatmap.vue`

**Features**:
- Leaflet map integration
- Vector tile rendering
- Cluster markers with count badges
- Inspection panel (side drawer)
- Folder grouping & navigation
- Quick view modal with file details

**Key Functions**:
- `loadHeatmapData()`: Fetch global heatmap
- `inspectLocation()`: Inspect cluster by ID
- `generateMarkerIcon()`: Create thumbnail markers

#### 5. Inspection Panel
**Location**: `frontend/src/views/Heatmap.vue`

**Purpose**: Side drawer that displays images from a clicked cluster.

**Performance Optimization (EXIF Thumbnails)**:
To ensure instant loading even for clusters with hundreds of images, the inspection panel specifically requests thumbnails with `size=small`.
- **Backend Behavior**: The backend detects `size=small` and prioritizes extracting the **embedded EXIF thumbnail** from the JPEG file (approx. 5-10ms) instead of decoding and resizing the full multi-megabyte image (approx. 100-500ms).
- **Result**: Drastically reduced CPU usage and memory footprint during inspection.

**Two-Level Navigation**:

1. **Folder List / Cluster Grouped View** (Tablets/Touch):
   - **Grouped Layout**: Thumbnails are visually grouped by their Cluster ID.
   - **Colored Sidebars**: Each group has a unique colored bar (hash of Cluster ID) for visual distinction.
    - **Zoom Target**: The sidebar contains a large "Target" icon (`my_location`). Tapping it flies the map to that cluster's location (Zoom 16) and triggers a visual "flash".
    - **Sticky Header**: The folder name and navigation controls are sticky at the top, ensuring navigation is available even when scrolling long lists.
    - **Clickable Folder Name**: Tapping the truncated folder name in the header navigates directly to that folder in the file browser.
    - **Thumbnail Sizing**: Thumbnails are sized to fit two per row (50% width) for optimal visibility on tablets.

2.  **Item Grid View**:
    - **Simplified Thumbnails**:
        - No overlay buttons (View/Folder) to obscure the image.
        - Filename displayed below the image (truncated).
    - **Lazy Loading**: Efficiently loads images as the user scrolls.
    - **Pagination**: "Load More" button for large lists.
    - **Interaction**: Clicking an image opens the **Quick View Modal**.

**Data Flow**:
```javascript
inspectLocation(coords) {
  // 1. Build API URL with cluster_id
  url = `/api/heatmap/inspect?lat=${lat}&lon=${lon}&cluster_id=${clusterID}`
  
  // 2. Fetch from backend
  const items = await fetch(url)
  
  // 3. Group by parent folder
  sidePanelData = items.map(item => ({
    name: item.name,
    path: item.path,
    source: item.source,
    parentPath: item.path.substring(0, item.path.lastIndexOf('/')),
    thumbUrl: getPreviewUrl(item.path, item.source, 'small')
  }))
  
  // 4. Create folder groups
  formattedSidePanelData = groupByParentPath(sidePanelData)
  
  // 5. Auto-open if single folder
  if (formattedSidePanelData.length === 1) {
    currentInspectionFolder = formattedSidePanelData[0]
  }
}
```

**Pagination**:
- Initial load: 50 items
- "Load More": +50 items per click
- Prevents browser freeze with large clusters

**Quick View Modal**:
- Full-size preview (not thumbnail)
- Click outside to close
- "Open Folder" button navigates to file browser

**Styling**:
- Side panel: 400px width, slide-in animation
- Thumbnails: 120x120 grid items
- Hover effects on thumbnails
- Responsive (mobile: full width)

#### 6. Dynamic Leader Lines
**Location**: `frontend/src/views/Heatmap.vue` (Lines 298-380)

**Purpose**: Visually connects the inspection panel content to its geographic origin on the map.

**Features**:
- **Interactive**: Appears when hovering over the color-coded cluster bar in the side panel.
- **Dynamic**: Updates in real-time during map panning, zooming, and resizing.
- **Visuals**: Dashed red line with animated start point, arrowhead endpoint, and glow filter for visibility.

**Implementation Details**:
1. **SVG Overlay**: Uses a full-screen `<svg>` element with `pointer-events: none` to avoid blocking map interactions.
2. **Coordinate Transformation**:
   - **Challenge**: The SVG overlay is positioned absolute at `(0,0)` of the viewport, but the Leaflet map container is often offset (e.g., by the global header).
   - **Solution**: The line calculation transforms coordinates from "Map Container Space" to "Screen Space" and then to "SVG Space":
     ```javascript
     point = map.latLngToContainerPoint(target)
     screenY = point.y + mapRect.top
     svgY = screenY - svgRect.top
     ```
3. **Centroid Targeting**:
   - **Problem**: A cluster connects items that may be far apart. Pointing to the "first item" often results in the line pointing to an arbitrary edge location.
   - **Solution**: The system calculates the **geometric centroid** (average lat/lon) of all items in the displayed cluster group.
   - **Fallback**: If `lat/lon` are missing on the item wrapper, it intelligently looks for nested `item.exif.latitude` data to ensure accuracy.

---



### Quick View Modal

The Quick View modal provides a high-res preview without leaving the map context.

- **Header Bar**:
  - Displays **Path** (truncated).
  - **Go to Image Button**: Navigates to the full file preview page (allows map editing/notes).
  - **Open Folder Button**: Navigates to the file browser folder.
  - **Close Button**: Large touch target for easy closing.
- **Image**:
  - Fetched with `size=large`.
  - Centered and scaled to fit the modal.
- **Aesthetics**:
  - Soft semi-transparent background (`rgba(44, 62, 80, 0.95)`).
  - Wide button spacing for touch usability ("fat finger" protection).

## File Viewing

[↑ Back to Top](#table-of-contents)

### .GeoJson Map Overlays
See [GeoJSON Visualization](#geojson-visualization) for detailed architecture.

### GeoJSON Visualization

#### Overview
The system supports rendering standard GeoJSON files as map overlays when browsing folders in the Heatmap view.

#### Discovery & Rendering
1. **Scanning**: When entering a folder on the map, the frontend filters the file list for `.geojson` extensions.
2. **Overlay**: These files are fetched and rendered using Leaflet's `L.geoJSON` layer.
3. **Navigation**: A dropdown menu allows toggling between available GeoJSON overlays in the current folder.

#### Styling (Sidecar Files)
To allow custom styling without modifying the GeoJSON data itself, the system uses a **Sidecar Style File**.

- **Filename**: `<original_name>.geojson.style.json`
- **Format**: JSON object containing Leaflet path options (color, weight, opacity, etc.) based on feature properties.
- **Example**:
  ```json
  {
    "default": {
        "color": "#3388ff",
        "weight": 3,
        "opacity": 1.0,
        "fillOpacity": 0.2
    },
    "rules": [
        {
            "if": { "property": "risk_level", "value": "high" },
            "style": { "color": "#ff0000", "weight": 5 }
        },
        {
            "if": { "property": "type", "value": "boundary" },
            "style": { "color": "#000000", "dashArray": "5, 5" }
        },
        {
            "if": { "property": "Name", "value": "Geneva to Zurich" },
            "style": { "color": "#002fff", "weight": 3, "opacity": 0.9 }
        }
    ]
  }
  ```

---

## Image Editing Features

[↑ Back to Top](#table-of-contents)

### Overview
The Preview component (`frontend/src/views/files/Preview.vue`) provides two main editing capabilities:
1. **Photoshop Instructions** (Image Notes)
2. **EXIF Location Data** (GPS Coordinates)

### 1. Photoshop Instructions Editor

**Label**: "Image Notes" / "IPTC Tab → Edit Notes"

**Location**: 
- Split pane view (lines 106-123)
- IPTC tab button (lines 209-216)

**Features**:
- Persistent split-pane editor (33% height on desktop)
- Textarea for editing notes
- Save/Close buttons
- Permission-based editing (`canEditInstructions`)

**Data Flow**:
```javascript
// Load
metadata.iptc → photoshopInstructions

// Edit
User types in textarea → photoshopInstructions (v-model)

// Save
saveInstructions() → PUT /api/xmp/instructions
  → Backend updates XMP:photoshop:Instructions tag
  → Refreshes metadata
```

**Backend**: `backend/http/xmp.go` - Updates XMP metadata

---

### 2. Location Editing (Map Tab)

**Top-Level Label**: "Map Tab"

The Map tab has **three states** and **two sub-tabs** when editing:

#### State 1: View Mode (Read-Only)
**Label**: "Map Tab → View Mode"

**When**: GPS coordinates exist, not editing

**Features**:
- Google Maps iframe (embedded, 400px min height)
- Compact toolbar showing:
  - Coordinates display (lat, lon to 5 decimals)
  - Matched location badge (if coordinates match saved location)
  - Copy button
  - "Open in Google Maps" link
  - "Add to Saved Locations" button
  - Edit button (if `canEditCoordinates`)

**Location**: Lines 387-456

---

#### State 2: Edit Mode
**Label**: "Map Tab → Edit Mode"

**When**: User clicks "Edit" or "Add Location"

**Features**: Two sub-tabs for different editing methods

##### Sub-Tab 1: "Map Search"
**Label**: "Map Tab → Edit Mode → Map Search Sub-Tab"

**Location**: Lines 358-372

**Components**:
1. **Coordinate Inputs** (top section):
   - Lat/Lon number inputs
   - Matched location badge (blue, shows if coordinates match saved location)
   
2. **Action Buttons**:
   - "Save to Image" (primary blue when unsaved changes)
   - "Cancel"
   - Bookmark add button (save to My Locations)
   - Clear location button (red, removes GPS data)

3. **Search Box**:
   - Text input with search button
   - Searches via Nominatim API

4. **Interactive Map** (Leaflet):
   - 300px fixed height
   - Click to set coordinates
   - Draggable marker
   - Map status text below

**Data Flow**:
```javascript
// Search
searchLocation() → Nominatim API
  → Updates map center
  → Sets marker

// Click map
map.on('click') → Updates editLat, editLon
  → Checks against savedLocations
  → Shows matched badge if found

// Save
saveCoordinates() → PUT /api/exif
  → Updates GPSLatitude, GPSLongitude
  → Refreshes metadata
```

##### Sub-Tab 2: "My Locations"
**Label**: "Map Tab → Edit Mode → My Locations Sub-Tab"

**Location**: Lines 374-384

**Features**:
- List of saved locations from user profile
- Each location shows:
  - Name (bold, blue, clickable)
  - Delete button (red trash icon)
- Click location to load coordinates into edit fields
- Scrollable list

**Data Flow**:
```javascript
// Load
mounted() → Fetch user profile
  → savedLocations = user.savedLocations

// Select
loadSavedLocationIdx(idx) → Sets editLat, editLon
  → Updates map marker

// Delete
deleteSavedLocationIdx(idx) → PUT /api/users/:id
  → Removes from user.savedLocations
  → Refreshes list

// Add (from Map Search tab)
saveLocationToProfile() → Prompts for name
  → PUT /api/users/:id
  → Adds to user.savedLocations
```

---

#### State 3: Empty State
**Label**: "Map Tab → Empty State"

**When**: No GPS coordinates exist

**Features**:
- Centered message: "No location data"
- "Add Location" button (if `canEditCoordinates`)
- Clicking button enters Edit Mode

**Location**: Lines 459-471

#### Persistent Pinning Architecture
**Problem**: When navigating between folders, the VueX state (user profile) is reloaded, causing a transient "unpinned" state. If the Map component initializes during this gap (race condition), it would mistakenly overwrite the pinned location with the new image's embedded GPS.

**Solution**: A "Bulletproof" initialization strategy in `Preview.vue`.

1. **Synchronous Data Initialization**:
   - The `localStorage` key `pinnedLocation` is read **directly inside the `data()` function**.
   - This ensures `editLat` and `editLon` are populated with the pinned values *before* the component is even created and before any watchers (e.g., `gpsCoordinates`) can fire.
   - Code:
     ```javascript
     data() {
         return {
             localPinValue: (() => { try { return JSON.parse(localStorage.getItem(...)) } ... })(),
             editLat: (() => { ... })(), // Initialized from LS immediately
         }
     }
     ```

2. **Map Initialization Guard**:
   - The `initMap()` function is explicitly prevented from overwriting input fields if a pin is active.
   - Logic: `if (!this.isPinned) { overwrite_inputs_with_gps() }`

3. **Polyfill Strategy**:
   - `localPinValue` acts as a synchronous bridge while the async VueX `state.user` loads.
   - The `activePin` computed property merges `state.user.pinnedLocation` (authoritative) and `localPinValue` (fast fallback).

---

### Map Tab Sub-Tab Navigation

**Tab Buttons** (lines 352-355):
```html
<button>Map Search</button>  <!-- editTab = 'location' -->
<button>My Locations</button> <!-- editTab = 'locations' -->
```

**State Variable**: `editTab` ('location' | 'locations')

---

### Backend APIs

#### Photoshop Instructions
- **Endpoint**: `PUT /api/xmp/instructions`
- **Handler**: `backend/http/xmp.go`
- **Payload**: `{ instructions: string }`
- **Updates**: XMP `photoshop:Instructions` tag

#### EXIF Location
- **Endpoint**: `PUT /api/exif`
- **Handler**: `backend/http/exif.go`
- **Payload**: `{ lat: number, lon: number }`
- **Updates**: EXIF `GPSLatitude`, `GPSLongitude`, `GPSLatitudeRef`, `GPSLongitudeRef`
- **Delete**: `DELETE /api/resources?action=exif&path=...&source=...` -> Removes GPS metadata

#### User Locations
- **Endpoint**: `PUT /api/users/:id`
- **Handler**: `backend/http/users.go`
- **Payload**: `{ savedLocations: [{ name, lat, lon }] }`
- **Updates**: User profile `savedLocations` array

---

### Component Labels Summary

For easy reference when requesting changes:

| Component | Label |
|-----------|-------|
| Photoshop editor split pane | "Image Notes Editor" |
| IPTC tab edit button | "IPTC Tab → Edit Notes Button" |
| Map tab (general) | "Map Tab" |
| View mode (read-only map) | "Map Tab → View Mode" |
| Edit mode | "Map Tab → Edit Mode" |
| Map search sub-tab | "Map Tab → Edit Mode → Map Search Sub-Tab" |
| My locations sub-tab | "Map Tab → Edit Mode → My Locations Sub-Tab" |
| Leaflet interactive map | "Map Search → Leaflet Map" |
| Google Maps iframe | "View Mode → Google Maps Iframe" |
| Coordinate inputs | "Map Search → Coordinate Inputs" |
| Search box | "Map Search → Search Box" |
| Saved locations list | "My Locations → Saved Locations List" |

---

### 3. Image Rotation (Temporary)

**Top-Level Label**: "Image Preview Rotation"

**Purpose**: Allows users to temporarily rotate images for proper viewing without permanently modifying the file.

**Features**:
- **Button**: "Rotate" button in the global header (next to Download) when viewing an image.
- **State**: 
  - Stored in Vuex (`state.previewRotation`)
  - Resets to 0° on file navigation (new file loaded).
  - Applies to 90° increments (0, 90, 180, 270).
- **Visuals**:
  - Rotates the image element via CSS `transform: rotate(Ndeg)`.
  - **Crucial**: Also rotates the Face Box Overlay (`.face-overlay`) simultaneously so face regions match the rotated image.

**Data Flow**:
```javascript
// User Clicks Rotate
Header (Default.vue) → mutations.rotatePreview() 
  → updates state.previewRotation

// Preview Component
Preview.vue (watch)
  → Computed property 'rotation' reads store
  → Applies style to wrapper div (image + overlay)

// Navigation
Preview.vue (watch raw) → mutations.resetPreviewRotation()
```

---

## Data Flow

[↑ Back to Top](#table-of-contents)

### File Upload Flow

```
User selects files
  ↓
Frontend: POST /api/resources/:path
  ↓
Backend: http/resources.go
  ↓
Save to filesystem
  ↓
Indexing: RefreshFileInfo()
  ↓
Update in-memory index
```

### Heatmap Generation Flow

```
Backend starts (or 24hr timer)
  ↓
heatmap.StartJob()
  ↓
For each user scope:
  ScanRecursive(source, path)
    ↓
    For each subfolder (parallel):
      ScanRecursive(subfolder)
    ↓
    AggregateLevel(current folder):
      - GetLocalClusters() → scan images, extract GPS
      - Read child heatmap.json files
      - Merge local + children
      - Save heatmap.json
    ↓
  PercolateUp(path):
    - Update parent folder
    - Repeat to root
```

### Heatmap Inspection Flow

```
User clicks cluster on map
  ↓
Frontend: inspectLocation(coords)
  ↓
GET /api/heatmap/inspect?lat=X&lon=Y&cluster_id=ID&zoom=Z
  ↓
Backend: heatmap_inspect.go
  ↓
1. Load cached/fresh heatmap for folder
2. Find cluster by ID
3. If cluster has Points: return them
4. If cluster is aggregated:
   - Load leaf heatmap.json
   - Find matching cluster by ID
   - Return Points
  ↓
Frontend: Display in inspection panel
  - Group by parent folder
  - Show thumbnails
  - Quick view on click
```

### Preview Generation Flow

```
Frontend requests thumbnail
  ↓
GET /api/preview?path=X&source=Y&size=small
  ↓
Backend: preview/preview.go
  ↓
Check cache
  ↓
If not cached:
  - Load image
  - Resize to 256x256
  - Save to cache
  ↓
Return image
```

### Metadata Pre-fetching Flow

**Goal**: Ensure instant metadata availability (EXIF, IPTC, XMP) when navigating between images.

```
User opens image (Preview.vue)
  ↓
updatePreview() calculates Previous/Next links
  ↓
For Previous AND Next image:
  1. Calculate Preview URL (prefetchUrl)
     → Browser caches the thumbnail request
  2. Call getMetadata(path)
     → GET /api/metadata?path=...
     → Backend extracts EXIF/IPTC/XMP
     → Frontend stores in metadataCache[path]
  ↓
User clicks "Next"
  ↓
Preview.vue checks metadataCache[newPath]
  ↓
immediate HIT (no network delay)
```

---

## Key Design Decisions

### 1. Random Cluster IDs
**Why**: Each geographic cluster needs a unique identifier that percolates up. Deterministic IDs (hash of path) don't work because multiple images at different GPS locations in the same folder would get the same ID.

**Trade-off**: IDs change on regeneration, but this is acceptable because:
- Heatmaps are regenerated infrequently (24hr cycle or manual)
- Frontend always fetches fresh data on load
- Inspection uses current cluster_id from loaded data

### 2. Cluster Percolation
**Why**: Top-level map needs to show aggregated counts without loading all leaf data.

**How**: Child clusters are copied to parent (without Points array), maintaining ID and metadata. **This percolation continues recursively up through grandparent, great-grandparent, etc., all the way to the top of the scope source**, ensuring the root heatmap contains all descendant clusters.

### 3. In-Memory Indexing
**Why**: Fast file metadata access without hitting filesystem.

**Trade-off**: Memory usage, but enables instant file browser navigation.

### 4. Caching Strategy
- **Heatmap**: 5-minute TTL (inspection requests)
- **Preview**: Persistent disk cache
- **Index**: In-memory, refreshed on file changes

---

## Version History

### Heatmap Version 3 (Current)
- Random cluster IDs
- Geographic clustering
- ID-based inspection

### Heatmap Version 2 (Reverted)
- Attempted deterministic IDs (SHA1 of path)
- Failed: Multiple clusters per folder need unique IDs

### Heatmap Version 1 (Original)
- Initial implementation
- Random IDs

---

## Heatmap Architecture: Selection & Inspection

This section details the architecture for selecting, interacting with, and inspecting heatmap clusters, specifically focusing on the drill-down mechanism for aggregated clusters.

### 1. Overview

The Heatmap system allows users to visualize large collections of geolocated images. To maintain performance, the backend aggregates individual images into "Clusters" and "Tiles".
- **Zoom < 15**: Images are aggregated into Clusters (counts only, no point data).
- **Zoom >= 15**: Individual points (Files) or small sub-clusters are sent.

**The Challenge**: When a user selects an aggregated cluster (which has no image data inside it), the system must "Drill Down" to fetch the actual image list from disk/backend.

### 2. Box Selection & Drill-Down Flow

When a user draws a box on the map to select multiple items, the following flow occurs:

#### A. Frontend: Box Capture (`Heatmap.vue`)
1.  **User Interaction**: User toggles "Select Area" mode (or uses Shift-Drag) and draws a box on the map.
2.  **Selection Target**:
    - The selection box targets **Leaflet Marker Objects** (`L.Marker`).
    - **Important**: This includes **Cluster Markers** (the aggregated circles with counts) as well as **Fan Markers** (individual photo points).
    - The code iterates over `markers.eachLayer(layer)` to find every object within the bounds.
3.  **Data Extraction**:
    - Each `L.Marker` object contains options attached during creation:
        - `clusterID`: Uniquely identifies the cluster (or single item). **CRITICAL**: This ID is propagated to all marker types (clusters, fan leaves, single points) during creation to ensure correct drill-down.
        - `path` & `source`: File location.
        - `count`: Number of items in this marker (e.g., 1 for a photo, 15 for a cluster).
4.  **Auto-Drill Decision**:
    - If the selection contains items from a **Single Folder**:
        - It calls `openFolderView(group)` immediately.
        - This triggers `inspectLocation` with the collected `clusterIDs` from those markers.
    - If multiple folders:
        - It opens the Side Panel showing the list of folders.
        - Clicking a folder triggers the drill-down for that specific folder.

#### B. Frontend: API Request (`inspectLocation`)
The frontend sends a request to the backend inspection API:
```
GET /api/heatmap/inspect?path=/My/Folder&source=PHOTOS&cluster_id=ID1,ID2,ID3&zoom=2
```
- **`path`**: The folder containing the clusters.
- **`cluster_id`**: A comma-separated list of the specific clusters selected.

#### C. Backend: Inspection Handler (`heatmap_inspect.go`)
The backend receives the request and performs the following:

1.  **Load Data (Cache Bypass)**:
    - It forces a reload of the `heatmap.json` for the requested folder/tile to ensure up-to-date data.
    - *Note*: It uses the "Leaf" folder path logic to find the correct JSON file.

2.  **Cluster Matching (The "Hydration" Step)**:
    - It iterates through the requested `cluster_id` list.
    - It finds the corresponding Cluster object in the loaded data.
    - **Crucial Check**: If the Cluster has `Points = nil` (Aggregated), it must "Hydrate" it.

3.  **Hydration Logic**:
    - The backend calculates the `leafFolder` (the actual directory where images reside).
    - It attempts to load the `heatmap.json` from that leaf folder.
    - **Path Matching**: It uses a robust "Suffix Match" to align the abstract Tile Path with the concrete Disk Path (handling Windows prefix issues).
    - **Extraction**: It extracts the list of files (Points) from the leaf cluster that match the ID.

4.  **Response**:
    - The backend returns a JSON array of *all* individual files found in the selected clusters.

### 3. Right-Click Inspection

Right-clicking a cluster badge follows a similar path but for a single item:

1.  **User Action**: Right-click on a Map Cluster Badge.
2.  **Context Menu**: A custom popup appears with an "Inspect Files" button.
3.  **Trigger**: Clicking "Inspect" calls `inspectLocation` with that single cluster's ID and coordinates.
4.  **Backend**: Follows the same Hydration flow as above to resolve that single cluster into its constituent images.


### 4. Key Components

#### Frontend (`Heatmap.vue`)
- `endSelection`: Handles box geometry and marker collision detection.
- `inspectLocation`: Central gateway to the backend API.
- `openFolderView`: specific logic to handle drilling down into a grouped folder.

#### Backend (`heatmap_inspect.go`)
- `handleInspect`: Main HTTP handler.
- `findMatch`: Helper to locate clusters by ID or Path (includes Suffix Matching).
- **Hydration Block**: The specific logic branch that detects empty points and loads child data.

---

## Collection Folder Permissions

This section documents the required filesystem permissions for the main collection folder (e.g., `PHOTOCOLLECTIONS`) to ensure proper access for the backend services and the `archive-rw` group.

**Example Path**: `mnt/exp8T/archive/PHOTOS/PHOTOCOLLECTIONS`

### ACL Settings (Access Control List)

The following ACL configuration ensures that the root user (owner) and the `archive-rw` group have full Read/Write/Execute permissions, while others have Read/Execute access. The default entries ensure that new files and directories inherit these permissions.

```bash
# file: mnt/exp8T/archive/PHOTOS/PHOTOCOLLECTIONS
# owner: root
# group: archive-rw
# flags: -s-
user::rwx
group::rwx
group:archive-rw:rwx
mask::rwx
other::r-x
default:user::rwx
default:group::rwx
default:group:archive-rw:rwx
default:mask::rwx
default:other::r-x
```

**Key Settings:**
- **Owner**: `root`
- **Group**: `archive-rw`
- **Flags**: `-s-` (SetGID bit) ensures new files inherit the group ownership.
- **Default ACLs**: `default:user::rwx`, `default:group::rwx`, etc., ensure inheritance for new subdirectories and files.

**Important for Copy Operations:**
These ACLs (specifically the `default` entries) are critical when copying files and folders to the server using external tools like **Beyond Compare**. They ensure that all new incoming content automatically inherits the correct group permissions (`archive-rw`), preventing access issues for the file browser and other services.


### 5. Inspection Navigation (History Stack)

To support complex drill-down workflows (e.g., Box Select -> Multi-Folder View -> Single Folder Drill-down), the frontend maintains a navigation stack.

**Problem**: When drilling down from a list of folders (e.g., "Vacation" and "Work") into a single folder ("Vacation"), the `sidePanelData` is overwritten. Standard "Back" logic would fail or require re-fetching.

    - **New Selection**: `keepHistory=false` (Stack is cleared).
3.  **Back Action**: `backToFolders` pops the last state from the stack and restores `sidePanelData` and `formattedSidePanelData`, instantly returning the user to the previous view without an API call.

---

## Media Playback Architecture

### Overview
The FileBrowser Quantum application supports viewing images and playing videos directly in the browser using native HTML5 elements (`<img>`, `<video>`, `<audio>`). The backend provides media streaming and MIME type detection, while the frontend handles rendering.

### Supported Image Formats

#### Standard Image Formats

| Format | Extensions | MIME Type | Browser Support | Notes |
|--------|-----------|-----------|----------------|-------|
| **JPEG** | `.jpg`, `.jpeg`, `.jpe`, `.jfif` | `image/jpeg` | ✅ Universal | Most common format |
| **PNG** | `.png`, `.x-png` | `image/png` | ✅ Universal | Supports transparency |
| **GIF** | `.gif` | `image/gif` | ✅ Universal | Supports animation |
| **BMP** | `.bmp` | `image/bmp` | ✅ Universal | Uncompressed bitmap |
| **WebP** | `.webp` | `image/webp` | ✅ Modern browsers | Google format, excellent compression |
| **SVG** | `.svg` | `image/svg+xml` | ✅ Universal | Vector graphics, scalable |
| **TIFF** | `.tif`, `.tiff` | `image/tiff` | ⚠️ Limited | May require download |
| **ICO** | `.ico` | `image/x-icon` | ✅ Universal | Icon format |

#### Legacy & Specialized Formats

| Format | Extensions | MIME Type | Browser Support | Notes |
|--------|-----------|-----------|----------------|-------|
| **PCX** | `.pcx` | `image/x-pcx` | ❌ Very limited | DOS-era format |
| **PICT** | `.pict`, `.pic`, `.pct` | `image/pict` | ❌ Limited | Mac Classic format |
| **XBM** | `.xbm` | `image/x-xbitmap` | ⚠️ Some browsers | X Window bitmap |
| **XPM** | `.xpm` | `image/x-xpixmap` | ⚠️ Some browsers | X Window pixmap |
| **XWD** | `.xwd` | `image/x-xwd` | ❌ Not supported | X Window dump |
| **PGM/PPM/PNM** | `.pgm`, `.ppm`, `.pnm` | `image/x-portable-*` | ❌ Not supported | Netpbm formats |
| **RGB** | `.rgb` | `image/x-rgb` | ❌ Not supported | SGI format |
| **RAS** | `.ras`, `.rast` | `image/cmu-raster` | ❌ Not supported | Sun raster |

**Note**: Unsupported formats can still be downloaded and viewed in dedicated image viewers like IrfanView, XnView, or GIMP.

#### RAW Camera Formats

RAW formats are **not directly viewable** in browsers and require download to view in specialized RAW processors (Adobe Lightroom, Capture One, RawTherapee, darktable).

| Camera Brand | Extensions | MIME Type | Status |
|--------------|-----------|-----------|--------|
| **Canon** | `.cr2`, `.cr3`, `.crw` | `image/x-canon-cr2` | ❌ Download required |
| **Nikon** | `.nef`, `.nrw` | `image/x-nikon-nef` | ❌ Download required |
| **Sony** | `.arw`, `.srf`, `.sr2` | `image/x-sony-arw` | ❌ Download required |
| **Fuji** | `.raf` | `image/x-fuji-raf` | ❌ Download required |
| **Olympus** | `.orf` | `image/x-olympus-orf` | ❌ Download required |
| **Panasonic** | `.rw2` | `image/x-panasonic-rw2` | ❌ Download required |
| **Pentax** | `.pef` | `image/x-pentax-pef` | ❌ Download required |
| **Adobe** | `.dng` | `image/x-adobe-dng` | ❌ Download required |
| **Generic** | `.raw` | `image/x-raw` | ❌ Download required |

**FileBrowser Quantum Workflow for RAW**:
1. Browser shows file icon (not preview)
2. User downloads RAW file
3. Opens in Lightroom/Capture One/etc
4. Edits and exports to JPEG/PNG
5. Uploads edited version back to FileBrowser

### Supported Document Formats

#### Text & Document Files

| Format | Extensions | MIME Type | Browser Support | Viewing Method |
|--------|-----------|-----------|----------------|----------------|
| **PDF** | `.pdf` | `application/pdf` | ✅ Universal | Inline PDF viewer |
| **Plain Text** | `.txt` | `text/plain` | ✅ Universal | Inline text viewer |
| **Markdown** | `.md`, `.markdown` | `text/markdown` | ✅ Universal | Rendered or plain text |
| **HTML** | `.html`, `.htm` | `text/html` | ✅ Universal | Rendered in browser |
| **XML** | `.xml` | `text/xml` | ✅ Universal | Syntax-highlighted view |
| **JSON** | `.json` | `application/json` | ✅ Universal | Formatted JSON viewer |
| **CSV** | `.csv` | `text/csv` | ✅ Universal | Table view or download |
| **Log Files** | `.log` | `text/plain` | ✅ Universal | Inline text viewer |
| **Code Files** | `.js`, `.py`, `.java`, `.c`, etc. | `text/plain` | ✅ Universal | Syntax-highlighted viewer |

**PDF Viewing**:
- Most browsers have built-in PDF viewers
- Supports zooming, page navigation, searching
- Can download if browser viewer has issues
- Large PDFs may require download for better performance

**Text File Viewing**:
- Plain text displayed with original formatting
- Line numbers available in code editor view
- Syntax highlighting for code files
- Large text files (>10MB) may require download

### Supported Video Formats

#### Format Support Matrix

| Format | Container | Codec | Browser Support | Notes |
|--------|-----------|-------|----------------|-------|
| **MP4** | `.mp4` | H.264 (AVC) | ✅ All modern browsers | Most compatible |
| **MP4** | `.mp4` | H.265 (HEVC) | ⚠️ Safari, some Edge | Limited support |
| **WebM** | `.webm` | VP8/VP9 | ✅ All modern browsers | Open format |
| **MOV** | `.mov` | H.264 (AVC) | ✅ All modern browsers | QuickTime container |
| **MOV** | `.mov` | H.265 (HEVC) | ⚠️ Safari, some Edge | Limited support |
| **MOV** | `.mov` | ProRes | ❌ Very limited | Download recommended |
| **AVI** | `.avi` | Various | ⚠️ Depends on codec | Legacy format |

**Note**: If a video doesn't play in the browser, users can always use the download button to play it in VLC or other native media players.

### Backend: MIME Type Detection

**Location**: `backend/adapters/fs/files/mime.go`

The backend uses a large map of file extensions to MIME types:

```go
var mimeTypes = map[string]string{
    ".mp4":  "video/mp4",
    ".webm": "video/webm",
    ".mov":  "video/quicktime",
    ".MOV":  "video/quicktime",
    ".qt":   "video/quicktime",
    ".avi":  "video/x-msvideo",
    ".wmv":  "video/x-ms-wmv",
    ".flv":  "video/x-flv",
    ".mkv":  "video/x-matroska",
    // ... more formats
}
```

**Case Sensitivity**: The map includes both lowercase and uppercase variants of common extensions (e.g., `.mov` and `.MOV`) to handle files from different operating systems.

### Frontend: Type Classification

**Location**: `frontend/src/utils/mimetype.js`

The frontend classifies MIME types into simple categories:

```javascript
export function getTypeInfo(mimeType) {
  if (mimeType.startsWith('video/')) {
    return { simpleType: 'video', icon: 'videocam' }
  }
  // ... other types
}
```

**Video MIME Types Recognized**:
- `video/mp4`
- `video/quicktime` (.mov files)
- `video/webm`
- `video/x-msvideo` (.avi files)
- `video/x-matroska` (.mkv files)
- All other `video/*` types

### Frontend: Video Player Implementation

**Location**: `frontend/src/views/files/Preview.vue` (lines 50-66)

#### Standard Implementation (Doesn't Work)

The expected Vue approach would be:

```vue
<video
  v-else-if="previewType == 'video'"
  :src="raw"
  controls
></video>
```

However, this **does not work** due to a Vue 3 reactivity bug.

#### Actual Implementation (Ref Callback Workaround)

**The Fix**:

```vue
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
```

**Why This Works**:

1. **Vue Reactivity Bug**: Vue 3's reactive binding system fails to properly bind computed properties to media element (`<video>`, `<audio>`) `src` attributes. The attribute remains empty even when the computed property has a valid value.

2. **Ref Callback Solution**: Instead of using `:src="raw"`, we use a ref callback function:
   ```javascript
   :ref="(el) => { if (el) { el.src = raw; el.load(); } }"
   ```

3. **How It Works**:
   - Vue calls the ref callback when creating the video element
   - The callback receives the DOM element (`el`)
   - We directly set `el.src = raw` using vanilla JavaScript
   - We call `el.load()` to initiate video loading
   - This bypasses Vue's reactive binding entirely

4. **Key Attribute**: `:key="req.path"` ensures the video element is recreated when navigating between different video files

### Data Flow

```
User clicks .mov file
  ↓
Frontend: fetchData() loads file metadata
  ↓
store/getters.js: previewType = getTypeInfo('video/quicktime').simpleType
  ↓
Preview.vue: Renders video element (v-else-if="previewType == 'video'")
  ↓
Ref callback executes:
  - Computes raw URL: api/raw?files=SOURCE::PATH&inline=true
  - Sets el.src = raw
  - Calls el.load()
  ↓
Browser attempts to load video
  ↓
Backend: Streams video file with correct MIME type
  ↓
If codec supported: Video plays
If codec unsupported: Browser shows error (user can download)
```

### URL Generation

**Location**: `frontend/src/api/files.js`

```javascript
export function getDownloadURL(source, path, inline, useExternal) {
  const params = {
    files: source + '::' + encodeURIComponent(path),
    ...(inline && { inline: 'true' })
  }
  return getApiPath('api/raw', params)
}
```

**Example URL**:
```
http://localhost:8080/api/raw?files=PHOTOS::%2Fvacation%2Fvideo.MOV&inline=true
```

The `inline=true` parameter tells the backend to set `Content-Disposition: inline` instead of `attachment`, allowing the browser to play the video instead of downloading it.

### Codec Detection Limitations

**Important**: The application cannot detect video codecs at the application level. Codec detection would require:
- Parsing video container headers (complex)
- Using external libraries (overhead)
- Server-side transcoding (resource intensive)

**Current Approach**: 
- Rely on browser's native codec support
- If video doesn't play, user can download to play in VLC/native player
- Most modern videos use H.264, which has universal browser support

### Troubleshooting

**Issue**: Video player shows black screen with controls

**Diagnosis**:
1. Check browser console for errors
2. Inspect video element's `src` attribute (should not be empty)
3. Check Network tab for 200 response on video URL
4. Verify MIME type in response headers

**Common Causes**:
- **Unsupported codec**: H.265/HEVC, ProRes → Solution: Download and use VLC
- **Empty src**: Vue binding bug → Solution: Already fixed with ref callback
- **CORS issues**: Rare, but check if external URLs are used
- **File permissions**: Backend can't read file → Check logs

### Audio Playback

Audio files use the same architecture but with the `<audio>` element:

```vue
<audio
  v-else-if="previewType == 'audio'"
  :ref="(el) => { if (el) { el.src = raw; el.load(); } }"
  controls
  :autoplay="autoPlay"
></audio>
```

**Supported Formats**: MP3, WAV, OGG, M4A, AAC

---


## Server Permissions

[↑ Back to Top](#table-of-contents)

### Overview
Users can be assigned specific permissions (e.g., admin, create, rename, delete) to control their meaningful actions within the file browser.

### Map Location Permissions
- **Permission**: updateMap`n- **Purpose**: Controls access to map regeneration and coordinate editing.
- **Frontend Effect**: Hides `Regenerate Heatmap` button and disables map editing inputs if permission is missing.

---

## Documentation Notes

[↑ Back to Top](#table-of-contents)

### Pending Updates & Ideas
<!-- 
Use this section to add notes, pending architecture changes, or ideas that need to be incorporated into the documentation. 
The AI assistant can read this section to understand what needs to be updated.
-->

### Administrative Functions

The application includes several advanced features restricted to users with **Admin** privileges to prevent misuse and clutter for standard users.

#### 1. Integrity Check
- **Feature**: "Integrity Check" button (`safety_check`) in the main header.
- **Purpose**: Scans the current folder for file corruption, missing metadata, or database inconsistencies.
- **Restriction**: **Admin Only**.
- **Icons**:
    - **Yellow Warning (⚠️)**: Minor issues (e.g., non-standard EXIF).
    - **Red Error (🚫)**: Critical issues (e.g., file corruption, zero bytes).
    - **Visibility**: These icons appear in the file list and inside the image preview (via "View Issue" button) **only for Admins**.

#### 2. Thumbnail Repair
- **Feature**: "Fix Thumbnails" button (`build`) in the main header.
- **Purpose**: Forces regeneration of EXIF thumbnails and embedded metadata for all files in the folder (recursive).
- **Restriction**: **Admin Only**.
- **Use Case**: Used when thumbnails appear black or corrupted due to bad existing embedded data.

#### 3. Heatmap Regeneration
- **Feature**: "Regenerate Heatmap" button (`sync`) in the header.
- **Purpose**: Manually triggers the geographic clustering process for the current folder.
- **Restriction**: Requires `updateMap` permission (often assigned to Admins).
- **Rationale**: Resource-intensive operation.

#### 4. Settings Management
- **Feature**: Access to the global "Settings" page.
- **Purpose**: Configure user accounts, permissions, and system-wide preferences.
- **Restriction**: **Admin Only**. 


