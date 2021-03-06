package codeowners

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/afero"
)

// Codeowners - patterns/owners mappings for the given repo
type Codeowners struct {
	repoRoot string
	Patterns []Codeowner
}

// Codeowner - owners for a given pattern
type Codeowner struct {
	Pattern string
	re      *regexp.Regexp
	Owners  []string
}

// IsGlobal - tell whenever the owner is global
func (c Codeowner) IsGlobal() bool {
	return c.Pattern == "*"
}

// AddPattern - add a new pattern to the codeowners file
func (c *Codeowners) AddPattern(pattern string, owners []string) (err error) {
	p, err := NewCodeowner(pattern, owners)
	if err != nil {
		return
	}
	c.Patterns = append(c.Patterns, p)
	return
}

func (c Codeowner) String() string {
	return fmt.Sprintf("%s\t%v", c.Pattern, strings.Join(c.Owners, ", "))
}

var fs = afero.NewOsFs()

// findCodeownersFile - find a CODEOWNERS file somewhere within or below
// the working directory (wd), and open it.
func findCodeownersFile(wd string) (io.Reader, string, error) {
	dir := wd
	for {
		for _, p := range []string{".", "docs", ".github", ".gitlab"} {
			pth := path.Join(dir, p)
			exists, err := afero.DirExists(fs, pth)
			if err != nil {
				return nil, "", err
			}
			if exists {
				f := path.Join(pth, "CODEOWNERS")
				_, err := fs.Stat(f)
				if err != nil {
					if os.IsNotExist(err) {
						continue
					}
					return nil, "", err
				}
				r, err := fs.Open(f)
				return r, dir, err
			}
		}
		odir := dir
		dir = path.Dir(odir)
		// if we can't go up any further...
		if odir == dir {
			break
		}
		// if we're heading above the volume name (relevant on Windows)...
		if len(dir) < len(filepath.VolumeName(odir)) {
			break
		}
	}
	return nil, "", nil
}

// Deprecated: Use FromFile(path) instead.
func NewCodeowners(path string) (*Codeowners, error) {
	return FromFile(path)
}

// EmptyCodeowners - create an empty codeowners file
func EmptyCodeowners() *Codeowners {
	return &Codeowners{
		repoRoot: ".",
		Patterns: []Codeowner{},
	}
}

// FromFile creates a Codeowners from the path to a local file.
func FromFile(path string) (*Codeowners, error) {
	r, root, err := findCodeownersFile(path)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, fmt.Errorf("No CODEOWNERS found in %s", path)
	}
	return FromReader(r, root)
}

// FromReader creates a Codeowners from a given Reader instance and root path.
func FromReader(r io.Reader, repoRoot string) (*Codeowners, error) {
	co := &Codeowners{
		repoRoot: repoRoot,
	}
	co.Patterns = parseCodeowners(r)
	return co, nil
}

// ToFile - serialize the Codeowners to file
func (c *Codeowners) ToFile(path string) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close() //TODO not checking if this fails
	escape := func(pattern string) string {
		if strings.HasPrefix(pattern, "#") {
			pattern = fmt.Sprint("\\", pattern)
		}
		return strings.ReplaceAll(pattern, " ", "\\ ")
	}
	w := bufio.NewWriter(f)
	for _, c := range c.Patterns {
		w.WriteString(fmt.Sprintf("%-25s %s\n", escape(c.Pattern), strings.Join(c.Owners, " ")))
	}
	err = w.Flush()
	return
}

// parseCodeowners parses a list of Codeowners from a Reader
func parseCodeowners(r io.Reader) []Codeowner {
	co := []Codeowner{}
	s := bufio.NewScanner(r)
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) > 0 && strings.HasPrefix(fields[0], "#") {
			continue
		}
		if len(fields) > 1 {
			fields = combineEscapedSpaces(fields)
			c, _ := NewCodeowner(fields[0], fields[1:])
			co = append(co, c)
		}
	}
	return co
}

