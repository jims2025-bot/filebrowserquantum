package settings

import (
	"os"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jims2025-bot/filebrowserquantum/backend/database/users"
)

func TestInitialize(t *testing.T) {
	type args struct {
		configFile string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Initialize(tt.args.configFile)
		})
	}
}

func Test_setDefaults(t *testing.T) {
	tests := []struct {
		name string
		want Settings
	}{
		{
			name: "Default Values",
			want: Settings{
				Server: Server{
					Port:               80,
					NumImageProcessors: 4,
					BaseURL:            "",
					Database:           "database.db",
					SourceMap:          map[string]Source{},
					NameToSource:       map[string]Source{},
					MaxArchiveSizeGB:   50,
					CacheDir:           "tmp",
				},
				Auth: Auth{
					AdminUsername:        "admin",
					AdminPassword:        "admin",
					TokenExpirationHours: 2,
					Methods: LoginMethods{
						PasswordAuth: PasswordAuthConfig{
							Enabled:   true,
							MinLength: 5,
							Signup:    false,
						},
					},
				},
				Frontend: Frontend{
					Name: "FileBrowser Quantum",
				},
				UserDefaults: UserDefaults{
					DisableOnlyOfficeExt: ".txt .csv .html .pdf",
					StickySidebar:        true,
					LockPassword:         false,
					ShowHidden:           false,
					DarkMode:             true,
					DisableSettings:      false,
					ViewMode:             "normal",
					Locale:               "en",
					GallerySize:          3,
					ThemeColor:           "var(--blue)",
					Permissions: users.Permissions{
						Modify: false,
						Share:  false,
						Admin:  false,
						Api:    false,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := setDefaults(); !reflect.DeepEqual(got, tt.want) {
				// Use cmp.Diff for cleaner error output (already imported)
				if diff := cmp.Diff(tt.want, got); diff != "" {
					t.Errorf("setDefaults() mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestGetScopeFromSourceName(t *testing.T) {
	// Setup mock config
	Config = Settings{
		Server: Server{
			NameToSource: map[string]Source{
				"MySource": {Name: "MySource", Path: "/data"},
			},
			SourceMap: map[string]Source{
				"/data": {Name: "MySource", Path: "/data"},
			},
		},
	}

	scopes := []users.SourceScope{
		{Name: "/data", Scope: "/", Alias: "my-alias"},
	}

	tests := []struct {
		name           string
		sourceName     string
		expectedScope  string
		expectedSource string
		expectError    bool
	}{
		{"Direct Match", "MySource", "/", "MySource", false},
		{"Case Insensitive Match", "mysource", "/", "MySource", false},
		{"Alias Match", "my-alias", "/", "MySource", false},
		{"Alias Case Insensitive", "MY-ALIAS", "/", "MySource", false},
		{"Not Found", "Unknown", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scope, source, err := GetScopeFromSourceName(scopes, tt.sourceName)
			if (err != nil) != tt.expectError {
				t.Errorf("expected error: %v, got: %v", tt.expectError, err)
			}
			if !tt.expectError {
				if scope != tt.expectedScope {
					t.Errorf("expected scope %s, got %s", tt.expectedScope, scope)
				}
				if source != tt.expectedSource {
					t.Errorf("expected source %s, got %s", tt.expectedSource, source)
				}
			}
		})
	}
}

func TestConfigLoadChanged(t *testing.T) {
	defaultConfig := setDefaults()
	err := loadConfigWithDefaults("./validConfig.yaml")
	if err != nil {
		t.Fatalf("error loading config file: %v", err)
	}
	// Use go-cmp to compare the two structs
	if diff := cmp.Diff(defaultConfig, Config); diff == "" {
		t.Errorf("No change when there should have been (-want +got):\n%s", diff)
	}
}

func TestConfigLoadEnvVars(t *testing.T) {
	defaultConfig := setDefaults()
	expectedKey := "MYKEY"
	// mock environment variables
	os.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", expectedKey)
	err := loadConfigWithDefaults("./validConfig.yaml")
	if err != nil {
		t.Fatalf("error loading config file: %v", err)
	}
	if Config.Integrations.OnlyOffice.Secret != expectedKey {
		t.Errorf("Expected OnlyOffice.Secret to be '%v', got '%s'", expectedKey, Config.Integrations.OnlyOffice.Secret)
	}
	// Use go-cmp to compare the two structs
	if diff := cmp.Diff(defaultConfig, Config); diff == "" {
		t.Errorf("No change when there should have been (-want +got):\n%s", diff)
	}
}

func TestConfigLoadSpecificValues(t *testing.T) {
	defaultConfig := setDefaults()
	err := loadConfigWithDefaults("./validConfig.yaml")
	if err != nil {
		t.Fatalf("error loading config file: %v", err)
	}
	testCases := []struct {
		fieldName string
		globalVal interface{}
		newVal    interface{}
	}{
		{"Server.Database", Config.Server.Database, defaultConfig.Server.Database},
	}

	for _, tc := range testCases {
		if tc.globalVal == tc.newVal {
			t.Errorf("Differences should have been found:\nConfig.%s: %v \nSetConfig: %v \n", tc.fieldName, tc.globalVal, tc.newVal)
		}
	}
}

func TestInvalidConfig(t *testing.T) {
	configFile := "./invalidConfig.yaml"
	err := loadConfigWithDefaults(configFile)
	if err == nil {
		t.Fatalf("expected error loading config file %s, got nil", configFile)
	}
}
