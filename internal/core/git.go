package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type SyncParams struct {
	Direction string // "WorkingToShadow" or "ShadowToWorking"
	WorkDir   string
	ShadowDir string
	Anchor    string
	Phase     string // "start", "before", "after", "init"
	Message   string
}

func Gitsync(p SyncParams) error {
	os.MkdirAll(p.ShadowDir, 0755)
	shadowGitDir := filepath.Join(p.ShadowDir, ".git")

	if _, err := os.Stat(shadowGitDir); os.IsNotExist(err) {
		exec.Command("git", "-C", p.ShadowDir, "init").Run()
	}

	tagName := fmt.Sprintf("%s-%s", p.Anchor, p.Phase)
	refPath := "refs/ggo/txn/" + tagName
	absWork, _ := filepath.Abs(p.WorkDir)

	if p.Direction == "WorkingToShadow" {
		exec.Command("git", "-C", p.ShadowDir, "fetch", absWork, "+refs/heads/*:refs/heads/*", "+refs/tags/*:refs/tags/*").Run()
		exec.Command("git", "-C", p.ShadowDir, "fetch", absWork, "+refs/stash:refs/stash").Run()

		hashOut, err := exec.Command("git", "-C", absWork, "rev-parse", "HEAD").Output()
		if err != nil {
			return fmt.Errorf("working directory %s must be a git repository with at least one commit", absWork)
		}
		hash := strings.TrimSpace(string(hashOut))

		branchOut, err := exec.Command("git", "-C", absWork, "symbolic-ref", "--short", "HEAD").Output()
		branch := strings.TrimSpace(string(branchOut))
		if err != nil || branch == "" {
			branch = "detached"
		}

		exec.Command("git", "-C", p.ShadowDir, "checkout", "-f", hash).Run()

		patchPath := filepath.Join(absWork, ".ggo_staged.patch")
		patchCmd := exec.Command("git", "-C", absWork, "diff", "--cached", "--binary")
		patchOut, _ := patchCmd.Output()
		if len(patchOut) > 0 {
			os.WriteFile(patchPath, patchOut, 0644)
		} else {
			os.Remove(patchPath)
		}

		SyncDirectory(absWork, p.ShadowDir)
		os.Remove(patchPath)

		exec.Command("git", "-C", p.ShadowDir, "add", "-A").Run()
		commitMsg := fmt.Sprintf("[branch: %s] [base: %s] %s", branch, hash, p.Message)
		exec.Command("git", "-C", p.ShadowDir, "commit", "--allow-empty", "-m", commitMsg).Run()

		overlayHashOut, _ := exec.Command("git", "-C", p.ShadowDir, "rev-parse", "HEAD").Output()
		overlayHash := strings.TrimSpace(string(overlayHashOut))

		exec.Command("git", "-C", p.ShadowDir, "update-ref", refPath, overlayHash).Run()

	} else if p.Direction == "ShadowToWorking" {
		overlayHashOut, err := exec.Command("git", "-C", p.ShadowDir, "rev-parse", refPath).Output()
		overlayHash := strings.TrimSpace(string(overlayHashOut))
		if err != nil || overlayHash == "" {
			return fmt.Errorf("transaction tag %s not found in vault", tagName)
		}

		msgOut, _ := exec.Command("git", "-C", p.ShadowDir, "log", "-1", "--format=%B", overlayHash).Output()

		re := regexp.MustCompile(`\[base:\s*([a-f0-9]+)\]`)
		matches := re.FindStringSubmatch(string(msgOut))
		if len(matches) < 2 {
			return fmt.Errorf("could not parse foundation hash from vaulted commit")
		}
		baseHash := matches[1]

		exec.Command("git", "-C", absWork, "checkout", "-f", baseHash).Run()
		exec.Command("git", "-C", p.ShadowDir, "checkout", "-f", overlayHash).Run()

		err = SyncDirectory(p.ShadowDir, absWork)
		if err != nil {
			return fmt.Errorf("failed to sync shadow to working tree: %v", err)
		}

		patchPath := filepath.Join(absWork, ".ggo_staged.patch")
		if stat, err := os.Stat(patchPath); err == nil && stat.Size() > 0 {
			applyCmd := exec.Command("git", "-C", absWork, "apply", "--cached", patchPath)
			if err := applyCmd.Run(); err != nil {
				fmt.Printf("[Warning] Failed to cleanly reconstruct staging area: %v\n", err)
			}
			os.Remove(patchPath)
		}
	}
	return nil
}

