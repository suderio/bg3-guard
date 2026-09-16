package recover

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleMetaLSX = `<?xml version="1.0" encoding="utf-8"?>
<save>
	<version major="4" minor="8" revision="0" build="700" lslib_meta="v1,bswap_guids" />
	<region id="MetaData">
		<node id="MetaData">
			<children>
				<node id="MetaData">
					<attribute id="GameSessionID" type="FixedString" value="f6711bc0-a85a-95d7-952c-d3f18dc6f974" />
					<attribute id="DishonorDifficultySelection" type="FixedString" value="1ed480b4-478e-4518-9722-cf30885da6f3" />
					<attribute id="DisabledSingleSave" type="bool" value="True" />
					<attribute id="Difficulty" type="uint8" value="1" />
					<attribute id="GameDifficulty" type="uint32" value="1" />
				</node>
			</children>
		</node>
	</region>
</save>`

const sampleProfileLSX = `<?xml version="1.0" encoding="utf-8"?>
<save>
  <region id="PlayerProfile">
    <node id="PlayerProfile">
      <children>
        <node id="DisabledSingleSaveSessions">
          <children>
            <node id="DisabledSingleSaveSession">
              <attribute id="Object" type="guid" value="f6711bc0-a85a-95d7-952c-d3f18dc6f974" />
            </node>
            <node id="DisabledSingleSaveSession">
              <attribute id="Object" type="guid" value="11111111-2222-3333-4444-555555555555" />
            </node>
          </children>
        </node>
      </children>
    </node>
  </region>
</save>`

func TestPatchMetaLSX(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "bg3guard_xml_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	metaFile := filepath.Join(tempDir, "meta.lsx")
	if err := os.WriteFile(metaFile, []byte(sampleMetaLSX), 0644); err != nil {
		t.Fatalf("Failed to write meta.lsx: %v", err)
	}

	sessionID, err := PatchMetaLSX(metaFile)
	if err != nil {
		t.Fatalf("PatchMetaLSX failed: %v", err)
	}
	if sessionID != "f6711bc0-a85a-95d7-952c-d3f18dc6f974" {
		t.Errorf("Expected session ID 'f6711bc0-a85a-95d7-952c-d3f18dc6f974', got %q", sessionID)
	}

	patchedData, _ := os.ReadFile(metaFile)
	patched := string(patchedData)

	if !strings.Contains(patched, `value="00000000-0000-0000-0000-000000000000"`) {
		t.Errorf("DishonorDifficultySelection not reset to zeroes: %s", patched)
	}
	if !strings.Contains(patched, `value="False"`) {
		t.Errorf("DisabledSingleSave not set to False: %s", patched)
	}
	if !strings.Contains(patched, `value="3"`) {
		t.Errorf("GameDifficulty not set to 3: %s", patched)
	}
}

func TestProfileLSXPatching(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "bg3guard_profile_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	profileFile := filepath.Join(tempDir, "profile8.lsx")
	if err := os.WriteFile(profileFile, []byte(sampleProfileLSX), 0644); err != nil {
		t.Fatalf("Failed to write profile.lsx: %v", err)
	}

	guids, err := GetDisabledSessionsFromProfileLSX(profileFile)
	if err != nil {
		t.Fatalf("GetDisabledSessionsFromProfileLSX failed: %v", err)
	}
	if len(guids) != 2 {
		t.Fatalf("Expected 2 disabled sessions, got %d", len(guids))
	}

	// Remove one session
	target := "f6711bc0-a85a-95d7-952c-d3f18dc6f974"
	modified, err := RemoveSessionFromProfileLSX(profileFile, target)
	if err != nil {
		t.Fatalf("RemoveSessionFromProfileLSX failed: %v", err)
	}
	if !modified {
		t.Errorf("Expected modified=true")
	}

	remainingGuids, _ := GetDisabledSessionsFromProfileLSX(profileFile)
	if len(remainingGuids) != 1 {
		t.Errorf("Expected 1 remaining session, got %d", len(remainingGuids))
	}
	if len(remainingGuids) > 0 && remainingGuids[0] != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("Unexpected remaining GUID: %s", remainingGuids[0])
	}
}
