package main

import (
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Icons that can be matched through a simple string lookup
type basicIcon struct {
	icon string
	pos  int
}

// Icons that need to be matched (globbed) to classify
type globIcon struct {
	pattern *regexp.Regexp
	basicIcon
}

type iconMap struct {
	basicIcons map[string]basicIcon
	globIcons  []globIcon
}

func parseIcons() iconMap {
	if env := os.Getenv("LF_ICONS"); env != "" {
		return parseIconsEnv(env)
	}

	defaultIcons := []string{
		"tw=🗀",
		"st=🗀",
		"ow=🗀",
		"di=🗀",
		"fi=🗎",
	}

	return parseIconsEnv(strings.Join(defaultIcons, ":"))
}

// Assert whether str is a basic file extension glob.
// eg. *.txt or *.png
func isBasicGlob(str string) (bool, error) {
	return regexp.MatchString("\\*.[[:alnum:]]+$", str)
}

// Convert a glob path to a basic regular-expression.
//
// WARN: only supports * and doesn't support escaping.
func globToRegexp(str string) (*regexp.Regexp, error) {
	/* quoted  */ str = regexp.QuoteMeta(str)
	/* globbed */ str = strings.ReplaceAll(str, "\\*", ".*")
	/* clamped */ str = "^" + str + "$"
	return regexp.Compile(str)
}

func parseIconsEnv(env string) iconMap {
	entries := strings.Split(env, ":")
	icons := iconMap{
		make(map[string]basicIcon),
		make([]globIcon, 0, len(entries)),
	}

	for i, entry := range entries {
		if entry == "" {
			continue
		}
		pair := strings.Split(entry, "=")
		if len(pair) != 2 {
			log.Printf("invalid $LF_ICONS entry: %s", entry)
			return icons
		}
		key, val := pair[0], pair[1]
		if isBasic, err := isBasicGlob(key); err != nil {
			log.Printf("failed to assert $LF_ICONS entry is basic: %s", key)
		} else if _, ok := fileIconTypes[key]; isBasic || ok {
			icons.basicIcons[key] = basicIcon{val, i}
		} else if pattern, err := globToRegexp(key); err != nil {
			log.Printf("failed to convert $LF_ICONS entry to regexp '%s': %s", key, err)
		} else {
			icons.globIcons = append(icons.globIcons, globIcon{pattern, basicIcon{val, i}})
		}
	}
	return icons
}

// Return the icon applicable to the file f.
func (im iconMap) get(f *file) string {
	ext := filepath.Ext(f.Name())
	base := filepath.Base(f.Name())

	if icon, ok := im.getFromName(base, ext); ok {
		return icon
	} else if icon, ok := im.getFromFile(f); ok {
		return icon
	} else {
		return " "
	}
}

// Get an icon for a file based only on its filename.
//
// The approach is to glob each pattern in our iconMap and return
// the icon associated with the first matching pattern. Because
// basic (extension) checks are so common, we store them seperately
// from glob patterns to try and optimise icon lookups.
func (im iconMap) getFromName(base, ext string) (string, bool) {
	var icon string
	var found bool

	upper := len(im.basicIcons) + len(im.globIcons)
	if basicIcon, ok := im.basicIcons["*"+filepath.Ext(ext)]; ok {
		// when an extension pattern is found, we only need to search
		// patterns upto just before it.
		upper = basicIcon.pos - 1
		icon = basicIcon.icon
		found = true
	}

	// check for any patterns upto upper which already match the basename
	for _, globIcon := range im.globIcons {
		if globIcon.pos > upper {
			break
		}
		if globIcon.pattern.MatchString(base) {
			icon = globIcon.icon
			found = true
			break
		}
	}

	return icon, found
}

// Map the types we can classify a file as with predicates used to assert
// whether a file is of that type.
//
// We store this as a map for constant-time checks in parseIconsEnv.
//
// WARN: go [[https://stackoverflow.com/questions/28930416/why-cant-go-iterate-maps-in-insertion-order#:~:text=Go%20maps%20do%20not%20maintain,to%20implement%20this%20behavior%20yourself.][doesn't]] guarantee the insertion order of maps, even static ones.
var fileIconTypes = map[string](func(f *file) bool){
	"tw": func(f *file) bool { return f.IsDir() && f.Mode()&os.ModeSticky != 0 && f.Mode()&0002 != 0 },
	"st": func(f *file) bool { return f.IsDir() && f.Mode()&os.ModeSticky != 0 },
	"ow": func(f *file) bool { return f.IsDir() && f.Mode()&0002 != 0 },
	"di": func(f *file) bool { return f.IsDir() },
	"ln": func(f *file) bool { return f.linkState == working },
	"or": func(f *file) bool { return f.linkState == broken },
	"pi": func(f *file) bool { return f.Mode()&os.ModeNamedPipe != 0 },
	"so": func(f *file) bool { return f.Mode()&os.ModeSocket != 0 },
	"cd": func(f *file) bool { return f.Mode()&os.ModeCharDevice != 0 },
	"bd": func(f *file) bool { return f.Mode()&os.ModeDevice != 0 },
	"su": func(f *file) bool { return f.Mode()&os.ModeSetuid != 0 },
	"sg": func(f *file) bool { return f.Mode()&os.ModeSetgid != 0 },
	"ex": func(f *file) bool { return f.Mode().IsRegular() && f.Mode()&0111 != 0 },
	"fi": func(f *file) bool { return true },
}

// The order in which fileIconTypes predicates should be checked.
var fileIconTypesOrder = []string{
	"tw", "st", "ow", "di", "ln", "or", "pi",
	"so", "cd", "bd", "su", "sg", "ex", "fi",
}

// Get icon through basic file type classification with fileIconTypes.
func (im iconMap) getFromFile(f *file) (string, bool) {
	for _, key := range fileIconTypesOrder {
		pred := fileIconTypes[key]
		if pred(f) {
			if basicIcon, ok := im.basicIcons[key]; ok {
				return basicIcon.icon, true
			}
			goto finish
		}
	}
finish:
	return "", false
}
