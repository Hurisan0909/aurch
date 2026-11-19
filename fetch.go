package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// makeWorkDir は mktemp -d 相当の作業ディレクトリを /tmp に作成する。
func makeWorkDir() (string, error) {
	dir, err := os.MkdirTemp("/tmp", "aurch-")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	return dir, nil
}

// cleanupDir は作業ディレクトリを削除する。
// chmod -R a-w 済みでも削除できるよう、先に書き込み権限を戻す。
func cleanupDir(dir string) error {
	if dir == "" {
		return nil
	}
	// 読み取り専用化されているため、削除前に権限を戻す
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		_ = os.Chmod(path, 0755)
		return nil
	})
	return os.RemoveAll(dir)
}

// fetchPackage は yay -G もしくは paru --getpkgbuild でAURパッケージを取得する。
// git clone を使うので、取得後に git:// URL 等もそのまま残る（解析対象）。
func fetchPackage(pkg, dir string) error {
	// yay と paru を順に探す（paru優先：インストール時と同じ順序）
	for _, helper := range []string{"paru", "yay"} {
		path, err := exec.LookPath(helper)
		if err != nil {
			continue
		}
		var args []string
		switch helper {
		case "paru":
			args = []string{"--getpkgbuild", pkg}
		case "yay":
			args = []string{"-G", pkg}
		}
		cmd := exec.Command(path, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s failed to fetch %q: %w\n%s",
				helper, pkg, err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	return fmt.Errorf("no AUR helper found: install 'paru' or 'yay' first")
}

// makeReadOnly は chmod -R a-w 相当のことを行う（全ファイル・ディレクトリを書き込み不可に）。
func makeReadOnly(dir string) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return nil
		}
		mode := info.Mode().Perm()
		// a-w: 全員からの書き込み権限を除去
		newMode := mode &^ (0222)
		return os.Chmod(path, newMode)
	})
}

// detectHelper は paru優先でAURヘルパーを検出する（exec.LookPath使用）。
func detectHelper() (string, error) {
	for _, h := range []string{"paru", "yay"} {
		if p, err := exec.LookPath(h); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("AUR helper (paru or yay) not found in PATH")
}

// installPackage は検出されたヘルパーでレビュー付きインストールを実行する。
//   paru --review --needed --cleanafter --removemake <pkg>
//   yay  --review --needed --cleanafter --removemake <pkg>
func installPackage(pkg string) error {
	helper, err := detectHelper()
	if err != nil {
		return err
	}
	cmd := exec.Command(helper, "--review", "--needed", "--cleanafter", "--removemake", pkg)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code := ee.ExitCode()
			return fmt.Errorf("%s exited with error code %d", filepath.Base(helper), code)
		}
		return fmt.Errorf("failed to run %s: %w", helper, err)
	}
	return nil
}
