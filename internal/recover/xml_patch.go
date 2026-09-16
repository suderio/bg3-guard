package recover

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

const (
	HonourRuleSetGUID = "b1935c10-0931-4148-8df0-7d722ec781aa"
	ZeroGUID          = "00000000-0000-0000-0000-000000000000"
)

// ExtractSessionIDFromMetaLSX extracts the GameSessionID from meta.lsx.
func ExtractSessionIDFromMetaLSX(metaLsxPath string) (string, error) {
	data, err := os.ReadFile(metaLsxPath)
	if err != nil {
		return "", err
	}
	content := string(data)

	// Look for: <attribute id="GameSessionID" type="FixedString" value="..." />
	re := regexp.MustCompile(`<attribute\s+id="GameSessionID"[^>]*value="([^"]+)"`)
	matches := re.FindStringSubmatch(content)
	if len(matches) == 2 {
		return matches[1], nil
	}
	return "", nil
}

// PatchMetaLSX modifies meta.lsx to reactivate Honour Mode settings.
func PatchMetaLSX(metaLsxPath string) (string, error) {
	data, err := os.ReadFile(metaLsxPath)
	if err != nil {
		return "", err
	}
	content := string(data)

	sessionID := ""
	reSession := regexp.MustCompile(`<attribute\s+id="GameSessionID"[^>]*value="([^"]+)"`)
	sessionMatches := reSession.FindStringSubmatch(content)
	if len(sessionMatches) == 2 {
		sessionID = sessionMatches[1]
	}

	// 1. Reset DishonorDifficultySelection to ZeroGUID
	reDishonor := regexp.MustCompile(`(<attribute\s+id="DishonorDifficultySelection"[^>]*value=")[^"]*(")`)
	content = reDishonor.ReplaceAllString(content, `${1}`+ZeroGUID+`${2}`)

	// 2. Set DisabledSingleSave to False
	reSingleSave := regexp.MustCompile(`(<attribute\s+id="DisabledSingleSave"[^>]*value=")[^"]*(")`)
	content = reSingleSave.ReplaceAllString(content, `${1}False${2}`)

	// 3. Set Difficulty to Honour or 2 (Honour uint8)
	// In BG3 uint8 Difficulty: 0=Story, 1=Balanced, 2=Tactician/Honour.
	// If Difficulty is string:
	reDiffStr := regexp.MustCompile(`(<attribute\s+id="Difficulty"\s+type="string"\s+value=")[^"]*(")`)
	if reDiffStr.MatchString(content) {
		content = reDiffStr.ReplaceAllString(content, `${1}Honour${2}`)
	}
	reGameDiff := regexp.MustCompile(`(<attribute\s+id="GameDifficulty"[^>]*value=")[^"]*(")`)
	if reGameDiff.MatchString(content) {
		content = reGameDiff.ReplaceAllString(content, `${1}3${2}`)
	}

	// 4. Update RuleSetId if present in older / alternate formats
	reRuleSetId := regexp.MustCompile(`(<RuleSetId[^>]*>)[^<]*(</RuleSetId>)`)
	if reRuleSetId.MatchString(content) {
		content = reRuleSetId.ReplaceAllString(content, `${1}`+HonourRuleSetGUID+`${2}`)
	}
	reRuleSetAttr := regexp.MustCompile(`(<attribute\s+id="RuleSetId"[^>]*value=")[^"]*(")`)
	if reRuleSetAttr.MatchString(content) {
		content = reRuleSetAttr.ReplaceAllString(content, `${1}`+HonourRuleSetGUID+`${2}`)
	}

	// 5. Remove any TotalPartyKill defeat node if present
	reTPK := regexp.MustCompile(`(?s)<node\s+id="TotalPartyKill">.*?</node>\s*`)
	content = reTPK.ReplaceAllString(content, "")

	if err := os.WriteFile(metaLsxPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write patched meta.lsx: %w", err)
	}

	return sessionID, nil
}

// GetDisabledSessionsFromProfileLSX returns all session GUIDs registered under DisabledSingleSaveSessions.
func GetDisabledSessionsFromProfileLSX(profileLsxPath string) ([]string, error) {
	data, err := os.ReadFile(profileLsxPath)
	if err != nil {
		return nil, err
	}
	content := string(data)

	startTag := `<node id="DisabledSingleSaveSessions">`
	startIdx := strings.Index(content, startTag)
	if startIdx == -1 {
		return nil, nil
	}

	sub := content[startIdx:]
	depth := 0
	endIdx := -1
	i := 0
	for i < len(sub) {
		if strings.HasPrefix(sub[i:], "<node") {
			depth++
			i += 5
		} else if strings.HasPrefix(sub[i:], "</node>") {
			depth--
			if depth == 0 {
				endIdx = i + 7
				break
			}
			i += 7
		} else {
			i++
		}
	}
	var block string
	if endIdx != -1 {
		block = sub[:endIdx]
	} else {
		block = sub
	}

	reGUID := regexp.MustCompile(`value="([a-f0-9\-]{36})"`)
	matches := reGUID.FindAllStringSubmatch(block, -1)
	var guids []string
	for _, m := range matches {
		if len(m) == 2 {
			guids = append(guids, m[1])
		}
	}
	return guids, nil
}

// RemoveSessionFromProfileLSX removes a specific session GUID (or all sessions if sessionGUID is empty)
// from the DisabledSingleSaveSessions section in profile8.lsx.
func RemoveSessionFromProfileLSX(profileLsxPath, sessionGUID string) (bool, error) {
	data, err := os.ReadFile(profileLsxPath)
	if err != nil {
		return false, err
	}
	content := string(data)

	// If sessionGUID is provided, remove nodes matching that GUID under DisabledSingleSaveSessions
	// Example node structure:
	// <node id="DisabledSingleSaveSessions">
	//   <children>
	//     <node id="DisabledSingleSaveSession">
	//       <attribute id="Object" type="guid" value="..." />
	//     </node>
	//   </children>
	// </node>

	modified := false
	if sessionGUID != "" {
		reSessionNode := regexp.MustCompile(`(?s)<node\s+id="[^"]*">\s*<attribute\s+id="Object"\s+type="guid"\s+value="` + regexp.QuoteMeta(sessionGUID) + `"\s*/>\s*</node>\s*`)
		if reSessionNode.MatchString(content) {
			content = reSessionNode.ReplaceAllString(content, "")
			modified = true
		}
	} else {
		// Clear all children in DisabledSingleSaveSessions
		reEmpty := regexp.MustCompile(`(?s)(<node\s+id="DisabledSingleSaveSessions">\s*<children>).*?(</children>\s*</node>)`)
		if reEmpty.MatchString(content) {
			content = reEmpty.ReplaceAllString(content, `${1}${2}`)
			modified = true
		}
	}

	if modified {
		if err := os.WriteFile(profileLsxPath, []byte(content), 0644); err != nil {
			return false, fmt.Errorf("failed to write patched profile8.lsx: %w", err)
		}
	}

	return modified, nil
}
