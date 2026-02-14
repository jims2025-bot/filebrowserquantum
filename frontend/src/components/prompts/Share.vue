<template>
  <div class="card floating share__promt__card" id="share">
    <div class="card-title">
      <h2>{{ $t("buttons.share") }}</h2>
    </div>
    <div class="share-type-toggle" v-if="!listing && user.permissions.manageServiceShares">
      <button :class="{ active: shareType === 'quick' }" @click="shareType = 'quick'">Quick Link</button>
      <button :class="{ active: shareType === 'service' }" @click="shareType = 'service'">Full App View</button>
    </div>
    <div aria-label="share-path" class="searchContext"> {{$t('search.path')}} {{ subpath }}</div>
    <p> {{ $t('share.notice') }} </p>
    <template v-if="listing">
      <div class="card-content">
        <table>
          <tbody>
            <tr>
              <th>#</th> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
              <th>{{ $t("settings.shareDuration") }}</th>
              <th></th>
              <th></th>
            </tr>

            <tr v-for="link in links" :key="link.hash">
              <td>{{ link.hash }}</td>
              <td>
                <template v-if="link.expire !== 0">{{ humanTime(link.expire) }}</template>
                <template v-else>{{ $t("permanent") }}</template>
              </td>
              <td class="small">
                <button
                  class="action copy-clipboard"
                  :data-clipboard-text="buildLink(link)"
                  :aria-label="$t('buttons.copyToClipboard')"
                  :title="$t('buttons.copyToClipboard')"
                >
                  <i class="material-icons">content_paste</i>
                </button>
              </td>
              <td class="small" v-if="hasDownloadLink()">
                <button
                  class="action copy-clipboard"
                  :data-clipboard-text="buildDownloadLink(link)"
                  :aria-label="$t('buttons.copyDownloadLinkToClipboard')"
                  :title="$t('buttons.copyDownloadLinkToClipboard')"
                >
                  <i class="material-icons">content_paste_go</i>
                </button>
              </td>
              <td class="small">
                <button
                  class="action"
                  @click="deleteLink($event, link)"
                  :aria-label="$t('buttons.delete')"
                  :title="$t('buttons.delete')"
                >
                  <i class="material-icons">delete</i>
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="card-action">
        <button
          class="button button--flat button--grey"
          @click="closeHovers"
          :aria-label="$t('buttons.close')"
          :title="$t('buttons.close')"
        >
          {{ $t("buttons.close") }}
        </button>
        <button
          class="button button--flat button--blue"
          @click="() => switchListing()"
          :aria-label="$t('buttons.new')"
          :title="$t('buttons.new')"
        >
          {{ $t("buttons.new") }}
        </button>
      </div>
    </template>

    <template v-else>
      <div class="card-content">
        <p>{{ $t("settings.shareDuration") }}</p>
        <div class="input-group input">
          <input
            v-focus
            type="number"
            max="2147483647"
            min="1"
            @keyup.enter="submit"
            v-model.trim="time"
          />
          <select class="right" v-model="unit" :aria-label="$t('time.unit')">
            <option value="seconds">{{ $t("time.seconds") }}</option>
            <option value="minutes">{{ $t("time.minutes") }}</option>
            <option value="hours">{{ $t("time.hours") }}</option>
            <option value="days">{{ $t("time.days") }}</option>
          </select>
        </div>
        <p>{{ $t("prompts.optionalPassword") }}</p>
        <input
          class="input input--block"
          type="password"
          v-model.trim="password"
          autocomplete="new-password"
        />
        <p v-if="password && password.length < 5" class="error-text">
          Password must be at least 5 characters.
        </p>

        <template v-if="shareType === 'service' && isFolder">
          <p>View Mode</p>
          <div class="share-type-toggle">
            <button :class="{ active: viewMode === 'folder' }" @click="viewMode = 'folder'">Folder View</button>
            <button :class="{ active: viewMode === 'map' }" @click="viewMode = 'map'">Map View</button>
          </div>

          <template v-if="viewMode === 'map'">
            <p>Default Overlay</p>
            <div v-if="overlays.length > 0" class="input-group input">
              <select class="input input--block" v-model="selectedOverlay">
                <option value="">None</option>
                <option v-for="ov in overlays" :key="ov.name" :value="ov.name">{{ ov.label || ov.name }}</option>
              </select>
            </div>
            <p v-else style="font-size: 0.9em; color: gray;">No overlays found in this folder. (Check for mapoverlays.json)</p>
          </template>
        </template>
      </div>

      <div class="card-action">
        <button
          class="button button--flat button--grey"
          @click="() => switchListing()"
          :aria-label="$t('buttons.cancel')"
          :title="$t('buttons.cancel')"
        >
          {{ $t("buttons.cancel") }}
        </button>
        <button
          class="button button--flat button--blue"
          @click="submit"
          aria-label="Share-Confirm"
          :title="$t('buttons.share')"
        >
          {{ $t("buttons.share") }}
        </button>
      </div>
    </template>
  </div>
