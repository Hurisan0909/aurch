package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// manyHitsThreshold はこの件数を超えると警告表示する。
const manyHitsThreshold = 10

type installResultMsg struct {
	err error
}

// TUIモデル。検知行は一律表示（色分けなし）。
type model struct {
	pkg      string
	workDir  string
	tree     []string
	findings []Finding
	err      error // フェッチ/解析時のエラー

	width, height int
	cursor        int // 検知行リストの選択位置
	scrolled      int // スクロール位置

	installing bool
	installErr error
	done       bool
}

func newModel(pkg, workDir string, tree []string, findings []Finding, err error) model {
	return model{
		pkg:      pkg,
		workDir:  workDir,
		tree:     tree,
		findings: findings,
		err:      err,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case installResultMsg:
		m.installing = false
		m.installErr = msg.err
		m.done = true
		return m, tea.Quit

	case tea.KeyMsg:
		if m.installing {
			return m, nil
		}
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.done = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.findings)-1 {
				m.cursor++
			}
		case "i":
			// 'i' キー：作業ディレクトリを削除し、paru/yay を自動検出してインストール実行
			if err := cleanupDir(m.workDir); err != nil {
				m.installErr = fmt.Errorf("failed to remove work dir: %w", err)
				m.done = true
				return m, tea.Quit
			}
			m.installing = true
			pkg := m.pkg
			return m, func() tea.Msg {
				return installResultMsg{err: installPackage(pkg)}
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("aurch - AUR package security checker: %s\n", m.pkg))
	b.WriteString(fmt.Sprintf("workdir: %s\n\n", m.workDir))

	if m.err != nil {
		b.WriteString("Error:\n")
		b.WriteString(indent(m.err.Error(), "  "))
		b.WriteString("\n\nPress 'q' to quit.\n")
		return b.String()
	}

	// ディレクトリツリー
	b.WriteString("Directory tree:\n")
	for _, l := range m.tree {
		b.WriteString("  " + l + "\n")
	}
	b.WriteString("\n")

	// 大量ヒット時の警告
	n := len(m.findings)
	if n >= manyHitsThreshold {
		b.WriteString(fmt.Sprintf("⚠️ %d件検出\n", n))
	} else {
		b.WriteString(fmt.Sprintf("%d hit(s) detected\n", n))
	}
	b.WriteString("\n")

	// 検知した行の一覧（一律表示・色分けなし）
	if n == 0 {
		b.WriteString("  No suspicious patterns found in PKGBUILD/.install files.\n")
		b.WriteString("  Note: aurch is a simple checker. Always review the PKGBUILD\n")
		b.WriteString("  yourself with yay/paru --review before installing.\n")
	} else {
		listHeight := m.height - m.countHeaderLines() - footerLines
		if listHeight < 3 {
			listHeight = 3
		}
		start := 0
		if m.cursor >= listHeight {
			start = m.cursor - listHeight + 1
		}
		end := start + listHeight
		if end > n {
			end = n
		}
		for i := start; i < end; i++ {
			f := m.findings[i]
			marker := "  "
			if i == m.cursor {
				marker = "> "
			}
			line := f.Line
			maxw := m.width - len(f.File) - len(f.Category) - 20
			if maxw > 8 && len(line) > maxw {
				line = line[:maxw] + "…"
			}
			b.WriteString(fmt.Sprintf("%s[%s] %s:%d: %s\n", marker, f.Category, f.File, f.LineNo, line))
		}
		if end < n {
			b.WriteString(fmt.Sprintf("  ... (%d more)\n", n-end))
		}
	}

	b.WriteString("\nKeys: j/k or ↑/↓ scroll | i: delete workdir & install (--review) | q: quit\n")
	b.WriteString("NOTE: aurch is a simple checker. Final PKGBUILD review is delegated to\n")
	b.WriteString("      yay/paru's --review feature at install time.\n")

	return b.String()
}

func (m model) countHeaderLines() int {
	// ヘッダー(2) + 空行 + ツリー + 空行 + サマリ + 空行
	return 2 + 1 + len(m.tree) + 1 + 1 + 1
}

const footerLines = 4

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}
