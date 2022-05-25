package compiler

import (
	"regexp"
	"strings"
)

// baseName returns the last path element of the name, with the last dotted suffix removed.
func baseName(name string) string {
	// Save our place
	saveName := name
	currentName := name
	found := true
	// Find the last element
	// if the last element is a version path find the previous element
	// if the last element has a - (hyphen) take the last element of the hyphenated token
	//  ex: repo.com/mine/go-good ==> good
	//  ex: another.org/yours/do-no-harm/v3 ==> harm
	for i := strings.LastIndex(name, "/"); found && i >= 0; i = strings.LastIndex(currentName, "/") {
		saveName = currentName[i+1:]
		currentName = currentName[:i]
		found, _ = regexp.MatchString("v[1-9]\\d*", saveName)
		if !found {
			if i = strings.LastIndex(saveName, "-"); i >= 0 {
				saveName = saveName[i+1:]
			}
		}
	}

	// Now drop the suffix
	if i := strings.LastIndex(saveName, "."); i >= 0 {
		saveName = saveName[0:i]
	}
	return saveName
}
