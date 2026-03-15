<template>
  <div class="card floating" id="changelog-modal">
    <div class="card-title">
      <h2>Shanton Archive Changelog</h2>
    </div>

    <div class="card-content" style="max-height: 500px; overflow-y: auto;">
      <div class="release">
        <h3 style="color: var(--primaryColor); margin-top: 0;">Version 5.0.9</h3>
        <span class="date" style="color: var(--textSecondary); font-size: 0.9em;">March 2026</span>
        <ul style="padding-left: 1.25em; margin-top: 0.5em;">
          <li style="margin-bottom: 0.5em;"><span class="badge new">New</span> <strong>System Debug Page:</strong> Added a comprehensive Debug configuration page for administrators.</li>
          <li style="margin-bottom: 0.5em;"><span class="badge new">New</span> <strong>Maintenance Mode:</strong> Admins can now put the site into 'TESTING' state, restricting access to normal users.</li>
          <li style="margin-bottom: 0.5em;"><span class="badge new">New</span> <strong>Debug Copy Button:</strong> Instantly copy raw configuration state to clipboard securely.</li>
          <li style="margin-bottom: 0.5em;"><span class="badge fix">Fix</span> <strong>Map Overlays:</strong> "Show ALL" map view now correctly aggregates your entire user root instead of being locked to the current folder view.</li>
          <li style="margin-bottom: 0.5em;"><span class="badge fix">Fix</span> <strong>Portainer Scopes:</strong> Fixed a path resolution bug causing user scopes to drop connection in non-Windows configurations.</li>
          <li style="margin-bottom: 0.5em;"><span class="badge update">Update</span> <strong>Dropdown Menus:</strong> The top-right Folder Options menu now accurately detects clicks outside of the menu window to close cleanly.</li>
        </ul>
      </div>
    </div>

    <div class="card-action">
      <button
        class="button button--flat button--grey"
        @click="close"
        :aria-label="$t('buttons.close')"
        :title="$t('buttons.close')"
      >
        {{ $t("buttons.close") }}
      </button>
    </div>
  </div>
</template>

<script>
import { mutations } from "@/store";

export default {
  name: "Changelog",
  methods: {
    close() {
      mutations.closeHovers();
    },
  },
};
</script>

<style scoped>
#changelog-modal {
    max-width: 600px;
    width: 90%;
}

.release {
    padding-bottom: 1em;
    border-bottom: 1px solid var(--surfaceSecondary);
}

.badge {
  display: inline-block;
  padding: 0.1rem 0.4rem;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: bold;
  margin-right: 0.5rem;
  vertical-align: middle;
}

.badge.new { background: #e3f2fd; color: #1565c0; }
.badge.fix { background: #ffebee; color: #c62828; }
.badge.update { background: #e8f5e9; color: #2e7d32; }

@media (prefers-color-scheme: dark) {
  .badge.new { background: #0d47a1; color: #bbdefb; }
  .badge.fix { background: #b71c1c; color: #ffcdd2; }
  .badge.update { background: #1b5e20; color: #c8e6c9; }
}
</style>
