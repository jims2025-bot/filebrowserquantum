<template>
  <h2
    class="message"
    v-if="user.loginMethod != 'password' && !stateUser.permissions.admin"
  >
    <i class="material-icons">sentiment_dissatisfied</i>
    <span>{{ $t("files.lonely") }}</span>
  </h2>
  <div v-if="user.loginMethod == 'password' && passwordAvailable && !isNew">
    <label for="password">{{ $t("settings.password") }}</label>
    <div class="form-group">
      <input
        class="input input--block form-form"
        :class="{ 'invalid-form': invalidPassword }"
        aria-label="Password1"
        type="password"
        :placeholder="$t('settings.enterPassword')"
        v-model="passwordRef"
      />
    </div>
    <div class="form-group">
      <input
        class="input input--block form-form"
        :class="{ 'flat-right': !isNew, 'invalid-form': invalidPassword }"
        aria-label="Password2"
        type="password"
        :placeholder="$t('settings.enterPasswordAgain')"
        v-model="user.password"
        id="password"
      />
      <button
        v-if="!isNew"
        type="button"
        class="button form-button"
        @click="submitUpdatePassword"
      >
        {{ $t("buttons.update") }}
      </button>
    </div>
    <div
      style="display: flex; flex-direction: column"
      v-if="stateUser.username == user.username"
    >
      <div class="settings-items">
        <ToggleSwitch class="item" v-model="user.otpEnabled" :name="$t('otp.name')" />
      </div>
      <button class="button" type="button" v-if="user.otpEnabled" :onclick="newOTP">
        {{ $t("buttons.generateNewOtp") }}
      </button>
    </div>
    <hr />
  </div>
  <div v-if="stateUser.permissions.admin">
    <p v-if="isNew">
      <label for="username">{{ $t("settings.username") }}</label>
      <input
        class="input input--block"
        type="text"
        v-model="user.username"
        id="username"
        @input="emitUpdate"
      />
    </p>

    <div v-if="user.loginMethod == 'password' && passwordAvailable && isNew">
      <label for="password">{{ $t("settings.password") }}</label>
      <div class="form-group">
        <input
          class="input input--block form-form"
          :class="{ 'invalid-form': invalidPassword }"
          aria-label="Password1"
          type="password"
          :placeholder="$t('settings.enterPassword')"
          v-model="passwordRef"
        />
      </div>
      <div class="form-group">
        <input
          class="input input--block form-form"
          :class="{ 'flat-right': !isNew, 'invalid-form': invalidPassword }"
          type="password"
          :placeholder="$t('settings.enterPasswordAgain')"
          aria-label="Password2"
          v-model="user.password"
          id="password"
        />
        <button
          v-if="!isNew"
          type="button"
          class="button form-button"
          @click="submitUpdatePassword"
        >
          {{ $t("buttons.update") }}
        </button>
      </div>
    </div>

    <div
      v-if="user.loginMethod == 'password' && passwordAvailable"
      class="settings-items"
    >
      <ToggleSwitch
        v-if="user.loginMethod === 'password' && stateUser.permissions?.admin"
        class="item"
        :modelValue="user.lockPassword"
        @update:modelValue="(val) => updateUserField('lockPassword', val)"
        :name="$t('settings.lockPassword')"
      />
    </div>

    <div style="padding-bottom: 1em" v-if="stateUser.permissions.admin">
      <label for="scopes">{{ $t("settings.scopes") }}</label>
      <div
        class="scope-list"
        :class="{ 'invalid-form': duplicateSources.includes(source.name) }"
        v-for="(source, index) in selectedSources"
        :key="index"
      >
        <select
          @change="handleSourceChange(source, $event, source.name)"
          class="input flat-right"
          v-model="source.name"
        >
          <option v-for="s in sourceList" :key="s.name" :value="s.name">
            {{ s.name }}
          </option>
        </select>

        <input
          class="input flat-left scope-input"
          placeholder="Path (e.g. /subfolder)"
          @input="updateParent({ source: source, input: $event })"
          :value="source.scope"
          :class="{ 'flat-right': selectedSources.length > 1 }"
        />
        <input
          class="input flat-left"
          style="width: 50%"
          placeholder="Alias (optional)"
          @input="updateAlias({ source: source, input: $event })"
          :value="source.alias"
          :class="{ 'flat-right': selectedSources.length > 1 }"
        />
        <button
          v-if="selectedSources.length > 1"
          class="button flat-left no-height"
          @click="removeScope(index)"
        >
          <i class="material-icons material-size">delete</i>
        </button>
      </div>
    </div>

    <button v-if="hasMoreSources" @click="addNewScopeSource" class="button no-height">
      <i class="material-icons material-size">add</i>
    </button>

    <div class="settings-items">
      <ToggleSwitch
        v-if="displayHomeDirectoryCheckbox"
        class="item"
        v-model="createUserDir"
        :name="$t('settings.createUserHomeDirectory')"
      />
    </div>

    <p v-if="stateUser.username !== user.username">
      <label for="locale">{{ $t("settings.language") }}</label>
      <languages
        class="input input--block"
        id="locale"
        v-model:locale="user.locale"
        @input="emitUpdate"
      ></languages>
    </p>
    <div v-if="stateUser.permissions.admin">
      <label for="loginMethod">{{ $t("settings.loginMethodDescription") }}</label>
      <select v-model="user.loginMethod" class="input input--block" id="loginMethod">
        <option value="password">Password</option> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
        <option value="oidc">OIDC</option> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
        <option value="proxy">Proxy</option> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
      </select>
    </div>
    <permissions v-if="stateUser.permissions.admin" :permissions="user.permissions" />
  </div>