// if any of the elements ends with a \, it was an escaped space
// put it back together properly so it's not treated as separate fields
func combineEscapedSpaces(fields []string) []string {
	outFields := make([]string, 0)
	escape := `\`
	for i := 0; i < len(fields); i++ {
		outField := fields[i]
		for strings.HasSuffix(fields[i], escape) && i+1 < len(fields) {
			outField = strings.Join([]string{strings.TrimRight(outField, escape), fields[i+1]}, " ")
			i++
		}
		outFields = append(outFields, outField)
	}

	return outFields
}

// NewCodeowner -
func NewCodeowner(pattern string, owners []string) (Codeowner, error) {
	re := getPattern(pattern)
	c := Codeowner{
		Pattern: pattern,
		re:      re,
		Owners:  owners,
	}
	return c, nil
}

// Owners - return the list of code owners for the given path
// (within the repo root)
func (c *Codeowners) Owners(path string) []string {
	if strings.HasPrefix(path, c.repoRoot) {
		path = strings.Replace(path, c.repoRoot, "", 1)
	}

	// Order is important; the last matching pattern takes the most precedence.
	for i := len(c.Patterns) - 1; i >= 0; i-- {
		p := c.Patterns[i]

		if p.re.MatchString(path) {
			return p.Owners
		}
	}

	return nil
}

// LocalOwners - return the list of code owners for the given path
// excluding the global owners
// (within the repo root)
func (c *Codeowners) LocalOwners(path string) []string {
	if strings.HasPrefix(path, c.repoRoot) {
		path = strings.Replace(path, c.repoRoot, "", 1)
	}

	// Order is important; the last matching pattern takes the most precedence.
	for i := len(c.Patterns) - 1; i >= 0; i-- {
		p := c.Patterns[i]

		if !p.IsGlobal() && p.re.MatchString(path) {
			return p.Owners
		}
	}

	return nil
}

// based on github.com/sabhiram/go-gitignore
// but modified so that 'dir/*' only matches files in 'dir/'
func getPattern(line string) *regexp.Regexp {
	// when # or ! is escaped with a \
	if regexp.MustCompile(`^(\\#|\\!)`).MatchString(line) {
		line = line[1:]
	}

	// If we encounter a foo/*.blah in a folder, prepend the / char
	if regexp.MustCompile(`([^\/+])/.*\*\.`).MatchString(line) && line[0] != '/' {
		line = "/" + line
	}

	// Handle escaping the "." char
	line = regexp.MustCompile(`\.`).ReplaceAllString(line, `\.`)

	magicStar := "#$~"

	// Handle "/**/" usage
	if strings.HasPrefix(line, "/**/") {
		line = line[1:]
	}
	line = regexp.MustCompile(`/\*\*/`).ReplaceAllString(line, `(/|/.+/)`)
	line = regexp.MustCompile(`\*\*/`).ReplaceAllString(line, `(|.`+magicStar+`/)`)
	line = regexp.MustCompile(`/\*\*`).ReplaceAllString(line, `(|/.`+magicStar+`)`)

	// Handle escaping the "*" char
	line = regexp.MustCompile(`\\\*`).ReplaceAllString(line, `\`+magicStar)
	line = regexp.MustCompile(`\*`).ReplaceAllString(line, `([^/]*)`)

	// Handle escaping the "?" char
	line = strings.Replace(line, "?", `\?`, -1)

	line = strings.Replace(line, magicStar, "*", -1)

	// Temporary regex
	var expr = ""
	if strings.HasSuffix(line, "/") {
		expr = line + "(|.*)$"
	} else {
		expr = line + "$"
	}
	if strings.HasPrefix(expr, "/") {
		expr = "^(|/)" + expr[1:]
	} else {
		expr = "^(|.*/)" + expr
	}
	pattern, _ := regexp.Compile(expr)

	return pattern
}
