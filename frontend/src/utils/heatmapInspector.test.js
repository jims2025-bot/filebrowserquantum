import { describe, it, expect } from 'vitest';
import { processDirectItems } from './heatmapInspector';

describe('Heatmap Inspection Utils', () => {

    const mockUrlGen = (path, source, size) => `/mock/${source}/${size}/${path}`;

    it('should return empty structure for empty input', () => {
        const result = processDirectItems([], mockUrlGen);
        expect(result.sidePanelData).toHaveLength(0);
        expect(result.formattedSidePanelData).toHaveLength(0);
        expect(result.singleGroup).toBeNull();
    });

    it('should correctly map raw items to side panel structure', () => {
        const rawItems = [
            { path: '/photos/A/img1.jpg', source: 'SRC1', count: 1, clusterID: 'C1' },
            { path: '/photos/A/img2.jpg', source: 'SRC1', count: 1, clusterID: 'C1' }
        ];

        const { sidePanelData } = processDirectItems(rawItems, mockUrlGen);

        expect(sidePanelData).toHaveLength(2);
        expect(sidePanelData[0].name).toBe('img1.jpg');
        expect(sidePanelData[0].parentPath).toBe('/photos/A');
        expect(sidePanelData[0].thumbUrl).toBe('/mock/SRC1/small//photos/A/img1.jpg');
        expect(sidePanelData[0].type).toBe('image');
    });

    it('should group items by parent path', () => {
        const rawItems = [
            { path: '/photos/A/img1.jpg', source: 'SRC1', count: 1 },
            { path: '/photos/B/img2.jpg', source: 'SRC1', count: 1 }
        ];

        const { formattedSidePanelData } = processDirectItems(rawItems, mockUrlGen);

        expect(formattedSidePanelData).toHaveLength(2);

        const groupA = formattedSidePanelData.find(g => g.path === '/photos/A');
        const groupB = formattedSidePanelData.find(g => g.path === '/photos/B');

        expect(groupA).toBeDefined();
        expect(groupA.count).toBe(1);
        expect(groupB).toBeDefined();
    });

    it('should identify single group for auto-drill', () => {
        const rawItems = [
            { path: '/photos/A/img1.jpg', source: 'SRC1' },
            { path: '/photos/A/img2.jpg', source: 'SRC1' }
        ];

        const { formattedSidePanelData, singleGroup } = processDirectItems(rawItems, mockUrlGen);

        expect(formattedSidePanelData).toHaveLength(1);
        expect(singleGroup).toBeDefined();
        expect(singleGroup.path).toBe('/photos/A');
    });

    it('should accumulate complex counts correctly', () => {
        // Simulating selection of a Cluster (count=5) and a single leaf (count=1)
        const rawItems = [
            { path: '/photos/A/cluster1', count: 5, clusterID: 'C1' },
            { path: '/photos/A/img1.jpg', count: 1, clusterID: 'L1' }
        ];

        const { formattedSidePanelData } = processDirectItems(rawItems, mockUrlGen);

        expect(formattedSidePanelData).toHaveLength(1);
        // Total count should be 5 + 1 = 6
        expect(formattedSidePanelData[0].count).toBe(6);
        // Should capture both cluster IDs
        expect(formattedSidePanelData[0].clusterIDs.has('C1')).toBe(true);
        expect(formattedSidePanelData[0].clusterIDs.has('L1')).toBe(true);
    });
});
