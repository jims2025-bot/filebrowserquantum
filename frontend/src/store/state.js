import { reactive } from 'vue';
import { detectLocale } from "@/i18n";

export const state = reactive({
  multiButtonState: "menu",
  multiButtonLastState: "menu",
  showOverflowMenu: false,
  isMetadataVisible: true,
  isInstructionsEditMode: false, // Persistent state for instructions editor
  sessionId: "",
  disableOnlyOfficeExt: "",
  isSafari: /^((?!chrome|android).)*safari/i.test(navigator.userAgent),
  activeSettingsView: "",
  isMobile: window.innerWidth <= 1024,
  showDebugInfo: false,
  isSearchActive: false,
  showSidebar: false,
  usages: {},
  editor: null,
  serverHasMultipleSources: false,
  realtimeActive: undefined,
  realtimeDownCount: 0,
  popupPreviewSource: "",
  previewRotation: 0,
  sources: {
    current: "",
    count: 1,
    hasSourceInfo: false,
    info: {},
  },
  user: {
    preview: {
      video: true,
      image: true,
      popup: true,
      highQuality: true,
    },
    loginType: "",
    username: "",
    quickDownloadEnabled: false,
    gallarySize: 0,
    disableSingleClick: false,
    pinnedLocation: pinnedLocationStartup(),
    stickySidebar: stickyStartup(),
    locale: detectLocale(), // Default to the locale from moment
    viewMode: 'normal', // Default to mosaic view
    showHidden: false, // Default to false, assuming this is a boolean
    scopes: [],
    permissions: {}, // Default to an empty object for permissions
    darkMode: true, // Default to false, assuming this is a boolean
    profile: { // Example of additional user properties
      username: '', // Default to an empty string
      email: '', // Default to an empty string
      avatarUrl: '' // Default to an empty string
    }
  },
  req: {
    sorting: {
      by: 'name', // Initial sorting field
      asc: true,  // Initial sorting order
    },
    items: [],
    numDirs: 0,
    numFiles: 0,
  },
  listing: {
    category: "folders",
    letter: "A",
    scrolling: false,
    scrollRatio: 0,
  },
  previewRaw: "",
  oldReq: {},
  clipboard: {
    key: "",
    items: [],
  },
  jwt: "",
  sharePassword: "",
  loading: [],
  reload: false,
  selected: [],
  multiple: false,
  upload: {
    uploads: {},
    queue: [],
    progress: [],
    sizes: [],
  },
  prompts: [],
  show: null,
  showConfirm: null,
  route: {},
  settings: {
    signup: false,
    createUserDir: false,
    userHomeBasePath: "",
    rules: [],
    frontend: {
      disableExternal: false,
      name: "",
      files: "",
    },
  },
});

function stickyStartup() {
  const stickyStatus = localStorage.getItem("stickySidebar");
  return stickyStatus == "true"
}

function pinnedLocationStartup() {
  try {
    const val = localStorage.getItem("pinnedLocation");
    if (val) return JSON.parse(val);
  } catch (e) {
    console.error("Failed to parse pinnedLocation", e);
  }
  return null;
}