package agent

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

// skillsFS holds 37Studio's own craft skills (3D scenes, motion explainers), learned from reels that did well.
//
//go:embed skills
var skillsFS embed.FS

// installSkills writes them into the project's .claude/skills before each run: Claude Code (and OpenCode) load a
// project's skills from there, so every copy of the app gets them without touching the person's own ~/.claude.
// Rewritten each run, so an app update brings the new version.
func installSkills(dir string) {
	fs.WalkDir(skillsFS, "skills", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := skillsFS.ReadFile(p)
		if err != nil {
			return err
		}
		dst := filepath.Join(dir, ".claude", filepath.FromSlash(p))
		if os.MkdirAll(filepath.Dir(dst), 0o755) == nil {
			os.WriteFile(dst, b, 0o644)
		}
		return nil
	})
}
