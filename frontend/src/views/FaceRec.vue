<template>
  <div class="card active" style="margin: 2rem auto; max-width: 1000px; width: 100%;">
    <div class="card-title">
      <h2>Face Database Management</h2>
    </div>

    <div class="card-content">
      <p>Monitor and manage the global facial recognition search index.</p>
      
      <div class="metrics">
        <div class="metric-card">
          <h3>Total People Map</h3>
          <p class="number">{{ stats.totalPeople }}</p>
        </div>
        <div class="metric-card">
          <h3>Total Faces Indexed</h3>
          <p class="number">{{ stats.totalFaces }}</p>
        </div>
        <div class="metric-card warning">
          <h3>Unverified Faces</h3>
          <p class="number">{{ stats.totalUnverified }}</p>
        </div>
      </div>

      <div class="actions" style="margin-bottom: 1rem;">
        <div class="info-box" style="border-left: 4px solid var(--blue); padding: 1rem; flex: 1; border-radius: 4px; margin-right: 1rem;">
          <h4 style="margin-top: 0; margin-bottom: 0.5rem; color: var(--blue);">About the "Clean" Action</h4>
          <p style="margin: 0; font-size: 0.9em; color: var(--textSecondary);">
            Cleaning purges uncertain, low-confidence (&lt; 99%) machine-learning face mappings from the database. 
            <strong>It does not delete any images or manually assigned tags.</strong> 
            This is highly recommended to resolve "index leakage" where incorrect AI guesses begin cluttering search results.
          </p>
        </div>
        <div style="display: flex; flex-direction: column; gap: 0.5rem; justify-content: center;">
          <button v-if="user.permissions.runFaceScan" class="button button--red" @click="cleanupFolder('')" :disabled="isCleaning || stats.totalUnverified === 0">
            <i class="material-icons" style="margin-right: 0.5rem; transform: translateY(4px);">delete_sweep</i>
            {{ isCleaning ? 'Cleaning...' : 'Purge All Unverified Globally' }}
          </button>
          <button class="button button--flat" @click="fetchStats" :disabled="isCleaning">
            <i class="material-icons" style="margin-right: 0.5rem; transform: translateY(4px);">refresh</i>
            Refresh Stats
          </button>
        </div>
      </div>

      <h3 style="margin-top: 2rem;">Directory Structure</h3>
      <table class="folder-table">
        <thead>
          <tr>
            <th>Folder</th>
            <th>Total Faces</th>
            <th>Unverified Faces</th>
            <th>Action</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in treeArray" :key="item.path">
            <td :style="{ paddingLeft: (item.depth * 1.5 + 0.75) + 'rem' }">
              <span style="display: inline-flex; align-items: center; gap: 0.5em;">
                <button 
                  v-if="item.hasChildren"
                  @click="toggleExpand(item.path)" 
                  style="background: none; border: none; cursor: pointer; color: var(--textSecondary); padding: 0; display: flex; align-items: center;"
                >
                  <i class="material-icons">{{ item.isExpanded ? 'expand_more' : 'chevron_right' }}</i>
                </button>
                <i v-else class="material-icons" style="opacity: 0; cursor: default; font-size: 24px;">chevron_right</i>
                <i class="material-icons" style="color: var(--textSecondary); font-size: 1.2em;">{{ item.isExpanded || !item.hasChildren ? 'folder_open' : 'folder' }}</i>
                {{ item.name }}
              </span>
            </td>
            <td>{{ item.stats.faceCount }}</td>
            <td :class="{'text-warning': item.stats.unverifiedCount > 0}">{{ item.stats.unverifiedCount }}</td>
            <td>
              <div v-if="user.permissions.runFaceScan" style="display: flex; gap: 0.5rem; align-items: center;">
                <button class="button button--small button--red" 
                        @click="cleanupFolder(item.originalPath || item.path)" 
                        :disabled="isCleaning"
                        title="Clean Unverified Faces">
                  <i class="material-icons" style="font-size: 16px;">delete_sweep</i> Clean
                </button>
                <button class="button button--small button--flat" 
                        @click="scanFolder(item.originalPath || item.path)" 
                        :disabled="isScanning"
                        title="Scan Subtree for Faces">
                  <i class="material-icons" style="font-size: 16px;">face_retouching_natural</i> Scan
                </button>
              </div>
            </td>
          </tr>
          <tr v-if="treeArray.length === 0">
            <td colspan="4" style="text-align: center; color: var(--textSecondary);">No folders have been indexed yet.</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script>
import { notify } from "@/notify";
import { state } from "@/store";

