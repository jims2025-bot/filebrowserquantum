/**
 * Utility functions for Heatmap Inspection Logic
 * Extracted for Unit Testing
 */

/**
 * Processes a raw list of items (e.g. from Box Selection or Cluster Click)
 * and prepares them for the Inspection Side Panel.
 * 
 * @param {Array} directItems - Raw items from the map interaction
 * @param {Function} urlGenerator - Function(path, source, size) to generate thumbUrl
 * @returns {Object} { sidePanelData, formattedSidePanelData, singleGroup }
 */
export function processDirectItems(directItems, urlGenerator) {
    if (!directItems || directItems.length === 0) {
        return { sidePanelData: [], formattedSidePanelData: [], singleGroup: null };
    }

    // 1. Map raw items to Side Panel Structure
    const sidePanelData = directItems.map(item => ({
        ...item,
        name: item.name || item.path.split('/').pop(),
        parentPath: item.path.substring(0, item.path.lastIndexOf('/')) || "Root",
        thumbUrl: urlGenerator ? urlGenerator(item.path, item.source, 'small') : "",
        type: item.type || 'image', // Assume images if not specified? 
        count: item.count || 1,
        clusterID: item.clusterID,
        totalImageCount: item.totalImageCount
    }));

    // 2. Group by Parent Path
    const groups = {};
    sidePanelData.forEach(item => {
        const p = item.parentPath || "Root";
        if (!groups[p]) {
            groups[p] = {
                path: p,
                count: 0,
                items: [],
                clusterIDs: new Set(), // Collect Cluster IDs
                source: item.source,
                lat: 0, // Centroid Lat
                lon: 0, // Centroid Lon
                coordCount: 0
            };
        }

        // Robust count accumulation
        const c = (item.count && item.count > 1) ? item.count : 1;
        groups[p].count += c;

        // Centroid Accumulation
        if (item.lat !== undefined && item.lon !== undefined) {
            groups[p].lat += Number(item.lat);
            groups[p].lon += Number(item.lon);
            groups[p].coordCount++;
        }

        // Capture TotalImageCount if available
        if (item.totalImageCount) {
            groups[p].totalImageCount = item.totalImageCount;
        }

        groups[p].items.push(item);

        if (item.clusterID) {
            groups[p].clusterIDs.add(item.clusterID);
        }
    });

    const formattedSidePanelData = Object.values(groups).map(g => {
        if (g.coordCount > 0) {
            g.lat = g.lat / g.coordCount;
            g.lon = g.lon / g.coordCount;
        } else {
            g.lat = null;
            g.lon = null;
        }
        return g;
    });

    // Check for single group optimization (Auto-Drill)
    // Return a flag or the group itself?
    const singleGroup = formattedSidePanelData.length === 1 ? formattedSidePanelData[0] : null;

    return { sidePanelData, formattedSidePanelData, singleGroup };
}