</template>
<script>
import { notify } from "@/notify";
import { state, getters, mutations } from "@/store";
import { shareApi, publicApi } from "@/api";
import { getApiPath } from "@/utils/url.js";
import { fetchURL } from "@/api/utils.js";
import Clipboard from "clipboard";
import { initAuth } from "@/utils/auth";

export default {
  name: "share",
  props: ["initialShareType"],
  data() {
    return {
      time: "",
      unit: "hours",
      links: [],
      clip: null,
      subpath: "",
      source: "",
      password: "",
      listing: true,
      shareType: this.initialShareType || "quick",
      viewMode: "folder",
      overlays: [],
      selectedOverlay: "",
    };
  },
  computed: {
    isFolder() {
      if (state.isSearchActive) {
        return state.selected[0]?.type === "directory";
      }
      if (getters.selectedCount() === 1) {
        return getters.getFirstSelected()?.type === "directory";
      }
      return state.req.type === "directory";
    },
    user() {
      return state.user;
    },
    closeHovers() {
      return mutations.closeHovers;
    },
    req() {
      return state.req; // Access state directly
    },
    selected() {
      return state.selected; // Access state directly
    },
    selectedCount() {
      return state.selected.length; // Compute selectedCount directly from state
    },
    isListing() {
      return getters.isListing(); // Access getter directly from the store
    },
    url() {
      if (state.isSearchActive) {
        return state.selected[0].url;
      }
      if (!this.isListing) {
        return state.route.path;
      }
      if (getters.selectedCount() !== 1) {
        // selecting current view image
        return state.route.path;
      }
      return state.req.items[this.selected[0]].url;
    },
  },
  async beforeMount() {
    try {
      await initAuth();
    } catch (e) {
      console.warn("User refresh failed", e);
    }

    let path = state.req.path;
    this.source = state.req.source;
    if (state.isSearchActive) {
      path = state.selected[0].path;
      this.source = state.selected[0].source;
    } else if (getters.selectedCount() === 1) {
      const selected = getters.getFirstSelected();
      path = selected.path;
      this.source = selected.source;
    }
    // double encode # to fix issue with # in path
    // replace all # with %23
    this.subpath = path.replace(/#/g, "%23");

    // Default shareType to 'service' for folders if user has permission
    if (this.isFolder && this.user.permissions.manageServiceShares) {
      this.shareType = "service";
    }

    try {
      // get last element of the path
      console.log("[Share] Fetching existing shares for path:", this.subpath, "source:", this.source);
      const links = await shareApi.get(this.subpath, this.source);
      this.links = links;
      console.log("[Share] Successfully fetched", this.links.length, "shares");
    } catch (err) {
      console.error("[Share] Failed to fetch shares:", err);
      notify.showError(err);
      return;
    }
    this.sort();

    if (this.isFolder) {
      this.viewMode = this.$route.name === "Heatmap" ? "map" : "folder";
      console.log("[Share] Checking for overlays for:", this.subpath, "Default viewMode:", this.viewMode);
      
      try {
        // Try construction with ensuring it's a valid relative path from current folder or absolute from root
        const overlayPath = this.subpath.endsWith("/") ? this.subpath + "mapoverlays.json" : this.subpath + "/mapoverlays.json";
        
        const apiPath = getApiPath("api/resources", {
          path: encodeURIComponent(overlayPath),
          source: this.source,
          content: "true",
        });
        
        console.log("[Share] Fetching from:", apiPath);
        const res = await fetchURL(apiPath, {}, true);
        const data = await res.json();
        
        if (data && data.content) {
          try {
            const parsed = JSON.parse(data.content);
            if (parsed && parsed.overlays && Array.isArray(parsed.overlays)) {
              this.overlays = parsed.overlays;
              console.log("[Share] Successfully loaded", this.overlays.length, "overlays");
            } else {
              console.warn("[Share] mapoverlays.json does not contain a valid 'overlays' array");
            }
          } catch (e) {
            console.warn("[Share] Failed to parse mapoverlays.json content:", e.message);
          }
        } else {
          console.log("[Share] Request successful but no content returned for mapoverlays.json (Type:", data.type, ")");
        }
      } catch (err) {
        console.warn("[Share] Could not fetch mapoverlays.json:", err.status || err.message || err);
      }
    }

    if (this.links.length === 0) {
      this.listing = false;
    }
  },
  mounted() {
    this.clip = new Clipboard(".copy-clipboard");
    this.clip.on("success", () => {
      notify.showSuccess(this.$t("success.linkCopied"));
    });
  },
  beforeUnmount() {
    this.clip.destroy();
  },
  methods: {
    async submit() {
      console.log("[Share] Submitting share request. Path:", this.subpath, "Source:", this.source, "Type:", this.shareType);
      let isPermanent = !this.time || this.time === 0;
      let res = null;
      if (this.shareType === "service" && this.password && this.password.length < 5) {
        this.$showError("Password must be at least 5 characters.");
        return;
      }

      let source = this.source;
      if (this.shareType === "service") {
        source = source.split(":")[0]; // Use base name for service share scopes
      }

      try {
        if (this.shareType === "service") {
          res = await shareApi.createServiceShare(
            this.subpath,
            source,
            this.password,
            this.time.toString(),
            this.unit
          );

          // Generate the deep link for Service Share
          const loginUrl = window.location.origin + getApiPath("login");
          
          let redirectUrl = "/files/";
          if (this.viewMode === 'map') {
            redirectUrl = `/heatmap?path=/&source=${encodeURIComponent(this.source)}`;
            if (this.selectedOverlay) {
              redirectUrl += `&overlay=${encodeURIComponent(this.selectedOverlay)}`;
            }
          }
          
          let fullLink = `${loginUrl}?u=${res.username}&redirect=${encodeURIComponent(redirectUrl)}`;
          if (!this.password) {
            fullLink += `&auth=${res.token}`;
          }

          res.hash = res.username;
          res.path = this.subpath;
          res.expire = this.calculateExpireUnix();
          res.isServiceShare = true;
          res.fullLink = fullLink;
        } else {
          if (isPermanent) {
            res = await shareApi.create(this.subpath, this.source, this.password);
          } else {
            res = await shareApi.create(
              this.subpath,
              this.source,
              this.password,
              this.time.toString(),
              this.unit
            );
          }
        }
        console.log("[Share] Share created successfully:", res);
      } catch (err) {
        console.error("[Share] Create share failed:", err.status || err.message || err);
        if (err.status === 403) {
          notify.showError(
            "Permission denied. You may have been logged out or switched to a restricted session. Refreshing state..."
          );
          initAuth(); // Force state refresh
          this.$store.commit("closeHovers"); // Close modal as it's likely stale
        } else {
          notify.showError(err);
        }
        return;
      }

      this.links.push(res);
      this.sort();

      this.time = "";
      this.unit = "hours";
      this.password = "";
      this.viewMode = "folder";
      this.selectedOverlay = "";

      this.listing = true;
    },
    calculateExpireUnix() {
      if (!this.time || this.time === 0) return 0;
      let add = 0;
      let num = parseInt(this.time);
      switch (this.unit) {
        case "seconds":
          add = num;
          break;
        case "minutes":
          add = num * 60;
          break;
        case "days":
          add = num * 24 * 3600;
          break;
        default:
          add = num * 3600;
      }
      return Math.floor(Date.now() / 1000) + add;
    },
    async deleteLink(event, link) {
      event.preventDefault();
      await shareApi.remove(link.hash);
      this.links = this.links.filter((item) => item.hash !== link.hash);
      if (this.links.length === 0) {
        this.listing = false;
      }
    },
    humanTime(time) {
      return getters.getTime(time);
    },
    buildLink(share) {
      if (share.isServiceShare) return share.fullLink;
      return shareApi.getShareURL(share);
    },
    hasDownloadLink() {
      if (state.isSearchActive) {
        return state.selected[0].type != "directory";
      }
      return this.selected.length === 1 && !state.req.items[this.selected[0]].isDir;
    },
    buildDownloadLink(share) {
      share.source = this.source;
      share.path = "/";
      return publicApi.getDownloadURL(share);
    },
    sort() {
      this.links = this.links.sort((a, b) => {
        if (a.expire === 0) return -1;
        if (b.expire === 0) return 1;
        return new Date(a.expire) - new Date(b.expire);
      });
    },
    switchListing() {
      if (this.links.length === 0 && !this.listing) {
        // Access the store directly if needed
        mutations.closeHovers();
      }

      this.listing = !this.listing;
    },
  },
};
</script>

<style scoped>
.share-type-toggle {
  display: flex;
  justify-content: center;
  gap: 1em;
  margin-bottom: 1em;
}

.share-type-toggle button {
  padding: 0.5em 1em;
  border: 1px solid var(--divider);
  background: transparent;
  cursor: pointer;
  border-radius: 4px;
}

.share-type-toggle button.active {
  background: var(--blue);
  color: white;
  border-color: var(--blue);
}
.error-text {
  color: var(--red);
  font-size: 0.8em;
  margin-top: 5px;
}
</style>
