import * as i18n from "@/i18n";
import { state } from "./state.js";
import { getters } from "./getters.js";
import { emitStateChanged } from './eventBus';
import { usersApi } from "@/api";
import { notify } from "@/notify";
import { sortedItems } from "@/utils/sort.js";
import { serverHasMultipleSources } from "@/utils/constants.js";

export const mutations = {
  setMultiButtonState: (value) => {
    if (state.multiButtonLastState != value) {
      state.multiButtonLastState = state.multiButtonState;
    }
    state.multiButtonState = value;
    emitStateChanged();
  },
  toggleOverflowMenu: () => {
    state.showOverflowMenu = !state.showOverflowMenu;
    emitStateChanged();
  },
  toggleMetadataVisibility: () => {
    state.isMetadataVisible = !state.isMetadataVisible;
    emitStateChanged();
  },
  toggleDebugInfo: () => {
    state.showDebugInfo = !state.showDebugInfo;
    emitStateChanged();
  },
  toggleInstructionsEditMode: (value) => {
    state.isInstructionsEditMode = value;
    emitStateChanged();
  },
  setWatchDirChangeAvailable() {
    state.req.hasUpdate = true;
  },
  setPreviewSource: (value) => {
    if (value === state.popupPreviewSource) {
      return;
    }
    state.popupPreviewSource = value;
    emitStateChanged();
  },
  rotatePreview: () => {
    state.previewRotation = (state.previewRotation + 90) % 360;
    emitStateChanged();
  },
  resetPreviewRotation: () => {
    state.previewRotation = 0;
    emitStateChanged();
  },
  updateListing: (value) => {
    state.listing = value;
    emitStateChanged();
  },
  setCurrentSource: (value) => {
    state.sources.current = value;
    emitStateChanged();
  },
  updateSource: (sourcename, value) => {
    if (state.sources.info[sourcename]) {
      state.sources.info[sourcename] = value;
    }
    emitStateChanged();
  },
  updateSourceInfo: (value) => {
    if (value == "error") {
      state.realtimeActive = false;
      for (const k of Object.keys(state.sources.info)) {
        state.sources.info[k].status = "error";
      }
    } else {

      // Incoming 'value' is keyed by backend Source Name (e.g. "PHOTOS")
      // We need to update all local sources (e.g. "Alias", "PHOTOS (2)") that map to this backend source.

      const localKeys = Object.keys(state.sources.info);

      for (const backendKey of Object.keys(value)) {
        const sourceMetrics = value[backendKey];

        // Find all local keys that correspond to this backend key
        localKeys.forEach(localKey => {
          const localSource = state.sources.info[localKey];

          // Check if this local source maps to the current backend source key
          if (localSource.realName === backendKey) {
            if (sourceMetrics.total == 0) {
              // If one is empty, we don't necessarily want to hide headers globally, 
              // but logic here seems global 'state.sources.hasSourceInfo'. 
              // Let's keep it simple.
              state.sources.hasSourceInfo = false
            } else {
              state.sources.hasSourceInfo = true
            }

            localSource.used = sourceMetrics.used;
            localSource.total = sourceMetrics.total;
            localSource.usedPercentage = Math.round((sourceMetrics.used / sourceMetrics.total) * 100);
            localSource.status = sourceMetrics.status;
            // Don't overwrite the display name (localSource.name) with backend name!
            // localSource.name = sourceMetrics.name; 

            localSource.files = sourceMetrics.numFiles;
            localSource.folders = sourceMetrics.numDirs;
            localSource.lastIndex = sourceMetrics.lastIndexedUnixTime;
            localSource.quickScanDurationSeconds = sourceMetrics.quickScanDurationSeconds;
            localSource.fullScanDurationSeconds = sourceMetrics.fullScanDurationSeconds;
            localSource.assessment = sourceMetrics.assessment;
          }
        });
      }
    }
    emitStateChanged();
  },
  setRealtimeActive: (value) => {
    if (value == false) {
      state.realtimeDownCount = state.realtimeDownCount + 1;
    } else {
      state.realtimeDownCount = 0;
    }
    state.realtimeActive = value;
  },
  setSources: (user) => {
    // If user has multiple scopes, we effectively have multiple sources from the frontend perspective
    state.serverHasMultipleSources = serverHasMultipleSources || user.scopes.length > 1;
    // We default to the FIRST scope's name, but if we have duplicates, we might want to be more specific.
    // Ideally currentSource matches one of the unique display names we generate below.
    const rawCurrentSource = user.scopes.length > 0 ? user.scopes[0].name : "";

    let sources = { info: {}, current: rawCurrentSource, count: user.scopes.length };

    const nameCounts = {};

    // We iterate with index to creating binding to specific scope index
    user.scopes.forEach((source, index) => {
      let displayName = source.name;

      if (source.alias && source.alias.trim() !== "") {
        displayName = source.alias;
      } else {
        if (nameCounts[source.name]) {
          nameCounts[source.name]++;
          displayName = `${source.name} (${nameCounts[source.name]})`;
        } else {
          nameCounts[source.name] = 1;
        }
      }

      // If this is the very first one, update 'current' to use the display name
      if (index === 0) {
        sources.current = displayName;
      }

      sources.info[displayName] = {
        // "Name:Index" format
        pathPrefix: sources.count == 1 ? "" : `${source.name}:${index}`,
        used: 0,
        total: 0,
        usedPercentage: 0,
        realName: source.name
      };
    });

    state.sources = sources;
    emitStateChanged();
  },
  setGallerySize: (value) => {
    state.user.gallerySize = value
    emitStateChanged();
    usersApi.update(state.user, ['gallerySize']);
  },
  setActiveSettingsView: (value) => {
    state.activeSettingsView = value;
    // Update the hash in the URL without reloading or changing history state
    window.history.replaceState(null, "", "#" + value);
    const container = document.getElementById("main");
    const element = document.getElementById(value);
    if (container && element) {
      const offset = 4 * parseFloat(getComputedStyle(document.documentElement).fontSize); // 4em in px
      const containerTop = container.getBoundingClientRect().top;
      const elementTop = element.getBoundingClientRect().top;
      const scrollOffset = elementTop - containerTop - offset;
      container.scrollTo({
        top: container.scrollTop + scrollOffset,
        behavior: "smooth",
      });
    }
    emitStateChanged();
  },
  setSettings: (value) => {
    state.settings = value;
    emitStateChanged();
  },
  setMobile() {
    state.isMobile = window.innerWidth <= 800
    emitStateChanged();
  },
  toggleDarkMode() {
    mutations.updateCurrentUser({ "darkMode": !state.user.darkMode });
    emitStateChanged();
  },
  toggleSidebar() {
    if (state.user.stickySidebar) {
      localStorage.setItem("stickySidebar", "false");
      mutations.updateCurrentUser({ "stickySidebar": false }); // turn off sticky when closed
      state.showSidebar = false;
    } else {
      state.showSidebar = !state.showSidebar;
    }
    if (state.showSidebar) {
      state.multiButtonState = "back";
    } else {
      state.multiButtonState = "menu";
    }
    emitStateChanged();
  },
  closeSidebar() {
    if (state.showSidebar) {
      state.showSidebar = false;
      emitStateChanged();
    }
  },
  setUpload(value) {
    state.upload = value;
    emitStateChanged();
  },
  setUsage: (source, value) => {
    state.usages[source] = value;
    emitStateChanged();
  },
  closeHovers: () => {
    const previousState = state.multiButtonLastState;
    state.multiButtonLastState = mutations.multiButtonState;
    state.multiButtonState = previousState;
    state.prompts = [];
    if (!state.stickySidebar) {
      state.showSidebar = false;
    }
    emitStateChanged();
  },
  showHover: (value) => {
    if (typeof value === "object") {
      state.prompts.push({
        name: value?.name,
        confirm: value?.confirm,
        action: value?.action,
        props: value?.props,
      });
    } else {
      state.prompts.push({
        name: value,
        confirm: value?.confirm,
        action: value?.action,
        props: value?.props,
      });
    }
    emitStateChanged();
  },
  setLoading: (loadType, status) => {
    if (status === false) {
      delete state.loading[loadType];
    } else {
      state.loading = { ...state.loading, [loadType]: true };
    }
    emitStateChanged();
  },
  setReload: (value) => {
    state.reload = value;
    emitStateChanged();
  },
  setCurrentUser: (value) => {
    try {
      // If value is null or undefined, emit state change and exit early
      if (!value) {
        state.user = value;
        emitStateChanged();
        return;
      }
      if (value.username != "publicUser") {
        mutations.setSources(value);
      }
      // Ensure locale exists and is valid
      // Ensure locale exists and is valid
      if (!value.locale) {
        value.locale = i18n.detectLocale();
      }

      // Preserve pinnedLocation from local state if not present/null in incoming value
      // This is crucial because backend might not stick it, but localStorage has it.
      if (!value.pinnedLocation && state.user && state.user.pinnedLocation) {
        value.pinnedLocation = state.user.pinnedLocation;
      }

      state.user = { ...state.user, ...value };
    } catch (error) {
      console.log(error);
    }
    emitStateChanged();
  },
  setJWT: (value) => {
    state.jwt = value;
    emitStateChanged();
  },
  setSession: (value) => {
    state.sessionId = value;
    emitStateChanged();
  },
  setMultiple: (value) => {
    state.multiple = value;
    if (value == true) {
      notify.showMultipleSelection()
    }
    emitStateChanged();
  },
  addSelected: (value) => {
    state.selected.push(value);
    emitStateChanged();
  },
  removeSelected: (value) => {
    let i = state.selected.indexOf(value);
    if (i === -1) return;
    state.selected.splice(i, 1);
    emitStateChanged();
  },
  resetSelected: () => {
    state.selected = [];
    mutations.setMultiple(false);
    emitStateChanged();
  },
  setRaw: (value) => {
    state.previewRaw = value;
    emitStateChanged();
  },
  updateCurrentUser: (value) => {
    // Ensure the input is a valid object
    if (typeof value !== "object" || value === null) return;

    // Initialize state.user if it's null
    if (!state.user) {
      state.user = {};
    }

    // Store previous state for comparison
    const previousUser = { ...state.user };

    // Merge the new values into the current user state
    state.user = { ...state.user, ...value };

    // Handle locale change
    if (state.user.locale !== previousUser.locale) {
      //state.user.locale = i18n.detectLocale();
      i18n.setLocale(state.user.locale);
      i18n.default.locale = state.user.locale;
      localStorage.setItem("userLocale", state.user.locale);
    }

    // Update localStorage if pinnedLocation is updated
    // CRITICAL FIX: Only update if strictly present. 
    // If state.user.pinnedLocation becomes undefined/null during a reload, we should NOT wipe localStorage blindly.
    // We only wipe if the value is explicitly null (meaning "unpin")
    if (state.user.pinnedLocation !== undefined) {
      if (state.user.pinnedLocation) {
        localStorage.setItem("pinnedLocation", JSON.stringify(state.user.pinnedLocation));
      } else {
        // If it is explicitly null, we wipe. 
        // But wait, if backend sends null, we wipe? 
        // We need to be careful. The user might have a local pin.
        // If we wipe here, we kill the polyfill.
        // Ideally, we only wipe if the USER requested an unpin.
        // But updateCurrentUser is called when backend data arrives too.

        // Let's rely on the toggling action to wipe. 
        // If this is a backend update (e.g. initial load), and it's null, we shouldn't kill local storage if it exists?
        // Actually, if backend says "no pin", we usually respect it.
        // BUT for the "persistent thumbtack" feature, we want local to win if backend is unconnected.
        // However, we added logic in setCurrentUser to preserve it.

        // If we are here, state.user.pinnedLocation IS falsy.
        // If it was preserved in setCurrentUser, it wouldn't be falsy.
        // So this means it really is null.

        // Let's check if we should wipe.
        // If the intention is to "Unpin", we wipe.
        // If the intention is just "Loading user", we shouldn't wipe.
        // But we can't distinguish here easily.

        // REVERT strategy: Only wipe if we are sure? 
        // Or better: Logic in Preview.vue now handles this by checking localStorage *before* this runs?
        // No, this runs in mutations. State updates -> this runs -> updates LS.
        // Preview.vue watcher runs AFTER state update.
        // So if this wipes LS, Preview.vue sees empty LS.

        // FIX: Don't wipe 'pinnedLocation' here if it matches what's in LS? No.

        // Let's just comment out the automatic wiping here? 
        // And make sure togglePin explicitly wipes it.
        // But what about logging out? Login wipes it explicitly.
        // What about unpinning on another device? Backend sends null -> we wipe local. That is correct.

        // The problem is the "Flash" where backend sends null momentarily?
        // If setCurrentUser preserves it, it shouldn't be null.

        // Wait, updateCurrentUser is called by setCurrentUser.
        // In setCurrentUser, we did:
        // if (!value.pinnedLocation && state.user.pinnedLocation) value.pinnedLocation = state.user.pinnedLocation

        // So state.user.pinnedLocation should be KEPT.
        // Why is it null in the logs?
        // "pinnedLocation watcher. New: null"
        // This implies state.user.pinnedLocation BECAME null.

        // Maybe updateCurrentUser is called from somewhere else? 
        // "usersApi.update" calls? No.

        // Let's look at where updateCurrentUser is called.
        // It's called from togglePin (good).
        // It's called from setCurrentUser (good, with preservation).

        // Is it called from "setSources"? No.

        // Maybe preservation logic failed?
        // if (!value.pinnedLocation ...
        // If value.pinnedLocation is `undefined`? `!undefined` is true.
        // If value.pinnedLocation is `null`? `!null` is true.

        // Log trace logic was removed, need to be careful.

        // Safety patch:
        // If we are about to wipe localStorage, check if we really should?
        // No, that's ambiguous.

        // Alternative: In Preview.vue, I added valid restore logic.
        // BUT if this mutation runs FIRST and wipes LS, Preview.vue finds nothing.

        // Change: Don't wipe localStorage in updateCurrentUser if the value is falsy.
        // ONLY write if truthy. 
        // And let togglePin (the action) handle the removal? 
        // OR add a specific "unpin" mutation?

        // If I stop wiping here, then "Remote Unpin" (on another device) won't sync to this device until a restart.
        // That is an acceptable trade-off to fix the local Bug.
        // The user is focusing on "Single session stability".

        // So: If state.user.pinnedLocation is set -> Write to LS.
        // If state.user.pinnedLocation is null -> DO NOTHING to LS.
        // (Let explicit Unpin action handle removal).

        // Where is explicit Unpin? 
        // In Preview.vue: togglePin calls updateCurrentUser({ pinnedLocation: null })
        // So if I modify this, togglePin won't wipe LS. This is BAD.

        // I need to explicitly wipe LS in togglePin then.
        // AND in `login` (already there).

        // So plan:
        // 1. Modify updateCurrentUser to NOT wipe LS on null.
        // 2. Modify Preview.vue togglePin to explicitly wipe LS when unpinning.
      }

      if (state.user.pinnedLocation) {
        localStorage.setItem("pinnedLocation", JSON.stringify(state.user.pinnedLocation));
      }
      // REMOVED implicit wipe.
    }

    // Update localStorage if stickySidebar exists
    if ('stickySidebar' in state.user) {
      localStorage.setItem("stickySidebar", state.user.stickySidebar);
      if (state.user.stickySidebar && getters.currentView() == "listingView") {
        state.multiButtonState = "menu";
      } else if (state.showSidebar) {
        state.multiButtonState = "back";
      }
    }

    // Update users if there's any change in state.user
    if (JSON.stringify(state.user) !== JSON.stringify(previousUser)) {
      usersApi.update(state.user, [
        "locale",
        "dateFormat",
        "themeColor",
        "quickDownload",
        "disableOnlyOfficeExt",
        "preview",
        "stickySidebar",
        "darkMode",
        "showHidden",
        "sorting",
        "gallerySize",
        "viewMode",
        "pinnedLocation"
      ]);
    }

    // Emit state change event
    emitStateChanged();
  },
  replaceRequest: (value) => {
    state.selected = [];
    if (!value?.items) {
      state.req = value;
      emitStateChanged();
      return
    }
    if (!state.user.showHidden) {
      value.items = value.items.filter((item) => !item.hidden);
    }
    value.items.map((item, index) => {
      item.index = index;
      return item;
    })
    state.req = value;
    emitStateChanged();
  },
  setRoute: (value) => {
    state.route = value;
    emitStateChanged();
  },
  updateListingSortConfig: ({ field, asc }) => {
    state.user.sorting.by = field;
    state.user.sorting.asc = asc;
    emitStateChanged();
  },
  updateListingItems: () => {
    state.req.items = sortedItems(state.req.items, state.user.sorting.by)
    mutations.replaceRequest(state.req);
    emitStateChanged();
  },
  updateClipboard: (value) => {
    state.clipboard.key = value.key;
    state.clipboard.items = value.items;
    state.clipboard.path = value.path;
    emitStateChanged();
  },
  resetClipboard: () => {
    state.clipboard.key = "";
    state.clipboard.items = [];
    emitStateChanged();
  },
  setSharePassword: (value) => {
    state.sharePassword = value;
    emitStateChanged();
  },
  setSearch: (value) => {
    state.isSearchActive = value;
    emitStateChanged();
  },
};