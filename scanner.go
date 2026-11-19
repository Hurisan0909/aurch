package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Finding は検知された1行を表す。
type Finding struct {
	File    string // 作業ディレクトリからの相対パス
	LineNo  int    // 行番号（1始まり）
	Line    string // マッチした行の内容（そのまま）
	Category string // 分類名（一律表示に使うだけ。色分けはしない）
}

// rule はカテゴリと正規パターンの組。
type rule struct {
	name    string
	pattern *regexp.Regexp
}

// 検知パターン定義。
// - URL: http://, https://, git://, ftp://
// - 禁止コマンド: rm -rf /, mkfs, dd if=, chmod 777, curl|sh, wget|bash など
// - パッケージマネージャ: go get, pip install, npm install, cargo build, yay -S, paru -S など
var rules = []rule{
	{"URL", regexp.MustCompile(`(?i)\b(?:https?|git|ftp)://`)},
	{"ForbiddenCmd", regexp.MustCompile(`(?i)` + strings.Join([]string{
		`\brm\s+(-[a-zA-Z]+\s+)*-?[rRf]{1,2}[a-zA-Z]*\s+/`, // rm -rf / など
		`\bmkfs(\.|\s)`,
		`\bdd\s+if=`,
		`\bchmod\s+(-[a-zA-Z]+\s+)*777\b`,
		`\b(?:curl|wget)\b[^|]*\|\s*(?:sudo\s+)?(?:ba)?sh\b`, // curl | sh, wget | bash など
		`>(/dev/sd|/etc/passwd|/etc/shadow)\b`,     // 追加: 直接書き込み
	}, "|"))},
	{"PkgManager", regexp.MustCompile(`(?i)` + strings.Join([]string{
		`\bgo\s+get\b`,
		`\bpip3?\s+install\b`,
		`\bnpm\s+(install|i)\b`,
		`\bcargo\s+build\b`,
		`\b(yay|paru)\s+(-{1,2}S|--sync)\b`,
		`\bpacman\s+(-{1,2}S|--sync)\b`,
		`\bapt(-get)?\s+install\b`,
		`\bdnf\s+install\b`,
	}, "|"))},
}

// isTargetFile は解析対象ファイル（PKGBUILD / *.install / *.install.txt 等）を判定する。
func isTargetFile(name string) bool {
	base := strings.ToLower(filepath.Base(name))
	if base == "pkgbuild" {
		return true
	}
	if strings.HasSuffix(base, ".install") {
		return true
	}
	// .SRCINFO もURL情報を含むため対象に含める
	if base == ".srcinfo" {
		return true
	}
	return false
}

// scanDir は dir 以下の PKGBUILD / .install ファイルを走査し、
// コメント行（# で始まる行）を除外して危険パターンにマッチした行を抽出する。
func scanDir(dir string) ([]Finding, error) {
	var findings []Finding

	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // 読めないディレクトリはスキップ
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if isTargetFile(path) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)

	for _, f := range files {
		rel, rerr := filepath.Rel(dir, f)
		if rerr != nil {
			rel = f
		}
		fl, err := scanFile(f, rel)
		if err != nil {
			return findings, err
		}
		findings = append(findings, fl...)
	}
	return findings, nil
}

func scanFile(path, rel string) ([]Finding, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", path, err)
	}
	defer fh.Close()

	var out []Finding
	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		// コメント行を除外
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		for _, r := range rules {
			if r.pattern.MatchString(line) {
				out = append(out, Finding{
					File:     rel,
					LineNo:   lineNo,
					Line:     line,
					Category: r.name,
				})
			}
		}
	}
	if err := sc.Err(); err != nil {
		return out, fmt.Errorf("failed to read %s: %w", path, err)
	}
	return out, nil
}

// treeLines は dir のディレクトリツリーを文字列スライスで返す。
func treeLines(dir string) []string {
	var lines []string
	lines = append(lines, filepath.Base(dir)+"/")

	entries, err := os.ReadDir(dir)
	if err != nil {
		return append(lines, "(unable to read directory)")
	}
	var names []string
	for _, e := range entries {
		if e.Name() == ".git" {
			names = append(names, ".git/")
			continue
		}
		if e.IsDir() {
			names = append(names, e.Name()+"/")
		} else {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for i, n := range names {
		branch := "├── "
		if i == len(names)-1 {
			branch = "└── "
		}
		lines = append(lines, branch+n)
	}
	return lines
}
