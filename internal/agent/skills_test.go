package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every shipped skill lands where Claude Code looks for a project's skills, with its name in the frontmatter.
func TestInstallSkills(t *testing.T) {
	dir := t.TempDir()
	installSkills(dir)
	for _, name := range []string{"cinema-3d", "motion-explainer"} {
		b, err := os.ReadFile(filepath.Join(dir, ".claude", "skills", name, "SKILL.md"))
		if err != nil || !strings.Contains(string(b), "name: "+name) {
			t.Errorf("%s: %v", name, err)
		}
	}
}