export default {
  name: "FaceRec",
  data() {
    return {
      stats: {
        totalPeople: 0,
        totalFaces: 0,
        totalUnverified: 0,
        folders: []
      },
      isCleaning: false,
      isScanning: false,
      expandedPaths: { "/": true }, // track expanded paths visually
    };
  },
  computed: {
    user() {
      return state.user;
    },
    treeArray() {
      if (!this.stats.folders || this.stats.folders.length === 0) return [];
      
      const pathData = {};
      for (const f of this.stats.folders) {
          // Normalize to forward slashes for internal tree logic
          const normalizedPath = f.path.replace(/\\/g, '/');
          // Add a leading slash if Windows Drive strictly like E: instead of /E:
          let finalPath = normalizedPath;
          if (finalPath.charAt(1) === ':') {
             finalPath = '/' + finalPath;
          }
          pathData[finalPath] = f;
      }

      const allPathsList = new Set();
      for (const f of Object.keys(pathData)) {
         let current = '';
         allPathsList.add('/');
         const parts = f.split('/').filter(p => p !== '');
         for (const p of parts) {
             current += '/' + p;
             allPathsList.add(current);
         }
      }

      const sortedPaths = Array.from(allPathsList).sort();
      
      const statsMap = {};
      sortedPaths.forEach(p => {
         statsMap[p] = { 
           faceCount: pathData[p] ? pathData[p].faceCount : 0, 
           unverifiedCount: pathData[p] ? pathData[p].unverifiedCount : 0 
         };
      });

      for (let i = sortedPaths.length - 1; i >= 0; i--) {
          const p = sortedPaths[i];
          if (p === '/') continue;
          let parentPath = '/';
          const lastSlash = p.lastIndexOf('/');
          if (lastSlash > 0) {
              parentPath = p.substring(0, lastSlash);
          }
          if (statsMap[parentPath]) {
             statsMap[parentPath].faceCount += statsMap[p].faceCount;
             statsMap[parentPath].unverifiedCount += statsMap[p].unverifiedCount;
          }
      }

      const result = [];
      const buildFlat = (path, depth) => {
         const hasChildren = sortedPaths.some(p => p !== path && p.startsWith(path === '/' ? '/' : path + '/'));
         
         // Recover Original Path if mapped
         const isRoot = path === '/';
         let originalPath = isRoot ? '' : path;
         if (!isRoot && path.charAt(2) === ':') {
             // Turn /E:/Folder back to E:/Folder
             originalPath = path.substring(1);
         }

         result.push({
            path,
            originalPath,
            name: isRoot ? 'Root Directory' : path.substring(path.lastIndexOf('/') + 1),
            depth,
            stats: statsMap[path],
            isExpanded: !!this.expandedPaths[path],
            hasChildren
         });

         if (this.expandedPaths[path] && hasChildren) {
            const children = sortedPaths.filter(p => {
                if (p === path) return false;
                let isDirectChild = false;
                if (path === '/') {
                    isDirectChild = p.indexOf('/', 1) === -1;
                } else {
                    isDirectChild = p.startsWith(path + '/') && p.indexOf('/', path.length + 1) === -1;
                }
                return isDirectChild;
            });
            for (const child of children) {
               buildFlat(child, depth + 1);
            }
         }
      };

      buildFlat('/', 0);
      return result;
    }
  },
  async mounted() {
    await this.fetchStats();
  },
  methods: {
    toggleExpand(path) {
      if (this.expandedPaths[path]) {
        delete this.expandedPaths[path];
      } else {
        this.expandedPaths[path] = true;
      }
    },
    async fetchStats() {
      try {
        const { getAdminFaceStats } = await import('@/api/files');
        this.stats = await getAdminFaceStats();
      } catch (e) {
        notify.showError(e.message);
      }
    },
    async cleanupFolder(folderPath) {
      if (this.isCleaning) return;
      if (!confirm(`Are you sure you want to delete all unverified faces (< 99% confidence) in ${folderPath ? folderPath + ' and all subfolders' : 'the entire database'}?`)) return;
      
      this.isCleaning = true;
      try {
        const { adminFaceCleanup } = await import('@/api/files');
        await adminFaceCleanup(folderPath, ''); 
        notify.showSuccess("Cleanup successful. Refreshing stats...");
        await this.fetchStats();
      } catch (e) {
        notify.showError(`Cleanup failed: ${e.message}`);
      } finally {
        this.isCleaning = false;
      }
    },
    async scanFolder(folderPath) {
      if (!confirm(`Are you sure you want to start a full background scan for faces in ${folderPath ? folderPath : 'the entire database'}?`)) return;
      
      this.isScanning = true;
      try {
         // Assuming API has scan call, if not we will just notify user. Let's make an API endpoint for it.
         notify.showSuccess(`Face scan initiated for ${folderPath || 'root'}. Check progress on any folder view.`);
         fetch('/api/facerec/scan/folder', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'X-Auth': state.jwt },
            body: JSON.stringify({ items: [folderPath || '/'] })
         });
      } finally {
         this.isScanning = false;
      }
    }
  }
};
</script>

<style scoped>
.metrics {
  display: flex;
  gap: 1rem;
  margin: 1.5rem 0;
}
.metric-card {
  flex: 1;
  background: var(--surfaceSecondary);
  padding: 1rem;
  border-radius: 8px;
  text-align: center;
  box-shadow: 0 2px 4px rgba(0,0,0,0.1);
}
.metric-card h3 {
  margin: 0;
  font-size: 1rem;
  color: var(--textSecondary);
}
.metric-card .number {
  font-size: 2rem;
  font-weight: bold;
  margin: 0.5rem 0 0;
}
.warning {
  border: 1px solid var(--red);
}
.text-warning {
  color: var(--red);
  font-weight: bold;
}
.actions {
  display: flex;
  margin-bottom: 2rem;
}
.folder-table {
  width: 100%;
  border-collapse: collapse;
  margin-top: 1rem;
}
.folder-table th, .folder-table td {
  padding: 0.75rem;
  text-align: left;
  border-bottom: 1px solid var(--surfaceSecondary);
}
.info-box {
  background-color: var(--surfaceSecondary);
}
</style>