</template>

<script>
import Languages from "./Languages.vue";
import Permissions from "./Permissions.vue";
import { mutations, state } from "@/store";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import { notify } from "@/notify";
import { usersApi, settingsApi } from "@/api";
import { passwordAvailable } from "@/utils/constants";

export default {
  name: "UserForm",
  components: {
    Permissions,
    Languages,
    ToggleSwitch,
  },
  props: {
    user: Object,
    isNew: Boolean,
  },
  data() {
    return {
      createUserDir: false,
      originalUserScope: ".",
      sourceList: [],
      availableSources: [],
      selectedSources: [],
      passwordRef: "",
    };
  },
  async mounted() {
    if (!this.stateUser.permissions.admin) {
      this.sourceList = this.user.scopes || [];
    } else {
      this.sourceList = await settingsApi.get("sources");
    }

    this.user.password = this.user.password || "";
    this.selectedSources = this.user.scopes || [];
    // Always allow all sources to be selected
    this.availableSources = this.sourceList; 
  },
  watch: {
    createUserDir(newVal) {
      // If creating user dir, we usually reset to default. 
      // But if user wants multiple scopes, this interaction might be tricky.
      // For now, keep existing behavior: if you toggle this, it resets scope.
      this.user.scopes = newVal ? { default: "" } : this.originalUserScope;
      this.emitUserUpdate();
    },
  },
  computed: {
    invalidPassword() {
      const matching =
        this.user.password != this.passwordRef && this.user.password.length > 0;
      return matching;
    },
    passwordAvailable: () => passwordAvailable,
    duplicateSources() {
      // Allow same name, but maybe flag if same name AND same scope?
      // Actually strictly speaking, duplicate names are fine now.
      // We only flag if Name AND Scope are identical.
      const entries = this.selectedSources.map((s) => s.name + "::" + s.scope);
      return this.selectedSources.filter((s, idx) => {
          const key = s.name + "::" + s.scope;
          return entries.indexOf(key) !== idx;
      }).map(s => s.name); 
      // Note: mapping back to name might flag all instances of that name as 'invalid-form' style
      // which is acceptable for visual feedback if they are EXACT duplicates.
    },
    hasMoreSources() {
      // Always allow adding more if we have at least one source definition
      return this.sourceList.length > 0;
    },
    stateUser() {
      return state.user;
    },
    passwordPlaceholder() {
      return this.isNew ? "" : this.$t("settings.avoidChanges");
    },
    displayHomeDirectoryCheckbox() {
      return this.isNew && this.createUserDir;
    },
  },
  methods: {
    newOTP() {
      mutations.showHover({
        name: "totp",
        props: {
          generate: true,
        },
      });
    },
    async submitUpdatePassword() {
      event.preventDefault();
      if (this.invalidPassword) {
        notify.showError(this.$t("settings.passwordsDoNotMatch"));
        return;
      }
      try {
        await usersApi.update(this.user, ["password"]);
        notify.showSuccess(this.$t("settings.userUpdated"));
      } catch (e) {
        notify.showError(e);
      }
    },
    emitUserUpdate() {
      this.$emit("update:user", { ...this.user, scopes: this.selectedSources });
    },
    emitUpdate() {
      this.$emit("update:user", { ...this.user });
    },
    setUpdatePassword() {
      this.$emit("update:updatePassword", true);
    },
    updateParent(input) {
      // We need to be careful updating by NAME if multiple have same name.
      // input.source is the object reference from the v-for loop.
      // We should update that object directly? 
      // The filtered updatedScopes logic in original code:
      // const updatedScopes = this.selectedSources.map((source) =>
      //   source.name === input.source.name
      //     ? { ...source, scope: input.input.target.value }
      //     : source
      // );
      // This will update ALL scopes with that name. That is BAD.
      
      // Since 'source' is passed by reference from the v-for, we can just mutate it
      // and then emit.
      input.source.scope = input.input.target.value;
      this.emitUserUpdate();
    },
    updateAlias(input) {
      input.source.alias = input.input.target.value;
      this.emitUserUpdate();
    },
    addNewScopeSource(event) {
      event.preventDefault();
      // Add a new empty/default scope
      if (this.sourceList.length > 0) {
        // Default to first source
        this.selectedSources.push({ name: this.sourceList[0].name, scope: "" });
        this.emitUserUpdate();
      }
    },
    removeScope(index) {
      this.selectedSources.splice(index, 1);
      this.emitUserUpdate();
    },
    handleSourceChange(source, event, oldName) {
      // Just update the name. No pool management needed.
      source.name = event.target.value;
      this.emitUserUpdate();
    },
    updateUserField(field, value) {
      this.user[field] = value;
      this.emitUserUpdate();
    },
  },
};
</script>

<style>
.scope-list {
  display: flex;
}

.scope-input {
  width: 100%;
}
.no-height {
  height: unset;
}
.material-size {
  font-size: 1em !important;
}
</style>
