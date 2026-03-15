<template>
  <nav
    id="sidebar"
    :class="{ active: active, 'dark-mode': isDarkMode, 'behind-overlay': behindOverlay }"
  >
    <SidebarSettings v-if="isSettings"></SidebarSettings>
    <SidebarGeneral v-else-if="isLoggedIn"></SidebarGeneral>

    <div class="buffer"></div>
    <div class="credits">
      <span>
        <a href="#" @click.prevent="showChangelog" title="View latest changes">v5.0.9 Changelog</a>
      </span>
      <span v-if="name != ''">
        <h4 style="margin: 0">{{ name }}</h4>
      </span>
    </div>
  </nav>
</template>

<script>
import { externalLinks, name } from "@/utils/constants";
import { getters, mutations, state } from "@/store"; // Import your custom store
import SidebarGeneral from "./General.vue";
import SidebarSettings from "./Settings.vue";

export default {
  name: "sidebar",
  components: {
    SidebarGeneral,
    SidebarSettings,
  },
  data() {
    return {
      externalLinks,
      name,
    };
  },
  computed: {
    isDarkMode: () => getters.isDarkMode(),
    isLoggedIn: () => getters.isLoggedIn(),
    isSettings: () => getters.isSettings(),
    active: () => getters.isSidebarVisible(),
    behindOverlay: () => state.isSearchActive,
  },
  methods: {
    // Show the help overlay
    help() {
      mutations.showHover("help");
    },
    showChangelog() {
      mutations.showHover("Changelog");
    },
  },
};
</script>

<style>
.sidebar-scroll-list {
  overflow: auto;
  margin-bottom: 0px !important;
}

#sidebar {
  display: flex;
  flex-direction: column;
  padding: 1em;
  width: 20em;
  position: fixed;
  z-index: 4;
  right: -20em; /* Hidden state: off-screen to the right */
  left: unset;  /* Unset left */
  height: 100%;
  box-shadow: 0 0 5px rgba(0, 0, 0, 0.1);
  transition: 0.5s ease;
  top: 4em;
  padding-bottom: 4em;
  background-color: #dddddd;
}

#sidebar.behind-overlay {
  z-index: 3;
}

#sidebar.sticky {
  z-index: 3;
}

@supports (backdrop-filter: none) {
  #sidebar {
    background-color: rgba(237, 237, 237, 0.33) !important;
    backdrop-filter: blur(16px) invert(0.1);
  }
}

body.rtl nav {
  right: unset;
  left: -20em;
}

#sidebar.active {
  right: 0; /* Visible state: 0 from right */
}

/* Mobile Styling */
@media (max-width: 768px) {
  #sidebar {
    flex-direction: row; /* Horizontal layout for tabs */
    width: 100%;
    height: auto;
    top: unset;
    
    /* Mobile Transition State */
    bottom: -100%; /* Start hidden below screen */
    right: 0;
    left: 0;
    
    padding: 0.5em; /* Reduce padding */
    padding-bottom: env(safe-area-inset-bottom); /* Handle iPhone home bar */
    box-shadow: 0 -2px 5px rgba(0, 0, 0, 0.1);
    
    /* Ensure overflow is visible for tooltips or handled */
    overflow: visible; 
  }
  
  #sidebar.active {
     bottom: 0;
     right: 0; /* Maintain right 0 */
  }

  /* Hide elements that don't fit in bottom bar */
  #sidebar .credits,
  #sidebar .buffer {
    display: none;
  }
  
  /* Adjust items to fit horizontally */
  #sidebar > * {
    flex: 1;
    display: flex;
    justify-content: center;
  }
}

#sidebar.rtl nav.active {
  right: unset;
  left: 0;
}

#sidebar .action {
  width: 100%;
  display: block;
  white-space: nowrap;
  height: 100%;
  overflow: hidden;
  padding: 0.5em;
  text-overflow: ellipsis;
}

body.rtl .action {
  direction: rtl;
  text-align: right;
}

#sidebar .action > * {
  vertical-align: middle;
}

/* * * * * * * * * * * * * * * *
 *            FOOTER           *
 * * * * * * * * * * * * * * * */

.credits {
  font-size: 1em;
  color: var(--textPrimary);
  padding-left: 1em;
  padding-bottom: 1em;
}

.credits > span {
  display: block;
  margin-top: 0.5em;
  margin-left: 0;
}

.credits a,
.credits a:hover {
  cursor: pointer;
}

.buffer {
  flex-grow: 1;
}

.clickable {
  cursor: pointer;
}

.clickable:hover {
  box-shadow: 0 2px 2px #00000024, 0 1px 5px #0000001f, 0 3px 1px -2px #0003;
}
</style>