func ExecuteTransaction(slug string, action func(anchor string, now time.Time) error) error {
	now := time.Now().UTC()
	anchor := now.Format("20060102T150405Z")

	centralRepoDir := GetProjectVaultDir(ProjectCfg.ProjectName)
	centralGitDir := filepath.Join(centralRepoDir, ".git")

	isInit := false
	if _, err := os.Stat(centralGitDir); os.IsNotExist(err) {
		isInit = true
		os.MkdirAll(centralRepoDir, 0755)
		exec.Command("git", "-C", centralRepoDir, "init").Run()
	}

	if isInit {
		SnapshotCentral("system: project initialized", anchor+"-init")
		SyncAllShadows(anchor, "init", "system: project initialized")
	}

	fmt.Printf("\033[90m[ggo] Locking transaction %s...\033[0m\n", anchor)
	SyncAllShadows(anchor, "start", "transaction started: "+slug)
	SnapshotCentral("start: "+slug, anchor+"-start")

	fmt.Printf("\033[90m[ggo] Securing 'before' state...\033[0m\n")
	SyncAllShadows(anchor, "before", "before: "+slug)
	SnapshotCentral("before: "+slug, anchor+"-before")

	err := action(anchor, now)

	fmt.Printf("\033[90m[ggo] Securing 'after' state...\033[0m\n")
	SyncAllShadows(anchor, "after", "after: "+slug)
	SnapshotCentral("after: "+slug, anchor+"-after")

	return err
}

func SyncAllShadows(anchor, phase, msg string) {

	for _, dir := range ProjectCfg.CodeDirs {
		absDir, _ := filepath.Abs(dir)
		repoID := GetRepoID(absDir)

		shadowDir := filepath.Join(GetConfigDir(), "shadow-repos", repoID)

		err := Gitsync(SyncParams{
			Direction: "WorkingToShadow",
			WorkDir:   absDir,
			ShadowDir: shadowDir,
			Anchor:    anchor,
			Phase:     phase,
			Message:   msg,
		})
		if err != nil {
			fmt.Printf("[Warning] Failed to sync shadow for %s: %v\n", dir, err)
		}
	}
}

func SnapshotCentral(msg, tag string) {
	centralRepoDir := GetProjectVaultDir(ProjectCfg.ProjectName)
	gitDir := filepath.Join(centralRepoDir, ".git")

	exec.Command("git", "--git-dir="+gitDir, "--work-tree="+centralRepoDir, "add", "-A").Run()
	exec.Command("git", "--git-dir="+gitDir, "--work-tree="+centralRepoDir, "commit", "--allow-empty", "-m", msg).Run()
	exec.Command("git", "--git-dir="+gitDir, "--work-tree="+centralRepoDir, "tag", tag).Run()
}

func SyncDirectory(src, dst string) error {
	os.MkdirAll(dst, 0755)
	src = strings.TrimSuffix(src, "/") + "/"
	dst = strings.TrimSuffix(dst, "/") + "/"
	cmd := exec.Command("rsync", "-a", "--delete", "--exclude=.git", "--exclude=.ggo/", src, dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("rsync failed: %s", string(out))
	}
	return nil
}

// GetRepoID generates a universally portable canonical name for the repository.
func GetRepoID(dir string) string {
	absDir, _ := filepath.Abs(dir)
	base := filepath.Base(absDir)

	// Try to get the root commit hash (the mathematical genesis of the repo)
	cmd := exec.Command("git", "-C", absDir, "rev-list", "--max-parents=0", "HEAD")
	out, err := cmd.Output()
	if err == nil {
		hashes := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(hashes) > 0 && len(hashes[0]) >= 8 {
			return fmt.Sprintf("%s-%s", base, hashes[0][:8])
		}
	}

	// Fallback for non-git dirs or completely empty repositories with no commits yet
	safe := strings.ReplaceAll(strings.TrimPrefix(absDir, "/"), "/", "_")
	safe = strings.ReplaceAll(safe, ":", "_") // Windows compat
	return safe
}

// CleanupTimeMachine acts as a teardown hook for the REPL shutdown
func CleanupTimeMachine() {
	// Active daemon pipes are gone, but this hook is reserved for clearing stale UI lockfiles.
}
