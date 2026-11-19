package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func usage() {
	fmt.Fprintln(os.Stderr, "usage: aurch <package-name>")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "aurch is a simple security checker for AUR packages.")
	fmt.Fprintln(os.Stderr, "It fetches the PKGBUILD via paru/yay, scans it for suspicious")
	fmt.Fprintln(os.Stderr, "patterns, and shows the results in a TUI.")
	fmt.Fprintln(os.Stderr, "Press 'i' in the TUI to install (delegated to paru/yay --review).")
}

func main() {
	if len(os.Args) != 2 || os.Args[1] == "-h" || os.Args[1] == "--help" {
		usage()
		os.Exit(2)
	}
	pkg := os.Args[1]

	// 1. /tmp に mktemp -d で作業ディレクトリを作成
	workDir, err := makeWorkDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	// 7. プログラム終了時に作業ディレクトリを削除（deferで確実に）
	defer func() {
		if cerr := cleanupDir(workDir); cerr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to clean up %s: %v\n", workDir, cerr)
		}
	}()

	// 2. git clone でAURパッケージを取得（paru --getpkgbuild / yay -G）
	//    3. 全ファイルを読み取り専用化（chmod -R a-w）
	//    4. PKGBUILD/.install を解析
	m := runPipeline(pkg, workDir)

	// 5. TUI表示
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error running TUI:", err)
		os.Exit(1)
	}

	fm, ok := final.(model)
	if !ok {
		os.Exit(0)
	}

	// 6. 'i' によるインストールの結果に応じて終了コードを返す
	if fm.installErr != nil {
		fmt.Fprintln(os.Stderr, "install error:", fm.installErr)
		os.Exit(1)
	}
}

// runPipeline はフェッチ→読み取り専用化→スキャンを行いTUIモデルを返す。
func runPipeline(pkg, workDir string) model {
	if err := fetchPackage(pkg, workDir); err != nil {
		return newModel(pkg, workDir, nil, nil, err)
	}
	if err := makeReadOnly(workDir); err != nil {
		return newModel(pkg, workDir, treeLines(workDir), nil,
			fmt.Errorf("failed to set read-only: %w", err))
	}
	findings, err := scanDir(workDir)
	if err != nil {
		return newModel(pkg, workDir, treeLines(workDir), findings,
			fmt.Errorf("scan error: %w", err))
	}
	return newModel(pkg, workDir, treeLines(workDir), findings, nil)
}
